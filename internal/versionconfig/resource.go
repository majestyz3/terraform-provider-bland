package versionconfig

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/majestyz3/terraform-provider-bland/internal/client"
)

type Spec struct{ TypeName, Description, ReadPath, PatchPath string }

type Model struct {
	ID           types.String `tfsdk:"id"`
	ParentID     types.String `tfsdk:"parent_id"`
	VersionID    types.String `tfsdk:"version_id"`
	ConfigJSON   types.String `tfsdk:"config_json"`
	ResponseJSON types.String `tfsdk:"response_json"`
}

type Resource struct {
	spec   Spec
	client *client.Client
}

func New(spec Spec) func() resource.Resource {
	return func() resource.Resource { return &Resource{spec: spec} }
}

func (r *Resource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_" + r.spec.TypeName
}

func (r *Resource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: r.spec.Description,
		Attributes: map[string]schema.Attribute{
			"id":            schema.StringAttribute{Computed: true},
			"parent_id":     schema.StringAttribute{Required: true},
			"version_id":    schema.StringAttribute{Required: true},
			"config_json":   schema.StringAttribute{Required: true},
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

func (r *Resource) path(tpl, parent, version string) string {
	return strings.ReplaceAll(strings.ReplaceAll(tpl, "{parent_id}", parent), "{version_id}", version)
}

func encode(v any) string { b, _ := json.MarshalIndent(v, "", "  "); return string(b) }

func (r *Resource) apply(ctx context.Context, m *Model, add func(string, string)) bool {
	var body map[string]any
	if err := json.Unmarshal([]byte(m.ConfigJSON.ValueString()), &body); err != nil {
		add("Invalid config_json", err.Error())
		return false
	}
	out, _, err := r.client.Do(ctx, http.MethodPatch, r.path(r.spec.PatchPath, m.ParentID.ValueString(), m.VersionID.ValueString()), body)
	if err != nil {
		add("Version update failed", err.Error())
		return false
	}
	m.ID = types.StringValue(m.ParentID.ValueString() + "/" + m.VersionID.ValueString())
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
	out, status, err := r.client.Do(ctx, http.MethodGet, r.path(r.spec.ReadPath, m.ParentID.ValueString(), m.VersionID.ValueString()), nil)
	if status == http.StatusNotFound {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Version read failed", err.Error())
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
func (r *Resource) Delete(_ context.Context, _ resource.DeleteRequest, resp *resource.DeleteResponse) {
	resp.Diagnostics.AddWarning("Version configuration retained", "Bland does not expose a separate delete for this editable version configuration; Terraform forgets it while the parent/version remains.")
}

var _ resource.Resource = (*Resource)(nil)
var _ resource.ResourceWithConfigure = (*Resource)(nil)
