package agentidentity

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
	resp.TypeName = req.ProviderTypeName + "_agent_identity"
}

func (r *Resource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages the mutable contact-card identity for a Bland V2 Agent. Bland exposes update/read semantics but no identity delete endpoint.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{Computed: true},
			"agent_id": schema.StringAttribute{
				Required:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"config_json": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Identity fields to manage, such as display_name, tagline, support_email, website, address, vcf_delivery_enabled, rcs_enabled, and bcid_enabled.",
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
	if len(m) == 0 {
		return nil, fmt.Errorf("config_json must contain at least one identity field")
	}
	return m, nil
}

func encode(v any) string {
	b, _ := json.MarshalIndent(v, "", "  ")
	return string(b)
}

func identityID(out map[string]any) string {
	if d, ok := out["data"].(map[string]any); ok {
		id, _ := d["id"].(string)
		return id
	}
	return ""
}

func (r *Resource) apply(ctx context.Context, m *Model, respDiags func(string, string)) (map[string]any, bool) {
	body, err := decode(m.ConfigJSON.ValueString())
	if err != nil {
		respDiags("Invalid identity configuration", err.Error())
		return nil, false
	}
	p := "/v2/agents/" + url.PathEscape(m.AgentID.ValueString()) + "/identity"
	out, _, err := r.client.Do(ctx, http.MethodPut, p, body)
	if err != nil {
		respDiags("Update Agent Identity failed", err.Error())
		return nil, false
	}
	return out, true
}

func (r *Resource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var m Model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	out, ok := r.apply(ctx, &m, resp.Diagnostics.AddError)
	if !ok {
		return
	}
	id := identityID(out)
	if id == "" {
		resp.Diagnostics.AddError("Missing Agent Identity ID", encode(out))
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
	p := "/v2/agents/" + url.PathEscape(m.AgentID.ValueString()) + "/identity"
	out, status, err := r.client.Do(ctx, http.MethodGet, p, nil)
	if status == http.StatusNotFound {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Read Agent Identity failed", err.Error())
		return
	}
	if id := identityID(out); id != "" {
		m.ID = types.StringValue(id)
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
	out, ok := r.apply(ctx, &planned, resp.Diagnostics.AddError)
	if !ok {
		return
	}
	if id := identityID(out); id != "" {
		planned.ID = types.StringValue(id)
	}
	planned.ResponseJSON = types.StringValue(encode(out))
	resp.Diagnostics.Append(resp.State.Set(ctx, &planned)...)
}

func (r *Resource) Delete(_ context.Context, _ resource.DeleteRequest, resp *resource.DeleteResponse) {
	resp.Diagnostics.AddWarning("Agent Identity retained", "Bland does not expose an identity delete endpoint. Terraform forgets this configuration while Bland retains the last identity values.")
}

func (r *Resource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("agent_id"), req.ID)...)
}

var _ resource.Resource = (*Resource)(nil)
var _ resource.ResourceWithConfigure = (*Resource)(nil)
var _ resource.ResourceWithImportState = (*Resource)(nil)
