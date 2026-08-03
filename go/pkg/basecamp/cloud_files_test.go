package basecamp

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

// The shared fixtures these tests read are the same ones the
// fixture-completeness guard validates against the CloudFile and
// GoogleDocument schemas (spec/fixtures/manifest.yaml), so a schema change
// that invalidates the fixture surfaces here too.
func loadSharedFixture(t *testing.T, resource string) []byte {
	t.Helper()
	path := filepath.Join("..", "..", "..", "spec", "fixtures", resource, "get.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read fixture %s: %v", path, err)
	}
	return data
}

func testCloudFilesServer(t *testing.T, handler http.HandlerFunc) *CloudFilesService {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	cfg := DefaultConfig()
	cfg.BaseURL = server.URL
	client := NewClient(cfg, &StaticTokenProvider{Token: "test-token"})
	return client.ForAccount("99999").CloudFiles()
}

func testGoogleDocumentsServer(t *testing.T, handler http.HandlerFunc) *GoogleDocumentsService {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	cfg := DefaultConfig()
	cfg.BaseURL = server.URL
	client := NewClient(cfg, &StaticTokenProvider{Token: "test-token"})
	return client.ForAccount("99999").GoogleDocuments()
}

// TestCloudFilesService_CreateURLIsVaultNestedAndBucketScoped pins the create
// route. BC3 draws cloud_files under `resources :vaults` INSIDE the bucket
// scope (config/routes.rb) — not at the flat /vaults/:id/cloud_files spelling
// that documents use, and not at /buckets/:id/cloud_files either. bc3's own API
// test drives bucket_vault_cloud_files_url. A wrong spelling here is a live 404,
// so assert the exact path.
func TestCloudFilesService_CreateURLIsVaultNestedAndBucketScoped(t *testing.T) {
	fixture := loadSharedFixture(t, "cloud_files")
	var gotPath string
	svc := testCloudFilesServer(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(201)
		_, _ = w.Write(fixture)
	})

	_, err := svc.Create(context.Background(), 2085958500, 1069479098, &CreateCloudFileRequest{
		URL:     "https://www.dropbox.com/s/abcd1234/brand.zip",
		Service: "dropbox",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := "/99999/buckets/2085958500/vaults/1069479098/cloud_files.json"
	if gotPath != want {
		t.Errorf("create path = %q, want %q", gotPath, want)
	}
}

// TestCloudFilesService_GetAndUpdateURLsAreFlat pins the read/write pair to the
// unscoped spelling BC3 documents as canonical
// (`resources :cloud_files, only: %i[ show update ]` at the top level). The
// bucket-scoped path BC3 also accepts is a documented legacy alias, not what the
// SDK sends. No .json suffix, matching GetDocument/ReplaceDocument — the format
// is resolved by the Accept header, and the route census normalizes the suffix
// away so both spellings satisfy parity.
func TestCloudFilesService_GetAndUpdateURLsAreFlat(t *testing.T) {
	fixture := loadSharedFixture(t, "cloud_files")

	var getPath string
	getSvc := testCloudFilesServer(t, func(w http.ResponseWriter, r *http.Request) {
		getPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(fixture)
	})
	if _, err := getSvc.Get(context.Background(), 1069480357); err != nil {
		t.Fatalf("unexpected Get error: %v", err)
	}
	if want := "/99999/cloud_files/1069480357"; getPath != want {
		t.Errorf("get path = %q, want %q", getPath, want)
	}

	var putPath string
	putSvc := testCloudFilesServer(t, func(w http.ResponseWriter, r *http.Request) {
		putPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(fixture)
	})
	_, err := putSvc.Update(context.Background(), 1069480357, &UpdateCloudFileRequest{
		URL:     "https://www.dropbox.com/s/abcd1234/brand-v2.zip",
		Service: "dropbox",
	})
	if err != nil {
		t.Fatalf("unexpected Update error: %v", err)
	}
	if want := "/99999/cloud_files/1069480357"; putPath != want {
		t.Errorf("update path = %q, want %q", putPath, want)
	}
}

// TestCloudFile_URLIsTheExternalLink guards the wire quirk: the cloud_files
// jbuilder renders the shared recording partial and THEN
// json.(recording.recordable, :url, :service), so `url` is the Dropbox/Figma/…
// link, not this record's API URL. Decoding it as the API URL would be a silent
// data error for any consumer following the link.
func TestCloudFile_URLIsTheExternalLink(t *testing.T) {
	fixture := loadSharedFixture(t, "cloud_files")
	svc := testCloudFilesServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(fixture)
	})

	cf, err := svc.Get(context.Background(), 1069480357)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if want := "https://www.dropbox.com/s/abcd1234/brand-draft.pdf"; cf.URL != want {
		t.Errorf("URL = %q, want the external link %q", cf.URL, want)
	}
	if want := "https://3.basecamp.com/999999999/buckets/2085958500/cloud_files/1069480357"; cf.AppURL != want {
		t.Errorf("AppURL = %q, want %q", cf.AppURL, want)
	}
	if cf.Service.Code != "dropbox" || cf.Service.Name != "Dropbox" {
		t.Errorf("Service = %+v, want code dropbox / name Dropbox", cf.Service)
	}
	if len(cf.Service.ValidPatterns) != 1 {
		t.Errorf("Service.ValidPatterns = %v, want one pattern", cf.Service.ValidPatterns)
	}
	if want := "a file or folder on Dropbox"; cf.Service.SupportingText != want {
		t.Errorf("Service.SupportingText = %q, want %q", cf.Service.SupportingText, want)
	}
	if cf.DescriptionAttachments == nil {
		t.Error("DescriptionAttachments = nil, want a non-nil (empty) slice for a server-sent []")
	}
}

