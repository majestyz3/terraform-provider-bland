package agentinbound

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
	BindingJSON  types.String `tfsdk:"binding_json"`
	ResponseJSON types.String `tfsdk:"response_json"`
}

type Resource struct{ client *client.Client }

func New() resource.Resource { return &Resource{} }

func (r *Resource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_agent_inbound_binding"
}

func (r *Resource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages inbound phone-number attachment to a Bland V2 Agent. The JSON body is passed directly to Bland's attach/detach endpoints.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{Computed: true},
			"agent_id": schema.StringAttribute{
				Required:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"binding_json": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Bland inbound attach/detach request JSON. Prefer jsonencode({...}). The same payload is used to detach the managed numbers.",
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
		return nil, fmt.Errorf("binding_json must be valid JSON: %w", err)
	}
	if len(m) == 0 {
		return nil, fmt.Errorf("binding_json must not be empty")
	}
	return m, nil
}

func encode(v any) string {
	b, _ := json.MarshalIndent(v, "", "  ")
	return string(b)
}

func basePath(agentID string) string {
	return "/v2/agents/" + url.PathEscape(agentID) + "/inbound"
}

func (r *Resource) attach(ctx context.Context, agentID string, body map[string]any) (map[string]any, error) {
	out, _, err := r.client.Do(ctx, http.MethodPost, basePath(agentID)+"/attach", body)
	return out, err
}

func (r *Resource) detach(ctx context.Context, agentID string, body map[string]any) (int, error) {
	_, status, err := r.client.Do(ctx, http.MethodPost, basePath(agentID)+"/detach", body)
	return status, err
}

func (r *Resource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var m Model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	body, err := decode(m.BindingJSON.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid inbound binding", err.Error())
		return
	}
	out, err := r.attach(ctx, m.AgentID.ValueString(), body)
	if err != nil {
		resp.Diagnostics.AddError("Attach Agent Numbers failed", err.Error())
		return
	}
	m.ID = types.StringValue(m.AgentID.ValueString())
	m.ResponseJSON = types.StringValue(encode(out))
	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
}

func (r *Resource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var m Model
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	out, status, err := r.client.Do(ctx, http.MethodGet, basePath(m.AgentID.ValueString()), nil)
	if status == http.StatusNotFound {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Read Agent Number Binding failed", err.Error())
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

	oldBody, err := decode(prior.BindingJSON.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid prior inbound binding", err.Error())
		return
	}
	newBody, err := decode(planned.BindingJSON.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid inbound binding", err.Error())
		return
	}

	status, err := r.detach(ctx, planned.AgentID.ValueString(), oldBody)
	if err != nil && status != http.StatusNotFound {
		resp.Diagnostics.AddError("Detach Previous Agent Numbers failed", err.Error())
		return
	}

	out, err := r.attach(ctx, planned.AgentID.ValueString(), newBody)
	if err != nil {
		if _, rollbackErr := r.attach(ctx, planned.AgentID.ValueString(), oldBody); rollbackErr != nil {
			resp.Diagnostics.AddError("Attach Agent Numbers failed and rollback failed", err.Error()+"; rollback: "+rollbackErr.Error())
			return
		}
		resp.Diagnostics.AddError("Attach Agent Numbers failed", err.Error()+"; previous binding was restored")
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
	body, err := decode(m.BindingJSON.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid inbound binding", err.Error())
		return
	}
	status, err := r.detach(ctx, m.AgentID.ValueString(), body)
	if err != nil && status != http.StatusNotFound {
		resp.Diagnostics.AddError("Detach Agent Numbers failed", err.Error())
	}
}

func (r *Resource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	if req.ID == "" {
		resp.Diagnostics.AddError("Invalid import ID", "Use the Bland agent_id.")
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("agent_id"), req.ID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
	resp.Diagnostics.AddWarning("Binding payload required after import", "Set binding_json in configuration after import so Terraform knows which phone-number attachment payload it owns and can detach it safely.")
}

var _ resource.Resource = (*Resource)(nil)
var _ resource.ResourceWithConfigure = (*Resource)(nil)
var _ resource.ResourceWithImportState = (*Resource)(nil)
