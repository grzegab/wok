# Implementation Plan: Login Authentication Middleware

## Overview
Total Steps: 31
Task Groups: 6
Expected Tests: 25-34

## Implementation Steps

### Task Group 1: Session Repository Layer
**Dependencies:** None
**Files to Modify:** `internal/repository/session_repository.go`, `internal/repository/redis_session_repository.go`, `internal/repository/in_memory_session_repository.go`, `internal/repository/redis_session_repository_test.go`, `internal/repository/in_memory_session_repository_test.go`
**Estimated Steps:** 7

- [x] 1.0 Complete Session Repository layer
  - [x] 1.1 Write 8 focused tests across the two repository test files
    - `in_memory_session_repository_test.go`: Create then Validate returns true; Validate on unknown token returns false; Validate after `sessionTTL` elapses returns false (use an injectable/short TTL or time-travel hook, not a real 24h sleep); Revoke then Validate returns false
    - `redis_session_repository_test.go`: same 4 behavioral cases run against a real/fake Redis instance; `t.Skip` cleanly if no test Redis is reachable (mirror "fast execution" / no infra-blocking guidance from spec Test Requirements)
  - [x] 1.2 Define `SessionRepository` interface in `internal/repository/session_repository.go`
    - `Create(ctx context.Context, token string) error`
    - `Validate(ctx context.Context, token string) (bool, error)` — mirrors `IpRepository.Exists`'s exact shape; `true` means a live, non-expired session
    - `Revoke(ctx context.Context, token string) error`
    - Reuse: `internal/repository/ip_repository.go` interface shape (verb-named methods, `context.Context` first param)
  - [x] 1.3 Implement `RedisSessionRepository` in `internal/repository/redis_session_repository.go`
    - Mirror `redis_ip_repository.go`: unexported `sessionTTL = 24 * time.Hour` constant (mirrors `banTime` pattern), key prefix `"session:"+token`, `rdb.Set(ctx, "session:"+token, ..., sessionTTL).Err()` for Create, `rdb.Exists(ctx, "session:"+token).Result()` for Validate, `rdb.Del(ctx, "session:"+token).Err()` for Revoke
    - Wrap Redis errors with `%w` per `error-handling.md`
    - Reuse: `internal/redis/client.go` `*redis.Client` instance (no new connection)
  - [x] 1.4 Implement `InMemorySessionRepository` in `internal/repository/in_memory_session_repository.go`
    - Map-backed struct storing `token -> expiresAt time.Time` (deliberate deviation from `InMemoryIpRepository.Add`, which has no TTL enforcement at all — per spec requirement 2, this is intentional and must NOT be "fixed" to match `InMemoryIpRepository`'s behavior)
    - `Create` stores `time.Now().Add(sessionTTL)` as the expiry
    - `Validate` checks current time against stored expiry; evicts (deletes the map entry) and returns `false` if expired; returns `false` for unknown tokens
    - `Revoke` deletes the map entry unconditionally
    - Guard map access with a mutex (concurrent request safety, consistent with `InMemoryIpRepository`'s struct shape but extended for the TTL field)
  - [x] 1.5 Run the 8 new tests in `internal/repository/...session_repository_test.go`
    - Do NOT run the entire test suite (none exists yet at this point, but scope to this package only going forward)

**Acceptance Criteria:**
- The 8 tests pass (4 in-memory + 4 Redis, Redis test skippable without a reachable instance)
- `InMemorySessionRepository.Validate` actually enforces `sessionTTL` (expired entries return `false`, not "lives forever")
- `RedisSessionRepository` mirrors `redis_ip_repository.go`'s key-prefix/TTL/error-wrapping conventions exactly
- `SessionRepository` interface signature matches spec exactly: `Validate(ctx, token) (bool, error)`

---

### Task Group 2: Auth Middleware
**Dependencies:** Group 1
**Files to Modify:** `internal/router/auth_middleware.go`, `internal/router/auth_middleware_test.go`
**Estimated Steps:** 5

- [x] 2.0 Complete RequireLogin middleware
  - [x] 2.1 Write 4 focused tests in `auth_middleware_test.go`
    - Redirects 303 to `/l0g1n` when no session cookie present
    - Redirects 303 when cookie present but `Validate` returns false
    - Calls `next.ServeHTTP` when `Validate` returns true
    - Exempts `GET /l0g1n`, `POST /l0g1n`, `POST /logout`, `/robots.txt` without checking the session at all (use a hand-rolled fake `SessionRepository` that panics/errors if `Validate` is called, to prove the exemption short-circuits before validation)
  - [x] 2.2 Implement `RequireLogin(sessionRepo repository.SessionRepository) func(http.Handler) http.Handler` in `internal/router/auth_middleware.go`
    - Factory pattern mirroring `CheckIP` (`internal/router/check_middleware.go`)
    - Self-exemption: check `r.URL.Path` against `{"/l0g1n", "/logout", "/robots.txt"}` before any session logic; matched paths call `next.ServeHTTP` immediately (avoids redirect loop on the login page itself)
    - Read session cookie via the shared `sessionCookieName` constant (define alongside this middleware or in `auth_handler.go` — must be referenced by both; place in `auth_middleware.go` since it's read here and written in the handler)
    - On missing cookie / `Validate` returning false / `Validate` returning an error: log non-fatally (mirror `CheckIP`'s `fmt.Printf`-style logging on repo errors, treat error as "not authenticated") and `http.Redirect(w, r, "/l0g1n", http.StatusSeeOther)` without calling `next.ServeHTTP`
    - On `Validate` returning true: call `next.ServeHTTP(w, r)`
    - Reuse: `internal/router/check_middleware.go` (`CheckIP`) factory-middleware pattern exactly
  - [x] 2.3 Run the 4 new tests in `internal/router/auth_middleware_test.go`
    - Do NOT run the entire test suite

**Acceptance Criteria:**
- The 4 tests pass
- Middleware never calls `sessionRepo.Validate` for exempted paths (proven by the fake-repo test)
- All "invalid" cases (missing cookie, malformed cookie, `Validate` false, `Validate` error) collapse to the same 303 redirect behavior

---

### Task Group 3: Login/Logout Handlers and Templates
**Dependencies:** Group 1
**Files to Modify:** `internal/router/auth_handler.go`, `internal/router/auth_handler_test.go`, `internal/templates/partials/login.gohtml`, `internal/templates/partials/header.gohtml`
**Visual References:**
- mockup: analysis/design-context/ascii/ui-mockups.md
  element: screen:login
  locator: "Mockup 1: Login Page — GET /l0g1n" (lines 42-89 of ui-mockups.md)
  acceptance: title "WOK — Sign in"; empty error region on first GET; labeled Username and Password fields (`<label for>` association, not placeholder-only); single "Log in" submit button; `<form method="POST" action="/l0g1n">`; no CSRF token field; rendered via `base.gohtml` using the same `ParseFS`/`ExecuteTemplate(w, "base", data)` pattern as `landing`
- mockup: analysis/design-context/ascii/ui-mockups.md
  element: screen:login-error
  locator: "Mockup 1b: Login Page — Failed Attempt State" (lines 91-118 of ui-mockups.md)
  acceptance: error banner "⚠ Invalid credentials" visible above the form using `role="alert"` semantic markup; username value preserved from the failed submission; password field rendered empty (never echoed); HTTP 401 status on this response
- mockup: analysis/design-context/ascii/ui-mockups.md
  element: screen:landing-authenticated
  locator: "Mockup 2: Landing Page (/) — With Logout Affordance" (lines 125-165 of ui-mockups.md)
  acceptance: header partial renders brand/title plus a Logout control positioned top-right; existing landing content below the header is unchanged; `base.gohtml`'s `{{ template "header" . }}` resolves without error (pre-existing bug fixed as a byproduct)
- mockup: analysis/design-context/ascii/ui-mockups.md
  element: component:logout-link
  locator: "Mockup 2" bottom section, lines 148-153 of ui-mockups.md
  acceptance: rendered as `<form method="POST" action="/logout"><button type="submit">Logout</button></form>` — a real submit button inside a POST form, not a GET anchor/link; rendered unconditionally inside `header.gohtml` since `RequireLogin` guarantees any request reaching `landing` is authenticated
- mockup: analysis/design-context/ascii/ui-mockups.md
  element: component:login-form
  locator: "Mockup 1" form region, lines 59-76 of ui-mockups.md
  acceptance: `username` and `password` form field names; explicit `<label for="username">`/`<label for="password">` associations; password input uses `type="password"`
- mockup: analysis/design-context/ascii/ui-mockups.md
  element: component:login-error-banner
  locator: "Mockup 1b" error banner region, lines 100-102 of ui-mockups.md
  acceptance: conditionally rendered only when `Error` is set in template data; uses `role="alert"` or equivalent semantic markup so the failure is announced, not just styled text
**Estimated Steps:** 8

- [x] 3.0 Complete login/logout handlers and templates
  - [x] 3.1 Write 5 focused tests in `auth_handler_test.go`
    - `GET /l0g1n` renders with empty error
    - `POST /l0g1n` with correct credentials sets cookie (assert `HttpOnly`, `Secure`, `SameSite=Strict`, `MaxAge: int(sessionTTL.Seconds())`) and redirects 303 to `/`
    - `POST /l0g1n` with wrong password re-renders with "Invalid credentials" + HTTP 401, password field not echoed
    - `POST /l0g1n` with wrong username behaves identically (no leak of which field was wrong)
    - `POST /logout` calls `Revoke`, clears cookie (`MaxAge: -1`), redirects 303 to `/l0g1n`
  - [x] 3.2 Create `internal/templates/partials/header.gohtml`
    - `{{ define "header" }}` block containing app brand/title plus the logout form (per Visual References: `component:logout-link`)
    - Satisfies `base.gohtml`'s existing `{{ template "header" . }}` call (line 5), resolving the pre-existing latent bug as a byproduct
    - No per-request "is logged in" check inside the template — `RequireLogin` already guarantees authentication for any request reaching `landing`
  - [x] 3.3 Create `internal/templates/partials/login.gohtml`
    - Username/password form posting to `/l0g1n`, labeled inputs (`<label for>`), conditionally-rendered error region (`role="alert"`) shown only when `Error` is set in template data (per Visual References: `component:login-form`, `component:login-error-banner`)
    - Top-level `{{ define }}` name distinct from `"header"` to avoid template-set collisions (both are parsed into the same set via the shared `partials/*.gohtml` glob)
  - [x] 3.4 Implement `login` handler (GET+POST) in `internal/router/auth_handler.go`
    - `GET /l0g1n`: render `partials/login.gohtml` via `base.gohtml` with empty `Error` field, same `template.ParseFS(templates.FS, "base.gohtml", "partials/*.gohtml")` + `ExecuteTemplate(w, "base", data)` pattern as `landing`
    - `POST /l0g1n`: read `username`/`password` form fields; server-side non-empty validation before bcrypt compare (per `validation.md`); compare `username` against `conf.LoginUser` (plain string equality), compare `password` against `conf.LoginPassHash` via `bcrypt.CompareHashAndPassword([]byte(conf.LoginPassHash), []byte(password))`
    - On success: generate 32-byte token via `crypto/rand`, hex/base64-encode it, `sessionRepo.Create(ctx, token)`, set cookie (`HttpOnly: true`, `Secure: true`, `SameSite: http.SameSiteStrictMode`, `Path: "/"`, `MaxAge: int(sessionTTL.Seconds())`, name = `sessionCookieName`), redirect 303 to `/`
    - On failure: re-render `partials/login.gohtml` with `Error: "Invalid credentials"`, HTTP 401, username value preserved, password field never repopulated; identical response whether username or password was wrong (no field-specific leak)
    - Mirror `landing`'s `http.Error(w, err.Error(), http.StatusInternalServerError)` pattern on `ParseFS`/`ExecuteTemplate` errors
    - Reuse: `internal/router/pages_handler.go` (`landing`) exact render-call shape
  - [x] 3.5 Implement `logout` handler in `internal/router/auth_handler.go`
    - `POST /logout`: read session cookie, call `sessionRepo.Revoke(ctx, token)`, clear cookie (`MaxAge: -1`, same name/path), redirect 303 to `/l0g1n`
  - [x] 3.6 Run the 5 new tests in `internal/router/auth_handler_test.go`
    - Do NOT run the entire test suite

**Acceptance Criteria:**
- The 5 tests pass
- Implementation matches each `acceptance` criterion declared in Visual References above
- Neither `LOGIN_PASS_HASH` nor the submitted password is ever logged or included in a client-facing error message
- `base.gohtml`'s `{{ template "header" . }}` resolves cleanly for both `/` and `/l0g1n` renders (no parse/execute error from the shared `partials/*.gohtml` glob)

---

### Task Group 4: Config and Environment Wiring
**Dependencies:** None
**Files to Modify:** `internal/config/loader.go`, `.env.default`
**Estimated Steps:** 2

- [x] 4.0 Complete config/env wiring
  - [x] 4.1 No new tests required for this group (trivial additive struct fields + env template lines; covered indirectly by Group 3's handler tests, which construct a `Config` with these fields)
  - [x] 4.2 Add `LoginUser` and `LoginPassHash` string fields to `Config` struct in `internal/config/loader.go`, populated via `os.Getenv("LOGIN_USER")` / `os.Getenv("LOGIN_PASS_HASH")` after `godotenv.Load()`, following the existing pattern for `RedisAddr`/`WebAddr`/`SentryDsn`
  - [x] 4.3 Add `LOGIN_USER=` and `LOGIN_PASS_HASH=` placeholder lines to `.env.default` (template only; real secrets stay in untracked `.env`)

**Acceptance Criteria:**
- `go build ./...` succeeds after the struct/env changes
- `.env.default` documents both new variables as empty placeholders
- Group 3's handler tests, which depend on `Config.LoginUser`/`LoginPassHash`, compile and pass against this struct shape

---

### Task Group 5: Dependency, Routing, and Bootstrap Wiring
**Dependencies:** Groups 1, 2, 3, 4
**Files to Modify:** `go.mod`, `go.sum`, `internal/router/routing.go`, `cmd/web/main.go`, `internal/router/routing_test.go`
**Estimated Steps:** 6

- [x] 5.0 Complete dependency, routing, and bootstrap wiring
  - [x] 5.1 Write 1 regression test in `routing_test.go` (the Chi-precedence guard the spec explicitly calls for)
    - `POST /l0g1n` and `POST /logout` reach their real handlers, not the catch-all `POST /*` ban handler — guards the empirically-verified-safe (per spec-audit.md) but refactor-fragile Chi routing precedence claim
  - [x] 5.2 Add `golang.org/x/crypto/bcrypt` dependency via `go get golang.org/x/crypto` (direct dependency, declared without `// indirect` in `go.mod`)
  - [x] 5.3 Modify `internal/router/routing.go`:
    - Add `sessionRepo repository.SessionRepository` field to `Router` struct
    - Add `sessionRepo repository.SessionRepository` as second parameter to `New(...)`
    - Insert `RequireLogin(sessionRepo)` via `r.Use(...)` immediately after `CheckIP(ipRepo)` in the existing middleware chain (`RealIP → CheckIP → RequireLogin → Timeout → Logger → Recoverer → SentryHandler`)
    - Replace the dead `//r.Get("/l0g1n", landing)` comment (line 65) with live registrations: `r.Get("/l0g1n", login)`, `r.Post("/l0g1n", login)`, `r.Post("/logout", logout)`, placed so they resolve ahead of the catch-all `r.Post("/*", ...)` ban route (Chi already prioritizes specific routes over wildcards regardless of registration order, per spec-audit.md's empirical verification against the pinned `chi v5.3.0` module — still guarded by 5.1's test)
  - [x] 5.4 Modify `cmd/web/main.go`:
    - Replicate the Redis-or-in-memory fallback selection block (existing lines 27-38, used for `ipRepo`) for `sessionRepo`: try `NewRedisSessionRepository(rdb)`, fall back to `NewInMemorySessionRepository()` with a non-fatal warning log on Redis connection failure
    - Pass both into `router.New(ipRepo, sessionRepo)`
  - [x] 5.5 Run `go build ./...` to confirm the new dependency and signature changes compile cleanly across all call sites
  - [x] 5.6 Run the 1 new test in `internal/router/routing_test.go`
    - Do NOT run the entire test suite

**Acceptance Criteria:**
- The 1 regression test passes
- `go build ./...` succeeds with `golang.org/x/crypto/bcrypt` declared as a direct dependency in `go.mod`/`go.sum`
- `POST /l0g1n` and `POST /logout` are not intercepted by the catch-all `POST /*` IP-ban route
- Only call site of `router.New` (`main.go`) is updated; no other call sites broken

---

### Task Group 6: Test Review & Gap Analysis
**Dependencies:** Groups 1, 2, 3, 4, 5
**Files to Modify:** `internal/repository/in_memory_session_repository_test.go`, `internal/repository/redis_session_repository_test.go`, `internal/router/auth_middleware_test.go`, `internal/router/auth_handler_test.go`, `internal/router/routing_test.go`
**Estimated Steps:** 4

- [x] 6.0 Review and fill critical gaps
  - [x] 6.1 Review tests from previous groups (18 existing tests: 8 repository + 4 middleware + 5 handler + 1 routing)
  - [x] 6.2 Analyze gaps for THIS feature only — candidate strategic additions per spec Success Criteria and Out of Scope boundaries:
    - End-to-end flow: unauthenticated `GET /` redirects 303 to `/l0g1n`, then after successful login a subsequent `GET /` succeeds (integration-style, exercising middleware + handler together)
    - `POST /logout` followed by `GET /` redirects to `/l0g1n` again (session actually revoked, not just cookie cleared client-side)
    - Cookie clearing on logout uses `MaxAge: -1` with the same name/path as the login-set cookie (symmetry check)
    - Server-side validation rejects empty username/password before attempting bcrypt compare (per `validation.md`)
  - [x] 6.3 Write up to 7 additional strategic tests covering the above (stay within max 10 additional)
  - [x] 6.4 Run feature-specific tests only across `internal/repository/...` and `internal/router/...` packages (expect ~25-34 total)
    - Do NOT run any unrelated package tests (none exist in this repo yet, but scope explicitly to these two packages)

**Acceptance Criteria:**
- All feature tests pass (~25-34 total across all groups)
- No more than 7 additional tests added in this group (within the max-10 budget)
- Full login → access → logout → re-redirect lifecycle is exercised by at least one test

## Execution Order

1. Group 1: Session Repository Layer (7 steps)
2. Group 4: Config and Environment Wiring (2 steps, no dependencies — can run in parallel with Group 1)
3. Group 2: Auth Middleware (5 steps, depends on 1)
4. Group 3: Login/Logout Handlers and Templates (8 steps, depends on 1; can run in parallel with Group 2 since both only depend on Group 1, but Group 3 also needs Group 4's `Config` fields for its handler tests)
5. Group 5: Dependency, Routing, and Bootstrap Wiring (6 steps, depends on 1, 2, 3, 4)
6. Group 6: Test Review & Gap Analysis (4 steps, depends on all previous groups)

## Standards Compliance

Follow standards from `.maister/docs/standards/`:
- `global/coding-style.md` — snake_case file names, PascalCase exported types, camelCase unexported fields, `New`-prefixed constructors, import grouping
- `global/error-handling.md` — wrap Redis errors with `%w`; non-fatal logging in `RequireLogin` on `Validate` errors
- `global/validation.md` — server-side non-empty checks on username/password before bcrypt compare; non-leaking "Invalid credentials" message
- `global/minimal-implementation.md` — no speculative methods beyond what has an immediate caller; `sessionTTL`/cookie name hardcoded as constants, no new config surface beyond `LOGIN_USER`/`LOGIN_PASS_HASH`
- `global/conventions.md` — `SessionRepository` accepted as interface type throughout (DI convention)
- `backend/api.md` — `POST` for state-changing actions; HTTP 401 on failed login
- `backend/infrastructure.md` — pinned module version for new `golang.org/x/crypto` dependency
- `testing/test-writing.md` — behavior-describing test names (e.g. `TestRequireLogin_RedirectsWhenSessionMissing`); Redis-backed tests skippable without infra

## Notes

- Test-Driven: Each group starts with its tests (or, for Group 4, defers coverage to the group that consumes the new fields)
- Run Incrementally: Only new tests after each group, not the entire suite
- Mark Progress: Check off steps as completed
- Reuse First: `IpRepository`/`CheckIP`/`landing` patterns are mirrored throughout per spec's Reusable Components section
- Groups 1+4, and Groups 2+3, are structurally independent of each other and may be executed concurrently by the plan executor; Group 5 serializes on all four since it touches the shared `routing.go`/`main.go` wiring points
