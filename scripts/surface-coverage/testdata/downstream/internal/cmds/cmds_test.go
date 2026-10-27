package cmds

import (
	"context"
	"testing"

	"github.com/basecamp/basecamp-sdk/go/pkg/basecamp"
)

// Tests are not coverage: this call must not count.
func TestGadgets(t *testing.T) {
	_ = basecamp.NewClient().ForAccount("1").Gadgets().Get(context.Background(), 1)
}
