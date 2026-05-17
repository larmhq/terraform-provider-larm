package provider

import (
	"time"

	"github.com/hashicorp/terraform-plugin-framework/types"

	larmgo "github.com/larmhq/larm-go/client"
)

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
