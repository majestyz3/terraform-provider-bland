package agentvariable

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
	Key          types.String `tfsdk:"key"`
	ValueJSON    types.String `tfsdk:"value_json"`
	IsSecret     types.Bool   `tfsdk:"is_secret"`
	ResponseJSON types.String `tfsdk:"response_json"`
}

type Resource struct{ client *client.Client }

func New() resource.Resource { return &Resource{} }

func (r *Resource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_agent_variable"
}

func (r *Resource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	replace := []planmodifier.String{stringplanmodifier.RequiresReplace()}
	resp.Schema = schema.Schema{Attributes: map[string]schema.Attribute{
		"id":            schema.StringAttribute{Computed: true},
		"agent_id":      schema.StringAttribute{Required: true, PlanModifiers: replace},
		"environment":   schema.StringAttribute{Required: true, PlanModifiers: replace},
		"key":           schema.StringAttribute{Required: true, PlanModifiers: replace},
		"value_json":    schema.StringAttribute{Required: true, Sensitive: true},
		"is_secret":     schema.BoolAttribute{Optional: true},
		"response_json": schema.StringAttribute{Computed: true, Sensitive: true},
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
	return "/v2/agents/" + url.PathEscape(m.AgentID.ValueString()) + "/environments/" + url.PathEscape(m.Environment.ValueString()) + "/variables/" + url.PathEscape(m.Key.ValueString())
}

func encode(v any) string { b, _ := json.MarshalIndent(v, "", "  "); return string(b) }

func (r *Resource) apply(ctx context.Context, m *Model, add func(string, string)) bool {
	var value any
	if err := json.Unmarshal([]byte(m.ValueJSON.ValueString()), &value); err != nil {
		add("Invalid value_json", err.Error())
		return false
	}
	out, _, err := r.client.Do(ctx, http.MethodPut, r.path(*m), map[string]any{"value": value, "is_secret": m.IsSecret.ValueBool()})
	if err != nil {
		add("Set Agent Variable failed", err.Error())
		return false
	}
	m.ID = types.StringValue(m.AgentID.ValueString() + "/" + m.Environment.ValueString() + "/" + m.Key.ValueString())
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
	out, status, err := r.client.Do(ctx, http.MethodGet, "/v2/agents/"+url.PathEscape(m.AgentID.ValueString())+"/environments/"+url.PathEscape(m.Environment.ValueString())+"/variables", nil)
	if status == http.StatusNotFound {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Read Agent Variables failed", err.Error())
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
		resp.Diagnostics.AddError("Delete Agent Variable failed", err.Error())
	}
}

var _ resource.Resource = (*Resource)(nil)
var _ resource.ResourceWithConfigure = (*Resource)(nil)
