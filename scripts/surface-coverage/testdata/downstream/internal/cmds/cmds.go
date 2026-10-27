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

// gadgetClient is an alias of a pointer to an SDK service.
type gadgetClient = *basecamp.GadgetsService

// getter is handed both a ThingsService and a GadgetsService, so a call
// through it could be either one's Get: it is credited to neither.
type getter interface {
	Get(ctx context.Context, id int64) error
}

func get(ctx context.Context, g getter) error { return g.Get(ctx, 9) }

// sprockets reaches the generated client's ListSprockets through a seam.
type sprockets interface {
	ListSprocketsWithResponse(ctx context.Context) error
}

func listSprockets(ctx context.Context, s sprockets) error { return s.ListSprocketsWithResponse(ctx) }

// count reaches ThingsService.Count through an interface written inline.
func count(ctx context.Context, c interface{ Count(context.Context) error }) error {
	return c.Count(ctx)
}

func Run(ctx context.Context, ac *basecamp.AccountClient, path string) {
	things := ac.Things()
	_ = things.Get(ctx, 1)  // a direct call
	create := things.Create // a method value
	_ = create(ctx)
	_ = ac.Things().Delete(ctx, 2)  // removed in v2
	_ = ac.Things().Summary(ctx, 3) // a composite: not GetGadget
	_ = list(ctx, ac.Things())      // through a seam
	var gadgets gadgetClient = ac.Gadgets()
	_ = remove(ctx, gadgets)                                            // through an embedded seam, as an alias
	_ = get(ctx, ac.Things())                                           // an ambiguous seam
	_ = get(ctx, ac.Gadgets())                                          // the same seam, another service
	_ = listSprockets(ctx, generated.NewClientWithResponses())          // the generated client, through a seam
	_ = count(ctx, ac.Things())                                         // through an inline seam
	_, _ = ac.Get(ctx, "/my/profile.json")                              // raw, literal
	_, _ = ac.Get(ctx, "/people/"+strconv.Itoa(4)+".json")              // raw, concatenated
	_, _ = ac.Get(ctx, fmt.Sprintf("/nowhere/%d.json", 5))              // raw, matches nothing
	_, _ = ac.Get(ctx, path)                                            // raw, computed
	_ = generated.NewClientWithResponses().ListWidgetsWithResponse(ctx) // the generated client, directly
}
