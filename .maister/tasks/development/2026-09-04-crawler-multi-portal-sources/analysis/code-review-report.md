# Code Review Report

**Date**: 2026-09-10
**Path**: /Users/grzegorzgabryel/Projekty/wok (uncommitted diff, branch `nofluff`)
**Scope**: all (quality, security, performance)
**Status**: ❌ Critical Issues

Reviewed files (production code only, per request):
`internal/category/category.go`, `internal/category/justjoin.go`, `internal/category/nofluff.go`,
`internal/nofluff/fetcher.go`, `internal/nofluff/persist.go`, `internal/nofluff/transformer.go`,
`internal/handlers/helper.go`, `internal/database/queries/possibilities.sql`,
`internal/database/generated/possibilities.sql.go`, `internal/database/generated/querier.go`,
`cmd/crawler/main.go`, `internal/templates/parse.go`, `internal/router/routing_test.go` (routing only),
`go.mod`, `docker/crawler/Dockerfile` (checked for consistency with `go.mod`).

Verified against `internal/nofluff/testdata/reponse_1.json` (real NoFluffJobs API response shape) and
against the equivalent `internal/justjoin/*` implementation used as the project's own reference pattern.

## Summary
- **Critical**: 4 issues
- **Warnings**: 9 issues
- **Info**: 4 issues

## Critical Issues

### C1. NoFluffJobs crawler never persists new offers (inverted "existing offer" check)
**Location**: `internal/nofluff/transformer.go:38-42`
```go
_, err := ot.existingOffers.ExistingPossibilityUrl(ctx, mainUrl)
if err != nil {
    // skip error, our loos but wheel keeps spinning
    continue
}
```
`ExistingPossibilityUrl` (`internal/database/queries/possibilities.sql:12-13`) returns
`pgx.ErrNoRows` precisely when the URL is **not yet in the database** — i.e. for every brand-new
offer. This code treats that as an error and `continue`s, so **new offers are always skipped**.
Only offers that already exist in `possibilities` (where `err == nil`) proceed to `FillOffer` /
`ConvertOffer` / insert, re-fetching and re-upserting data that's already there. Net effect: the
NoFluffJobs source can never add a job that isn't already in the table — the crawler is
functionally a no-op for its stated purpose.

Compare with the project's own reference implementation, `internal/justjoin/transformer.go:103-133`
(`unseen`), which correctly treats a **found** URL as "already known, skip" and an unknown one as
"new, scrape it" — the exact opposite of this logic.

**Fix**: invert the condition — proceed when `err != nil` (not found / new), skip when `err == nil`
(found / already known).

### C2. Only 1 of ~191 postings per category is actually enqueued, plus garbage empty offers
**Location**: `internal/nofluff/fetcher.go:84-107`
```go
dec := jsontext.NewDecoder(resp.Body)
dec.ReadToken()

for dec.PeekKind() != '}' {
    var o RawOffer
    key, _ := dec.ReadToken()
    if key.String() == "postings" {
        dec.ReadToken() // '['
        for dec.PeekKind() != ']' {
            json.UnmarshalDecode(dec, &o)
            o.Category = c.Name
        }
        dec.ReadToken() // ']'
    } else {
        dec.SkipValue()
    }

    select {
    case of.RawOfferBuff <- o:
    case <-ctx.Done():
        return
    }
}
```
The response's top level has 11 keys (`criteriaSearch`, `postings`, `totalCount`, `totalPages`,
`exactMatchesPages`, `rawSearch`, `locationCriteria`, `divs`, `additionalSearch`,
`salaryMatchBlock`, `overridenSalaryFilter` — confirmed against `internal/nofluff/testdata/reponse_1.json`).
The `select` that sends `o` sits **outside** the inner `postings` loop but **inside** the outer
per-key loop, so it fires once per top-level key, not once per posting:
- For the 10 non-`postings` keys, `o` is the zero-value `RawOffer{}` (never populated) — 10 empty,
  garbage offers are pushed onto `RawOfferBuff` per category per fetch.
- For the `postings` key, the inner loop overwrites the same `o` on every one of the 191 elements
  without sending; only the state left after the **last** element is ever sent. **190 of 191
  postings in the sample data are silently dropped.**

**Fix**: move the `select`/send inside the inner `postings` loop (one send per decoded posting),
and don't send anything for non-`postings` keys.

