package basecamp

import (
	"context"
	"fmt"
)

type ThingsService struct{ client *AccountClient }

func (s *ThingsService) Get(ctx context.Context, id int64) error {
	return s.client.parent.gen.GetThingWithResponse(ctx, id)
}

func (s *ThingsService) List(ctx context.Context) error {
	return s.client.parent.gen.ListThingsWithResponse(ctx)
}

func (s *ThingsService) Create(ctx context.Context) error {
	return s.client.parent.gen.CreateThingWithBodyWithResponse(ctx)
}

// Update reads, then writes through a raw PUT, so it performs GetThing and
// ReplaceThing.
func (s *ThingsService) Update(ctx context.Context, id int64) error {
	if err := s.Get(ctx, id); err != nil {
		return err
	}
	return s.replace(ctx, id)
}

func (s *ThingsService) replace(ctx context.Context, id int64) error {
	_, err := s.client.Put(ctx, fmt.Sprintf("/things/%d.json", id), nil)
	return err
}

func (s *ThingsService) Delete(ctx context.Context, id int64) error {
	return s.client.parent.gen.DeleteThingWithResponse(ctx, id)
}

// Summary is a composite: it reads a gadget through GadgetsService, which
// does not make GetGadget a Things operation.
func (s *ThingsService) Summary(ctx context.Context, id int64) error {
	return s.client.Gadgets().Get(ctx, id)
}

func (s *ThingsService) Count(ctx context.Context) error {
	return s.client.parent.gen.CountThingsWithResponse(ctx)
}
