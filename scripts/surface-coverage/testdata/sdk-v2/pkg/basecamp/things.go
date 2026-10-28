package basecamp

import (
	"context"
	"fmt"
)

type ThingsService struct{ client *AccountClient }

// Get now reads through FetchThing; nothing calls GetThing any more.
func (s *ThingsService) Get(ctx context.Context, id int64) error {
	return s.client.parent.gen.FetchThingWithResponse(ctx, id)
}

func (s *ThingsService) List(ctx context.Context) error {
	return s.client.parent.gen.ListThingsWithResponse(ctx)
}

func (s *ThingsService) Create(ctx context.Context) error {
	return s.client.parent.gen.CreateThingWithBodyWithResponse(ctx)
}

func (s *ThingsService) Update(ctx context.Context, id int64) error {
	if err := s.Get(ctx, id); err != nil {
		return err
	}
	_, err := s.client.Put(ctx, fmt.Sprintf("/things/%d.json", id), nil)
	return err
}

func (s *ThingsService) Archive(ctx context.Context, id int64) error {
	return s.client.parent.gen.ArchiveThingWithResponse(ctx, id)
}

func (s *ThingsService) Summary(ctx context.Context, id int64) error {
	return s.client.Gadgets().Get(ctx, id)
}

func (s *ThingsService) Count(ctx context.Context) error {
	return s.client.parent.gen.CountThingsWithResponse(ctx)
}
