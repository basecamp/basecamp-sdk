// Package generate provides code generation directives for the Basecamp SDK.
//
// Run `go generate ./...` from the go directory to regenerate the client code.
//
//go:generate go tool oapi-codegen -config oapi-codegen.yaml ../openapi.json
//go:generate ../scripts/normalize-go-deprecation-godoc.sh pkg/generated/client.gen.go
package generate
