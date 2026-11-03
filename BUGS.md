# Debugging Findings

## Bug 1: Duplicate event IDs were only de-duplicated inside one batch
- **What:** `seen` was created inside `processBatch`, so an event repeated in a later batch was counted again.
- **Why:** The map was reset every time a batch was processed. The input is larger than one batch, so duplicates across batch boundaries were missed.
- **Fix:** Moved the event-ID set to package scope as `seenEventIDs`, keeping it for the full file-processing run.
- **Verification:** Ran `go run . events.jsonl` and compared output with `expected_output.txt`; `diff` returned no differences.

## Bug 2: Unique opens were tracked globally by contact instead of per campaign
- **What:** A contact opening campaign A caused an open by the same contact in campaign B not to count as a unique open.
- **Why:** `openedBy` used only `ContactID` as its map key, but the requirement defines uniqueness within each campaign.
- **Fix:** The key is now `CampaignID + "\\x00" + ContactID`.
- **Verification:** The final output's `unique_opens` values match `expected_output.txt` for all campaigns.

## Bug 3: Daily delivered buckets used local time
- **What:** Daily delivery counts could be assigned to the wrong calendar date depending on the machine's timezone.
- **Why:** `Timestamp.Local()` converts the UTC event timestamp to the host's local timezone before extracting the date.
- **Fix:** Changed it to `Timestamp.UTC()` before formatting the date.
- **Verification:** Daily delivered buckets match every date and count in `expected_output.txt`.

## Bug 4: Worker goroutines updated shared counters without synchronization
- **What:** Concurrent workers could lose increments and cause data races.
- **Why:** Multiple goroutines modified the same campaign's integer counters in `apply` at the same time.
- **Fix:** Added `applyMu` and lock around the counter updates in `apply`.
- **Verification:** `go run -race . events.jsonl` completed without race reports, and output matched `expected_output.txt`.
