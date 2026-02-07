package basecamp

import (
	"regexp"
	"testing"
)

var reSHA = regexp.MustCompile(`^[0-9a-f]{40}$`)

func TestProvenanceBC3(t *testing.T) {
	p := Provenance()
	if !reSHA.MatchString(p.BC3.Revision) {
		t.Errorf("BC3.Revision = %q, want 40-hex SHA", p.BC3.Revision)
	}
	if p.BC3.Date == "" {
		t.Error("BC3.Date is empty")
	}
}

func TestProvenanceBC3API(t *testing.T) {
	p := Provenance()
	if !reSHA.MatchString(p.BC3API.Revision) {
		t.Errorf("BC3API.Revision = %q, want 40-hex SHA", p.BC3API.Revision)
	}
	if p.BC3API.Date == "" {
		t.Error("BC3API.Date is empty")
	}
}
