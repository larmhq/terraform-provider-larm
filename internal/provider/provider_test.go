package provider

import (
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
)

// testAccProtoV6ProviderFactories provides the provider for acceptance tests.
// Acceptance tests are gated by the TF_ACC environment variable and require
// a reachable Larm backend (LARM_API_KEY env var must be set).
var testAccProtoV6ProviderFactories = map[string]func() (tfprotov6.ProviderServer, error){
	"larm": providerserver.NewProtocol6WithError(New("test")()),
}
