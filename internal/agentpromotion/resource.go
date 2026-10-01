package agentpromotion

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
	AgentID          types.String `tfsdk:"agent_id"`
	Triggers         types.Map    `tfsdk:"triggers"`
	VersionNumber    types.String `tfsdk:"version_number"`
	EnvironmentsJSON types.String `tfsdk:"environments_json"`
	ResponseJSON     types.String `tfsdk:"response_json"`
}

type Resource struct{ client *client.Client }

func New() resource.Resource { return &Resource{} }

func (r *Resource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_agent_promotion"
}

func (r *Resource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Explicitly promotes the Bland V2 Agent version currently pinned to staging into production. Bland does not run checks automatically during this call; run and gate required checks before creating this resource. Most users should leave production promotion as a deliberate/manual step.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{Computed: true},
			"agent_id": schema.StringAttribute{
				Required:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"triggers": schema.MapAttribute{
				Optional:    true,
				ElementType: types.StringType,
				PlanModifiers: []planmodifier.Map{
					mapplanmodifier.RequiresReplace(),
				},
				MarkdownDescription: "Optional arbitrary values used only to force a new explicit promotion.",
			},
			"version_number": schema.StringAttribute{Computed: true},
			"environments_json": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Normalized JSON for the environment pins returned by promotion.",
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

func productionVersionID(envs []any) string {
	for _, raw := range envs {
		env, _ := raw.(map[string]any)
		if env == nil {
			continue
		}
		if env["env_type"] == "production" {
			id, _ := env["current_version_id"].(string)
			return id
		}
	}
	return ""
}

func (r *Resource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var m Model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}

	out, _, err := r.client.Do(ctx, http.MethodPost, "/v2/agents/"+url.PathEscape(m.AgentID.ValueString())+"/promote", nil)
	if err != nil {
		resp.Diagnostics.AddError("Promote Agent failed", err.Error())
		return
	}
	data, _ := out["data"].(map[string]any)
	if data == nil {
		resp.Diagnostics.AddError("Missing Agent Promotion data", encode(out))
		return
	}
	semver, _ := data["semver"].(string)
	envs, _ := data["environments"].([]any)
	if envs == nil {
		envs = []any{}
	}
	productionID := productionVersionID(envs)
	if productionID == "" {
		resp.Diagnostics.AddError("Missing production version after promotion", encode(out))
		return
	}

	m.ID = types.StringValue(m.AgentID.ValueString() + "/" + productionID)
	m.VersionNumber = types.StringValue(semver)
	m.EnvironmentsJSON = types.StringValue(encode(envs))
	m.ResponseJSON = types.StringValue(encode(out))
	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
}

func (r *Resource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var m Model
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	out, status, err := r.client.Do(ctx, http.MethodGet, "/v2/agents/"+url.PathEscape(m.AgentID.ValueString())+"/environments", nil)
	if status == http.StatusNotFound {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Read Agent Promotion environment state failed", err.Error())
		return
	}
	m.EnvironmentsJSON = types.StringValue(encode(out["data"]))
	m.ResponseJSON = types.StringValue(encode(out))
	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
}

func (r *Resource) Update(context.Context, resource.UpdateRequest, *resource.UpdateResponse) {}

func (r *Resource) Delete(_ context.Context, _ resource.DeleteRequest, resp *resource.DeleteResponse) {
	resp.Diagnostics.AddWarning("Production promotion retained", "Terraform removes only its promotion record from state. It does not roll production back.")
}

var _ resource.Resource = (*Resource)(nil)
var _ resource.ResourceWithConfigure = (*Resource)(nil)