// TestCloudFilesService_CreateVisibleToClients verifies the tri-state
// visible_to_clients flag reaches the wire correctly: nil omits the key, true
// is sent verbatim, and an explicit false is sent (not dropped).
func TestCloudFilesService_CreateVisibleToClients(t *testing.T) {
	fixture := loadSharedFixture(t, "cloud_files")
	tru, fls := true, false
	cases := []struct {
		name    string
		value   *bool
		present bool
		want    bool
	}{
		{"nil omits the field", nil, false, false},
		{"true is sent", &tru, true, true},
		{"explicit false is sent, not dropped", &fls, true, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var receivedBody map[string]any
			svc := testCloudFilesServer(t, func(w http.ResponseWriter, r *http.Request) {
				receivedBody = decodeRequestBody(t, r)
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(201)
				_, _ = w.Write(fixture)
			})

			_, err := svc.Create(context.Background(), 2085958500, 1069479098, &CreateCloudFileRequest{
				URL:              "https://www.dropbox.com/s/abcd1234/brand.zip",
				Service:          "dropbox",
				VisibleToClients: tc.value,
			})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			val, ok := receivedBody["visible_to_clients"]
			if ok != tc.present {
				t.Fatalf("visible_to_clients present=%v, want %v (body=%v)", ok, tc.present, receivedBody)
			}
			if tc.present && val != tc.want {
				t.Errorf("visible_to_clients=%v, want %v", val, tc.want)
			}
		})
	}
}

// TestCloudFilesService_RequiredFieldsAreRefusedLocally checks that url and
// service are refused before a request goes out. BC3 validates both, so
// omitting them is a 422 — catching it locally saves the round trip and gives a
// usage error rather than a server error.
func TestCloudFilesService_RequiredFieldsAreRefusedLocally(t *testing.T) {
	cases := []struct {
		name string
		req  *CreateCloudFileRequest
	}{
		{"nil request", nil},
		{"missing url", &CreateCloudFileRequest{Service: "dropbox"}},
		{"missing service", &CreateCloudFileRequest{URL: "https://www.dropbox.com/s/a/b.pdf"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc := testCloudFilesServer(t, func(w http.ResponseWriter, r *http.Request) {
				t.Error("request should not have been sent")
			})
			if _, err := svc.Create(context.Background(), 1, 2, tc.req); err == nil {
				t.Fatal("expected an error, got nil")
			}
		})
	}
}

// TestGoogleDocumentsService_CreateURLIsVaultNestedAndBucketScoped mirrors the
// cloud-file route assertion for Google documents.
func TestGoogleDocumentsService_CreateURLIsVaultNestedAndBucketScoped(t *testing.T) {
	fixture := loadSharedFixture(t, "google_documents")
	var gotPath string
	svc := testGoogleDocumentsServer(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(201)
		_, _ = w.Write(fixture)
	})

	_, err := svc.Create(context.Background(), 2085958500, 1069479098, &CreateGoogleDocumentRequest{
		URL:          "https://docs.google.com/document/d/abcd1234/edit",
		DocumentType: "doc",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := "/99999/buckets/2085958500/vaults/1069479098/google_documents.json"
	if gotPath != want {
		t.Errorf("create path = %q, want %q", gotPath, want)
	}
}

// TestGoogleDocument_URLIsTheExternalLink guards the same recordable-overwrites-
// recording rendering that CloudFile has.
func TestGoogleDocument_URLIsTheExternalLink(t *testing.T) {
	fixture := loadSharedFixture(t, "google_documents")
	var gotPath string
	svc := testGoogleDocumentsServer(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(fixture)
	})

	gd, err := svc.Get(context.Background(), 1069480366)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if want := "/99999/google_documents/1069480366"; gotPath != want {
		t.Errorf("get path = %q, want %q", gotPath, want)
	}
	if want := "https://docs.google.com/document/d/abcd1234/edit"; gd.URL != want {
		t.Errorf("URL = %q, want the external link %q", gd.URL, want)
	}
	if gd.DocumentType != "doc" {
		t.Errorf("DocumentType = %q, want doc", gd.DocumentType)
	}
	if gd.DescriptionAttachments == nil {
		t.Error("DescriptionAttachments = nil, want a non-nil (empty) slice for a server-sent []")
	}
}

// TestGoogleDocumentsService_RequiredFieldsAreRefusedLocally checks url and
// document_type are refused before a request goes out. An unrecognized
// document_type is a hardcoded 422 in BC3's before_action, and an absent one
// fails the same way.
func TestGoogleDocumentsService_RequiredFieldsAreRefusedLocally(t *testing.T) {
	cases := []struct {
		name string
		req  *CreateGoogleDocumentRequest
	}{
		{"nil request", nil},
		{"missing url", &CreateGoogleDocumentRequest{DocumentType: "doc"}},
		{"missing document_type", &CreateGoogleDocumentRequest{URL: "https://docs.google.com/d/x"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc := testGoogleDocumentsServer(t, func(w http.ResponseWriter, r *http.Request) {
				t.Error("request should not have been sent")
			})
			if _, err := svc.Create(context.Background(), 1, 2, tc.req); err == nil {
				t.Fatal("expected an error, got nil")
			}
		})
	}
}
