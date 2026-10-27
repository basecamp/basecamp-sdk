package basecamp

import (
	"context"

	"github.com/basecamp/basecamp-sdk/go/pkg/generated"
)

type Response struct{}

type Client struct {
	gen *generated.ClientWithResponses
}

type AccountClient struct{ parent *Client }

func NewClient() *Client { return &Client{gen: generated.NewClientWithResponses()} }

func (c *Client) ForAccount(id string) *AccountClient { return &AccountClient{parent: c} }

func (c *Client) Get(ctx context.Context, path string) (*Response, error)         { return nil, nil }
func (ac *AccountClient) Get(ctx context.Context, path string) (*Response, error) { return nil, nil }
func (ac *AccountClient) Put(ctx context.Context, path string, body any) (*Response, error) {
	return nil, nil
}

func (ac *AccountClient) Things() *ThingsService   { return &ThingsService{client: ac} }
func (ac *AccountClient) Gadgets() *GadgetsService { return &GadgetsService{client: ac} }
