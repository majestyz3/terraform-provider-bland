package knowledgebase

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/majestyz3/terraform-provider-bland/internal/client"
)

type Model struct {
	ID           types.String `tfsdk:"id"`
	ConfigJSON   types.String `tfsdk:"config_json"`
	ReadyTimeout types.String `tfsdk:"ready_timeout"`
	Status       types.String `tfsdk:"status"`
	ResponseJSON types.String `tfsdk:"response_json"`
}

type Resource struct{ client *client.Client }

func New() resource.Resource { return &Resource{} }

func (r *Resource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_knowledge_base"
}

func (r *Resource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a text-backed Bland Knowledge Base item and waits for asynchronous processing to reach COMPLETED.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{Computed: true},
			"config_json": schema.StringAttribute{
				Required:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
				MarkdownDescription: "JSON request body for POST /v1/knowledge/learn. Prefer jsonencode({...}).",
			},
			"ready_timeout": schema.StringAttribute{
				Optional:            true,
				Default:             stringdefault.StaticString("5m"),
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
				MarkdownDescription: "Maximum time to wait for status COMPLETED after creation. Go duration syntax; defaults to 5m.",
			},
			"status": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Current Bland knowledge processing status.",
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

func decodeConfig(v string) (map[string]any, error) {
	var out map[string]any
	if err := json.Unmarshal([]byte(v), &out); err != nil {
		return nil, fmt.Errorf("config_json must be valid JSON: %w", err)
	}
	return out, nil
}

func extractID(out map[string]any) string {
	if data, _ := out["data"].(map[string]any); data != nil {
		if id, _ := data["id"].(string); id != "" {
			return id
		}
		if id, _ := data["knowledge_base_id"].(string); id != "" {
			return id
		}
	}
	if id, _ := out["id"].(string); id != "" {
		return id
	}
	return ""
}

func statusFields(out map[string]any) (string, string) {
	data, _ := out["data"].(map[string]any)
	if data == nil {
		return "", ""
	}
	if status, _ := data["status"].(string); status != "" {
		message, _ := data["error_message"].(string)
		return status, message
	}
	for _, key := range []string{"knowledge_base", "item"} {
		if nested, _ := data[key].(map[string]any); nested != nil {
			if status, _ := nested["status"].(string); status != "" {
				message, _ := nested["error_message"].(string)
				return status, message
			}
		}
	}
	return "", ""
}

func (r *Resource) waitReady(ctx context.Context, id string, timeout time.Duration) (map[string]any, string, error) {
	deadline := time.Now().Add(timeout)
	for {
		out, _, err := r.client.Do(ctx, http.MethodGet, "/v1/knowledge/"+url.PathEscape(id), nil)
		if err != nil {
			return out, "", err
		}
		status, message := statusFields(out)
		if status == "" {
			return out, "", fmt.Errorf("knowledge base status missing from response: %s", encode(out))
		}
		switch status {
		case "COMPLETED":
			return out, status, nil
		case "FAILED":
			if message == "" {
				message = "Bland reported FAILED status"
			}
			return out, status, fmt.Errorf("%s", message)
		case "DELETED":
			return out, status, fmt.Errorf("knowledge base was deleted while waiting for readiness")
		}
		if time.Now().After(deadline) {
			return out, status, fmt.Errorf("timed out after %s waiting for COMPLETED; last status %q", timeout, status)
		}
		select {
		case <-ctx.Done():
			return out, status, ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}
}

func (r *Resource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var m Model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}

	timeoutText := m.ReadyTimeout.ValueString()
	if m.ReadyTimeout.IsNull() || m.ReadyTimeout.IsUnknown() || timeoutText == "" {
		timeoutText = "5m"
		m.ReadyTimeout = types.StringValue(timeoutText)
	}
	timeout, err := time.ParseDuration(timeoutText)
	if err != nil || timeout <= 0 {
		resp.Diagnostics.AddError("Invalid ready_timeout", "Use a positive Go duration such as 30s, 5m, or 10m.")
		return
	}

	body, err := decodeConfig(m.ConfigJSON.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid configuration", err.Error())
		return
	}
	out, _, err := r.client.Do(ctx, http.MethodPost, "/v1/knowledge/learn", body)
	if err != nil {
		resp.Diagnostics.AddError("Create Knowledge Base failed", err.Error())
		return
	}
	id := extractID(out)
	if id == "" {
		resp.Diagnostics.AddError("Missing Knowledge Base ID", encode(out))
		return
	}

	m.ID = types.StringValue(id)
	ready, status, err := r.waitReady(ctx, id, timeout)
	if err != nil {
		resp.Diagnostics.AddError("Knowledge Base did not become ready", err.Error())
		return
	}
	m.Status = types.StringValue(status)
	m.ResponseJSON = types.StringValue(encode(ready))
	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
}

func (r *Resource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var m Model
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	out, statusCode, err := r.client.Do(ctx, http.MethodGet, "/v1/knowledge/"+url.PathEscape(m.ID.ValueString()), nil)
	if statusCode == http.StatusNotFound {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Read Knowledge Base failed", err.Error())
		return
	}
	status, _ := statusFields(out)
	m.Status = types.StringValue(status)
	m.ResponseJSON = types.StringValue(encode(out))
	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
}

func (r *Resource) Update(context.Context, resource.UpdateRequest, *resource.UpdateResponse) {}

func (r *Resource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var m Model
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	_, status, err := r.client.Do(ctx, http.MethodDelete, "/v1/knowledge/"+url.PathEscape(m.ID.ValueString()), nil)
	if err != nil && status != http.StatusNotFound {
		resp.Diagnostics.AddError("Delete Knowledge Base failed", err.Error())
	}
}

func (r *Resource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("ready_timeout"), "5m")...)
}

var _ resource.Resource = (*Resource)(nil)
var _ resource.ResourceWithConfigure = (*Resource)(nil)
var _ resource.ResourceWithImportState = (*Resource)(nil)
