package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

const maxRequestBody = 2 << 20 // 2 MiB

var allowedTypes = map[string]bool{
	"sent": true, "delivered": true, "opened": true, "clicked": true,
}

// Store protects the event map because HTTP handlers may run concurrently.
type Store struct {
	mu     sync.RWMutex
	events map[string]Event // event_id -> first accepted event
}

func NewStore() *Store {
	return &Store{events: make(map[string]Event)}
}

func (s *Store) Add(event Event) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.events[event.EventID]; exists {
		return false
	}
	s.events[event.EventID] = event
	return true
}

func (s *Store) Stats(campaignID string) CampaignStats {
	s.mu.RLock()
	defer s.mu.RUnlock()

	stats := CampaignStats{CampaignID: campaignID}
	for _, event := range s.events {
		if event.CampaignID != campaignID {
			continue
		}
		stats.TotalEvents++
		switch event.Type {
		case "sent":
			stats.Sent++
		case "delivered":
			stats.Delivered++
		case "opened":
			stats.Opened++
		case "clicked":
			stats.Clicked++
		}
	}
	return stats
}

func main() {
	store := NewStore()
	mux := http.NewServeMux()
	mux.HandleFunc("POST /events", handlePostEvents(store))
	mux.HandleFunc("GET /campaigns/{campaignID}/stats", handleGetStats(store))

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	addr := ":" + port
	log.Printf("campaign events service listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, mux))
}

func handlePostEvents(store *Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, maxRequestBody)
		body, err := io.ReadAll(r.Body)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "request body could not be read or is too large"})
			return
		}

		var items []json.RawMessage
		decoder := json.NewDecoder(bytes.NewReader(body))
		if err := decoder.Decode(&items); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "request body must be a JSON array"})
			return
		}
		// Reject trailing JSON values rather than silently ignoring them.
		if err := decoder.Decode(new(any)); !errors.Is(err, io.EOF) {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "request body must contain only one JSON value"})
			return
		}

		if items == nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "request body must be a JSON array"})
			return
		}

		result := IngestResponse{Rejected: make([]RejectedEvent, 0)}
		for i, raw := range items {
			var event Event
			if err := json.Unmarshal(raw, &event); err != nil {
				result.Rejected = append(result.Rejected, RejectedEvent{Index: i, Error: "invalid event JSON or field type"})
				continue
			}
			if err := validateEvent(event); err != nil {
				result.Rejected = append(result.Rejected, RejectedEvent{Index: i, Error: err.Error()})
				continue
			}
			if store.Add(event) {
				result.Accepted++
			} else {
				result.Duplicates++
			}
		}

		// A syntactically valid batch gets 200 even if individual entries are rejected.
		writeJSON(w, http.StatusOK, result)
	}
}

func validateEvent(event Event) error {
	if strings.TrimSpace(event.EventID) == "" {
		return fmt.Errorf("event_id is required")
	}
	if strings.TrimSpace(event.CampaignID) == "" {
		return fmt.Errorf("campaign_id is required")
	}
	if strings.TrimSpace(event.ContactID) == "" {
		return fmt.Errorf("contact_id is required")
	}
	if !allowedTypes[event.Type] {
		return fmt.Errorf("type must be sent, delivered, opened, or clicked")
	}
	if _, err := time.Parse(time.RFC3339, event.Timestamp); err != nil {
		return fmt.Errorf("timestamp must be a valid RFC3339 timestamp")
	}
	return nil
}

func handleGetStats(store *Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		campaignID := strings.TrimSpace(r.PathValue("campaignID"))
		if campaignID == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "campaign_id is required"})
			return
		}
		writeJSON(w, http.StatusOK, store.Stats(campaignID))
	}
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		log.Printf("writeJSON: %v", err)
	}
}
