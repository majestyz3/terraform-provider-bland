package agenttestscenarios

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/majestyz3/terraform-provider-bland/internal/client"
)

type ScenarioModel struct {
	ID   types.String `tfsdk:"id"`
	Name types.String `tfsdk:"name"`
}

type Model struct {
	AgentID    types.String    `tfsdk:"agent_id"`
	NamePrefix types.String    `tfsdk:"name_prefix"`
	Scenarios  []ScenarioModel `tfsdk:"scenarios"`
	NameToID   types.Map       `tfsdk:"name_to_id"`
}

type DataSource struct{ client *client.Client }

func New() datasource.DataSource { return &DataSource{} }

func (d *DataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_agent_test_scenarios"
}

func (d *DataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Lists Agent Testing scenarios that target one Bland V2 Agent and optionally filters by name prefix.",
		Attributes: map[string]schema.Attribute{
			"agent_id": schema.StringAttribute{Required: true},
			"name_prefix": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Optional case-sensitive scenario name prefix.",
			},
			"scenarios": schema.ListNestedAttribute{
				Computed: true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id":   schema.StringAttribute{Computed: true},
						"name": schema.StringAttribute{Computed: true},
					},
				},
			},
			"name_to_id": schema.MapAttribute{
				Computed:    true,
				ElementType: types.StringType,
			},
		},
	}
}

func (d *DataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	c, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError("Unexpected provider data", fmt.Sprintf("got %T", req.ProviderData))
		return
	}
	d.client = c
}

func scenarioList(out map[string]any) []any {
	if data, ok := out["data"].([]any); ok {
		return data
	}
	if data, ok := out["data"].(map[string]any); ok {
		if scenarios, ok := data["scenarios"].([]any); ok {
			return scenarios
		}
	}
	return nil
}

func filterScenarios(out map[string]any, agentID, prefix string) ([]ScenarioModel, map[string]string) {
	items := []ScenarioModel{}
	ids := map[string]string{}
	for _, raw := range scenarioList(out) {
		s, _ := raw.(map[string]any)
		if s == nil {
			continue
		}
		target, _ := s["agent_id"].(string)
		if target != agentID {
			continue
		}
		id, _ := s["id"].(string)
		name, _ := s["name"].(string)
		if id == "" || name == "" || (prefix != "" && !strings.HasPrefix(name, prefix)) {
			continue
		}
		items = append(items, ScenarioModel{ID: types.StringValue(id), Name: types.StringValue(name)})
		ids[name] = id
	}
	return items, ids
}

func (d *DataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var m Model
	resp.Diagnostics.Append(req.Config.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}

	out, _, err := d.client.Do(ctx, http.MethodGet, "/v1/agent-testing/scenarios", nil)
	if err != nil {
		resp.Diagnostics.AddError("List Agent Test Scenarios failed", err.Error())
		return
	}

	prefix := ""
	if !m.NamePrefix.IsNull() && !m.NamePrefix.IsUnknown() {
		prefix = m.NamePrefix.ValueString()
	}
	m.Scenarios, _ = filterScenarios(out, m.AgentID.ValueString(), prefix)
	_, ids := filterScenarios(out, m.AgentID.ValueString(), prefix)
	nameMap, diags := types.MapValueFrom(ctx, types.StringType, ids)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	m.NameToID = nameMap
	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
}

var _ datasource.DataSource = (*DataSource)(nil)
var _ datasource.DataSourceWithConfigure = (*DataSource)(nil)
