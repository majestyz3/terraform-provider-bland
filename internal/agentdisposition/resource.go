package agentdisposition

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/majestyz3/terraform-provider-bland/internal/client"
)

type Model struct {
	ID             types.String `tfsdk:"id"`
	AgentID        types.String `tfsdk:"agent_id"`
	Key            types.String `tfsdk:"key"`
	DefinitionJSON types.String `tfsdk:"definition_json"`
	ResponseJSON   types.String `tfsdk:"response_json"`
}

type Resource struct{ client *client.Client }

func New() resource.Resource { return &Resource{} }

func (r *Resource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_agent_disposition"
}

func (r *Resource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	replace := []planmodifier.String{stringplanmodifier.RequiresReplace()}
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a Bland V2 Agent disposition and its editable draft definition. Publishing remains an explicit release action outside Terraform.",
		Attributes: map[string]schema.Attribute{
			"id":              schema.StringAttribute{Computed: true},
			"agent_id":        schema.StringAttribute{Required: true, PlanModifiers: replace},
			"key":             schema.StringAttribute{Required: true, PlanModifiers: replace, MarkdownDescription: "Stable disposition key. Changing it replaces the disposition."},
			"definition_json": schema.StringAttribute{Required: true, MarkdownDescription: "Disposition draft definition JSON. Prefer jsonencode({...})."},
			"response_json":   schema.StringAttribute{Computed: true},
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

func decodeDefinition(s string) (map[string]any, error) {
	var m map[string]any
	if err := json.Unmarshal([]byte(s), &m); err != nil {
		return nil, fmt.Errorf("definition_json must be valid JSON: %w", err)
	}
	if len(m) == 0 {
		return nil, fmt.Errorf("definition_json must not be empty")
	}
	return m, nil
}

func encode(v any) string {
	b, _ := json.MarshalIndent(v, "", "  ")
	return string(b)
}

func dispositionID(out map[string]any) string {
	if d, ok := out["data"].(map[string]any); ok {
		if id, _ := d["id"].(string); id != "" {
			return id
		}
		if disposition, ok := d["disposition"].(map[string]any); ok {
			id, _ := disposition["id"].(string)
			return id
		}
	}
	if id, _ := out["id"].(string); id != "" {
		return id
	}
	return ""
}

func dispositionKey(out map[string]any) string {
	if d, ok := out["data"].(map[string]any); ok {
		if key, _ := d["key"].(string); key != "" {
			return key
		}
		if disposition, ok := d["disposition"].(map[string]any); ok {
			key, _ := disposition["key"].(string)
			return key
		}
	}
	return ""
}

func draftDefinition(out map[string]any) (map[string]any, bool) {
	d, ok := out["data"].(map[string]any)
	if !ok {
		return nil, false
	}
	if definition, ok := d["definition"].(map[string]any); ok {
		return definition, true
	}
	if draft, ok := d["draft"].(map[string]any); ok {
		if definition, ok := draft["definition"].(map[string]any); ok {
			return definition, true
		}
	}
	return nil, false
}

func basePath(agentID, dispositionID string) string {
	return "/v2/agents/" + url.PathEscape(agentID) + "/dispositions/" + url.PathEscape(dispositionID)
}

func (r *Resource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var m Model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	definition, err := decodeDefinition(m.DefinitionJSON.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid disposition definition", err.Error())
		return
	}
	body := map[string]any{"key": m.Key.ValueString(), "definition": definition}
	out, _, err := r.client.Do(ctx, http.MethodPost, "/v2/agents/"+url.PathEscape(m.AgentID.ValueString())+"/dispositions", body)
	if err != nil {
		resp.Diagnostics.AddError("Create Agent Disposition failed", err.Error())
		return
	}
	id := dispositionID(out)
	if id == "" {
		resp.Diagnostics.AddError("Missing Disposition ID", encode(out))
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
	p := basePath(m.AgentID.ValueString(), m.ID.ValueString())
	summary, status, err := r.client.Do(ctx, http.MethodGet, p, nil)
	if status == http.StatusNotFound {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Read Agent Disposition failed", err.Error())
		return
	}
	if key := dispositionKey(summary); key != "" {
		m.Key = types.StringValue(key)
	}

	draft, draftStatus, draftErr := r.client.Do(ctx, http.MethodGet, p+"/draft", nil)
	if draftErr == nil {
		if definition, ok := draftDefinition(draft); ok {
			m.DefinitionJSON = types.StringValue(encode(definition))
		}
	} else if draftStatus != http.StatusNotFound {
		resp.Diagnostics.AddError("Read Disposition Draft failed", draftErr.Error())
		return
	}

	m.ResponseJSON = types.StringValue(encode(map[string]any{"summary": summary, "draft": draft}))
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
	definition, err := decodeDefinition(planned.DefinitionJSON.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid disposition definition", err.Error())
		return
	}
	out, _, err := r.client.Do(ctx, http.MethodPatch, basePath(planned.AgentID.ValueString(), planned.ID.ValueString())+"/draft", map[string]any{"definition": definition})
	if err != nil {
		resp.Diagnostics.AddError("Update Disposition Draft failed", err.Error())
		return
	}
	planned.ResponseJSON = types.StringValue(encode(out))
	resp.Diagnostics.Append(resp.State.Set(ctx, &planned)...)
}

func (r *Resource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var m Model
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	_, status, err := r.client.Do(ctx, http.MethodDelete, basePath(m.AgentID.ValueString(), m.ID.ValueString()), nil)
	if err != nil && status != http.StatusNotFound {
		resp.Diagnostics.AddError("Delete Agent Disposition failed", err.Error())
	}
}

func (r *Resource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.Split(req.ID, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		resp.Diagnostics.AddError("Invalid import ID", "Use agent_id/disposition_id.")
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("agent_id"), parts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), parts[1])...)
}

var _ resource.Resource = (*Resource)(nil)
var _ resource.ResourceWithConfigure = (*Resource)(nil)
var _ resource.ResourceWithImportState = (*Resource)(nil)
