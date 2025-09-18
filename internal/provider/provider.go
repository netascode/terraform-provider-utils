package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/function"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/resource"
)

// provider satisfies the tfsdk.Provider interface and usually is included
// with all Resource and DataSource implementations.
type utilsProvider struct {
	// configured is set to true at the end of the Configure method.
	// This can be used in Resource and DataSource implementations to verify
	// that the provider was previously configured.
	configured bool

	// version is set to the provider version on release, "dev" when the
	// provider is built and ran locally, and "test" when running acceptance
	// testing.
	version string
}

// Metadata returns the provider type name.
func (p *utilsProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "utils"
}

func (p *utilsProvider) Schema(ctx context.Context, req provider.SchemaRequest, resp *provider.SchemaResponse) {
}

func (p *utilsProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	p.configured = true
}

func (p *utilsProvider) Resources(ctx context.Context) []func() resource.Resource {
	return nil
}

func (p *utilsProvider) DataSources(ctx context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		NewYamlMergeDataSource,
	}
}

func (p *utilsProvider) Functions(ctx context.Context) []func() function.Function {
	return []func() function.Function{
		NewYamlMergeFunction,
		NewNormalizeVlansFunction,
	}
}

func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &utilsProvider{
			version: version,
		}
	}
}
