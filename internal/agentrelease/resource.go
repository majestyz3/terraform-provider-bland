package agentrelease

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/majestyz3/terraform-provider-bland/internal/client"
)

type Model struct {
	ID               types.String `tfsdk:"id"`
	AgentID          types.String `tfsdk:"agent_id"`
	VersionID        types.String `tfsdk:"version_id"`
	Bump             types.String `tfsdk:"bump"`
	VersionNumber    types.String `tfsdk:"version_number"`
	EnvironmentsJSON types.String `tfsdk:"environments_json"`
	WarningsJSON     types.String `tfsdk:"warnings_json"`
	ResponseJSON     types.String `tfsdk:"response_json"`
}

type Resource struct{ client *client.Client }

func New() resource.Resource { return &Resource{} }

func (r *Resource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_agent_release"
}

func (r *Resource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	replace := []planmodifier.String{stringplanmodifier.RequiresReplace()}
	resp.Schema = schema.Schema{
		MarkdownDescription: "Explicitly publishes a saved Bland V2 Agent version to staging. This resource is opt-in and never runs implicitly from agent-version changes.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{Computed: true},
			"agent_id": schema.StringAttribute{
				Required:      true,
				PlanModifiers: replace,
			},
			"version_id": schema.StringAttribute{
				Required:      true,
				PlanModifiers: replace,
			},
			"bump": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString("patch"),
				PlanModifiers:       replace,
				MarkdownDescription: "Semantic version component to increment when the version is first published: patch, minor, or major. Defaults to patch.",
			},
			"version_number": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Semantic version recorded for the published deployment.",
			},
			"environments_json": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Normalized JSON for the returned dev/staging/production environment pins.",
			},
			"warnings_json": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Normalized JSON array of non-blocking publish warnings.",
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

func (r *Resource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var m Model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}

	bump := m.Bump.ValueString()
	if m.Bump.IsNull() || m.Bump.IsUnknown() || bump == "" {
		bump = "patch"
		m.Bump = types.StringValue(bump)
	}
	if bump != "patch" && bump != "minor" && bump != "major" {
		resp.Diagnostics.AddError("Invalid bump", "bump must be one of: patch, minor, major.")
		return
	}

	body := map[string]any{
		"version_id": m.VersionID.ValueString(),
		"bump":       bump,
	}
	out, _, err := r.client.Do(ctx, http.MethodPost, "/v2/agents/"+url.PathEscape(m.AgentID.ValueString())+"/publish", body)
	if err != nil {
		resp.Diagnostics.AddError("Publish Agent Version failed", err.Error())
		return
	}

	data, _ := out["data"].(map[string]any)
	if data == nil {
		resp.Diagnostics.AddError("Missing Agent Release data", encode(out))
		return
	}
	semver, _ := data["semver"].(string)
	if semver == "" {
		resp.Diagnostics.AddError("Missing Agent Release version number", encode(out))
		return
	}

	envs, _ := data["environments"].([]any)
	warnings, _ := data["warnings"].([]any)
	if envs == nil {
		envs = []any{}
	}
	if warnings == nil {
		warnings = []any{}
	}

	m.ID = types.StringValue(m.AgentID.ValueString() + "/" + m.VersionID.ValueString())
	m.VersionNumber = types.StringValue(semver)
	m.EnvironmentsJSON = types.StringValue(encode(envs))
	m.WarningsJSON = types.StringValue(encode(warnings))
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
		resp.Diagnostics.AddError("Read Agent Release environment state failed", err.Error())
		return
	}

	data := out["data"]
	m.EnvironmentsJSON = types.StringValue(encode(data))
	m.ResponseJSON = types.StringValue(encode(out))
	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
}

func (r *Resource) Update(context.Context, resource.UpdateRequest, *resource.UpdateResponse) {}

func (r *Resource) Delete(_ context.Context, _ resource.DeleteRequest, resp *resource.DeleteResponse) {
	resp.Diagnostics.AddWarning("Agent release retained", "Terraform removes only the release record from state. It does not automatically roll staging back.")
}

var _ resource.Resource = (*Resource)(nil)
var _ resource.ResourceWithConfigure = (*Resource)(nil)
