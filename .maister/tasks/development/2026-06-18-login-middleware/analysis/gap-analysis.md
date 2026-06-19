# Gap Analysis: Add Login Authentication to WOK

## Summary
- **Risk Level**: Medium
- **Estimated Effort**: Medium
- **Detected Characteristics**: modifies_existing_code, creates_new_entities, involves_data_operations, ui_heavy

## Task Characteristics
- Has reproducible defect: no
- Modifies existing code: yes (routing.go, main.go, config/loader.go, .env.default, go.mod)
- Creates new entities: yes (SessionRepository, auth middleware, login handler, login template, Config credential/session fields)
- Involves data operations: yes (session CRUD: Create/Validate/Revoke against Redis or in-memory store)
- UI heavy: yes (new login form page, no existing form/partial pattern to mirror)

## Gaps Identified

### Missing Features (none exist today — confirmed by direct read of all touched files)
- **No authentication concept anywhere**: no session, cookie, credential, or user concept in the codebase. `internal/repository/` has only `IpRepository`. `internal/router/` has only `CheckIP` middleware and `landing`/`ban`/`robotsTxt` handlers.
- **No `/l0g1n` route registered**: `internal/router/routing.go:65` has it as a dead comment (`//r.Get("/l0g1n", landing)`), pointing at `landing` (the home handler) — not a real login handler. Must be replaced with real GET+POST registrations.
- **No global auth middleware**: `routing.go:49-54` chain is `RealIP → CheckIP → Timeout → Logger → Recoverer → SentryHandler` — nothing checks session state today. All routes are effectively public except the IP-ban gate.
- **No bcrypt dependency**: verified `go.mod` (5 deps, all `// indirect`, no `golang.org/x/crypto`) and `go.sum` has zero `x/crypto` lines. Clarification #1 requires bcrypt — this is a **new third-party dependency**, not a stdlib-only addition as the codebase-analysis report assumed ("0 new third-party deps... stdlib `crypto/rand`"). `go.mod`/`go.sum` will need `go get golang.org/x/crypto` and the `// indirect` comment convention should be dropped for a now-direct dependency (existing deps are all marked `// indirect`, which is itself unusual/likely a pre-existing oddity in this repo — worth flagging but not fixing here).
- **No login template/form pattern exists**: `internal/templates/partials/` directory is **empty** (verified via `ls`). There is no existing form, no CSS/styling convention, no input-field naming convention anywhere in the project to mirror. This is fully greenfield UI work, contrary to the codebase-analysis report's framing of "mirror landing handler" — the *handler* pattern can be mirrored, but the *form markup* cannot.
- **Pre-existing latent gap discovered**: `internal/templates/base.gohtml:5` references `{{ template "header" . }}`, but no file defines a `"header"` template anywhere in `internal/templates/`. This means the existing `landing` handler likely already fails at `ExecuteTemplate` time today (or silently no-ops the block) — this is a pre-existing bug, not caused by this task, but the new login handler will hit the exact same broken dependency if it reuses `base.gohtml` via `ExecuteTemplate(w, "base", data)`. **Flagging as out-of-scope but blocking-adjacent**: implementer should verify whether `landing` actually works today before assuming `base.gohtml` is a safe foundation for the login page.

### Incomplete Features
- N/A — no partial implementation exists; this is pure greenfield (additive) work layered onto existing middleware/handler/repository patterns.

### Behavioral Changes Needed
- `Router` struct (`routing.go:13-16`) gains a `sessionRepo` field and `New()` gains a second parameter — this is a **breaking signature change**, but verified only one call site exists (`cmd/web/main.go:49`), so blast radius is contained.
- `Config` struct (`config/loader.go:10-14`) gains new fields for `LOGIN_USER`, `LOGIN_PASS_HASH`, and session TTL — additive, no breaking change.
- Middleware chain order: new `RequireLogin` middleware must be inserted via `r.Use(...)` per clarification #2 (global, not chi.Group-scoped — this **overrides** the codebase-analysis report's recommendation #6, which suggested `chi.Group` scoping instead of global `r.Use`). Confirmed clarifications.md explicitly resolves this in favor of global + self-exempting middleware.

## User Journey Impact Assessment

