package agentbranch

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
	Name         types.String `tfsdk:"name"`
	ResponseJSON types.String `tfsdk:"response_json"`
}

type Resource struct{ client *client.Client }

func New() resource.Resource { return &Resource{} }

func (r *Resource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_agent_branch"
}

func (r *Resource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	replace := []planmodifier.String{stringplanmodifier.RequiresReplace()}
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a Bland V2 Agent development branch. Branches are created from the current dev head and are immutable by name; deleting the Terraform resource abandons the open branch.",
		Attributes: map[string]schema.Attribute{
			"id":            schema.StringAttribute{Computed: true},
			"agent_id":      schema.StringAttribute{Required: true, PlanModifiers: replace},
			"name":          schema.StringAttribute{Required: true, PlanModifiers: replace},
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

func branchID(out map[string]any) string {
	if d, ok := out["data"].(map[string]any); ok {
		id, _ := d["id"].(string)
		return id
	}
	return ""
}

func findBranch(out map[string]any, id string) (map[string]any, bool) {
	data, ok := out["data"].([]any)
	if !ok {
		return nil, false
	}
	for _, item := range data {
		branch, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if branchID, _ := branch["id"].(string); branchID == id {
			return branch, true
		}
	}
	return nil, false
}

func (r *Resource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var m Model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	out, _, err := r.client.Do(ctx, http.MethodPost, "/v2/agents/"+url.PathEscape(m.AgentID.ValueString())+"/branches", map[string]any{
		"name": m.Name.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Create Agent Branch failed", err.Error())
		return
	}
	id := branchID(out)
	if id == "" {
		resp.Diagnostics.AddError("Missing Agent Branch ID", encode(out))
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
	out, status, err := r.client.Do(ctx, http.MethodGet, "/v2/agents/"+url.PathEscape(m.AgentID.ValueString())+"/branches", nil)
	if status == http.StatusNotFound {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("List Agent Branches failed", err.Error())
		return
	}
	branch, ok := findBranch(out, m.ID.ValueString())
	if !ok {
		resp.State.RemoveResource(ctx)
		return
	}
	if name, ok := branch["name"].(string); ok {
		m.Name = types.StringValue(name)
	}
	m.ResponseJSON = types.StringValue(encode(map[string]any{"data": branch, "errors": nil}))
	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
}

func (r *Resource) Update(context.Context, resource.UpdateRequest, *resource.UpdateResponse) {}

func (r *Resource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var m Model
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	_, status, err := r.client.Do(ctx, http.MethodDelete, "/v2/agents/"+url.PathEscape(m.AgentID.ValueString())+"/branches/"+url.PathEscape(m.ID.ValueString()), nil)
	if err != nil && status != http.StatusNotFound {
		resp.Diagnostics.AddError("Delete Agent Branch failed", err.Error())
	}
}

func (r *Resource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := splitImportID(req.ID)
	if len(parts) != 2 {
		resp.Diagnostics.AddError("Invalid import ID", "Use agent_id/branch_id.")
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("agent_id"), parts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), parts[1])...)
}

func splitImportID(id string) []string {
	left, right, ok := strings.Cut(id, "/")
	if !ok || left == "" || right == "" || strings.Contains(right, "/") {
		return nil
	}
	return []string{left, right}
}

var _ resource.Resource = (*Resource)(nil)
var _ resource.ResourceWithConfigure = (*Resource)(nil)
var _ resource.ResourceWithImportState = (*Resource)(nil)
