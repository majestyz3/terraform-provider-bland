package alarms

import (
	"context"
	"fmt"
	"net/http"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/majestyz3/terraform-provider-bland/internal/client"
)

type AlarmModel struct {
	ID         types.String `tfsdk:"id"`
	Name       types.String `tfsdk:"name"`
	MetricType types.String `tfsdk:"metric_type"`
}

type Model struct {
	Alarms   []AlarmModel `tfsdk:"alarms"`
	NameToID types.Map    `tfsdk:"name_to_id"`
}

type DataSource struct{ client *client.Client }

func New() datasource.DataSource { return &DataSource{} }

func (d *DataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_alarms"
}

func (d *DataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Lists Bland alarm configurations visible through GET /v1/alarms.",
		Attributes: map[string]schema.Attribute{
			"alarms": schema.ListNestedAttribute{
				Computed: true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id":          schema.StringAttribute{Computed: true},
						"name":        schema.StringAttribute{Computed: true},
						"metric_type": schema.StringAttribute{Computed: true},
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

func alarmList(out map[string]any) []any {
	if data, ok := out["data"].([]any); ok {
		return data
	}
	if data, ok := out["data"].(map[string]any); ok {
		if alarms, ok := data["alarms"].([]any); ok {
			return alarms
		}
	}
	return nil
}

func parseAlarms(out map[string]any) ([]AlarmModel, map[string]string) {
	items := []AlarmModel{}
	nameMap := map[string]string{}
	for _, raw := range alarmList(out) {
		a, _ := raw.(map[string]any)
		if a == nil {
			continue
		}
		id, _ := a["id"].(string)
		metric, _ := a["metric_type"].(string)
		name, _ := a["name"].(string)
		if id == "" {
			continue
		}
		items = append(items, AlarmModel{
			ID:         types.StringValue(id),
			Name:       types.StringValue(name),
			MetricType: types.StringValue(metric),
		})
		if name != "" {
			nameMap[name] = id
		}
	}
	return items, nameMap
}

func (d *DataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var m Model
	out, _, err := d.client.Do(ctx, http.MethodGet, "/v1/alarms", nil)
	if err != nil {
		resp.Diagnostics.AddError("List Alarms failed", err.Error())
		return
	}
	m.Alarms, _ = parseAlarms(out)
	_, ids := parseAlarms(out)
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
