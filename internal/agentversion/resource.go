package agentversion

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/majestyz3/terraform-provider-bland/internal/client"
)

type Model struct {
	ID           types.String `tfsdk:"id"`
	AgentID      types.String `tfsdk:"agent_id"`
	Name         types.String `tfsdk:"name"`
	SnapshotJSON types.String `tfsdk:"snapshot_json"`
	ResponseJSON types.String `tfsdk:"response_json"`
}

type Resource struct{ client *client.Client }

func New() resource.Resource { return &Resource{} }

func (r *Resource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_agent_version"
}

func (r *Resource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	replace := []planmodifier.String{stringplanmodifier.RequiresReplace()}
	resp.Schema = schema.Schema{
		MarkdownDescription: "Creates an immutable Bland V2 Agent Version snapshot.",
		Attributes: map[string]schema.Attribute{
			"id":            schema.StringAttribute{Computed: true},
			"agent_id":      schema.StringAttribute{Required: true, PlanModifiers: replace},
			"name":          schema.StringAttribute{Required: true, PlanModifiers: replace},
			"snapshot_json": schema.StringAttribute{Required: true, PlanModifiers: replace},
			"response_json": schema.StringAttribute{Computed: true},
		},
	}
}

func (r *Resource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	c, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError("Unexpected provider data", fmt.Sprintf("got %T", req.ProviderData))
		return
	}
	r.client = c
}

func encode(v any) string {
	b, _ := json.MarshalIndent(v, "", "  ")
	return string(b)
}

func (r *Resource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var m Model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var snapshot map[string]any
	if err := json.Unmarshal([]byte(m.SnapshotJSON.ValueString()), &snapshot); err != nil {
		resp.Diagnostics.AddError("Invalid snapshot_json", err.Error())
		return
	}
	body := map[string]any{"name": m.Name.ValueString(), "snapshot": snapshot}
	out, _, err := r.client.Do(ctx, http.MethodPost, "/v2/agents/"+m.AgentID.ValueString()+"/versions", body)
	if err != nil {
		resp.Diagnostics.AddError("Create Agent Version failed", err.Error())
		return
	}
	id := ""
	if d, ok := out["data"].(map[string]any); ok {
		if v, ok := d["version"].(map[string]any); ok {
			id, _ = v["id"].(string)
		}
		if id == "" {
			id, _ = d["id"].(string)
		}
	}
	if id == "" {
		resp.Diagnostics.AddError("Missing Agent Version ID", encode(out))
		return
	}
	m.ID = types.StringValue(id)
	m.ResponseJSON = types.StringValue(encode(out))
	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
}

func versionsListPath(agentID string) string {
	return "/v2/agents/" + url.PathEscape(agentID) + "/versions?limit=200"
}

func applyListResult(m *Model, out map[string]any) bool {
	data, _ := out["data"].([]any)
	for _, raw := range data {
		entry, _ := raw.(map[string]any)
		if entry == nil {
			continue
		}
		if id, _ := entry["id"].(string); id == m.ID.ValueString() {
			m.ResponseJSON = types.StringValue(encode(entry))
			return true
		}
	}
	return false
}

func (r *Resource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var m Model
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	out, status, err := r.client.Do(ctx, http.MethodGet, versionsListPath(m.AgentID.ValueString()), nil)
	if status == http.StatusNotFound {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Read Agent Version failed", err.Error())
		return
	}
	if !applyListResult(&m, out) {
		resp.State.RemoveResource(ctx)
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
}

func (r *Resource) Update(context.Context, resource.UpdateRequest, *resource.UpdateResponse) {}

func (r *Resource) Delete(_ context.Context, _ resource.DeleteRequest, resp *resource.DeleteResponse) {
	resp.Diagnostics.AddWarning("Agent Version retained", "Bland Agent Versions are immutable and the public API exposes no delete endpoint; Terraform forgets the version while Bland retains audit history.")
}

var _ resource.Resource = (*Resource)(nil)
var _ resource.ResourceWithConfigure = (*Resource)(nil)
