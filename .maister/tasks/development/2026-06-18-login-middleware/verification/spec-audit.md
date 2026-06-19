# Specification Audit: Login Authentication Middleware

**Audited spec**: `.maister/tasks/development/2026-06-18-login-middleware/implementation/spec.md`
**Audit type**: Pre-implementation readiness audit (no code for this feature exists yet)
**Method**: Independent verification — every claim re-derived from source files, supplemented by empirical tests run against the exact pinned `go-chi/chi/v5 v5.3.0` module and Go's `html/template`/`embed` packages (not from memory or the spec's own assertions).

---

## Summary

The spec is well-structured and the two highest-risk technical claims flagged for this audit are **both correct**, verified empirically:

1. **Chi routing claim (CONFIRMED CORRECT)**: A static route (`/l0g1n`) registered on the same `chi.Mux` as a `/*` wildcard always wins, **regardless of registration order**. Verified by writing and running two Go tests against the exact pinned version `github.com/go-chi/chi/v5 v5.3.0` (from `go.mod`) — one registering the wildcard first then the static route, one the reverse order. Both resolved to the static handler. Chi's underlying radix/trie router (`httprouter`-style) ranks node types by specificity (static > param > wildcard) independent of insertion order, so this holds even though `routing.go`'s catch-all `r.Post("/*", ...)` (line 79) currently sits physically before where the new `r.Post("/l0g1n", ...)` would be added near line 65.
2. **Header template claim (CONFIRMED CORRECT)**: Verified empirically by reconstructing the exact `embed.FS` + `template.ParseFS(FS, "base.gohtml", "partials/*.gohtml")` + `ExecuteTemplate(w, "base", data)` call shape used in `pages_handler.go:11,19`. Adding a `partials/header.gohtml` with `{{ define "header" }}...{{ end }}` resolves `base.gohtml`'s existing `{{ template "header" . }}` (line 5) cleanly, with zero changes to `embed.go` or the `ParseFS` glob. `internal/templates/embed.go:5` already embeds `partials/*`, and the `partials/` directory already exists (currently empty), so no path/embed errors occur.

Both of the spec's two riskiest claims survive rigorous, tool-verified scrutiny. The remaining issues are in the **session/cookie contract** (some real ambiguity, mostly Low/Medium) and in the **Testing Approach section's lack of a concrete enumerated test list**, which is a real spec gap relative to its own internal forward-references ("see Test Requirements").

