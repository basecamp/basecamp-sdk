// Package unused is not imported by cmd/app, so it is not shipped.
package unused

import (
	"context"

	"github.com/basecamp/basecamp-sdk/go/pkg/basecamp"
)

func Update(ctx context.Context, ac *basecamp.AccountClient) error {
	return ac.Things().Update(ctx, 1)
}
