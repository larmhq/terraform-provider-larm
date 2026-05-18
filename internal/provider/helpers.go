package provider

import (
	"fmt"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"

	larmgo "github.com/larmhq/larm-go/client"
)

// clientFromProviderData type-asserts the provider data into the SDK client.
// All resources call this from Configure; nil data is treated as a no-op
// (the framework calls Configure once before provider Configure has run).
func clientFromProviderData(data any) (*larmgo.ClientWithResponses, diag.Diagnostics) {
	var diags diag.Diagnostics
	if data == nil {
		return nil, diags
	}
	client, ok := data.(*larmgo.ClientWithResponses)
	if !ok {
		diags.AddError(
			"Unexpected provider data type",
			fmt.Sprintf("Expected *larmgo.ClientWithResponses, got %T. This is a bug in terraform-provider-larm.", data),
		)
		return nil, diags
	}
	return client, diags
}

func optionalInt(p *int) types.Int64 {
	if p == nil {
		return types.Int64Null()
	}
	return types.Int64Value(int64(*p))
}

func optionalString(p *string) types.String {
	if p == nil {
		return types.StringNull()
	}
	return types.StringValue(*p)
}

func optionalTime(t *time.Time) types.String {
	if t == nil {
		return types.StringNull()
	}
	return types.StringValue(t.Format(time.RFC3339))
}

func formatTime(t time.Time) types.String {
	return types.StringValue(t.Format(time.RFC3339))
}

func responseError(status int, body []byte) string {
	return larmgo.ParseAPIError(status, body).Error()
}
