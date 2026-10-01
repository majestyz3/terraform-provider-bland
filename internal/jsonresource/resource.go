package jsonresource

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/majestyz3/terraform-provider-bland/internal/client"
)

type Spec struct {
	TypeName                 string
	Description              string
	CreateMethod, CreatePath string
	ReadMethod, ReadPath     string
	UpdateMethod, UpdatePath string
	DeleteMethod, DeletePath string
	IDPaths                  []string
	Immutable                bool
	CreateOnlyKeys           []string
	PostCreateUpdate         bool
}

type Model struct {
	ID           types.String `tfsdk:"id"`
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
	config := schema.StringAttribute{
		Required:            true,
		MarkdownDescription: "JSON request body. Prefer jsonencode({...}).",
	}
	if r.spec.Immutable {
		config.PlanModifiers = []planmodifier.String{stringplanmodifier.RequiresReplace()}
	}
	resp.Schema = schema.Schema{
		MarkdownDescription: r.spec.Description,
		Attributes: map[string]schema.Attribute{
			"id":            schema.StringAttribute{Computed: true},
			"config_json":   config,
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

func interpolate(s, id string) string { return strings.ReplaceAll(s, "{id}", id) }

func lookup(m map[string]any, dotted string) (string, bool) {
	var cur any = m
	for _, key := range strings.Split(dotted, ".") {
		next, ok := cur.(map[string]any)
		if !ok {
			return "", false
		}
		cur, ok = next[key]
		if !ok {
			return "", false
		}
	}
	s, ok := cur.(string)
	return s, ok && s != ""
}

func extractID(m map[string]any, candidates []string) (string, bool) {
	candidates = append(candidates, "data.id", "id", "data.uuid", "uuid", "data.vector_id", "vector_id")
	for _, p := range candidates {
		if s, ok := lookup(m, p); ok {
			return s, true
		}
	}
	return "", false
}

func selectKeys(m map[string]any, keys []string) map[string]any {
	if len(keys) == 0 {
		return m
	}
	out := map[string]any{}
	for _, k := range keys {
		if v, ok := m[k]; ok {
			out[k] = v
		}
	}
	return out
}

func (r *Resource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan Model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	body, err := decode(plan.ConfigJSON.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid configuration", err.Error())
		return
	}
	out, _, err := r.client.Do(ctx, r.spec.CreateMethod, r.spec.CreatePath, selectKeys(body, r.spec.CreateOnlyKeys))
	if err != nil {
		resp.Diagnostics.AddError("Bland API create failed", err.Error())
		return
	}
	id, ok := extractID(out, r.spec.IDPaths)
	if !ok {
		resp.Diagnostics.AddError("Missing resource id", encode(out))
		return
	}
	if r.spec.PostCreateUpdate && r.spec.UpdateMethod != "" {
		updated, _, uerr := r.client.Do(ctx, r.spec.UpdateMethod, interpolate(r.spec.UpdatePath, id), body)
		if uerr != nil {
			if r.spec.DeleteMethod != "" {
				_, _, _ = r.client.Do(ctx, r.spec.DeleteMethod, interpolate(r.spec.DeletePath, id), nil)
			}
			resp.Diagnostics.AddError("Post-create configuration failed", uerr.Error())
			return
		}
		out = updated
	}
	plan.ID = types.StringValue(id)
	plan.ResponseJSON = types.StringValue(encode(out))
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *Resource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state Model
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	out, status, err := r.client.Do(ctx, r.spec.ReadMethod, interpolate(r.spec.ReadPath, state.ID.ValueString()), nil)
	if status == http.StatusNotFound {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Bland API read failed", err.Error())
		return
	}
	state.ResponseJSON = types.StringValue(encode(out))
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *Resource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan Model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var prior Model
	resp.Diagnostics.Append(req.State.Get(ctx, &prior)...)
	if resp.Diagnostics.HasError() {
		return
	}
	plan.ID = prior.ID
	body, err := decode(plan.ConfigJSON.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid configuration", err.Error())
		return
	}
	out, _, err := r.client.Do(ctx, r.spec.UpdateMethod, interpolate(r.spec.UpdatePath, plan.ID.ValueString()), body)
	if err != nil {
		resp.Diagnostics.AddError("Bland API update failed", err.Error())
		return
	}
	plan.ResponseJSON = types.StringValue(encode(out))
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *Resource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state Model
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	_, status, err := r.client.Do(ctx, r.spec.DeleteMethod, interpolate(r.spec.DeletePath, state.ID.ValueString()), nil)
	if err != nil && status != http.StatusNotFound {
		resp.Diagnostics.AddError("Bland API delete failed", err.Error())
	}
}

func (r *Resource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

var _ resource.Resource = (*Resource)(nil)
var _ resource.ResourceWithConfigure = (*Resource)(nil)
var _ resource.ResourceWithImportState = (*Resource)(nil)
