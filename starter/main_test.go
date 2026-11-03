package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"
)

func TestIngestDeduplicatesAndReturnsCampaignStats(t *testing.T) {
	store := NewStore()
	body := `[
		{"event_id":"evt-1","campaign_id":"cmp-1","contact_id":"ct-1","type":"opened","timestamp":"2026-08-10T09:15:04Z"},
		{"event_id":"evt-1","campaign_id":"cmp-1","contact_id":"ct-1","type":"opened","timestamp":"2026-08-10T09:15:04Z"},
		{"event_id":"evt-2","campaign_id":"cmp-1","contact_id":"ct-1","type":"delivered","timestamp":"2026-08-10T09:10:04Z"},
		{"event_id":"bad","campaign_id":"cmp-1","type":"clicked","timestamp":"not-a-date"}
	]`
	req := httptest.NewRequest(http.MethodPost, "/events", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()
	handlePostEvents(store)(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	var result IngestResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.Accepted != 2 || result.Duplicates != 1 || len(result.Rejected) != 1 {
		t.Fatalf("unexpected ingest result: %+v", result)
	}

	stats := store.Stats("cmp-1")
	if stats.Opened != 1 || stats.Delivered != 1 || stats.TotalEvents != 2 {
		t.Fatalf("unexpected stats: %+v", stats)
	}
}

func TestStoreIsSafeForConcurrentDuplicateRequests(t *testing.T) {
	store := NewStore()
	event := Event{EventID: "same-event", CampaignID: "cmp-1", ContactID: "ct-1", Type: "sent", Timestamp: "2026-08-10T09:15:04Z"}
	const workers = 100
	var wg sync.WaitGroup
	var mu sync.Mutex
	accepted := 0
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if store.Add(event) {
				mu.Lock()
				accepted++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	if accepted != 1 {
		t.Fatalf("accepted = %d, want exactly 1", accepted)
	}
	if got := store.Stats("cmp-1").Sent; got != 1 {
		t.Fatalf("sent = %d, want 1", got)
	}
}

func TestMalformedBatchGetsBadRequest(t *testing.T) {
	store := NewStore()
	req := httptest.NewRequest(http.MethodPost, "/events", bytes.NewBufferString(`{"event_id":"not-an-array"}`))
	rec := httptest.NewRecorder()
	handlePostEvents(store)(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestSeedBatchIsHandledWithoutLosingValidItems(t *testing.T) {
	seed, err := os.ReadFile("seed/events.json")
	if err != nil {
		t.Fatal(err)
	}
	store := NewStore()
	req := httptest.NewRequest(http.MethodPost, "/events", bytes.NewReader(seed))
	rec := httptest.NewRecorder()
	handlePostEvents(store)(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("seed batch status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	var result IngestResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.Accepted == 0 || len(result.Rejected) == 0 {
		t.Fatalf("expected some accepted and some rejected seed records; got %+v", result)
	}
	t.Logf("seed records: accepted=%d duplicates=%d rejected=%d", result.Accepted, result.Duplicates, len(result.Rejected))
}
