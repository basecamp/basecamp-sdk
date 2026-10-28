package basecamp

import "context"

type GadgetsService struct{ client *AccountClient }

func (s *GadgetsService) Get(ctx context.Context, id int64) error {
	return s.client.parent.gen.GetGadgetWithResponse(ctx, id)
}

func (s *GadgetsService) Delete(ctx context.Context, id int64) error {
	return s.client.parent.gen.DeleteGadgetWithResponse(ctx, id)
}
