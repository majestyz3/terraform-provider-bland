package evalagent

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/majestyz3/terraform-provider-bland/internal/client"
)

type Model struct {
	ID               types.String `tfsdk:"id"`
	ConfigJSON       types.String `tfsdk:"config_json"`
	ResponseJSON     types.String `tfsdk:"response_json"`
	CurrentVersionID types.String `tfsdk:"current_version_id"`
	ActiveVersionID  types.String `tfsdk:"active_version_id"`
}

type Resource struct{ client *client.Client }

func New() resource.Resource { return &Resource{} }

func (r *Resource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_eval_agent"
}

func (r *Resource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a Bland Eval Agent and exposes its editable and active version IDs.",
		Attributes: map[string]schema.Attribute{
			"id":                 schema.StringAttribute{Computed: true},
			"config_json":        schema.StringAttribute{Required: true, MarkdownDescription: "JSON request body. Prefer jsonencode({...})."},
			"response_json":      schema.StringAttribute{Computed: true},
			"current_version_id": schema.StringAttribute{Computed: true, MarkdownDescription: "ID of the current editable draft version."},
			"active_version_id":  schema.StringAttribute{Computed: true, MarkdownDescription: "ID of the published active version, when one exists."},
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

func decodeConfig(v string) (map[string]any, error) {
	var out map[string]any
	if err := json.Unmarshal([]byte(v), &out); err != nil {
		return nil, fmt.Errorf("config_json must be valid JSON: %w", err)
	}
	return out, nil
}

func applyVersionFields(m *Model, out map[string]any) {
	data, _ := out["data"].(map[string]any)
	if data == nil {
		return
	}
	agent, _ := data["agent"].(map[string]any)
	if agent == nil {
		agent = data
	}
	if v, _ := agent["current_version_id"].(string); v != "" {
		m.CurrentVersionID = types.StringValue(v)
	}
	if v, ok := agent["active_version_id"].(string); ok && v != "" {
		m.ActiveVersionID = types.StringValue(v)
	} else {
		m.ActiveVersionID = types.StringNull()
	}
}

func extractID(out map[string]any) string {
	data, _ := out["data"].(map[string]any)
	if data != nil {
		if agent, _ := data["agent"].(map[string]any); agent != nil {
			if id, _ := agent["id"].(string); id != "" {
				return id
			}
		}
		if id, _ := data["id"].(string); id != "" {
			return id
		}
	}
	if id, _ := out["id"].(string); id != "" {
		return id
	}
	return ""
}

func (r *Resource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var m Model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	body, err := decodeConfig(m.ConfigJSON.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid configuration", err.Error())
		return
	}
	out, _, err := r.client.Do(ctx, http.MethodPost, "/v1/evals/agents", body)
	if err != nil {
		resp.Diagnostics.AddError("Create Eval Agent failed", err.Error())
		return
	}
	id := extractID(out)
	if id == "" {
		resp.Diagnostics.AddError("Missing Eval Agent ID", encode(out))
		return
	}
	m.ID = types.StringValue(id)
	applyVersionFields(&m, out)
	if m.CurrentVersionID.IsNull() || m.CurrentVersionID.ValueString() == "" {
		fresh, _, readErr := r.client.Do(ctx, http.MethodGet, "/v1/evals/agents/"+url.PathEscape(id), nil)
		if readErr == nil {
			out = fresh
			applyVersionFields(&m, fresh)
		}
	}
	m.ResponseJSON = types.StringValue(encode(out))
	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
}

func (r *Resource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var m Model
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	out, status, err := r.client.Do(ctx, http.MethodGet, "/v1/evals/agents/"+url.PathEscape(m.ID.ValueString()), nil)
	if status == http.StatusNotFound {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Read Eval Agent failed", err.Error())
		return
	}
	applyVersionFields(&m, out)
	m.ResponseJSON = types.StringValue(encode(out))
	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
}

func (r *Resource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var m Model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var prior Model
	resp.Diagnostics.Append(req.State.Get(ctx, &prior)...)
	if resp.Diagnostics.HasError() {
		return
	}
	m.ID = prior.ID
	body, err := decodeConfig(m.ConfigJSON.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid configuration", err.Error())
		return
	}
	out, _, err := r.client.Do(ctx, http.MethodPatch, "/v1/evals/agents/"+url.PathEscape(m.ID.ValueString()), body)
	if err != nil {
		resp.Diagnostics.AddError("Update Eval Agent failed", err.Error())
		return
	}
	fresh, _, readErr := r.client.Do(ctx, http.MethodGet, "/v1/evals/agents/"+url.PathEscape(m.ID.ValueString()), nil)
	if readErr == nil {
		out = fresh
	}
	applyVersionFields(&m, out)
	m.ResponseJSON = types.StringValue(encode(out))
	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
}

func (r *Resource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var m Model
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	_, status, err := r.client.Do(ctx, http.MethodDelete, "/v1/evals/agents/"+url.PathEscape(m.ID.ValueString()), nil)
	if err != nil && status != http.StatusNotFound {
		resp.Diagnostics.AddError("Delete Eval Agent failed", err.Error())
	}
}

func (r *Resource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

var _ resource.Resource = (*Resource)(nil)
var _ resource.ResourceWithConfigure = (*Resource)(nil)
var _ resource.ResourceWithImportState = (*Resource)(nil)
