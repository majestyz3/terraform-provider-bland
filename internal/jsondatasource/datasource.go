package jsondatasource

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/majestyz3/terraform-provider-bland/internal/client"
)

type Spec struct {
	TypeName    string
	Description string
	Path        string
}

type Model struct {
	ID           types.String `tfsdk:"id"`
	ResponseJSON types.String `tfsdk:"response_json"`
}

type DataSource struct {
	spec   Spec
	client *client.Client
}

func New(spec Spec) func() datasource.DataSource {
	return func() datasource.DataSource { return &DataSource{spec: spec} }
}

func (d *DataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_" + d.spec.TypeName
}

func (d *DataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: d.spec.Description,
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Bland object identifier used to resolve this read-only API view.",
			},
			"response_json": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Full Bland API response encoded as JSON.",
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

func interpolate(path, id string) string {
	return strings.ReplaceAll(path, "{id}", id)
}

func (d *DataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config Model
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	out, status, err := d.client.Do(ctx, http.MethodGet, interpolate(d.spec.Path, config.ID.ValueString()), nil)
	if err != nil {
		if status == http.StatusNotFound {
			resp.Diagnostics.AddError("Bland object not found", "The requested object or read-only view does not exist.")
			return
		}
		resp.Diagnostics.AddError("Bland API read failed", err.Error())
		return
	}

	config.ResponseJSON = types.StringValue(encode(out))
	resp.Diagnostics.Append(resp.State.Set(ctx, &config)...)
}

var _ datasource.DataSource = (*DataSource)(nil)
var _ datasource.DataSourceWithConfigure = (*DataSource)(nil)
