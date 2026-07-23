package basecamp

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

// Distinct bucketID/cardTableID/wormholeID — a future swap of the argument order
// would build a path with the IDs in the wrong slots and fail the assertion.
const (
	wormholesTestBucketID    = int64(2085958499)
	wormholesTestCardTableID = int64(1069479345)
	wormholesTestWormholeID  = int64(1069479400)
	wormholesTestDestColumn  = int64(1069479500)
)

// testWormholesServer creates an httptest.Server and a WormholesService wired to it.
func testWormholesServer(t *testing.T, handler http.HandlerFunc) *WormholesService {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	cfg := DefaultConfig()
	cfg.BaseURL = server.URL
	token := &StaticTokenProvider{Token: "test-token"}
	client := NewClient(cfg, token)
	account := client.ForAccount("99999")
	return account.Wormholes()
}

func TestWormholesService_Create(t *testing.T) {
	fixture := loadCardsFixture(t, "wormhole.json")
	wantPath := "/99999/buckets/2085958499/card_tables/1069479345/wormholes.json"

	var receivedBody map[string]any
	svc := testWormholesServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != wantPath {
			t.Errorf("expected path %s, got %s", wantPath, r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&receivedBody); err != nil {
			t.Fatalf("failed to decode request body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(201)
		_, _ = w.Write(fixture)
	})

	wormhole, err := svc.Create(context.Background(), wormholesTestBucketID, wormholesTestCardTableID, wormholesTestDestColumn)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := receivedBody["destination_recording_id"]; got != float64(wormholesTestDestColumn) {
		t.Errorf("expected destination_recording_id %d, got %v", wormholesTestDestColumn, got)
	}
	if wormhole.ID != wormholesTestWormholeID {
		t.Errorf("expected ID %d, got %d", wormholesTestWormholeID, wormhole.ID)
	}
	if !wormhole.Linked {
		t.Error("expected linked wormhole")
	}
	if wormhole.DestinationURL == nil {
		t.Fatal("expected destination URL to be set for linked wormhole")
	}
}

func TestWormholesService_Create_ValidationLimit(t *testing.T) {
	// A card table may hold at most four wormholes; the server returns 422 at the limit.
	svc := testWormholesServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(422)
	})

	_, err := svc.Create(context.Background(), wormholesTestBucketID, wormholesTestCardTableID, wormholesTestDestColumn)
	if err == nil {
		t.Fatal("expected error for 422")
	}
	apiErr, ok := errors.AsType[*Error](err)
	if !ok || apiErr.Code != CodeValidation {
		t.Errorf("expected validation error, got: %v", err)
	}
}

func TestWormholesService_Create_NotFoundDestination(t *testing.T) {
	// An invalid/inaccessible/same-board destination fails the filtered .find with 404.
	svc := testWormholesServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
	})

	_, err := svc.Create(context.Background(), wormholesTestBucketID, wormholesTestCardTableID, wormholesTestDestColumn)
	if err == nil {
		t.Fatal("expected error for 404")
	}
	apiErr, ok := errors.AsType[*Error](err)
	if !ok || apiErr.Code != CodeNotFound {
		t.Errorf("expected not_found error, got: %v", err)
	}
}

func TestWormholesService_Create_RequiresIDs(t *testing.T) {
	svc := testWormholesServer(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("server should not be called when input is invalid")
	})

	if _, err := svc.Create(context.Background(), 0, wormholesTestCardTableID, wormholesTestDestColumn); err == nil {
		t.Error("expected error for missing project ID")
	}
	if _, err := svc.Create(context.Background(), wormholesTestBucketID, 0, wormholesTestDestColumn); err == nil {
		t.Error("expected error for missing card table ID")
	}
	if _, err := svc.Create(context.Background(), wormholesTestBucketID, wormholesTestCardTableID, 0); err == nil {
		t.Error("expected error for missing destination recording ID")
	}
}

func TestWormholesService_Update(t *testing.T) {
	fixture := loadCardsFixture(t, "wormhole.json")
	wantPath := "/99999/buckets/2085958499/card_tables/wormholes/1069479400"

	var receivedBody map[string]any
	svc := testWormholesServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("expected PUT, got %s", r.Method)
		}
		if r.URL.Path != wantPath {
			t.Errorf("expected path %s, got %s", wantPath, r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&receivedBody); err != nil {
			t.Fatalf("failed to decode request body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		_, _ = w.Write(fixture)
	})

	wormhole, err := svc.Update(context.Background(), wormholesTestBucketID, wormholesTestWormholeID, wormholesTestDestColumn)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := receivedBody["destination_recording_id"]; got != float64(wormholesTestDestColumn) {
		t.Errorf("expected destination_recording_id %d, got %v", wormholesTestDestColumn, got)
	}
	if wormhole.ID != wormholesTestWormholeID {
		t.Errorf("expected ID %d, got %d", wormholesTestWormholeID, wormhole.ID)
	}
}

