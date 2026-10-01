package agentchecks

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
	Environment  types.String `tfsdk:"environment"`
	ConfigJSON   types.String `tfsdk:"config_json"`
	ResponseJSON types.String `tfsdk:"response_json"`
}

type Resource struct{ client *client.Client }

func New() resource.Resource { return &Resource{} }

func (r *Resource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_agent_checks"
}

func (r *Resource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	replace := []planmodifier.String{stringplanmodifier.RequiresReplace()}
	resp.Schema = schema.Schema{Attributes: map[string]schema.Attribute{
		"id":            schema.StringAttribute{Computed: true},
		"agent_id":      schema.StringAttribute{Required: true, PlanModifiers: replace},
		"environment":   schema.StringAttribute{Required: true, PlanModifiers: replace},
		"config_json":   schema.StringAttribute{Required: true},
		"response_json": schema.StringAttribute{Computed: true},
	}}
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

func (r *Resource) path(m Model) string {
	return "/v2/agents/" + url.PathEscape(m.AgentID.ValueString()) + "/environments/" + url.PathEscape(m.Environment.ValueString()) + "/checks"
}

func encode(v any) string { b, _ := json.MarshalIndent(v, "", "  "); return string(b) }

func (r *Resource) apply(ctx context.Context, m *Model, add func(string, string)) bool {
	var body map[string]any
	if err := json.Unmarshal([]byte(m.ConfigJSON.ValueString()), &body); err != nil {
		add("Invalid config_json", err.Error())
		return false
	}
	out, _, err := r.client.Do(ctx, http.MethodPut, r.path(*m), body)
	if err != nil {
		add("Set Agent Checks failed", err.Error())
		return false
	}
	m.ID = types.StringValue(m.AgentID.ValueString() + "/" + m.Environment.ValueString())
	m.ResponseJSON = types.StringValue(encode(out))
	return true
}

func (r *Resource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var m Model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &m)...)
	if !resp.Diagnostics.HasError() && r.apply(ctx, &m, resp.Diagnostics.AddError) {
		resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
	}
}

func (r *Resource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var m Model
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	out, status, err := r.client.Do(ctx, http.MethodGet, r.path(m), nil)
	if status == http.StatusNotFound {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Read Agent Checks failed", err.Error())
		return
	}
	m.ResponseJSON = types.StringValue(encode(out))
	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
}

func (r *Resource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var m Model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &m)...)
	if !resp.Diagnostics.HasError() && r.apply(ctx, &m, resp.Diagnostics.AddError) {
		resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
	}
}

func (r *Resource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var m Model
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	_, status, err := r.client.Do(ctx, http.MethodDelete, r.path(m), nil)
	if err != nil && status != http.StatusNotFound {
		resp.Diagnostics.AddError("Delete Agent Checks failed", err.Error())
	}
}

var _ resource.Resource = (*Resource)(nil)
var _ resource.ResourceWithConfigure = (*Resource)(nil)