| Dimension | Current | After | Assessment |
|-----------|---------|-------|------------|
| Reachability | No login exists; `/` is open to anyone | Unauthenticated visit to `/` (or any path) redirects to `/l0g1n` | ✅ — standard PRG redirect pattern, matches `CheckIP`'s existing style |
| Discoverability | N/A | `/l0g1n` is a deliberately obfuscated, undocumented path (not linked anywhere, decoy `/login` exists in `banUri`) | ⚠️ intentional by design — discoverability is *deliberately low* for outsiders (security-by-obscurity), but the legitimate single user must remember/bookmark the exact path. This is an accepted tradeoff per the project vision (IP-blocking, obfuscated paths), not a defect. |
| Flow Integration | None | After login, POST handler should redirect somewhere sensible (presumably `/`) | ⚠️ — codebase-analysis recommends redirect to `/` after login; not contradicted by clarifications, but not explicitly confirmed. Low risk, sensible default. |

Note: standard "is the feature discoverable via navigation" framing applies loosely here since this is a single-personal-user app with no menu/nav system at all (no header/nav template exists in the codebase). Discoverability scoring (1-10) is not meaningfully applicable — there is no UI navigation system to integrate into.

## Data Lifecycle Analysis

### Entity: Session

| Operation | Backend | UI | Access | Status |
|-----------|---------|-----|--------|--------|
| CREATE | `SessionRepository.Create(ctx, ...) (token, error)` — to be added, mirrors `IpRepository.Add` | POST handler on `/l0g1n` sets cookie after `Create` succeeds | Triggered by login form submit | ✅ planned, not yet orphaned — all 3 layers covered in the plan |
| READ (validate) | `SessionRepository.Validate(ctx, token) (bool, error)` | Middleware reads cookie on every request | Implicit, every request | ✅ planned |
| DELETE (revoke/logout) | `SessionRepository.Revoke(ctx, token) error` — **interface method proposed by codebase-analysis report but NOT in scope per the task description** | **No logout button/route specified anywhere in task, clarifications, or codebase-analysis recommendations beyond the interface stub** | **None** | ❌ **ORPHANED if `Revoke` is implemented but no route/button calls it** |
| UPDATE (TTL refresh / re-auth) | Not discussed | Not discussed | Not discussed | N/A — not requested, no gap (simply out of scope) |

**Completeness**: 75% (3 of 4 standard CRUD-style operations are clearly speced; DELETE/logout is ambiguous)

**Orphaned Operations**: `Revoke` method, if implemented per the codebase-analysis report's suggested interface shape (`Create`/`Validate`/`Revoke`), would have backend capability with **zero UI or route to trigger it** — classic orphaned-write scenario (CREATE without corresponding user-facing DELETE path). This is a real risk: the task description never mentions "logout" at all.

**Missing Touchpoints**: Logout route/button. For a single-personal-user app this is lower-priority than a multi-user system, but its complete absence from the task scope is worth flagging — sessions would only end via TTL expiry, never explicit user action.

## Defect Analysis
Not applicable — `has_reproducible_defect` is false. This is new capability work, not a bug fix.

## Issues Requiring Decisions

### Critical (Must Decide Before Proceeding)

1. **Logout functionality scope**: The task description, codebase-analysis report, and clarifications.md never explicitly state whether logout is in scope. The codebase-analysis report's *suggested* `SessionRepository` interface includes a `Revoke` method "for completeness," which — if implemented without a corresponding route — becomes a textbook orphaned operation (backend capability, zero user access).
   - Options:
     a. **In scope**: Add `Revoke` to the interface AND add a `/logout` (or similarly obfuscated) route + a visible link/button to call it.
     b. **Out of scope for this task**: Implement only `Create`/`Validate` on `SessionRepository`; sessions end purely via TTL expiry. Add `Revoke` later if/when logout is requested (avoids speculative/unused code per `minimal-implementation.md` standard).
   - Recommendation: **(b)** — out of scope. The task description's three numbered requirements (middleware, `/l0g1n` page, tests) say nothing about logout, and the `minimal-implementation.md` standard explicitly discourages building unrequested capability ("avoiding future stubs and speculative abstractions"). Implementing `Revoke` with no caller would itself be the kind of dead/speculative code that standard prohibits.

2. **Session TTL default value**: Clarifications confirm Redis-with-TTL + in-memory fallback, but no specific TTL duration is given anywhere (task description, codebase-analysis, or clarifications). The existing `IpRepository` pattern hardcodes `banTime = 30 * time.Minute` as an unexported package constant rather than a config value.
   - Options:
     a. Hardcode a constant in the repository package (matches existing `banTime` convention exactly) — e.g., `sessionTTL = 24 * time.Hour`.
     b. Make it configurable via a new env var (e.g., `SESSION_TTL`), parsed as a duration in `config/loader.go`.
   - Recommendation: **(a)**, hardcoded constant, for consistency with the one existing precedent (`banTime`) in this exact same codebase, unless the user wants it operator-tunable. A reasonable default for a personal single-user app: 24 hours (long enough to avoid daily re-login friction, short enough to bound session-hijack exposure). Flag for orchestrator to confirm exact duration value with user since it's a security/UX tradeoff, not a technical fact.

