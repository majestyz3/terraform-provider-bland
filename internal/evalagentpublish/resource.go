package evalagentpublish

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/mapplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/majestyz3/terraform-provider-bland/internal/client"
)

type Model struct {
	ID               types.String `tfsdk:"id"`
	EvalAgentID      types.String `tfsdk:"eval_agent_id"`
	Triggers         types.Map    `tfsdk:"triggers"`
	ActiveVersionID  types.String `tfsdk:"active_version_id"`
	CurrentVersionID types.String `tfsdk:"current_version_id"`
	ResponseJSON     types.String `tfsdk:"response_json"`
}

type Resource struct{ client *client.Client }

func New() resource.Resource { return &Resource{} }

func (r *Resource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_eval_agent_publish"
}

func (r *Resource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Publishes the current Bland Eval Agent draft as the active archived version. Publication is explicit and never happens implicitly from bland_eval_agent_version_config.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{Computed: true},
			"eval_agent_id": schema.StringAttribute{
				Required:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"triggers": schema.MapAttribute{
				Optional:    true,
				ElementType: types.StringType,
				PlanModifiers: []planmodifier.Map{
					mapplanmodifier.RequiresReplace(),
				},
				MarkdownDescription: "Optional arbitrary values used only to force a fresh publication when dependent configuration changes.",
			},
			"active_version_id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "ID of the published archived version.",
			},
			"current_version_id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "ID of the new editable draft created after publishing.",
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

func encode(v any) string {
	b, _ := json.MarshalIndent(v, "", "  ")
	return string(b)
}

func applyFields(m *Model, out map[string]any) {
	data, _ := out["data"].(map[string]any)
	if data == nil {
		return
	}
	if agent, _ := data["agent"].(map[string]any); agent != nil {
		data = agent
	}
	if v, _ := data["active_version_id"].(string); v != "" {
		m.ActiveVersionID = types.StringValue(v)
	}
	if v, _ := data["current_version_id"].(string); v != "" {
		m.CurrentVersionID = types.StringValue(v)
	}
}

func (r *Resource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var m Model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}

	path := "/v1/evals/agents/" + url.PathEscape(m.EvalAgentID.ValueString()) + "/publications"
	out, _, err := r.client.Do(ctx, http.MethodPost, path, nil)
	if err != nil {
		resp.Diagnostics.AddError("Publish Eval Agent failed", err.Error())
		return
	}
	applyFields(&m, out)
	if m.ActiveVersionID.IsNull() || m.ActiveVersionID.ValueString() == "" || m.CurrentVersionID.IsNull() || m.CurrentVersionID.ValueString() == "" {
		resp.Diagnostics.AddError("Missing Eval Agent publication version IDs", encode(out))
		return
	}
	m.ID = types.StringValue(m.EvalAgentID.ValueString() + "/" + m.ActiveVersionID.ValueString())
	m.ResponseJSON = types.StringValue(encode(out))
	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
}

func (r *Resource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var m Model
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	out, status, err := r.client.Do(ctx, http.MethodGet, "/v1/evals/agents/"+url.PathEscape(m.EvalAgentID.ValueString()), nil)
	if status == http.StatusNotFound {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Read Eval Agent publication state failed", err.Error())
		return
	}
	applyFields(&m, out)
	m.ResponseJSON = types.StringValue(encode(out))
	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
}

func (r *Resource) Update(context.Context, resource.UpdateRequest, *resource.UpdateResponse) {}

func (r *Resource) Delete(_ context.Context, _ resource.DeleteRequest, resp *resource.DeleteResponse) {
	resp.Diagnostics.AddWarning("Eval Agent publication retained", "Publishing creates immutable Eval Agent history. Terraform removes only its own publication state and does not roll back the active version.")
}

var _ resource.Resource = (*Resource)(nil)
var _ resource.ResourceWithConfigure = (*Resource)(nil)
