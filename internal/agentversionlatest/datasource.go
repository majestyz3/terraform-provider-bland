package agentversionlatest

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/majestyz3/terraform-provider-bland/internal/client"
)

type Model struct {
	AgentID      types.String `tfsdk:"agent_id"`
	ID           types.String `tfsdk:"id"`
	SnapshotJSON types.String `tfsdk:"snapshot_json"`
	ResponseJSON types.String `tfsdk:"response_json"`
}

type DataSource struct{ client *client.Client }

func New() datasource.DataSource { return &DataSource{} }

func (d *DataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_agent_version_latest"
}

func (d *DataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Exports the latest saved Bland V2 Agent version, including its normalized snapshot JSON.",
		Attributes: map[string]schema.Attribute{
			"agent_id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Bland V2 Agent ID.",
			},
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Latest saved Agent Version ID.",
			},
			"snapshot_json": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Normalized JSON for the version snapshot, suitable for checking into a configuration repository.",
			},
			"response_json": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Full Bland API response encoded as normalized JSON.",
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

func encode(v any) string {
	b, _ := json.MarshalIndent(v, "", "  ")
	return string(b)
}

func (d *DataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var m Model
	resp.Diagnostics.Append(req.Config.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}

	out, _, err := d.client.Do(ctx, http.MethodGet, "/v2/agents/"+url.PathEscape(m.AgentID.ValueString())+"/versions/latest", nil)
	if err != nil {
		resp.Diagnostics.AddError("Read Latest Agent Version failed", err.Error())
		return
	}

	data, ok := out["data"].(map[string]any)
	if !ok {
		resp.Diagnostics.AddError("Missing Agent Version data", encode(out))
		return
	}
	id, _ := data["id"].(string)
	if id == "" {
		resp.Diagnostics.AddError("Missing Agent Version ID", encode(out))
		return
	}
	snapshot, ok := data["snapshot"].(map[string]any)
	if !ok {
		resp.Diagnostics.AddError("Missing Agent Version snapshot", encode(out))
		return
	}

	m.ID = types.StringValue(id)
	m.SnapshotJSON = types.StringValue(encode(snapshot))
	m.ResponseJSON = types.StringValue(encode(out))
	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
}

var _ datasource.DataSource = (*DataSource)(nil)
var _ datasource.DataSourceWithConfigure = (*DataSource)(nil)
