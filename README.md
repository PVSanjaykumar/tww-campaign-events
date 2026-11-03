# Think Wide Web — Campaign Events Assessment

A Go HTTP service that accepts provider campaign events and returns per-campaign event counts. The optional frontend, deployment, and event-list endpoint are intentionally out of scope.

## Requirements

- Go 1.22 or later (tested with Go 1.23.2)
- No third-party dependencies

## Run the service

```bash
cd starter
go run .
```

The server listens on `http://localhost:8080`.

## Ingest events

```bash
curl -i -X POST http://localhost:8080/events \
  -H 'Content-Type: application/json' \
  -d '[{"event_id":"evt_1","campaign_id":"cmp_summer_sale","contact_id":"ct_001","type":"delivered","timestamp":"2026-08-10T06:15:00Z"}]'
```

A valid batch returns HTTP 200 with per-item results:

```json
{
  "accepted": 1,
  "duplicates": 0,
  "rejected": []
}
```

Each event must have non-empty `event_id`, `campaign_id`, and `contact_id`; a supported `type` (`sent`, `delivered`, `opened`, or `clicked`); and a valid RFC3339 `timestamp`. `metadata` is optional. Invalid individual records are rejected while valid records in the same syntactically valid array are processed. A malformed outer JSON body returns HTTP 400.

The first valid event with an `event_id` is retained. Later occurrences are counted as duplicates and do not overwrite it, even if their payload differs. The implementation assumes event IDs are globally unique.

## Read campaign stats

```bash
curl -i http://localhost:8080/campaigns/cmp_summer_sale/stats
```

The response contains `campaign_id`, `sent`, `delivered`, `opened`, `clicked`, and `total_events`. These are counts of accepted events, not unique contacts. Late events are included when received; their provider timestamps do not affect the cumulative totals.

## Tests

```bash
cd starter
go test ./...
go test -race ./...
```

The tests cover idempotency, partial batch rejection, campaign counts, concurrent duplicate insertion, and the supplied seed batch.

## Storage and limitations

Events are stored in a process-local in-memory map protected by `sync.RWMutex`. This is simple and dependency-free, but all data is lost on restart, memory usage grows with event volume, and each stats request scans stored events. A durable database with a unique constraint on `event_id` and transactional aggregate updates would be the next step for production.

## Debugging exercise

```bash
cd debugging
go run . events.jsonl > actual_output.txt
diff actual_output.txt expected_output.txt
go run -race . events.jsonl
```

The four minimal fixes are documented in `BUGS.md`.
