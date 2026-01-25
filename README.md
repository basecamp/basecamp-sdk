# basecamp-sdk

Go SDK for the Basecamp 4 API.

## Installation

```bash
go get github.com/basecamp/basecamp-sdk
```

## Usage

```go
package main

import (
    "context"
    "fmt"
    "os"

    "github.com/basecamp/basecamp-sdk/pkg/basecamp"
)

func main() {
    // Create client with environment-based config
    cfg := basecamp.ConfigFromEnv()
    ts := basecamp.NewEnvTokenSource()
    client := basecamp.NewClient(cfg, ts)

    // List projects
    projects, err := client.Projects().List(context.Background(), nil)
    if err != nil {
        fmt.Fprintf(os.Stderr, "Error: %v\n", err)
        os.Exit(1)
    }

    for _, p := range projects {
        fmt.Printf("%d: %s\n", p.ID, p.Name)
    }
}
```

## Environment Variables

- `BASECAMP_ACCESS_TOKEN` - OAuth access token
- `BASECAMP_ACCOUNT_ID` - Default account ID
- `BASECAMP_PROJECT_ID` - Default project ID (optional)
- `BASECAMP_BASE_URL` - API base URL (default: `https://3.basecampapi.com`)

## License

MIT
