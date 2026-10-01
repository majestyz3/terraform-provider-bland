package agentexperiment

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"github.com/hashicorp/terraform-plugin-framework/path"
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
	ConfigJSON   types.String `tfsdk:"config_json"`
	ResponseJSON types.String `tfsdk:"response_json"`
}

type Resource struct{ client *client.Client }

func New() resource.Resource { return &Resource{} }

func (r *Resource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_agent_experiment"
}

func (r *Resource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a Bland V2 Agent A/B experiment. Destroying the resource stops an active experiment; completed experiment history remains in Bland.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{Computed: true},
			"agent_id": schema.StringAttribute{
				Required:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"config_json": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Experiment configuration JSON. Create supports variant_version_id, traffic_percentage, baseline_env, run_hours, starts_at, ends_at, and variant_call_quota. Updates send only mutable fields.",
			},
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

func decode(s string) (map[string]any, error) {
	var m map[string]any
	if err := json.Unmarshal([]byte(s), &m); err != nil {
		return nil, fmt.Errorf("config_json must be valid JSON: %w", err)
	}
	return m, nil
}

func encode(v any) string {
	b, _ := json.MarshalIndent(v, "", "  ")
	return string(b)
}

func experimentID(out map[string]any) string {
	if d, ok := out["data"].(map[string]any); ok {
		if exp, ok := d["experiment"].(map[string]any); ok {
			id, _ := exp["id"].(string)
			return id
		}
		id, _ := d["id"].(string)
		return id
	}
	return ""
}

func mutableConfig(in map[string]any) map[string]any {
	out := map[string]any{}
	for _, key := range []string{"traffic_percentage", "run_hours", "starts_at", "ends_at", "variant_call_quota"} {
		if value, ok := in[key]; ok {
			out[key] = value
		}
	}
	return out
}

func (r *Resource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var m Model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	body, err := decode(m.ConfigJSON.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid experiment configuration", err.Error())
		return
	}
	out, _, err := r.client.Do(ctx, http.MethodPost, "/v2/agents/"+url.PathEscape(m.AgentID.ValueString())+"/experiments", body)
	if err != nil {
		resp.Diagnostics.AddError("Create Agent Experiment failed", err.Error())
		return
	}
	id := experimentID(out)
	if id == "" {
		resp.Diagnostics.AddError("Missing Experiment ID", encode(out))
		return
	}
	m.ID = types.StringValue(id)
	m.ResponseJSON = types.StringValue(encode(out))
	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
}

func (r *Resource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var m Model
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	p := "/v2/agents/" + url.PathEscape(m.AgentID.ValueString()) + "/experiments/" + url.PathEscape(m.ID.ValueString())
	out, status, err := r.client.Do(ctx, http.MethodGet, p, nil)
	if status == http.StatusNotFound {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Read Agent Experiment failed", err.Error())
		return
	}
	m.ResponseJSON = types.StringValue(encode(out))
	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
}

func (r *Resource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var planned Model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &planned)...)
	var prior Model
	resp.Diagnostics.Append(req.State.Get(ctx, &prior)...)
	if resp.Diagnostics.HasError() {
		return
	}
	planned.ID = prior.ID
	body, err := decode(planned.ConfigJSON.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid experiment configuration", err.Error())
		return
	}
	p := "/v2/agents/" + url.PathEscape(planned.AgentID.ValueString()) + "/experiments/" + url.PathEscape(planned.ID.ValueString())
	out, _, err := r.client.Do(ctx, http.MethodPatch, p, mutableConfig(body))
	if err != nil {
		resp.Diagnostics.AddError("Update Agent Experiment failed", err.Error())
		return
	}
	planned.ResponseJSON = types.StringValue(encode(out))
	resp.Diagnostics.Append(resp.State.Set(ctx, &planned)...)
}

func (r *Resource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var m Model
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	p := "/v2/agents/" + url.PathEscape(m.AgentID.ValueString()) + "/experiments/" + url.PathEscape(m.ID.ValueString()) + "/stop"
	_, status, err := r.client.Do(ctx, http.MethodPost, p, nil)
	if err != nil && status != http.StatusNotFound && status != http.StatusConflict {
		resp.Diagnostics.AddError("Stop Agent Experiment failed", err.Error())
	}
}

func (r *Resource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	agentID, experimentID, ok := splitImportID(req.ID)
	if !ok {
		resp.Diagnostics.AddError("Invalid import ID", "Use agent_id/experiment_id.")
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("agent_id"), agentID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), experimentID)...)
}

func splitImportID(id string) (string, string, bool) {
	for i := 0; i < len(id); i++ {
		if id[i] == '/' && i > 0 && i < len(id)-1 {
			return id[:i], id[i+1:], true
		}
	}
	return "", "", false
}

var _ resource.Resource = (*Resource)(nil)
var _ resource.ResourceWithConfigure = (*Resource)(nil)
var _ resource.ResourceWithImportState = (*Resource)(nil)