func TestWormholesService_Update_NotFound(t *testing.T) {
	svc := testWormholesServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
	})

	_, err := svc.Update(context.Background(), wormholesTestBucketID, wormholesTestWormholeID, wormholesTestDestColumn)
	if err == nil {
		t.Fatal("expected error for 404")
	}
	apiErr, ok := errors.AsType[*Error](err)
	if !ok || apiErr.Code != CodeNotFound {
		t.Errorf("expected not_found error, got: %v", err)
	}
}

func TestWormholesService_Update_RequiresIDs(t *testing.T) {
	svc := testWormholesServer(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("server should not be called when input is invalid")
	})

	if _, err := svc.Update(context.Background(), 0, wormholesTestWormholeID, wormholesTestDestColumn); err == nil {
		t.Error("expected error for missing project ID")
	}
	if _, err := svc.Update(context.Background(), wormholesTestBucketID, 0, wormholesTestDestColumn); err == nil {
		t.Error("expected error for missing wormhole ID")
	}
	if _, err := svc.Update(context.Background(), wormholesTestBucketID, wormholesTestWormholeID, 0); err == nil {
		t.Error("expected error for missing destination recording ID")
	}
}

func TestWormholesService_Delete(t *testing.T) {
	wantPath := "/99999/buckets/2085958499/card_tables/wormholes/1069479400"

	svc := testWormholesServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("expected DELETE, got %s", r.Method)
		}
		if r.URL.Path != wantPath {
			t.Errorf("expected path %s, got %s", wantPath, r.URL.Path)
		}
		w.WriteHeader(204)
	})

	if err := svc.Delete(context.Background(), wormholesTestBucketID, wormholesTestWormholeID); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestWormholesService_Delete_Forbidden(t *testing.T) {
	svc := testWormholesServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(403)
	})

	err := svc.Delete(context.Background(), wormholesTestBucketID, wormholesTestWormholeID)
	if err == nil {
		t.Fatal("expected error for 403")
	}
	apiErr, ok := errors.AsType[*Error](err)
	if !ok || apiErr.Code != CodeForbidden {
		t.Errorf("expected forbidden error, got: %v", err)
	}
}

func TestWormholesService_Delete_NotFound(t *testing.T) {
	svc := testWormholesServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
	})

	err := svc.Delete(context.Background(), wormholesTestBucketID, wormholesTestWormholeID)
	if err == nil {
		t.Fatal("expected error for 404")
	}
	apiErr, ok := errors.AsType[*Error](err)
	if !ok || apiErr.Code != CodeNotFound {
		t.Errorf("expected not_found error, got: %v", err)
	}
}

func TestWormholesService_Delete_RequiresIDs(t *testing.T) {
	svc := testWormholesServer(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("server should not be called when input is invalid")
	})

	if err := svc.Delete(context.Background(), 0, wormholesTestWormholeID); err == nil {
		t.Error("expected error for missing project ID")
	}
	if err := svc.Delete(context.Background(), wormholesTestBucketID, 0); err == nil {
		t.Error("expected error for missing wormhole ID")
	}
}

// TestCardTablesService_Get_DecodesWormholes exercises cardTableFromGenerated,
// asserting that both a linked and an unlinked wormhole flow through the
// converter — the path TestCardTable_Unmarshal bypasses.
func TestCardTablesService_Get_DecodesWormholes(t *testing.T) {
	fixture := loadCardsFixture(t, "card_table.json")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		_, _ = w.Write(fixture)
	}))
	t.Cleanup(server.Close)

	cfg := DefaultConfig()
	cfg.BaseURL = server.URL
	client := NewClient(cfg, &StaticTokenProvider{Token: "test-token"})
	svc := client.ForAccount("99999").CardTables()

	cardTable, err := svc.Get(context.Background(), wormholesTestCardTableID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cardTable.Wormholes) != 2 {
		t.Fatalf("expected 2 wormholes, got %d", len(cardTable.Wormholes))
	}

	linked := cardTable.Wormholes[0]
	if !linked.Linked {
		t.Error("expected first wormhole to be linked")
	}
	if linked.DestinationURL == nil {
		t.Fatal("expected linked wormhole to carry a destination URL")
	}
	if linked.Color != "#f5d76e" {
		t.Errorf("expected color to decode, got %q", linked.Color)
	}
	if linked.Bucket == nil || linked.Creator == nil || linked.Parent == nil {
		t.Error("expected recording associations to decode")
	}

	unlinked := cardTable.Wormholes[1]
	if unlinked.Linked {
		t.Error("expected second wormhole to be unlinked")
	}
	if unlinked.DestinationURL != nil {
		t.Errorf("expected nil destination URL for unlinked wormhole, got %q", *unlinked.DestinationURL)
	}
}
