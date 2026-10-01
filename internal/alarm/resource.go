package alarm

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
	ID           types.String `tfsdk:"id"`
	ConfigJSON   types.String `tfsdk:"config_json"`
	ResponseJSON types.String `tfsdk:"response_json"`
}

type Resource struct{ client *client.Client }

func New() resource.Resource { return &Resource{} }

func (r *Resource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_alarm"
}

func (r *Resource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a Bland organization alarm using only the public Alarm API.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{Computed: true},
			"config_json": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Alarm request JSON. Public fields are metric_type, threshold, webhook_config, email_addresses, and sms_numbers.",
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

func decode(v string) (map[string]any, error) {
	var out map[string]any
	if err := json.Unmarshal([]byte(v), &out); err != nil {
		return nil, fmt.Errorf("config_json must be valid JSON: %w", err)
	}
	return out, nil
}

func pretty(v any) string {
	b, _ := json.MarshalIndent(v, "", "  ")
	return string(b)
}

func compact(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}

func alarmData(out map[string]any) map[string]any {
	if data, ok := out["data"].(map[string]any); ok {
		if alarm, ok := data["alarm"].(map[string]any); ok {
			return alarm
		}
		return data
	}
	return out
}

func alarmID(out map[string]any) string {
	id, _ := alarmData(out)["id"].(string)
	return id
}

func requestConfigFromResponse(out map[string]any) map[string]any {
	data := alarmData(out)
	cfg := map[string]any{}
	for _, key := range []string{"metric_type", "threshold", "webhook_config", "email_addresses", "sms_numbers"} {
		if value, ok := data[key]; ok {
			cfg[key] = value
		}
	}
	return cfg
}

func (r *Resource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var m Model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	body, err := decode(m.ConfigJSON.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid Alarm configuration", err.Error())
		return
	}
	out, _, err := r.client.Do(ctx, http.MethodPost, "/v1/alarms", body)
	if err != nil {
		resp.Diagnostics.AddError("Create Alarm failed", err.Error())
		return
	}
	id := alarmID(out)
	if id == "" {
		resp.Diagnostics.AddError("Missing Alarm ID", pretty(out))
		return
	}
	m.ID = types.StringValue(id)
	m.ResponseJSON = types.StringValue(pretty(out))
	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
}

func (r *Resource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var m Model
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	out, status, err := r.client.Do(ctx, http.MethodGet, "/v1/alarms/"+url.PathEscape(m.ID.ValueString()), nil)
	if status == http.StatusNotFound {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Read Alarm failed", err.Error())
		return
	}
	if m.ConfigJSON.IsNull() || m.ConfigJSON.IsUnknown() || m.ConfigJSON.ValueString() == "" {
		m.ConfigJSON = types.StringValue(compact(requestConfigFromResponse(out)))
	}
	m.ResponseJSON = types.StringValue(pretty(out))
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
	body, err := decode(m.ConfigJSON.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid Alarm configuration", err.Error())
		return
	}
	out, _, err := r.client.Do(ctx, http.MethodPatch, "/v1/alarms/"+url.PathEscape(m.ID.ValueString()), body)
	if err != nil {
		resp.Diagnostics.AddError("Update Alarm failed", err.Error())
		return
	}
	m.ResponseJSON = types.StringValue(pretty(out))
	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
}

func (r *Resource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var m Model
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	_, status, err := r.client.Do(ctx, http.MethodDelete, "/v1/alarms/"+url.PathEscape(m.ID.ValueString()), nil)
	if err != nil && status != http.StatusNotFound {
		resp.Diagnostics.AddError("Delete Alarm failed", err.Error())
	}
}

func (r *Resource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

var _ resource.Resource = (*Resource)(nil)
var _ resource.ResourceWithConfigure = (*Resource)(nil)
var _ resource.ResourceWithImportState = (*Resource)(nil)
