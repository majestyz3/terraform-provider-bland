package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/majestyz3/terraform-provider-bland/internal/client"
)

type Model struct {
	ID           types.String `tfsdk:"id"`
	ConfigJSON   types.String `tfsdk:"config_json"`
	ResponseJSON types.String `tfsdk:"response_json"`
}

type Resource struct{ client *client.Client }

func New() resource.Resource { return &Resource{} }

func (r *Resource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_tool"
}

func (r *Resource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a Bland V2 Tool. The public API currently exposes create, update, and list but no delete endpoint.",
		Attributes: map[string]schema.Attribute{
			"id":            schema.StringAttribute{Computed: true},
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

func encode(v any) string { b, _ := json.MarshalIndent(v, "", "  "); return string(b) }
func decode(s string) (map[string]any, error) {
	var m map[string]any
	err := json.Unmarshal([]byte(s), &m)
	return m, err
}

func toolID(out map[string]any) string {
	if d, ok := out["data"].(map[string]any); ok {
		id, _ := d["id"].(string)
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
	body, err := decode(m.ConfigJSON.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid config_json", err.Error())
		return
	}
	out, _, err := r.client.Do(ctx, http.MethodPost, "/v2/tools", body)
	if err != nil {
		resp.Diagnostics.AddError("Create Tool failed", err.Error())
		return
	}
	id := toolID(out)
	if id == "" {
		resp.Diagnostics.AddError("Missing Tool ID", encode(out))
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
	out, _, err := r.client.Do(ctx, http.MethodGet, "/v2/tools?limit=1000&page=1", nil)
	if err != nil {
		resp.Diagnostics.AddError("List Tools failed", err.Error())
		return
	}
	m.ResponseJSON = types.StringValue(encode(out))
	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
}

func (r *Resource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var m Model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &m)...)
	var prior Model
	resp.Diagnostics.Append(req.State.Get(ctx, &prior)...)
	if resp.Diagnostics.HasError() {
		return
	}
	m.ID = prior.ID
	body, err := decode(m.ConfigJSON.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid config_json", err.Error())
		return
	}
	out, _, err := r.client.Do(ctx, http.MethodPost, "/v2/tools/"+url.PathEscape(m.ID.ValueString()), body)
	if err != nil {
		resp.Diagnostics.AddError("Update Tool failed", err.Error())
		return
	}
	m.ResponseJSON = types.StringValue(encode(out))
	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
}

func (r *Resource) Delete(_ context.Context, _ resource.DeleteRequest, resp *resource.DeleteResponse) {
	resp.Diagnostics.AddWarning("Tool retained", "Bland's documented V2 Tools API has no delete endpoint; Terraform forgets the tool while Bland retains it.")
}

var _ resource.Resource = (*Resource)(nil)
var _ resource.ResourceWithConfigure = (*Resource)(nil)