**Overall Readiness: ⚠️ Mostly Compliant / Ready with minor fixes** — the spec is implementable as-is by a competent planner who fills a few small gaps with reasonable defaults, but two Medium-severity ambiguities (Validate's return contract, cookie MaxAge vs. session TTL sync) and one structural gap (no actual "Test Requirements" section despite being referenced twice) should be patched before handing to the implementation-planner, to avoid the planner guessing or under-sizing test work.

---

## Critical Issues

None. No Critical-severity findings — both candidate "could break production" claims (Chi routing, header template) were verified correct.

---

## Important Gaps

### Finding 1: "Test Requirements" section referenced but does not exist

**Spec Reference**: spec.md line 26 ("...placed so they resolve before the catch-all... — verify with a test, see Test Requirements") and line 71 ("...this must be exercised by a test... see Test Requirements").

**Evidence**: Read the full spec.md (114 lines). There is no section literally titled "Test Requirements" anywhere in the document. The only test-related content is:
- "Testing Approach" subsection (lines 80-83): three bullet points about test density (2-8 per group), stdlib-only tooling, and per-group verification scope — no enumeration of test cases.
- "Standards Compliance" subsection, `testing/test-writing.md` bullet (line 92): two example test names (`TestRequireLogin_RedirectsWhenSessionMissing`, `TestRequireLogin_ExemptsLoginPath`) plus general guidance about Redis-skippability.
- Requirement 14 (line 28): "Tests for every new file (see Test Requirements section)" — another forward-reference to the same nonexistent section.
- Success Criteria (line 112): "All new files... have passing tests (2-8 per file/group) covering their stated behaviors" — no specifics.

No section anywhere lists concrete test cases per file (e.g., "RedisSessionRepository: Create stores with TTL; Validate returns true for live token; Validate returns false for expired/missing token; Revoke deletes key").

**Category**: Missing (the section is referenced by name three times but never written — this reads like a drafting artifact, as if an earlier outline had a "Test Requirements" section that was deleted/never filled in but the cross-references were left in place).

**Severity**: High. An implementation-planner sizing task groups by "distinct testable units, edge cases, and effort" (per this audit's mandate) has only two worked examples and a density range (2-8) to go on — for 5 new files (`session_repository.go`'s 2 impls, `auth_middleware.go`, `auth_handler.go` x2 handlers) this is a 10-40 test range with no per-file breakdown of *which* edge cases matter (e.g., is "Validate called with empty token string" a required test? Is "Revoke called twice / revoke of nonexistent token" required? Is the Chi-routing-precedence test itself supposed to live in `routing_test.go`, a new file not mentioned anywhere in the "files to create" list?). The planner will have to invent this breakdown rather than read it off the spec, which is exactly the kind of guessing this audit exists to catch. This is not a blocking defect (a competent planner can derive a reasonable list from the Core Requirements and Success Criteria sections), but it is a concrete, fixable gap.

**Recommendation**: Either (a) add a real "Test Requirements" section enumerating test cases per new file (mirroring the level of detail already present in Success Criteria), or (b) remove the two forward-references on lines 26 and 71 and replace with a generic pointer to "Testing Approach" + "Success Criteria," so the spec doesn't promise content it doesn't contain.

---

### Finding 2: `SessionRepository.Validate` return contract is underspecified

**Spec Reference**: spec.md line 13 ("`SessionRepository` interface... with `Create`, `Validate`, `Revoke` methods, mirroring `IpRepository`'s shape") and line 72 ("`RequireLogin` reads the same cookie name on every request and calls `sessionRepo.Validate(ctx, token)`").

**Evidence**: `internal/repository/ip_repository.go:5-8` defines `Exists(ctx, ip) (bool, error)` — a pure existence check, no payload returned. The spec says `Validate` should mirror this shape but never states the exact signature. Two readings are both consistent with the spec's prose:
- `Validate(ctx, token) (bool, error)` — matches `Exists` exactly (boolean only).
- `Validate(ctx, token) (valid bool, userID string, err error)` or similar — the codebase-analysis.md (line 21, an untrusted input document) speculates `(valid bool/userID, err error)`, showing this ambiguity was already noticed during analysis and never resolved in the spec itself.

Since this is a single-hardcoded-user app (no user ID concept exists anywhere — `Config` has no `UserID` field, `LOGIN_USER` is a username string compared directly), a payload-returning `Validate` would have nothing meaningful to return beyond the boolean. The spec's Core Requirements (line 13) and Technical Approach (line 72) never pin down the exact signature with a code-shaped declaration the way line 19's bcrypt call (`bcrypt.CompareHashAndPassword([]byte(...), []byte(...))`) is pinned down.

**Category**: Ambiguous.

**Severity**: Medium. For a single-user app the most natural reading is `Validate(ctx, token) (bool, error)` (exactly mirroring `Exists`), and a planner/implementer will likely land there without guidance. But "mirroring IpRepository's shape" is stated as a *requirement* (line 13) without giving the actual method signature the way `Create`'s contract is partially pinned down in the Technical Approach (line 72: caller generates token, TTL applied inside repo). Leaving one of three interface methods without an explicit signature, when the other two are effectively pinned, is an inconsistency in specification depth.

**Recommendation**: Add an explicit signature line, e.g.: `Validate(ctx context.Context, token string) (bool, error)` — same shape as `IpRepository.Exists`, no user/session payload needed since there's only one user.

---

### Finding 3: Cookie expiration (`MaxAge`/`Expires`) not synced with session TTL — gap, not just omission

**Spec Reference**: spec.md line 73 ("Cookie attributes: `HttpOnly: true`, `Secure: true`, `SameSite: http.SameSiteStrictMode`, `Path: "/"`.") — no `MaxAge` or `Expires` field listed. Compare to line 14's `sessionTTL = 24 * time.Hour` server-side TTL.

**Evidence**: `redis_ip_repository.go:21` shows the existing pattern: `r.rdb.Set(ctx, "banned_ip:"+ip, reason, banTime).Err()` — Redis TTL is set server-side via the `EX`-equivalent argument to `Set`. The spec correctly says `sessionTTL` should be applied the same way inside `RedisSessionRepository.Create`. But cookies are independently time-bounded by the browser via `MaxAge`/`Expires`; without one, Go's `net/http.Cookie` defaults to a **session cookie** (deleted when the browser closes), not a 24-hour-lived cookie. This creates a real behavioral mismatch: the server-side session lives 24 hours, but the cookie the browser holds it in could vanish on browser-close (forcing an earlier-than-intended re-login) — the opposite-direction risk (cookie outliving server session) isn't a problem since `Validate` would reject an expired/missing server-side session regardless, but the spec's Success Criteria don't address the browser-session-cookie-vs-24h-mismatch at all.

**Category**: Missing.

**Severity**: Medium. Not a security issue (worst case is extra unwanted re-logins, not unauthorized access — `Validate` is still authoritative), but it's a real functional gap relative to the stated 24-hour-TTL intent in line 14/User Story 3 ("instead of waiting for the 24-hour TTL to expire" implies the user expects to *stay* logged in for 24 hours across browser restarts, which a session-only cookie would violate).

**Recommendation**: Add `MaxAge: int(sessionTTL.Seconds())` to the cookie attribute list in the Technical Approach section (line 73), so cookie lifetime matches `sessionTTL`.

---

## Minor Discrepancies

### Finding 4: In-memory session store has no TTL/expiry sweep — spec doesn't address whether this matters

**Spec Reference**: spec.md line 14 ("mirroring `redis_ip_repository.go` / `in_memory_ip_repository.go` exactly... TTL via a hardcoded `sessionTTL` constant").

**Evidence**: `internal/repository/in_memory_ip_repository.go:16-19` shows `Add` just does `r.ips[ip] = reason; return nil` — **no TTL is ever applied or checked** in the in-memory implementation; entries live forever until process restart. The spec's instruction to mirror this "exactly" for `InMemorySessionRepository` would produce a session store where sessions, once created, **never expire** in the in-memory (Redis-down) fallback path — directly contradicting the 24-hour-TTL requirement (line 14) and User Story 3 for that specific deployment mode. The existing `IpRepository` in-memory fallback has the same latent property (bans never expire when Redis is down), but that's a pre-existing, lower-stakes issue (over-banning is fail-safe-ish); for sessions, "never expires in fallback mode" is a more meaningful security/UX deviation (sessions outlive their stated TTL indefinitely whenever Redis is unavailable).

**Category**: Incomplete (the spec says "mirror exactly" but the source being mirrored doesn't actually implement TTL at all for the in-memory case, so faithful mirroring silently drops the TTL requirement for one of the two backends).

**Severity**: Low-Medium. Downgraded from Medium because: (a) Redis is the expected primary backend in normal operation (per tech-stack.md), in-memory is an explicit fallback-only path; (b) this is a single-personal-user app, so an unbounded-lifetime session in a degraded-infra scenario is a narrow window, not a multi-tenant risk. Still worth a one-line decision in the spec so the implementer doesn't have to choose silently between "store an expiry timestamp and check it on `Validate`" vs. "accept the same fire-and-forget pattern as `InMemoryIpRepository`."

**Recommendation**: Add a one-line note to Technical Approach: either explicitly accept the no-expiry-in-fallback-mode behavior (consistent with existing `InMemoryIpRepository` precedent, and document why that's acceptable for sessions) or require storing a `time.Time` expiry alongside the token and checking it in `Validate`/lazily evicting in `Exists`-equivalent. Either is fine; the spec should pick one rather than leaving "mirror exactly" to implicitly resolve it.

---

### Finding 5: "Session not found" vs "expired" vs "malformed cookie" are not distinguished, though this is likely fine

**Spec Reference**: spec.md line 16 (middleware "validates the session cookie via `sessionRepo.Validate`, and on missing/invalid session issues... redirect").

**Evidence**: The spec treats "missing cookie," "malformed cookie value," "valid-format but nonexistent token," and "expired token" (Redis: key naturally gone; in-memory: per Finding 4, never expires) all as a single "invalid" bucket that triggers the same redirect. This is consistent with `CheckIP`'s existing all-errors-are-non-fatal-and-logged style (`check_middleware.go:23-25`) and is a reasonable, minimal design for a single-user app. Flagging only because Risk Area 3 in the audit brief specifically asked about this distinction.

**Category**: N/A — not a gap, the spec's implicit choice (collapse all failure modes to one redirect) is consistent with the codebase's existing error-handling style and the `error-handling.md` standard's "non-fatal logging in handlers" guidance.

**Severity**: Low (documentation clarity only — the spec could state explicitly "all Validate-related failures, including malformed cookies and `r.Cookie` returning `http.ErrNoCookie`, are treated identically" to remove any doubt, but this is a "would be nice" not a real ambiguity risk).

**Recommendation**: Optional — add one sentence to Technical Approach confirming the all-failures-collapse-to-redirect behavior explicitly, for the implementer's confidence.

---

### Finding 6: bcrypt dependency claim — confirmed correct, slightly underspecified on go.sum mechanics

**Spec Reference**: spec.md line 25 ("New dependency `golang.org/x/crypto/bcrypt`, added via `go get golang.org/x/crypto` (direct dependency, not `// indirect`)").

**Evidence — independently verified**:
- `go.mod` (read directly): 5 requires, all marked `// indirect` (including `go-chi/chi/v5`, which is actually used directly in `routing.go` — a pre-existing oddity in this repo's `go.mod`, not something this spec needs to fix).
- `go.sum`: `grep -i crypto go.sum` returned **zero matches** (exit code 1) — confirms `golang.org/x/crypto` is not present even as a transitive dependency today. The spec's framing of this as "a new dependency" (not "promoting an existing indirect one") is **correct** — there is nothing to promote, gap-analysis.md's claim of the same is also verified correct independently.

**Category**: N/A (claim verified accurate, no gap). Noted as a Minor Discrepancy slot only to record the verification, since the audit brief asked for it explicitly.

**Severity**: N/A.

---

### Finding 7: Config struct fields claim — confirmed correct

**Spec Reference**: spec.md line 65 ("`Config` only has `RedisAddr`, `WebAddr`, `SentryDsn`").

**Evidence**: `internal/config/loader.go:10-14` — `Config` struct has exactly `RedisAddr string`, `WebAddr string`, `SentryDsn string`. Claim is accurate, verbatim match.

**Category**: N/A (verified accurate).

---

### Finding 8: main.go Redis-or-fallback block location claim — off by a few lines but structurally correct

**Spec Reference**: spec.md line 56 ("`cmd/web/main.go` lines 27-38 — Redis-or-fallback selection block").

**Evidence**: `cmd/web/main.go` — the actual block spans lines 27-38 exactly: `var ipRepo repository.IpRepository` (27) through the closing `}` of the else-branch (38). This is precisely accurate — the try/catch-style Redis connection, warning log, and `NewInMemoryIpRepository()` fallback vs. `NewRedisIpRepository(rdb)` success path is all there. Replicating this block for `sessionRepo` is mechanically straightforward and the line numbers are correct as of the current repo state.

**Category**: N/A (verified accurate).

**Severity**: N/A — included for completeness per audit brief request.

---

## Clarification Needed

### Q1: Exact `SessionRepository.Validate` signature
Per Finding 2 — should the spec pin this down as `Validate(ctx context.Context, token string) (bool, error)`, exactly mirroring `IpRepository.Exists`? This audit recommends yes, but the spec author/user should confirm since requirement 1 (line 13) leaves it implicit.

### Q2: Should the in-memory `SessionRepository` fallback actually enforce `sessionTTL`, or is "lives forever until process restart" (mirroring `InMemoryIpRepository`'s actual behavior) acceptable for sessions specifically?
Per Finding 4 — these have different risk profiles (over-banning vs. over-long sessions), and the spec's "mirror exactly" instruction doesn't address the divergence. A decision either way is fine; it just needs to be made explicit so test cases for "session expiry in in-memory mode" are scoped correctly (either tested-and-required, or explicitly out of scope).

### Q3: Should cookie `MaxAge` be added to match `sessionTTL`?
Per Finding 3 — recommend yes (`MaxAge: int(sessionTTL.Seconds())`), but confirm intent: is a 24-hour *persistent* cookie desired (survives browser restart), or is "must re-login every time the browser is closed, but no later than 24h" an acceptable/intended UX given the personal-single-user threat model? Either is defensible; the spec should say which.

---

## Extra Features

None identified. Every capability the spec describes (`SessionRepository`, `RequireLogin`, login/logout handlers, two new templates, config fields, env template entries, bcrypt dependency, routing changes, main.go wiring) traces directly to a Core Requirement, a resolved clarification, or a stated Success Criterion. Nothing in the spec appears speculative or unrequested — consistent with the project's `minimal-implementation.md` standard, and the spec explicitly calls out several things it is *not* doing (Out of Scope section, lines 94-102) which is good practice.

---

## Detailed Verification of the Two Flagged High-Risk Claims

### Claim A — Chi v5 routing precedence (spec.md lines 16, 26, 71)

**Spec claim**: explicit `r.Get("/l0g1n", login)`, `r.Post("/l0g1n", login)`, `r.Post("/logout", logout)` will resolve ahead of the existing `r.Post("/*", ...)` catch-all ban route "regardless of registration order... Chi already prefers more specific patterns over wildcards."

**Verification method**: Wrote and ran two standalone Go tests against the exact pinned dependency version from this repo's `go.mod` (`github.com/go-chi/chi/v5 v5.3.0`):

```go
// Test 1: wildcard registered BEFORE static route (does NOT mirror current routing.go,
// included to test true order-independence)
r.Post("/*", wildcardHandler)
r.Post("/l0g1n", staticHandler)
// Result: POST /l0g1n -> staticHandler  (PASS)

// Test 2: static route registered BEFORE wildcard
r.Post("/l0g1n", staticHandler)
r.Post("/*", wildcardHandler)
// Result: POST /l0g1n -> staticHandler  (PASS)
```

Both tests passed — `hit = "static"` in both orderings. This is run against the *actual* trie-based router implementation pinned in this project, not inferred from documentation or training-data recall.

**Verdict: The spec's claim is definitively CORRECT.** Chi v5's routing tree ranks node types by specificity (static segment > named param > catch-all wildcard) as a property of the tree structure itself, not registration order — confirmed by direct execution, not by reading source and reasoning about it. This holds for same-method (`POST`), same-router-instance, no-`Group`/`With`-scoping cases, which is exactly the situation in `routing.go` (no groups or `With`-scoped sub-routers are used anywhere in the file — confirmed by reading `routing.go` in full, no `r.Group(` or `r.Route(` calls exist).

**Caveat worth flagging to the implementer** (not a spec defect, just a note): this guarantee holds only because the new routes and the existing wildcard live on the *same* `chi.Mux` with no `Group`/`Route`/`With` scoping introduced between them. If a future refactor wraps the `/l0g1n`/`/logout` registrations or the wildcard bans in separate `r.Group(...)` sub-routers, the precedence-over-wildcard guarantee still holds *within* chi's matching algorithm (group-scoping doesn't change node specificity ranking), but it's worth the spec's own recommended test (line 71) actually existing in the codebase as a regression guard — see Finding 1, this is one of the concrete cases that the missing "Test Requirements" section should explicitly enumerate (e.g. `TestRouting_LoginPOSTNotInterceptedByCatchAllBan`).

---

### Claim B — `partials/header.gohtml` fix via existing `ParseFS` pattern (spec.md lines 21, 44, 63, 75, 109)

**Spec claim**: adding `partials/header.gohtml` with `{{ define "header" }}` will satisfy `base.gohtml`'s existing `{{ template "header" . }}` (line 5), via the existing `template.ParseFS(templates.FS, "base.gohtml", "partials/*.gohtml")` pattern, with base.gohtml "unmodified" and without breaking the existing landing page render.

**Verification method**: Independently read all three relevant files in full:
- `internal/templates/base.gohtml` (11 lines) — confirmed line 5 is `{{ template "header" . }}`, and the file's only top-level definition is `{{ define "base" }}`.
- `internal/templates/embed.go` (9 lines) — confirmed `//go:embed base.gohtml partials/* other/*` already embeds the entire `partials/` directory; no changes needed here, matching the spec's claim exactly.
- `internal/router/pages_handler.go` (`landing`, lines 10-23) — confirmed the exact call shape: `template.ParseFS(templates.FS, "base.gohtml", "partials/*.gohtml")` then `tmpl.ExecuteTemplate(w, "base", nil)`.
- Confirmed via `ls`/`find` that `internal/templates/partials/` exists as a directory but is currently **empty** — so the `partials/*.gohtml` glob currently matches zero files, meaning `landing`'s current `ExecuteTemplate(w, "base", nil)` call today either errors at execute time (undefined template "header") or — per Go's `html/template` semantics — actually **does error**, since `{{ template "header" . }}` with no `"header"` definition anywhere in the parsed template set is a hard execute-time error (`template: "header" is undefined`), not a silent no-op. This confirms the gap-analysis.md's characterization of this as a live, pre-existing bug (not hypothetical) is accurate — `landing` is broken today before any of this feature's code is added.

Then reconstructed the exact mechanism end-to-end in an isolated Go module, using a real `embed.FS`, a real two-pattern `ParseFS` call, and a real `ExecuteTemplate(w, "base", ...)` call — bit-for-bit matching the production call shape:

```go
//go:embed base.gohtml partials/*
var FS embed.FS
...
tmpl, err := template.ParseFS(FS, "base.gohtml", "partials/*.gohtml")
err = tmpl.ExecuteTemplate(os.Stdout, "base", nil)
```

With `partials/header.gohtml` containing `{{ define "header" }}<header>BRAND</header>{{ end }}`, this **executed successfully** and rendered the header content inline at the `{{ template "header" . }}` call site, with no parse or execute errors, and no changes needed to the glob pattern or to `base.gohtml`.

**Verdict: The spec's claim is definitively CORRECT**, on all sub-claims:
- `embed.go`'s glob already covers `partials/*.gohtml` — no embed change needed. **Confirmed.**
- `ParseFS`'s two-pattern call (`"base.gohtml"`, `"partials/*.gohtml"`) does associate templates defined via `{{ define }}` in glob-matched files into the same template set as the `{{ template "header" . }}` call site in `base.gohtml` — Go's `html/template`/`text/template` associated-template mechanism works exactly as the spec assumes; this is standard, well-documented `ParseFS`/`ParseGlob` behavior (all files passed to a single `Parse*` call share one `*template.Template` namespace), confirmed by direct execution rather than assumed. **Confirmed.**
- No naming collisions or multiple-definition errors arise from this specific addition (only one new file defines `"header"`; nothing else in the glob currently defines that name since `partials/` is empty today). **Confirmed — though see note below.**
- `base.gohtml` itself needs zero modification. **Confirmed** — the existing `{{ template "header" . }}` reference was already forward-compatible; it was simply missing its dependency, which is precisely what `partials/header.gohtml` supplies.
- `pages_handler.go`'s `ExecuteTemplate` call needs zero modification — confirmed by the test reproducing the exact call and succeeding without changes to the execute-call shape.

**One caveat worth surfacing** (not a defect in the spec's claim, but worth the implementer's awareness): because `partials/login.gohtml` will *also* be added in the same glob (`partials/*.gohtml`), and both `login.gohtml` and `header.gohtml` will be parsed into the *same* template set whenever `login`/`logout` handlers call the identical `ParseFS(..., "partials/*.gohtml")` pattern (per spec line 44's claim that both new templates use "the same... pattern already used by `landing`"), any `{{ define }}` name collision between `login.gohtml` and `header.gohtml` (e.g., if both accidentally defined a template literally named `"header"`) would cause a hard parse-time panic/error ("template: multiple definits of template..."). This is a non-issue as long as the implementer gives `login.gohtml`'s top-level define a distinct name (e.g., `{{ define "login" }}` or just unnamed content blocks), which the spec implies but does not explicitly forbid collisions on. Low severity, mentioned for completeness — not a defect, just a naming-discipline note for whoever writes the templates.

---

## Recommendations (Prioritized)

1. **(High)** Resolve Finding 1 — either write a real "Test Requirements" section with an enumerated per-file test list, or strip the two dangling forward-references (lines 26, 71) so the spec doesn't cite content that doesn't exist. This is the single most actionable fix before handing to an implementation-planner.
2. **(Medium)** Resolve Finding 2 — pin down `Validate`'s exact signature (`(bool, error)` recommended) in the Core Requirements section.
3. **(Medium)** Resolve Finding 3 — add `MaxAge: int(sessionTTL.Seconds())` (or explicitly state the intentional session-cookie-only design) to the cookie attributes list in Technical Approach.
4. **(Low-Medium)** Resolve Finding 4 — make an explicit call on in-memory session TTL enforcement (enforce vs. accept never-expires-until-restart, matching `InMemoryIpRepository`'s actual current behavior) and state it in Technical Approach.
5. **(Low)** Optional: add one sentence confirming all `Validate`-failure modes (missing cookie, malformed value, nonexistent/expired token) are treated identically by `RequireLogin` (Finding 5) — purely for implementer confidence, not a real ambiguity.
6. No action needed on the two flagged headline risks (Chi routing precedence, header template fix) — both are verified correct and implementable as written.

---

## Compliance / Readiness Status

**⚠️ Mostly Compliant — Ready to implement after 2-3 small spec patches.**

- No Critical findings.
- One High finding (missing "Test Requirements" section content despite being referenced) — should be fixed before implementation-planning to avoid the planner under- or over-sizing test work from guesswork.
- Three Medium/Low-Medium findings, all in the session/cookie contract area, each fixable with a one-to-two-sentence addition to the existing Technical Approach section.
- Zero Extra/unrequested features.
- The two specific claims this audit was commissioned to scrutinize most rigorously (Chi v5 routing precedence and the header-template-via-ParseFS fix) are both **empirically confirmed correct** against the exact dependency versions and call patterns this codebase actually uses — these are not analogical or training-data-recalled conclusions, they were verified by running real code against the pinned `chi v5.3.0` module and reconstructing the exact `embed.FS`/`ParseFS`/`ExecuteTemplate` call chain from `pages_handler.go`.
