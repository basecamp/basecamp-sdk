// Package cmds exercises each way a downstream reaches the SDK.
package cmds

import (
	"context"
	"fmt"
	"strconv"

	"github.com/basecamp/basecamp-sdk/go/pkg/basecamp"
	"github.com/basecamp/basecamp-sdk/go/pkg/generated"
)

// lister is a seam: ThingsService.List is called through it.
type lister interface {
	List(ctx context.Context) error
}

func list(ctx context.Context, l lister) error { return l.List(ctx) }

// remover reaches GadgetsService.Delete through an embedded interface, whose
// method is called with the embedded interface as its receiver.
type deleter interface {
	Delete(ctx context.Context, id int64) error
}

type remover interface{ deleter }

func remove(ctx context.Context, r remover) error { return r.Delete(ctx, 6) }

// count reaches ThingsService.Count through an interface written inline.
func count(ctx context.Context, c interface{ Count(context.Context) error }) error {
	return c.Count(ctx)
}

func Run(ctx context.Context, ac *basecamp.AccountClient, path string) {
	things := ac.Things()
	_ = things.Get(ctx, 1)  // a direct call
	create := things.Create // a method value
	_ = create(ctx)
	_ = ac.Things().Delete(ctx, 2)                                      // removed in v2
	_ = ac.Things().Summary(ctx, 3)                                     // a composite: not GetGadget
	_ = list(ctx, ac.Things())                                          // through a seam
	_ = remove(ctx, ac.Gadgets())                                       // through an embedded seam
	_ = count(ctx, ac.Things())                                         // through an inline seam
	_, _ = ac.Get(ctx, "/my/profile.json")                              // raw, literal
	_, _ = ac.Get(ctx, "/people/"+strconv.Itoa(4)+".json")              // raw, concatenated
	_, _ = ac.Get(ctx, fmt.Sprintf("/nowhere/%d.json", 5))              // raw, matches nothing
	_, _ = ac.Get(ctx, path)                                            // raw, computed
	_ = generated.NewClientWithResponses().ListWidgetsWithResponse(ctx) // the generated client, directly
}
