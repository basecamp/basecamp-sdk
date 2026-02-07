module github.com/basecamp/basecamp-sdk/conformance/runner/go

go 1.25.7

require github.com/basecamp/basecamp-sdk/go v0.0.0

require (
	github.com/apapsch/go-jsonmerge/v2 v2.0.0 // indirect
	github.com/google/uuid v1.5.0 // indirect
	github.com/oapi-codegen/runtime v1.1.2 // indirect
)

replace github.com/basecamp/basecamp-sdk/go => ../../../go
