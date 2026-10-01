package provider

import (
	"context"
	"net/http"
	"os"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/majestyz3/terraform-provider-bland/internal/agent"
	"github.com/majestyz3/terraform-provider-bland/internal/agentbranch"
	"github.com/majestyz3/terraform-provider-bland/internal/agentchecks"
	"github.com/majestyz3/terraform-provider-bland/internal/agentdisposition"
	"github.com/majestyz3/terraform-provider-bland/internal/agentexperiment"
	"github.com/majestyz3/terraform-provider-bland/internal/agentidentity"
	"github.com/majestyz3/terraform-provider-bland/internal/agentinbound"
	"github.com/majestyz3/terraform-provider-bland/internal/agentvariable"
	"github.com/majestyz3/terraform-provider-bland/internal/agentversion"
	"github.com/majestyz3/terraform-provider-bland/internal/client"
	"github.com/majestyz3/terraform-provider-bland/internal/dispositionextractor"
	"github.com/majestyz3/terraform-provider-bland/internal/jsondatasource"
	"github.com/majestyz3/terraform-provider-bland/internal/jsonresource"
	"github.com/majestyz3/terraform-provider-bland/internal/tool"
	"github.com/majestyz3/terraform-provider-bland/internal/versionconfig"
)

type BlandProvider struct{ version string }

type ProviderModel struct {
	APIKey  types.String `tfsdk:"api_key"`
	BaseURL types.String `tfsdk:"base_url"`
}

func New(version string) func() provider.Provider {
	return func() provider.Provider { return &BlandProvider{version: version} }
}

func (p *BlandProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "bland"
	resp.Version = p.version
}

func (p *BlandProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Terraform provider for Bland AI public REST APIs.",
		Attributes: map[string]schema.Attribute{
			"api_key":  schema.StringAttribute{Optional: true, Sensitive: true, MarkdownDescription: "Bland API key; may also be set with BLAND_API_KEY."},
			"base_url": schema.StringAttribute{Optional: true, MarkdownDescription: "Bland API base URL; defaults to https://api.bland.ai."},
		},
	}
}

func (p *BlandProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var cfg ProviderModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}

	key := os.Getenv("BLAND_API_KEY")
	if !cfg.APIKey.IsNull() && cfg.APIKey.ValueString() != "" {
		key = cfg.APIKey.ValueString()
	}
	if key == "" {
		resp.Diagnostics.AddError("Missing Bland API key", "Set provider api_key or BLAND_API_KEY.")
		return
	}

	base := os.Getenv("BLAND_BASE_URL")
	if !cfg.BaseURL.IsNull() && cfg.BaseURL.ValueString() != "" {
		base = cfg.BaseURL.ValueString()
	}

	c := client.New(base, key)
	resp.ResourceData = c
	resp.DataSourceData = c
}

func spec(typeName, description, createMethod, createPath, readMethod, readPath, updateMethod, updatePath, deleteMethod, deletePath string, idPaths ...string) func() resource.Resource {
	return jsonresource.New(jsonresource.Spec{
		TypeName: typeName, Description: description,
		CreateMethod: createMethod, CreatePath: createPath,
		ReadMethod: readMethod, ReadPath: readPath,
		UpdateMethod: updateMethod, UpdatePath: updatePath,
		DeleteMethod: deleteMethod, DeletePath: deletePath,
		IDPaths: idPaths,
	})
}

func pathwaySpec() func() resource.Resource {
	return jsonresource.New(jsonresource.Spec{
		TypeName:     "conversational_pathway",
		Description:  "Manages a Bland Conversational Pathway.",
		CreateMethod: http.MethodPost, CreatePath: "/v1/pathway/create",
		ReadMethod: http.MethodGet, ReadPath: "/v1/pathway/{id}",
		UpdateMethod: http.MethodPost, UpdatePath: "/convo_pathway/update",
		DeleteMethod: http.MethodDelete, DeletePath: "/v1/pathway/{id}",
		IDPaths:          []string{"data.id", "data.pathway_id", "pathway_id"},
		CreateOnlyKeys:   []string{"name", "description"},
		PostCreateUpdate: true,
	})
}

