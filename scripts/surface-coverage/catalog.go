package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

// Operation is one SDK operation, as the SDK's embedded catalog lists it.
type Operation struct {
	ID     string `json:"operationId"`
	Tag    string `json:"tag"`
	Method string `json:"method"`
	Path   string `json:"path"`
}

// catalogRelPath is where the SDK's Go module embeds its operation catalog,
// relative to the module root. The catalog is distilled from openapi.json by
// `make catalog` and gated byte-for-byte by `make catalog-check`, so its
// operation set is openapi.json's.
const catalogRelPath = "pkg/basecamp/catalog/catalog.json"

func readCatalog(moduleDir string) ([]Operation, error) {
	data, err := os.ReadFile(filepath.Join(moduleDir, catalogRelPath))
	if err != nil {
		return nil, fmt.Errorf("reading the SDK operation catalog: %w", err)
	}
	var doc struct {
		Operations []Operation `json:"operations"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", catalogRelPath, err)
	}
	if len(doc.Operations) == 0 {
		return nil, fmt.Errorf("%s lists no operations", catalogRelPath)
	}
	sort.Slice(doc.Operations, func(i, j int) bool { return doc.Operations[i].ID < doc.Operations[j].ID })
	return doc.Operations, nil
}
