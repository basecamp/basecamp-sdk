// Package generated stands in for the oapi-codegen client: one entry point
// per operation, spelled from its operationId.
package generated

import "context"

type ClientWithResponses struct{}

func (c *ClientWithResponses) GetThingWithResponse(ctx context.Context, id int64) error  { return nil }
func (c *ClientWithResponses) ListThingsWithResponse(ctx context.Context) error          { return nil }
func (c *ClientWithResponses) CreateThingWithBodyWithResponse(ctx context.Context) error { return nil }
func (c *ClientWithResponses) DeleteThingWithResponse(ctx context.Context, id int64) error {
	return nil
}
func (c *ClientWithResponses) GetGadgetWithResponse(ctx context.Context, id int64) error { return nil }

// NewClientWithResponses is not an operation.
func (c *ClientWithResponses) DeleteGadgetWithResponse(ctx context.Context, id int64) error {
	return nil
}

func NewClientWithResponses() *ClientWithResponses { return &ClientWithResponses{} }