### Important (Should Decide)

1. **Exact env var name for session/cookie config**: Clarifications confirm `LOGIN_USER` and `LOGIN_PASS_HASH` (resolved, no decision needed). Not resolved: whether a TTL-related env var is needed at all (see Critical #2) and whether a cookie name constant (e.g., `wok_session`) needs to be configurable or can be hardcoded. Recommendation: hardcode the cookie name as an unexported constant (e.g., `sessionCookieName = "wok_session"`) — no config needed, consistent with project's "minimal dependencies"/"build only what's needed" standards.

2. **bcrypt as a new third-party dependency**: Not flagged as a decision in clarifications.md, but verified as a real gap: `golang.org/x/crypto/bcrypt` is not currently in `go.mod`/`go.sum`. The project's `tech-stack.md` describes "Go 1.26 stdlib-first" with "key dependencies (chi, go-redis, godotenv, Sentry)" — bcrypt would be a 5th dependency family. This is implied/required by clarification #1 (bcrypt-hashed password) and is not really optional, but the orchestrator/user should be aware this is a stdlib-first project taking on a new dependency for this feature, and it should be added via `go get golang.org/x/crypto` (not vendored/hand-rolled) and pinned per `infrastructure.md`'s "pinned module versions" standard. Recommendation: proceed with `golang.org/x/crypto/bcrypt` (it's the standard, well-audited choice; reimplementing bcrypt or using weaker stdlib-only hashing like sha256 would violate the explicit "bcrypt-hashed" clarification).

3. **Login form field names / CSRF protection scope**: No existing form anywhere in the codebase to mirror (verified `partials/` is empty). Two sub-decisions:
   - Field names: no convention exists; recommend simple `username`/`password` (or `user`/`pass`) form field names — low-stakes, implementer can decide directly without blocking.
   - CSRF protection: not mentioned anywhere in task description, codebase-analysis, or clarifications. For a single-user personal app behind already-existing IP-ban protection, with `SameSite=Strict` cookies (already decided in clarification #3), CSRF risk on the login POST itself is low (no authenticated state to forge before login) but worth a single explicit decision since it's commonly expected security hygiene.
   - Recommendation: Skip CSRF token implementation for the login form specifically — `SameSite=Strict` cookies already mitigate cross-site POST replay for session-authenticated actions, and the login form itself doesn't act on any pre-existing authenticated state. Flag this as a default the orchestrator should confirm rather than silently deciding, since CSRF is a security posture choice.

4. **Redirect target after successful login**: Codebase-analysis recommends redirecting to `/` after successful POST to `/l0g1n` — sensible default, not explicitly contradicted by clarifications. Recommend confirming this default (`/`) rather than something like a `?next=` redirect-back parameter (which would be speculative complexity for a single-route app — there's only one protected page, `/`, today).

## Recommendations
- Treat `Revoke`/logout as explicitly out of scope unless the user says otherwise — implementing it without a caller creates exactly the kind of orphaned-operation gap this analysis is designed to catch.
- Hardcode session TTL and cookie name as package constants (mirroring `banTime` precedent) rather than introducing new config surface, unless the user wants operational tunability.
- Confirm bcrypt as an accepted new dependency before specification — it's required by the existing credential-storage clarification, but worth surfacing explicitly since the project bills itself as stdlib-first.
- Before building the login page atop `base.gohtml`, verify whether the pre-existing `{{ template "header" . }}` reference (with no defined `"header"` template anywhere) is a live bug affecting `landing` today; if so, the login handler will inherit the same broken rendering path unless addressed (even minimally, e.g., defining an empty `"header"` block).
- No existing form markup/CSS conventions exist; the login template will set a precedent for future page styling — keep it minimal (raw HTML form, no JS framework) per stdlib-first tech stack.

## Risk Assessment
- **Complexity Risk**: Medium — mechanical parts (repository/middleware/handler) are low-risk due to strong existing patterns (`IpRepository`/`CheckIP`/`landing`), but credential hashing, cookie security attributes, and the never-tested template rendering path raise the overall risk above "low."
- **Integration Risk**: Low-Medium — only one call site (`main.go:49`) for the breaking `router.New()` signature change; config and `.env.default` changes are purely additive.
- **Regression Risk**: Low — no existing tests to break (repo has zero tests currently), and the new global middleware is additive to the chain; main regression vector is the middleware accidentally also blocking `/l0g1n` itself (redirect loop) if the self-exemption logic has a bug — this should be a primary test case.
