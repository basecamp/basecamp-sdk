package basecamp

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestProjectsService_GetLogo(t *testing.T) {
	var fixture map[string]any
	if err := json.Unmarshal(loadFixture(t, "get.json"), &fixture); err != nil {
		t.Fatal(err)
	}
	fixture["logo"] = map[string]any{"url": "https://3.basecampapi.com/99999/projects/42/logo?v=1"}
	svc := testProjectsServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(fixture); err != nil {
			t.Error(err)
		}
	})
	project, err := svc.Get(context.Background(), 42)
	if err != nil {
		t.Fatal(err)
	}
	if project.Logo == nil || project.Logo.URL != fixture["logo"].(map[string]any)["url"] {
		t.Fatalf("logo not preserved: %+v", project.Logo)
	}
	absent := make(map[string]any, len(fixture))
	for k, v := range fixture {
		if k != "logo" {
			absent[k] = v
		}
	}
	absentSvc := testProjectsServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(absent); err != nil {
			t.Error(err)
		}
	})
	project, err = absentSvc.Get(context.Background(), 42)
	if err != nil {
		t.Fatal(err)
	}
	if project.Logo != nil {
		t.Fatal("absent logo should be nil")
	}
}

func TestProjectsService_UpdateLogoMultipart(t *testing.T) {
	svc := testProjectsServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "PUT" || r.URL.Path != "/99999/projects/42/logo.json" || r.URL.RawQuery != "" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL)
		}
		if err := r.ParseMultipartForm(1024); err != nil {
			t.Fatal(err)
		}
		f, header, err := r.FormFile("logo")
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		data, err := io.ReadAll(f)
		if err != nil {
			t.Fatal(err)
		}
		if string(data) != "logo-bytes" || header.Filename != "logo.png" || header.Header.Get("Content-Type") != "image/png" {
			t.Fatal("multipart did not preserve the file")
		}
		if len(r.MultipartForm.File) != 1 {
			t.Fatal("unexpected multipart fields")
		}
		w.WriteHeader(204)
	})
	if err := svc.UpdateLogo(context.Background(), 42, strings.NewReader("logo-bytes"), "logo.png", "image/png"); err != nil {
		t.Fatal(err)
	}
}

func TestProjectsService_UpdateLogoRetriesReplayBody(t *testing.T) {
	var bodies [][]byte
	svc := testProjectsServer(t, func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Error(err)
		}
		bodies = append(bodies, body)
		if len(bodies) == 1 {
			w.Header().Set("Retry-After", "1")
			w.WriteHeader(503)
		} else {
			w.WriteHeader(204)
		}
	})
	if err := svc.UpdateLogo(context.Background(), 42, strings.NewReader("logo-bytes"), "logo.png", "image/png"); err != nil {
		t.Fatal(err)
	}
	if len(bodies) != 2 || string(bodies[0]) != string(bodies[1]) {
		t.Fatal("retry did not replay the exact multipart body")
	}
}