### C3. `OfferTransformer.OffersDb` channel is never closed — graceful shutdown hangs forever
**Location**: `internal/nofluff/transformer.go:34-56`
```go
func (ot *OfferTransformer) Transform(ctx context.Context, buffer <-chan RawOffer) {
	for ro := range buffer {
		...
		ot.OffersDb <- possibility
	}
}
```
When `buffer` (the fetcher's `RawOfferBuff`) closes during shutdown, this loop ends and `Transform`
returns — but `ot.OffersDb` is never closed. Downstream, `OfferPersist.Persist`
(`internal/nofluff/persist.go:24`) does `for c := range ch`, which then blocks forever waiting for
a value or a close that never comes. In `cmd/crawler/main.go:58-71`, `nfp.Persist(nft.OffersDb)`
runs inside the tracked `sync.WaitGroup`, so `wg.Wait()` (`cmd/crawler/main.go:82`) never returns
and the process cannot shut down cleanly on SIGTERM — it requires a hard kill.

Contrast with the project's own pattern in `internal/justjoin/transformer.go:47-53`, which
explicitly does `close(ot.Offers)` after the loop.

**Fix**: `close(ot.OffersDb)` after the `for ro := range buffer` loop in `Transform`.

### C4. `go.mod` requires Go 1.27 but the crawler's Docker builder image is pinned to 1.26.1
**Location**: `go.mod:3` (`go 1.27`) vs `docker/crawler/Dockerfile:2` (`FROM golang:1.26.1-alpine AS builder`)
This directly contradicts the documented backend infrastructure standard ("Go toolchain version
alignment", `.maister/docs/standards/backend/infrastructure.md`). A `go 1.27` directive with a
1.26.1 toolchain forces Go's automatic toolchain-switching to download 1.27 during
`go mod download`/`go build` in the builder stage; if the build environment has restricted network
access (common for hardened CI/production build pipelines) or `GOTOOLCHAIN=local` is set anywhere
in the pipeline, the crawler image build fails outright. This is a production build risk introduced
in this diff (`go.mod` bump from `go 1.26` to `go 1.27` — confirmed via `git diff go.mod`).

**Fix**: bump `docker/crawler/Dockerfile` (and any other Dockerfile building this module) to a
`golang:1.27.x-alpine` base image matching `go.mod`, or pin `go.mod` back to a version the Docker
image actually ships.

## Warnings

### W1. `fetchAll` aborts the whole category loop (silently) on the first per-category error
**Location**: `internal/nofluff/fetcher.go:65-80`
`json.Marshal`, `http.NewRequest`, and `http.DefaultClient.Do` errors all just `return` with no
logging. Because these `return`s are inside `for _, c := range of.categories`, a single failure
(e.g. a transient network blip on category 1) silently aborts the fetch for **every remaining
category** that cycle — no error is logged, and no partial progress is possible. Compare with
`internal/justjoin/fetcher.go:89-104`, which retries with backoff and logs failures per category.

### W2. `Persist` permanently stops on the first DB error
**Location**: `internal/nofluff/persist.go:24-35`
```go
saved, err := op.offerStore.CreatePossibility(ctx, c)
if err != nil {
    log.Printf("Error saving possibility: %v\n", err)
    return
}
```
`return` here exits the `Persist` goroutine for good (it's the body of `for c := range ch`, and
`return` returns from `Persist` itself, not just this iteration). One transient DB error halts all
future persistence for the remainder of the process. Compare with `internal/justjoin/persist.go:25-29`,
which factors the per-item work into `persistOne` so the range loop continues after a failure.

### W3. Per-item `defer cancel()` inside the persist loop leaks until the goroutine exits
**Location**: `internal/nofluff/persist.go:24-33`
```go
for c := range ch {
    ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
    defer cancel()
    ...
}
```
`defer` inside a `for range` loop accumulates — none of these `cancel()` calls actually run until
`Persist` itself returns, so every timeout context created over the process lifetime is kept alive
until shutdown. `internal/justjoin/persist.go` avoids this by delegating to a `persistOne` helper
whose `defer cancel()` fires per call.

### W4. `defer resp.Body.Close()` inside the per-category fetch loop
**Location**: `internal/nofluff/fetcher.go:82`
Same pattern as W3: the `defer` is inside `for _, c := range of.categories`, so response bodies
from earlier categories stay open until `fetchAll` returns rather than being released as soon as
each is processed. With 7 configured categories (`internal/category/nofluff.go:7-15`), up to 7
response bodies can be held open simultaneously within one fetch pass.

### W5. HTTP requests ignore `ctx` and use a client with no timeout
**Location**: `internal/nofluff/fetcher.go:71-80`, `internal/nofluff/transformer.go:58-70`
Both `fetchAll` and `FillOffer` build requests with `http.NewRequest` (not
`http.NewRequestWithContext(ctx, ...)`) and send them via the bare `http.DefaultClient`, which has
no `Timeout`. `ctx` cancellation (e.g. on SIGTERM in `cmd/crawler/main.go`) will not abort an
in-flight request, and a slow/hanging response from nofluffjobs.com can block the fetch/transform
goroutine indefinitely. Additionally, `FillOffer` calls the bare statement `ctx.Done()` twice
(`internal/nofluff/transformer.go:61`, `:68`) — this discards the returned channel and has no
effect; it looks like leftover/incomplete error-handling.

### W6. `posted_at` is always NULL for NoFluffJobs offers
**Location**: `internal/nofluff/transformer.go:92`, `internal/nofluff/transformer.go:97-104`
```go
PostedAt: optDate(strconv.Itoa(offer.PostedAt)),
...
func optDate(s string) pgtype.Date {
	day, err := time.Parse("2006-01-02", strings.TrimSpace(s))
	...
```
`RawOffer.PostedAt` is decoded from the API's `posted` field, which is an epoch-millisecond
timestamp (e.g. `1788260241830`, confirmed in `internal/nofluff/testdata/reponse_1.json`).
Formatting that integer with `strconv.Itoa` and parsing it against the `"2006-01-02"` layout can
never succeed, so `optDate` always hits its error path and returns an invalid/NULL `pgtype.Date`.
Every NoFluffJobs-origin possibility gets `posted_at = NULL`.

**Fix**: use `time.UnixMilli(int64(offer.PostedAt))` (or equivalent) instead of round-tripping
through a date-formatted string.

### W7. `Location`/`Tech` stored as raw JSON, breaking tech stats and the UI
**Location**: `internal/nofluff/transformer.go:89-90`
```go
Location: optText(string(offer.Location)),
Tech:     optText(string(offer.Tech)),
```
`offer.Tech` comes from the API's `tiles` field, which is a JSON object
(`{"values":[{"value":"data","type":"category"}, ...]}`), and `offer.Location` is a nested JSON
object (`{"places":[{...}], "fullyRemote":false, ...}`) — confirmed in the sample response. Storing
these verbatim as text means:
- `StatsPossibilityByCategory` (`internal/database/queries/possibilities.sql:24-32`) does
  `string_to_array(tech, ',')` to aggregate tech tags — comma-splitting a JSON blob produces
  meaningless fragments (e.g. `{"values":[{"value":"data"`, `"type":"category"}`, ...), corrupting
  the tech stats page for any category that has NoFluffJobs offers.
- `formatText` (`internal/handlers/helper.go:64-69`) just prints the raw string, so `Location`
  renders as a raw JSON blob in the UI instead of a human-readable location, unlike JustJoin offers
  (`internal/justjoin/transformer.go:239`, a plain scraped string like `"al. 29 Listopada 20, Krakow"`).

**Fix**: extract the actual tech-tag names (e.g. from `tiles.values[].value`) and a readable
location string (e.g. `location.places[0].city`) before storing, matching the plain-text format
JustJoin already writes into the same columns.

### W8. `Source` is always empty/NULL for NoFluffJobs offers
**Location**: `internal/nofluff/fetcher.go:26` (`Source string \`json:"source"\``), `internal/nofluff/transformer.go:93`
The `RawOffer.Source` field is decoded from a `source` key that does not exist anywhere in the
NoFluffJobs posting payload (confirmed keys: `category, flavors, fullyRemote, help4Ua, highlighted,
id, location, logo, name, onlineInterviewAvailable, posted, reference, regions, renewed, salary,
searchBoost, seniority, technology, tiles, title, topInSearch, url` — no `source`). So
`offer.Source` is always `""`, and `Source: optText(offer.Source)` (`transformer.go:93`) always
stores NULL. `internal/justjoin/transformer.go:27` instead hardcodes `const source = "justjoin.it"`
and sets it explicitly (`justjoin/transformer.go:243`). NoFluffJobs offers should do the same
(e.g. `const source = "nofluffjobs.com"`) rather than relying on a field the API never sends.

### W9. N+1 existence checks and unthrottled per-offer detail fetches
**Location**: `internal/nofluff/transformer.go:34-55`
`Transform` calls `ExistingPossibilityUrl` (one DB round trip) and `FillOffer` (one blocking HTTP
GET) **per offer**, inside a tight loop, with no delay between requests. With ~191 postings per
category across 7 categories (`internal/category/nofluff.go:7-15`), that's on the order of a
thousand+ synchronous DB queries and HTTP requests per fetch cycle. This is both:
- An N+1 query pattern the backend standards call out explicitly
  (`.maister/docs/standards/backend/queries.md` "Avoid N+1") — contrast with
  `internal/justjoin/transformer.go:103-133` (`unseen`), which does one batched
  `ExistingPossibilityUrls` call per category.
- A politeness/reliability risk: `internal/justjoin/transformer.go:86-94` explicitly rate-limits
  with a 5-12s random pause "to be polite to the server" and to avoid "a transient problem [turning]
  into a hammering loop"; `nofluff.Transform` has no equivalent delay at all, risking throttling or
  an IP ban from nofluffjobs.com.

## Informational

### I1. Leftover commented-out debug URLs
**Location**: `internal/category/nofluff.go:28-31`
```go
//https://nofluffjobs.com/pl/job/senior-data-platform-engineer-python-or-go-link-group-warszawa-1


// https://nofluffjobs.com/api/posting/senior-data-platform-engineer-python-or-go-link-group-warszawa-1
```
Dead/commented-out exploration notes left in production code; remove per the project's "No Dead
Code" coding-style standard.

### I2. Fetch jitter computed once instead of per cycle
**Location**: `internal/nofluff/fetcher.go:45-46`
```go
func (of *Fetcher) FetchOffers(ctx context.Context) {
	wait := frequency + rand.N(maxFrequencyJitter)
	for {
		of.fetchAll(ctx)
		...
```
`wait` is drawn once before the loop and reused for every cycle, so the "jitter" doesn't actually
vary between passes. `internal/justjoin/fetcher.go:44-58` redraws `wait` inside the loop on every
pass specifically "so the crawl doesn't settle into a fixed rhythm the site could recognise" — the
same rationale applies here.

### I3. `RawOffer.MainTech` is captured but never used
**Location**: `internal/nofluff/fetcher.go:24` (`MainTech jsontext.Value \`json:"technology"\``)
Nothing in `transformer.go`'s `ConvertOffer` reads `offer.MainTech`. Either use it or drop the
field per the project's minimal-implementation standard ("unused code is debt").

### I4. Repeated typo in comments
**Location**: `internal/nofluff/transformer.go:41, 46, 51`
`// skip error, our loos but wheel keeps spinning` appears three times — likely meant "our loss".
Minor, but worth a single fix-and-reuse (or just deleting the comment, since the behavior is being
changed per C1 anyway).

## Metrics
- Max function length in reviewed files: ~50 lines (`nofluff/fetcher.go: fetchAll`)
- Max nesting depth: 4 (`fetcher.go` decode loop: for → if → for → select)
- Functional/correctness defects (Critical): 3 (C1-C3) + 1 build-risk (C4)
- N+1 query risks: 1 (W9)
- Fields silently mismatched against the real upstream API shape: 2 (`posted_at` mis-parsed — W6;
  `source` never populated — W8)

## Prioritized Recommendations
1. Fix the decode loop in `fetcher.go` (C2) — this is the root reason almost no NoFluffJobs data
   reaches the database at all.
2. Invert the existing-offer check in `transformer.go` (C1) — even with C2 fixed, this alone
   prevents any new offer from ever being persisted.
3. Close `OffersDb` after the fetch loop in `transformer.go` (C3) — required for the crawler to
   shut down cleanly; currently masked by C1/C2 producing almost no traffic through the pipeline.
4. Align the crawler's Docker builder Go version with `go.mod` (C4) before this ships, to avoid a
   broken production build.
5. Fix `posted_at` (W6) and extract plain-text `Location`/`Tech`/`Source` (W7, W8) — otherwise even
   once offers persist correctly, their data will be wrong or will corrupt the existing tech-stats
   feature.
6. Replace the per-category abort-on-error (W1) and permanent-stop-on-error (W2) patterns with
   per-item/per-category error handling that logs and continues, matching `internal/justjoin`'s
   established pattern in this codebase.
7. Add request-scoped context and a client timeout (W5), batch the existence check and add a
   politeness delay between per-offer requests (W9), and fix the two loop-scoped `defer` leaks
   (W3, W4).
8. Clean up the informational items (I1-I4) while in the area.
