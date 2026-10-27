// Package generated stands in for the oapi-codegen client. Against v1:
// DeleteThing is gone; FetchThing, ArchiveThing and ListGadgets are new; and
// ListWidgets keeps its catalog entry but loses its entry point here.
package generated

import "context"

type ClientWithResponses struct{}

func (c *ClientWithResponses) GetThingWithResponse(ctx context.Context, id int64) error   { return nil }
func (c *ClientWithResponses) FetchThingWithResponse(ctx context.Context, id int64) error { return nil }
func (c *ClientWithResponses) ListThingsWithResponse(ctx context.Context) error           { return nil }
func (c *ClientWithResponses) CreateThingWithBodyWithResponse(ctx context.Context) error  { return nil }
func (c *ClientWithResponses) ArchiveThingWithResponse(ctx context.Context, id int64) error {
	return nil
}
func (c *ClientWithResponses) GetGadgetWithResponse(ctx context.Context, id int64) error { return nil }
func (c *ClientWithResponses) ListGadgetsWithResponse(ctx context.Context) error         { return nil }

func (c *ClientWithResponses) DeleteGadgetWithResponse(ctx context.Context, id int64) error {
	return nil
}

func NewClientWithResponses() *ClientWithResponses { return &ClientWithResponses{} }