func (p *BlandProvider) Resources(_ context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		func() resource.Resource { return agent.New() },
		func() resource.Resource { return agentbranch.New() },
		func() resource.Resource { return agentdisposition.New() },
		func() resource.Resource { return agentexperiment.New() },
		func() resource.Resource { return agentidentity.New() },
		func() resource.Resource { return agentinbound.New() },
		func() resource.Resource { return agentversion.New() },
		func() resource.Resource { return agentvariable.New() },
		func() resource.Resource { return agentchecks.New() },
		func() resource.Resource { return dispositionextractor.New() },
		func() resource.Resource { return tool.New() },

		spec("persona", "Manages a Bland Persona.", http.MethodPost, "/v1/personas", http.MethodGet, "/v1/personas/{id}", http.MethodPatch, "/v1/personas/{id}", http.MethodDelete, "/v1/personas/{id}", "data.id"),
		pathwaySpec(),
		jsonresource.New(jsonresource.Spec{TypeName: "knowledge_base", Description: "Manages a text-backed Bland Knowledge Base item.", CreateMethod: http.MethodPost, CreatePath: "/v1/knowledge/learn", ReadMethod: http.MethodGet, ReadPath: "/v1/knowledge/{id}", DeleteMethod: http.MethodDelete, DeletePath: "/v1/knowledge/{id}", IDPaths: []string{"data.id"}, Immutable: true}),
		spec("guard_rail", "Manages a Bland Guard Rail.", http.MethodPost, "/v1/guard_rails", http.MethodGet, "/v1/guard_rails/{id}", http.MethodPatch, "/v1/guard_rails/{id}", http.MethodDelete, "/v1/guard_rails/{id}", "data.id"),
		spec("test_scenario", "Manages a Bland Agent Testing scenario.", http.MethodPost, "/v1/agent-testing/scenarios", http.MethodGet, "/v1/agent-testing/scenarios/{id}", http.MethodPut, "/v1/agent-testing/scenarios/{id}", http.MethodDelete, "/v1/agent-testing/scenarios/{id}", "data.id", "id"),
		spec("eval_agent", "Manages a Bland Eval Agent.", http.MethodPost, "/v1/evals/agents", http.MethodGet, "/v1/evals/agents/{id}", http.MethodPatch, "/v1/evals/agents/{id}", http.MethodDelete, "/v1/evals/agents/{id}", "data.agent.id", "data.id", "id"),
		spec("eval_workbench", "Manages a Bland Eval Workbench.", http.MethodPost, "/v1/evals/workbench-setups", http.MethodGet, "/v1/evals/workbench-setups/{id}", http.MethodPatch, "/v1/evals/workbench-setups/{id}", http.MethodDelete, "/v1/evals/workbench-setups/{id}", "data.setup.id", "data.id", "id"),
		spec("alarm", "Manages a Bland Alarm.", http.MethodPost, "/v1/alarms", http.MethodGet, "/v1/alarms/{id}", http.MethodPatch, "/v1/alarms/{id}", http.MethodDelete, "/v1/alarms/{id}", "data.alarm.id", "data.id", "id"),

		versionconfig.New(versionconfig.Spec{TypeName: "eval_agent_version_config", Description: "Configures an editable Bland Eval Agent version.", ReadPath: "/v1/evals/agents/{parent_id}/versions/{version_id}", PatchPath: "/v1/evals/agents/{parent_id}/versions/{version_id}"}),
		versionconfig.New(versionconfig.Spec{TypeName: "eval_workbench_version_config", Description: "Configures an editable Bland Eval Workbench version.", ReadPath: "/v1/evals/workbench-setups/{parent_id}/versions/{version_id}", PatchPath: "/v1/evals/workbench-setups/{parent_id}/versions/{version_id}"}),
	}
}

func (p *BlandProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		jsondatasource.New(jsondatasource.Spec{TypeName: "agent_info", Description: "Reads a Bland V2 Agent and its environment pointers.", Path: "/v2/agents/{id}"}),
		jsondatasource.New(jsondatasource.Spec{TypeName: "agent_environments", Description: "Reads a Bland V2 Agent's dev, staging, and production environment pins.", Path: "/v2/agents/{id}/environments"}),
		jsondatasource.New(jsondatasource.Spec{TypeName: "agent_inbound", Description: "Reads inbound phone-number bindings for a Bland V2 Agent.", Path: "/v2/agents/{id}/inbound"}),
		jsondatasource.New(jsondatasource.Spec{TypeName: "agent_memory_schema", Description: "Reads the schema Bland uses to remember structured information about contacts between conversations.", Path: "/v2/agents/{id}/memory-schema"}),
	}
}

var _ provider.Provider = (*BlandProvider)(nil)
