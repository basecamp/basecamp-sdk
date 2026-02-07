package basecamp

import (
	_ "embed"
	"encoding/json"
)

//go:embed api-provenance.json
var provenanceJSON []byte

// APIProvenance describes which upstream API revision the SDK was built against.
type APIProvenance struct {
	BC3 UpstreamRef `json:"bc3"`
}

// UpstreamRef is a git revision and the date it was synced.
type UpstreamRef struct {
	Revision string `json:"revision"`
	Date     string `json:"date"`
}

// Provenance returns the upstream API revision this SDK was built against.
func Provenance() APIProvenance {
	var p APIProvenance
	_ = json.Unmarshal(provenanceJSON, &p)
	return p
}
