# Engineering Notes — Campaign Events

## Part 1: Read and think first

### 1. My interpretation
- The service receives batches of provider events and stores valid events so campaign statistics can be calculated.
- The main correctness requirements are preventing retries from inflating counts, handling events that arrive late or out of order, and remaining safe when requests arrive concurrently.
- The dashboard needs counts for `sent`, `delivered`, `opened`, and `clicked` for a campaign.

### 2. Assumptions
- `event_id` identifies one event globally and is the idempotency key.
- The first valid event stored for an `event_id` wins. A later event with the same ID is treated as a duplicate and does not overwrite the original, even if its payload differs.
- Each accepted event contributes one to the count for its event type. These are event counts, not unique-contact counts.
- `timestamp` must be a valid RFC3339 timestamp. It represents provider event time; arrival order does not affect aggregation.
- A batch is processed per item: valid events are accepted, duplicates are reported separately, and malformed/invalid items are rejected without discarding other valid items.
- The initial implementation will use in-memory storage protected by a mutex. Data will be lost when the process restarts; this is acceptable for a time-boxed prototype and will be documented.

### 3. Ambiguities noticed
- Is `event_id` globally unique across all providers, or only unique within a provider?
- If the same `event_id` arrives with different content, should the first version win, should it be rejected as a conflict, or should it be updated?
- Should `opened` and `clicked` count every event, or only unique contacts?
- Should a batch be all-or-nothing, or should valid items be accepted when other items are invalid?
- Are future timestamps or very old timestamps valid?
- Are event types case-sensitive, and should unknown fields be ignored?
- What retention period and durability guarantees are expected?

### 4. Questions I would ask the PM
- Can you confirm the scope of uniqueness for `event_id` and the expected behaviour for conflicting duplicate IDs?
- Should the dashboard show total events or unique contacts for opens and clicks?
- Should the API return HTTP 2xx when some items in a batch are rejected, and what response format does the provider expect?
- What timestamp range and timezone rules should be enforced?
- What are the expected data-retention and availability requirements?

### 5. Priorities in the available time
1. Implement the two required endpoints with clear validation and JSON responses.
2. Make event ingestion idempotent and concurrency-safe.
3. Test valid events, duplicates, invalid items, and campaign statistics.
4. Fix and verify all four bugs in the supplied debugging exercise.
5. Complete the scale and marketer reasoning sections, then document run instructions and AI usage.
6. Skip the optional events-list endpoint and frontend unless all required work is complete.

## Part 4: Scale memo

- **What breaks first:** The in-memory map grows without bound and all data disappears on restart. Each stats request scans every stored event, so response time grows with total history. A single process and one in-memory store also limit horizontal scaling. The global mutex can become a write bottleneck under heavy concurrent ingestion.
- **What I would change, in order:** First measure traffic shape and query latency. Move events to durable storage with a unique constraint on `event_id`; add indexes for campaign and event type/time. Add retention/partitioning and pre-aggregated counters if stats scans become expensive. Then scale API instances behind a load balancer and tune database connections and batching.
- **API and storage:** Keep the basic ingestion and stats concepts, but consider adding time-range filters, pagination, and a documented freshness indicator. Move from process memory to a durable database, likely partitioned event storage plus aggregate tables/materialized views when justified.
- **Queue:** I would introduce a durable queue if provider traffic bursts or database writes cannot keep up. It smooths bursts and separates ingestion from processing, but adds eventual consistency, consumer retries, poison messages/dead-letter queues, ordering questions, operational cost, and more failure modes.
- **New duplicate paths:** Providers can retry the HTTP request; the API can publish to a queue and crash before recording the publish result; consumers can process a message again after a crash or lost acknowledgement; and a replay/backfill can re-submit old events. Idempotency must therefore exist at the durable event-store/consumer boundary, not only in one API process.
- **Keeping counters trustworthy:** Insert the event under a unique `event_id` constraint and update its aggregate in the same database transaction. Reconcile aggregates periodically against source events, track rejected/conflicting events, and make replays idempotent.
- **What I would monitor:** Request rate, latency and error rate; accepted/duplicate/rejected counts; queue depth and consumer lag; database latency, lock contention, storage growth and connection saturation; aggregate freshness; and reconciliation mismatches.
- **What I would not solve yet:** Multi-region active-active writes, exactly-once delivery claims, complex stream processing, and a large microservice split before measurements show they are needed.

## Part 5: The angry marketer

### Plausible explanations before assuming a bug
- The dashboard may use a rolling time window, and the window or timezone boundary may have changed between snapshots.
- The definition of `opened` may have changed from raw events to unique contacts, or the aggregation may have been recalculated using a different deduplication rule.
- A late data correction, provider replay, or backfill may have caused duplicate opens to be removed.
- Invalid, bot-generated, privacy-proxy, or otherwise filtered open events may have been excluded during a reprocessing run.
- Events may have been reassigned to another campaign/contact, or corrected provider data may have removed an earlier attribution.
- The 10:00 and 10:30 views may have come from different replicas, cache versions, or partially completed aggregation jobs.
- A dashboard filter, campaign timezone, segment, or date range may differ between the two views.
- An aggregation bug or race condition may have overwritten a newer count with an older snapshot.
- The delivered count can legitimately rise because delivery callbacks arrive late or are retried and then accepted once.

### What I would check first
1. Confirm both screenshots/API calls use the same campaign, time range, timezone, filters, and metric definitions.
2. Compare the raw event records and aggregation/query version for the two snapshots, especially newly ingested, deleted, deduplicated, or reclassified `opened` events.
3. Check cache/replica freshness and whether an aggregation rebuild or backfill ran between 10:00 and 10:30.
4. Recompute the opened count from the source events using the documented rules and compare it with the stored aggregate.

I would call it a bug if the same source data and same metric definition produce different counts, or if events were lost by a race/incorrect update. A decrease can be expected if the metric is recalculated after corrections or filtering, but the dashboard should make that behaviour and freshness clear. Under this assignment's append-only, event-ID-deduplicated event-count model, `opened` should not decrease; a decrease would be suspicious and needs investigation.

**Is delivered rising suspicious?** Not by itself. Delivery events can arrive late, so a cumulative delivered count normally rises as callbacks arrive. It becomes suspicious if it exceeds the number of sent messages under a one-delivery-per-message definition, or if duplicate events are being counted.

## What I completed / what I intentionally skipped

- Completed the required event-ingestion and campaign-stats endpoints, validation, duplicate handling, in-memory storage, and focused tests.
- Completed the four debugging fixes and verified exact output against the supplied expected output, including a clean race-detector run.
- Completed the scale memo and angry-marketer analysis.
- Intentionally skipped the optional campaign events listing endpoint, frontend, deployment, Docker, and persistent database to prioritise the required work within the time box.

## If I had another day

- Add persistent SQLite/PostgreSQL storage with a unique constraint on `event_id` and transactional writes.
- Add pagination for campaign events, richer integration tests, request metrics, and load testing.
- Clarify provider identity and conflicting duplicate semantics with the PM, then refine the idempotency key and conflict response accordingly.
