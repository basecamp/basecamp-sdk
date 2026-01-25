# basecamp-sdk

Official SDKs for the Basecamp 4 API.

## Languages

| Language | Path | Status |
|----------|------|--------|
| Go | [`go/`](go/) | Active |
| Ruby | `ruby/` | Planned |
| TypeScript | `ts/` | Planned |

## Specification

The [`spec/`](spec/) directory contains the API specification in Smithy IDL format, which drives SDK generation and testing across all languages.

## Go SDK

### Installation

```bash
go get github.com/basecamp/basecamp-sdk/go
```

### Usage

```go
package main

import (
    "context"
    "fmt"
    "os"

    "github.com/basecamp/basecamp-sdk/go/pkg/basecamp"
)

func main() {
    cfg := basecamp.ConfigFromEnv()
    ts := basecamp.NewEnvTokenSource()
    client := basecamp.NewClient(cfg, ts)

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

### Environment Variables

- `BASECAMP_TOKEN` - OAuth access token
- `BASECAMP_ACCOUNT_ID` - Default account ID
- `BASECAMP_PROJECT_ID` - Default project ID (optional)
- `BASECAMP_BASE_URL` - API base URL (default: `https://3.basecampapi.com`)

## License

MIT
