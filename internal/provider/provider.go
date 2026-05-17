// Package provider implements the Larm Terraform provider.
package provider

import (
	"context"
	"os"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"

	larmgo "github.com/larmhq/larm-go/client"
)

const defaultEndpoint = "https://app.larm.dev/api/v1"

// larmProvider implements [provider.Provider].
type larmProvider struct {
	version string
}

// New returns a constructor for the provider, used by main.go and tests.
func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &larmProvider{version: version}
	}
}

type providerModel struct {
	Endpoint types.String `tfsdk:"endpoint"`
	APIKey   types.String `tfsdk:"api_key"`
}

func (p *larmProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "larm"
	resp.Version = p.version
}

func (p *larmProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Official Terraform provider for [Larm](https://larm.dev) — uptime monitoring and status pages.",
		Attributes: map[string]schema.Attribute{
			"endpoint": schema.StringAttribute{
				MarkdownDescription: "Base URL of the Larm API. Defaults to `https://app.larm.dev/api/v1`. Can also be set via the `LARM_ENDPOINT` environment variable.",
				Optional:            true,
			},
			"api_key": schema.StringAttribute{
				MarkdownDescription: "API key for the Larm API. Create one in Settings > API Keys. Can also be set via the `LARM_API_KEY` environment variable.",
				Optional:            true,
				Sensitive:           true,
			},
		},
	}
}

func (p *larmProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var data providerModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	endpoint := os.Getenv("LARM_ENDPOINT")
	if !data.Endpoint.IsNull() && !data.Endpoint.IsUnknown() {
		endpoint = data.Endpoint.ValueString()
	}
	if endpoint == "" {
		endpoint = defaultEndpoint
	}

	apiKey := os.Getenv("LARM_API_KEY")
	if !data.APIKey.IsNull() && !data.APIKey.IsUnknown() {
		apiKey = data.APIKey.ValueString()
	}
	if apiKey == "" {
		resp.Diagnostics.AddAttributeError(
			path.Root("api_key"),
			"Missing API key",
			"The Larm provider requires an API key. Set it on the provider block via `api_key` or via the LARM_API_KEY environment variable.",
		)
		return
	}

	client, err := larmgo.New(endpoint,
		larmgo.WithToken(apiKey),
		larmgo.WithUserAgent("terraform-provider-larm/"+p.version),
	)
	if err != nil {
		resp.Diagnostics.AddError("Failed to construct Larm API client", err.Error())
		return
	}

	resp.ResourceData = client
	resp.DataSourceData = client
}

func (p *larmProvider) Resources(_ context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewMonitorResource,
		NewAlertChannelResource,
	}
}

func (p *larmProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return nil
}
