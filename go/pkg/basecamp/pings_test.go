package basecamp

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func loadPingsFixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "..", "spec", "fixtures", "pings", name))
	if err != nil {
		t.Fatalf("failed to read fixture %s: %v", name, err)
	}
	return data
}

func testPingsServer(t *testing.T, handler http.HandlerFunc) *PingsService {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	cfg := DefaultConfig()
	cfg.BaseURL = server.URL
	client := NewClient(cfg, &StaticTokenProvider{Token: "test-token"})
	return client.ForAccount("99999").Pings()
}

func TestPingsService_List(t *testing.T) {
	svc := testPingsServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" || r.URL.Path != "/99999/circles.json" {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		w.Write(loadPingsFixture(t, "list.json"))
	})

	pings, err := svc.List(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pings) != 2 {
		t.Fatalf("expected 2 pings, got %d", len(pings))
	}
	group, direct := pings[0], pings[1]
	if group.ID != 2085958600 || group.Title != "Launch crew" || !group.Named {
		t.Errorf("unexpected group ping: %+v", group)
	}
	if len(group.People) != 2 || group.People[1].Name != "Cheryl Walters" {
		t.Errorf("expected the group's two other people, got %+v", group.People)
	}
	if group.EditTitleURL == "" || group.ImageURL == "" {
		t.Errorf("expected the group's edit_title_url and image_url, got %q and %q", group.EditTitleURL, group.ImageURL)
	}
	if direct.Named || direct.EditTitleURL != "" || direct.ImageURL != "" {
		t.Errorf("a direct ping carries no name, edit_title_url or image_url: %+v", direct)
	}
	if direct.Links.Lines != "https://3.basecampapi.com/195539477/buckets/2085958601/chats/1069479601/lines.json" {
		t.Errorf("unexpected lines link %q", direct.Links.Lines)
	}
}

func TestPingsService_List_NoContent(t *testing.T) {
	svc := testPingsServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	pings, err := svc.List(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pings == nil || len(pings) != 0 {
		t.Errorf("expected an empty, non-nil list, got %#v", pings)
	}
}

func TestPingsService_List_PingsDisabled(t *testing.T) {
	svc := testPingsServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte(`{"error":"Pings have been disabled"}`))
	})

	pings, err := svc.List(context.Background())
	apiErr, ok := errors.AsType[*Error](err)
	if !ok || apiErr.Code != CodeForbidden {
		t.Errorf("expected %s, got: %v", CodeForbidden, err)
	}
	if pings != nil {
		t.Errorf("expected no pings with the error, got %#v", pings)
	}
}

func TestPingsService_Get(t *testing.T) {
	svc := testPingsServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" || r.URL.Path != "/99999/circles/2085958600" {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		w.Write(loadPingsFixture(t, "get.json"))
	})

	ping, err := svc.Get(context.Background(), 2085958600)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ping.ID != 2085958600 {
		t.Errorf("expected ID 2085958600, got %d", ping.ID)
	}
	if ping.RoomURL != "https://3.basecampapi.com/195539477/buckets/2085958600/chats/1069479600.json" {
		t.Errorf("unexpected room_url %q", ping.RoomURL)
	}
	if id, ok := ping.CampfireID(); !ok || id != 1069479600 {
		t.Errorf("expected Campfire 1069479600, got %d (%t)", id, ok)
	}
}

func TestPingsService_Get_Errors(t *testing.T) {
	for name, tc := range map[string]struct {
		status int
		body   string
		code   string
	}{
		"not a ping":     {404, `{"error":"Record not found"}`, CodeNotFound},
		"pings disabled": {403, `{"error":"Pings have been disabled"}`, CodeForbidden},
	} {
		t.Run(name, func(t *testing.T) {
			svc := testPingsServer(t, func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tc.status)
				w.Write([]byte(tc.body))
			})
			_, err := svc.Get(context.Background(), 1)
			apiErr, ok := errors.AsType[*Error](err)
			if !ok || apiErr.Code != tc.code {
				t.Errorf("expected %s, got: %v", tc.code, err)
			}
		})
	}
}

func TestPingsService_Create(t *testing.T) {
	var received map[string]any
	svc := testPingsServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" || r.URL.Path != "/99999/circles.json" {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &received)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(201)
		w.Write(loadPingsFixture(t, "get.json"))
	})

	ping, err := svc.Create(context.Background(), &CreatePingRequest{UserIDs: []int64{1049715915, 1049715916}, NewGroup: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ping.ID != 2085958600 {
		t.Errorf("expected ID 2085958600, got %d", ping.ID)
	}
	ids, ok := received["user_ids"].([]any)
	if !ok || len(ids) != 2 || ids[0] != float64(1049715915) {
		t.Errorf("expected user_ids in body, got %v", received["user_ids"])
	}
	if received["new_group"] != true {
		t.Errorf("expected new_group true in body, got %v", received["new_group"])
	}
}

func TestPingsService_Create_OmitsNewGroupByDefault(t *testing.T) {
	var received map[string]any
	svc := testPingsServer(t, func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &received)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(201)
		w.Write(loadPingsFixture(t, "get.json"))
	})

	if _, err := svc.Create(context.Background(), &CreatePingRequest{UserIDs: []int64{1049715915}}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, present := received["new_group"]; present {
		t.Errorf("expected no new_group in body, got %v", received["new_group"])
	}
}

func TestPingsService_Create_Refusals(t *testing.T) {
	svc := testPingsServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
	})

	_, err := svc.Create(context.Background(), &CreatePingRequest{UserIDs: []int64{42}})
	apiErr, ok := errors.AsType[*Error](err)
	if !ok || apiErr.HTTPStatus != http.StatusUnprocessableEntity {
		t.Errorf("expected the bodiless 422 as an API error, got: %v", err)
	}

	for name, req := range map[string]*CreatePingRequest{
		"nil request": nil,
		"no people":   {},
	} {
		blocked := testPingsServer(t, func(w http.ResponseWriter, r *http.Request) {
			t.Errorf("%s: a request that fails client-side validation must not reach the wire", name)
		})
		_, err := blocked.Create(context.Background(), req)
		apiErr, ok := errors.AsType[*Error](err)
		if !ok || apiErr.Code != CodeUsage {
			t.Errorf("%s: expected usage error, got: %v", name, err)
		}
	}
}

func TestPing_CampfireID(t *testing.T) {
	for name, tc := range map[string]struct {
		roomURL string
		want    int64
		ok      bool
	}{
		"bucket-scoped room": {"https://3.basecampapi.com/195539477/buckets/2085958600/chats/1069479600.json", 1069479600, true},
		"flat room":          {"https://3.basecampapi.com/195539477/chats/1069479600.json", 1069479600, true},
		"not a campfire":     {"https://3.basecampapi.com/195539477/buckets/2085958600/todos/1069479600.json", 0, false},
		"its lines":          {"https://3.basecampapi.com/195539477/buckets/2085958600/chats/1069479600/lines.json", 0, false},
		"empty":              {"", 0, false},
	} {
		t.Run(name, func(t *testing.T) {
			got, ok := (&Ping{RoomURL: tc.roomURL}).CampfireID()
			if got != tc.want || ok != tc.ok {
				t.Errorf("CampfireID(%q) = %d, %t; want %d, %t", tc.roomURL, got, ok, tc.want, tc.ok)
			}
		})
	}
}
