package main

import (
	"context"

	"example.com/downstream/internal/cmds"
	"github.com/basecamp/basecamp-sdk/go/pkg/basecamp"
)

func main() {
	cmds.Run(context.Background(), basecamp.NewClient().ForAccount("1"), "/anything")
}
