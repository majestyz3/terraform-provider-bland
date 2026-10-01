package agenttestscenario

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/majestyz3/terraform-provider-bland/internal/client"
)

type Model struct {
	ID           types.String `tfsdk:"id"`
	AgentID      types.String `tfsdk:"agent_id"`
	Name         types.String `tfsdk:"name"`
	Prompt       types.String `tfsdk:"prompt"`
	MaxTurns     types.Int64  `tfsdk:"max_turns"`
	Description  types.String `tfsdk:"description"`
	ExtraJSON    types.String `tfsdk:"extra_json"`
	ResponseJSON types.String `tfsdk:"response_json"`
}

type Resource struct{ client *client.Client }

func New() resource.Resource { return &Resource{} }

func (r *Resource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_agent_test_scenario"
}

func (r *Resource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a Bland Agent Testing scenario that targets a V2 Agent. The public scenario API uses agent_id for dashboard-created V2 Agent scenarios.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{Computed: true},
			"agent_id": schema.StringAttribute{
				Required:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"name": schema.StringAttribute{Required: true},
			"prompt": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Simulated caller instructions. Sent as tester_persona_prompt.",
			},
			"max_turns": schema.Int64Attribute{
				Optional:            true,
				Default:             int64default.StaticInt64(20),
				MarkdownDescription: "Maximum simulated conversation turns. Must be between 1 and 50.",
			},
			"description": schema.StringAttribute{Optional: true},
			"extra_json": schema.StringAttribute{
				Optional:            true,
				Default:             stringdefault.StaticString("{}"),
				MarkdownDescription: "Additional documented Agent Testing fields merged into the request body. Core fields managed by this resource take precedence.",
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

func scenarioData(out map[string]any) map[string]any {
	if data, ok := out["data"].(map[string]any); ok {
		return data
	}
	return out
}

func scenarioID(out map[string]any) string {
	data := scenarioData(out)
	id, _ := data["id"].(string)
	return id
}

func validateMaxTurns(v int64) error {
	if v < 1 || v > 50 {
		return fmt.Errorf("max_turns must be between 1 and 50")
	}
	return nil
}

func bodyFromModel(m Model) (map[string]any, error) {
	body := map[string]any{}
	if !m.ExtraJSON.IsNull() && !m.ExtraJSON.IsUnknown() && m.ExtraJSON.ValueString() != "" {
		if err := json.Unmarshal([]byte(m.ExtraJSON.ValueString()), &body); err != nil {
			return nil, fmt.Errorf("extra_json must be a JSON object: %w", err)
		}
	}
	if err := validateMaxTurns(m.MaxTurns.ValueInt64()); err != nil {
		return nil, err
	}

	body["agent_id"] = m.AgentID.ValueString()
	body["name"] = m.Name.ValueString()
	body["tester_persona_prompt"] = m.Prompt.ValueString()
	body["max_turns"] = m.MaxTurns.ValueInt64()
	body["scenario_type"] = "AGENT"
	if !m.Description.IsNull() && !m.Description.IsUnknown() {
		body["description"] = m.Description.ValueString()
	}
	return body, nil
}

func extraConfigFromResponse(data map[string]any) map[string]any {
	extra := map[string]any{}
	for _, key := range []string{
		"category", "tester_persona_name", "request_data", "start_node_id",
		"bland_tone_enabled", "is_required_for_promotion", "input_messages",
		"advanced_instructions", "metadata", "assertions",
	} {
		if value, ok := data[key]; ok && value != nil {
			extra[key] = value
		}
	}
	return extra
}

func compact(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}

func hydrate(m *Model, out map[string]any) error {
	data := scenarioData(out)
	id, _ := data["id"].(string)
	agentID, _ := data["agent_id"].(string)
	name, _ := data["name"].(string)
	prompt, _ := data["tester_persona_prompt"].(string)
	maxTurns, ok := data["max_turns"].(float64)
	if id == "" || agentID == "" || name == "" || prompt == "" || !ok {
		return fmt.Errorf("scenario response is missing id, agent_id, name, tester_persona_prompt, or max_turns")
	}
	m.ID = types.StringValue(id)
	m.AgentID = types.StringValue(agentID)
	m.Name = types.StringValue(name)
	m.Prompt = types.StringValue(prompt)
	m.MaxTurns = types.Int64Value(int64(maxTurns))
	if description, ok := data["description"].(string); ok {
		m.Description = types.StringValue(description)
	} else {
		m.Description = types.StringNull()
	}
	if m.ExtraJSON.IsNull() || m.ExtraJSON.IsUnknown() || m.ExtraJSON.ValueString() == "" {
		m.ExtraJSON = types.StringValue(compact(extraConfigFromResponse(data)))
	}
	m.ResponseJSON = types.StringValue(encode(out))
	return nil
}

func (r *Resource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var m Model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	body, err := bodyFromModel(m)
	if err != nil {
		resp.Diagnostics.AddError("Invalid Agent Test Scenario", err.Error())
		return
	}
	out, _, err := r.client.Do(ctx, http.MethodPost, "/v1/agent-testing/scenarios", body)
	if err != nil {
		resp.Diagnostics.AddError("Create Agent Test Scenario failed", err.Error())
		return
	}
	if err := hydrate(&m, out); err != nil {
		resp.Diagnostics.AddError("Invalid Agent Test Scenario response", err.Error()+": "+encode(out))
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
}

func (r *Resource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var m Model
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	out, status, err := r.client.Do(ctx, http.MethodGet, "/v1/agent-testing/scenarios/"+url.PathEscape(m.ID.ValueString()), nil)
	if status == http.StatusNotFound {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Read Agent Test Scenario failed", err.Error())
		return
	}
	if err := hydrate(&m, out); err != nil {
		resp.Diagnostics.AddError("Invalid Agent Test Scenario response", err.Error()+": "+encode(out))
		return
	}
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
	body, err := bodyFromModel(m)
	if err != nil {
		resp.Diagnostics.AddError("Invalid Agent Test Scenario", err.Error())
		return
	}
	out, _, err := r.client.Do(ctx, http.MethodPut, "/v1/agent-testing/scenarios/"+url.PathEscape(m.ID.ValueString()), body)
	if err != nil {
		resp.Diagnostics.AddError("Update Agent Test Scenario failed", err.Error())
		return
	}
	if err := hydrate(&m, out); err != nil {
		resp.Diagnostics.AddError("Invalid Agent Test Scenario response", err.Error()+": "+encode(out))
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
}

func (r *Resource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var m Model
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	_, status, err := r.client.Do(ctx, http.MethodDelete, "/v1/agent-testing/scenarios/"+url.PathEscape(m.ID.ValueString()), nil)
	if err != nil && status != http.StatusNotFound {
		resp.Diagnostics.AddError("Delete Agent Test Scenario failed", err.Error())
	}
}

func (r *Resource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

var _ resource.Resource = (*Resource)(nil)
var _ resource.ResourceWithConfigure = (*Resource)(nil)
var _ resource.ResourceWithImportState = (*Resource)(nil)
