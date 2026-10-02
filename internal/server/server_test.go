package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/AmpedWasTaken/cbx/internal/model"
	"github.com/AmpedWasTaken/cbx/internal/store"
)

func TestCallbackStoresRedactedEvent(t *testing.T) {
	home := t.TempDir()
	st := store.New(home)
	if err := st.Init(); err != nil {
		t.Fatal(err)
	}
	cb := model.Callback{
		ID:        "test-token",
		Type:      "http",
		CreatedAt: time.Now().UTC(),
		ExpiresAt: time.Now().Add(time.Hour).UTC(),
	}
	if err := st.CreateCallback(cb); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/c/test-token", nil)
	req.Header.Set("Authorization", "Bearer secret")
	rec := httptest.NewRecorder()

	New(st).ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d", rec.Code)
	}

	events, err := st.ListEvents(cb.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 {
		t.Fatalf("events = %d", len(events))
	}
	if events[0].Headers["Authorization"] != "[REDACTED]" {
		t.Fatalf("authorization not redacted: %q", events[0].Headers["Authorization"])
	}
	if events[0].EvidenceHash == "" {
		t.Fatal("missing evidence hash")
	}
}
