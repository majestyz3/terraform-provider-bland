package dispositionextractor

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
	ID           types.String `tfsdk:"id"`
	AgentID      types.String `tfsdk:"agent_id"`
	ConfigJSON   types.String `tfsdk:"config_json"`
	ResponseJSON types.String `tfsdk:"response_json"`
}

type Resource struct{ client *client.Client }

func New() resource.Resource { return &Resource{} }

func (r *Resource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_disposition_extractor"
}

func (r *Resource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a Bland V2 disposition extractor and its editable draft. Publishing is an explicit release action and is not performed implicitly by Terraform.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{Computed: true},
			"agent_id": schema.StringAttribute{
				Required:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"config_json": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Extractor creation/draft JSON. Prefer jsonencode({...}) so newly-added Bland fields remain usable without waiting for a provider release.",
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

func decode(s string) (map[string]any, error) {
	var m map[string]any
	if err := json.Unmarshal([]byte(s), &m); err != nil {
		return nil, fmt.Errorf("config_json must be valid JSON: %w", err)
	}
	if len(m) == 0 {
		return nil, fmt.Errorf("config_json must not be empty")
	}
	return m, nil
}

func encode(v any) string {
	b, _ := json.MarshalIndent(v, "", "  ")
	return string(b)
}

func extractorID(out map[string]any) string {
	candidates := []any{out["data"], out}
	for _, candidate := range candidates {
		m, ok := candidate.(map[string]any)
		if !ok {
			continue
		}
		if id, _ := m["id"].(string); id != "" {
			return id
		}
		if ex, ok := m["extractor"].(map[string]any); ok {
			if id, _ := ex["id"].(string); id != "" {
				return id
			}
		}
	}
	return ""
}

func extractorPath(agentID, extractorID string) string {
	return "/v2/agents/" + url.PathEscape(agentID) + "/dispositions/extractors/" + url.PathEscape(extractorID)
}

func draftBody(body map[string]any) map[string]any {
	if draft, ok := body["draft"].(map[string]any); ok {
		return draft
	}
	if definition, ok := body["definition"].(map[string]any); ok {
		return map[string]any{"definition": definition}
	}
	return body
}

func (r *Resource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var m Model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	body, err := decode(m.ConfigJSON.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid extractor configuration", err.Error())
		return
	}
	p := "/v2/agents/" + url.PathEscape(m.AgentID.ValueString()) + "/dispositions/extractors"
	out, _, err := r.client.Do(ctx, http.MethodPost, p, body)
	if err != nil {
		resp.Diagnostics.AddError("Create Disposition Extractor failed", err.Error())
		return
	}
	id := extractorID(out)
	if id == "" {
		resp.Diagnostics.AddError("Missing Extractor ID", encode(out))
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
	out, status, err := r.client.Do(ctx, http.MethodGet, extractorPath(m.AgentID.ValueString(), m.ID.ValueString()), nil)
	if status == http.StatusNotFound {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Read Disposition Extractor failed", err.Error())
		return
	}
	m.ResponseJSON = types.StringValue(encode(out))
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
	body, err := decode(planned.ConfigJSON.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid extractor configuration", err.Error())
		return
	}
	p := extractorPath(planned.AgentID.ValueString(), planned.ID.ValueString()) + "/draft"
	out, _, err := r.client.Do(ctx, http.MethodPatch, p, draftBody(body))
	if err != nil {
		resp.Diagnostics.AddError("Update Disposition Extractor Draft failed", err.Error())
		return
	}
	planned.ResponseJSON = types.StringValue(encode(out))
	resp.Diagnostics.Append(resp.State.Set(ctx, &planned)...)
}

func (r *Resource) Delete(_ context.Context, _ resource.DeleteRequest, resp *resource.DeleteResponse) {
	resp.Diagnostics.AddWarning(
		"Disposition Extractor retained",
		"Bland's documented V2 extractor API has no delete endpoint. Terraform forgets the extractor while Bland retains its draft/published history.",
	)
}

func (r *Resource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.Split(req.ID, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		resp.Diagnostics.AddError("Invalid import ID", "Use agent_id/extractor_id.")
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("agent_id"), parts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), parts[1])...)
}

var _ resource.Resource = (*Resource)(nil)
var _ resource.ResourceWithConfigure = (*Resource)(nil)
var _ resource.ResourceWithImportState = (*Resource)(nil)
