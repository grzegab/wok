# Specification: Login Authentication Middleware

## Goal
Add a global login-required gate to WOK: an unauthenticated visitor hitting any route is redirected to a login page at `/l0g1n`; after submitting valid credentials they get a server-side session (Redis-backed, in-memory fallback) and access to the app; a logout control ends the session early.

## User Stories
- As the single WOK user, when I visit any page without a valid session, I want to be redirected to `/l0g1n` so unauthenticated visitors cannot see my application data.
- As the single WOK user, I want to log in with a username and password at `/l0g1n` so I can access the app, and see a clear "Invalid credentials" message if I mistype something.
- As the single WOK user, I want a visible Logout control on the landing page so I can end my session deliberately instead of waiting for the 24-hour TTL to expire.

## Core Requirements

1. `SessionRepository` interface (`internal/repository/session_repository.go`):
   ```go
   type SessionRepository interface {
       Create(ctx context.Context, token string) error
       Validate(ctx context.Context, token string) (bool, error)
       Revoke(ctx context.Context, token string) error
   }
   ```
   `Validate` returns `(bool, error)` mirroring `IpRepository.Exists`'s shape exactly — `true` means a live, non-expired session.
2. `RedisSessionRepository` and `InMemorySessionRepository` implementations, mirroring `redis_ip_repository.go` / `in_memory_ip_repository.go` (key-prefix convention, TTL via a hardcoded `sessionTTL = 24 * time.Hour` constant — same pattern as `banTime`), with one deliberate deviation: `InMemoryIpRepository.Add` was confirmed (spec audit) to have **no TTL enforcement at all** — entries never expire. `InMemorySessionRepository` must NOT mirror that gap; it must actually enforce `sessionTTL` (e.g. store `expiresAt time.Time` alongside the token and check it in `Validate`, evicting expired entries), since silently-immortal sessions in the in-memory fallback would defeat the 24h requirement whenever Redis is unavailable.
3. Session token: random 32-byte value from `crypto/rand`, hex/base64-encoded, stored client-side in a cookie with `HttpOnly`, `Secure`, `SameSite=Strict`, and explicit `MaxAge: int(sessionTTL.Seconds())` attributes (without `MaxAge`/`Expires`, Go defaults to a session-only cookie, contradicting the 24h TTL); no JWT/HMAC signing.
4. `RequireLogin` global middleware (`internal/router/auth_middleware.go`), registered via `r.Use(...)` in the existing middleware chain (after `CheckIP`). It self-exempts `GET /l0g1n`, `POST /l0g1n`, `POST /logout`, and `/robots.txt` by request path, validates the session cookie via `sessionRepo.Validate`, and on missing/invalid session issues `http.StatusSeeOther` (303) redirect to `/l0g1n` without calling `next.ServeHTTP`.
5. `login` handler (`internal/router/auth_handler.go`):
   - `GET /l0g1n` renders `partials/login.gohtml` via `base.gohtml` with an empty error field.
   - `POST /l0g1n` reads `username`/`password` form fields, compares `username` against `LOGIN_USER` and `password` against `LOGIN_PASS_HASH` via `bcrypt.CompareHashAndPassword`. On success: create a session (`sessionRepo.Create`), set the session cookie, redirect 303 to `/`. On failure: re-render `partials/login.gohtml` with `Error: "Invalid credentials"`, HTTP 401, username value preserved, password field never repopulated.
6. `logout` handler (`internal/router/auth_handler.go`): `POST /logout` reads the session cookie, calls `sessionRepo.Revoke`, clears the cookie (`MaxAge: -1`), redirects 303 to `/l0g1n`.
7. `partials/header.gohtml` (new): defines the `"header"` template that `base.gohtml` already references (currently undefined — a pre-existing latent bug). Contains the app brand/title plus a `<form method="POST" action="/logout"><button type="submit">Logout</button></form>`. Since `RequireLogin` guarantees any request reaching `landing` is authenticated, the logout form is rendered unconditionally — no per-request "is logged in" check needed inside the template.
8. `partials/login.gohtml` (new): username/password form posting to `/l0g1n`, with labeled inputs (`<label for>`) and a conditionally-rendered error region (`role="alert"`) shown only when `Error` is set in the template data.
9. `Config` (`internal/config/loader.go`) gains `LoginUser` and `LoginPassHash` string fields, loaded via `os.Getenv("LOGIN_USER")` / `os.Getenv("LOGIN_PASS_HASH")`.
10. `.env.default` gains `LOGIN_USER=` and `LOGIN_PASS_HASH=` placeholder lines (template only; real secrets stay in the untracked `.env`).
11. New dependency `golang.org/x/crypto/bcrypt`, added via `go get golang.org/x/crypto` (direct dependency, not `// indirect`).
12. `routing.go` changes: `Router` struct gains `sessionRepo repository.SessionRepository` field; `New()` gains a second parameter; the dead `//r.Get("/l0g1n", landing)` comment (line 65) is replaced with live `r.Get("/l0g1n", login)`, `r.Post("/l0g1n", login)`, `r.Post("/logout", logout)` registrations, placed so they resolve before the catch-all `POST /*` ban route (Chi already prioritizes specific routes over wildcards — verify with a test, see Test Requirements).
13. `main.go` changes: replicate the Redis-or-in-memory fallback selection block used for `ipRepo`, for `sessionRepo`; pass both into `router.New(ipRepo, sessionRepo)`.
14. Tests for every new file (see Test Requirements section) using stdlib `testing` + `net/http/httptest`, with hand-rolled fakes for `SessionRepository` (no mocking library — none exists in `go.mod`).

## Visual Design

Mockups in `analysis/design-context/` (`analysis/design-context/INDEX.md`, `analysis/design-context/ascii/ui-mockups.md`) are binding inputs — the implementation-planner will attach `Visual References` to UI-related task groups.

Referenced screens/components:
- **`screen:login`** (`ui-mockups.md#login-page`) — `GET /l0g1n`: title "WOK — Sign in", empty error region, labeled Username/Password fields, "Log in" submit button, `<form method="POST" action="/l0g1n">`, no CSRF token.
- **`screen:login-error`** (`ui-mockups.md#login-error-state`) — same template, rendered after a failed `POST /l0g1n`: error banner "⚠ Invalid credentials" visible above the form, username value preserved, password field empty (never echoed).
- **`screen:landing-authenticated`** (`ui-mockups.md#landing-with-logout`) — existing `/` landing page, now wrapped by a real `header` template showing brand/title with a "Logout" control top-right.
- **`component:logout-link`** — `POST`-form button (not a GET link/anchor) inside the new `partials/header.gohtml`, since logout is a state-changing action and the app has no JS to intercept link clicks.
- **`component:login-form`** — `partials/login.gohtml`: username + password inputs with explicit `<label for>` associations (not placeholder-only).
- **`component:login-error-banner`** — error region using `role="alert"` or equivalent semantic markup so failure is announced, not just styled text.

Fidelity level: structural/low-fidelity ASCII wireframes — no pixel-level CSS direction given (no CSS framework exists in this stdlib-first SSR app). Implementer should produce plain, minimal HTML consistent with the rest of the codebase's "no JS, no framework" approach; these two new templates become the de facto form-markup convention for the project (first forms in the app).

Layout guideline: both new templates render through the existing single layout wrapper, `base.gohtml`, unmodified, via the same `template.ParseFS(templates.FS, "base.gohtml", "partials/*.gohtml")` + `ExecuteTemplate(w, "base", data)` pattern already used by `landing`.

## Reusable Components

### Existing Code to Leverage
- `internal/repository/ip_repository.go` — interface shape to mirror for `SessionRepository` (small, verb-named methods, `context.Context` first param).
- `internal/repository/redis_ip_repository.go` — Redis implementation pattern to mirror: unexported package constant for TTL (`banTime` → new `sessionTTL`), key-prefix string concatenation (`"banned_ip:"+ip` → e.g. `"session:"+token`), `rdb.Set(...).Err()` / `rdb.Exists(...).Result()` call shapes.
- `internal/repository/in_memory_ip_repository.go` — in-memory fallback pattern to mirror: map-backed struct, same method signatures as the Redis impl.
- `internal/router/check_middleware.go` (`CheckIP`) — factory-middleware pattern to mirror for `RequireLogin`: `func Xxx(repo) func(next http.Handler) http.Handler`, non-fatal `fmt.Printf`-style logging on repository errors, write-response-and-return-without-calling-next on the "blocked" branch.
- `internal/router/pages_handler.go` (`landing`) — exact `template.ParseFS` + `ExecuteTemplate(w, "base", data)` call shape to mirror for `login`/`logout` handlers; same `Content-Type` header setting and `http.Error` on parse/execute failure.
- `internal/redis/client.go` (`NewClient`) — reuse the same `*redis.Client` instance already created in `main.go` for `ipRepo`; no second Redis connection needed for sessions.
- `internal/templates/embed.go` — `partials/*` glob already covers both new templates; no changes needed here.
- `cmd/web/main.go` lines 27-38 — Redis-or-fallback selection block, structurally duplicated (not abstracted into a shared helper, since there are only two call sites and the existing codebase doesn't abstract this either) for `sessionRepo`.

### New Components Required
- `SessionRepository` interface + 2 implementations: no existing session/auth concept exists anywhere in the codebase (confirmed: `internal/repository/` has only `IpRepository`). New code is required because there's nothing to extend.
- `RequireLogin` middleware: no existing middleware checks session/login state; `CheckIP` only checks IP bans. New code required.
- `login`/`logout` handlers: no existing handler deals with credentials, cookies, or sessions. New code required.
- `partials/login.gohtml`: `partials/` is empty today — no form markup exists anywhere to reuse. New code required (first form in the app).
- `partials/header.gohtml`: no header/nav template exists; `base.gohtml` references an undefined `"header"` template (pre-existing latent bug). New code required — this template both supplies the logout control's home and incidentally fixes the bug as a side effect of needing a render target for the logout form.
- bcrypt dependency: no password-hashing capability exists in `go.mod` (stdlib `crypto/...` has no native bcrypt). Required by the already-made credential-storage decision (clarifications.md #1); added via `go get`, not hand-rolled, since bcrypt is the audited standard choice and reimplementing it would violate sound security practice.
- `Config.LoginUser` / `Config.LoginPassHash` fields: no credential config exists today (`Config` only has `RedisAddr`, `WebAddr`, `SentryDsn`).

## Technical Approach

- **Middleware chain placement**: `RequireLogin(sessionRepo)` is registered via `r.Use(...)` immediately after `CheckIP(ipRepo)` in `routing.go`'s existing chain (`RealIP → CheckIP → RequireLogin → Timeout → Logger → Recoverer → SentryHandler`), so IP bans are checked before authentication. It is global (not `chi.Group`-scoped), per the resolved clarification, and exempts specific paths itself rather than relying on route-scoping — this keeps the registration site simple and mirrors `CheckIP`'s unconditional style.
- **Self-exemption logic**: the middleware checks `r.URL.Path` against an exemption set (`/l0g1n`, `/logout`, `/robots.txt`) before doing any session validation; matched paths call `next.ServeHTTP` immediately. This avoids a redirect loop on the login page itself.
- **Route registration order matters**: in `routing.go`, the explicit `r.Get("/l0g1n", ...)`, `r.Post("/l0g1n", ...)`, `r.Post("/logout", ...)` registrations must be added so Chi's router resolves them ahead of the catch-all `r.Post("/*", ...)` ban route (lines ~79-90 today) — Chi already prefers more specific patterns over wildcards, but this must be exercised by a test (a `POST /l0g1n` integration-style test asserting it reaches the `login` handler, not the IP-ban handler) since incorrectly banning legitimate login attempts would be a severe regression.
- **Session token flow**: `login` handler generates a token via `crypto/rand` (32 bytes) on successful credential check, passes it to `sessionRepo.Create(ctx, token)` (TTL applied inside the repository implementation via the `sessionTTL` constant, consistent with how `banTime` is applied inside `RedisIpRepository.Add` rather than passed in by the caller), then sets it as the cookie value. `RequireLogin` reads the same cookie name on every request and calls `sessionRepo.Validate(ctx, token)`.
- **Cookie attributes**: `HttpOnly: true`, `Secure: true`, `SameSite: http.SameSiteStrictMode`, `Path: "/"`. Cookie name is an unexported package constant (e.g. `sessionCookieName = "wok_session"`) — no new config surface, consistent with hardcoding `sessionTTL` rather than introducing env vars for these.
- **Credential check**: `LOGIN_USER` is compared with plain string equality (it's a username, not a secret); `LOGIN_PASS_HASH` is compared against the submitted password via `bcrypt.CompareHashAndPassword([]byte(conf.LoginPassHash), []byte(submittedPassword))`. Neither `LOGIN_PASS_HASH` nor the submitted password is ever logged or included in an error message returned to the client.
- **Header template fix scope**: `partials/header.gohtml` only needs to satisfy `base.gohtml`'s existing `{{ template "header" . }}` call (line 5) with brand/title markup plus the logout form — no broader navigation system, menu, or styling overhaul. This is a minimal fix scoped exactly to "give the logout button somewhere to live," not a general UI pass.
- **Error handling for template failures**: `login`/`logout` handlers mirror `landing`'s existing pattern — `http.Error(w, err.Error(), http.StatusInternalServerError)` on `ParseFS`/`ExecuteTemplate` errors. (Note: `landing`'s current error message exposes `err.Error()` directly to the client; this spec does not change that existing pattern for consistency, though it's a minor existing deviation from the "clear user messages without exposing internals" standard — out of scope to fix here since it predates this feature and isn't part of the requested change.)

## Implementation Guidance

### Testing Approach
- 2-8 focused tests per implementation step group (repository, middleware, handlers) — do not attempt full-suite coverage in one group.
- Each test file should run independently using stdlib `testing` + `net/http/httptest`; no new test dependency (no testify) — this repo currently has zero tests, so these become the first and the de facto pattern for future test files.
- Verification of each step group runs only its own new tests, not the entire suite.

### Standards Compliance
- **`coding-style.md`**: snake_case file names (`session_repository.go`, `auth_middleware.go`, `auth_handler.go`), PascalCase exported types (`SessionRepository`, `RedisSessionRepository`, `InMemorySessionRepository`), camelCase unexported fields, `New`-prefixed constructors returning pointers, stdlib/third-party/internal import grouping.
- **`error-handling.md`**: wrap external-dependency errors with `%w` (Redis calls in `RedisSessionRepository`); non-fatal logging in `RequireLogin` on `sessionRepo.Validate` errors (log and treat as "not authenticated" rather than crashing the request, mirroring `CheckIP`'s `Exists`-error handling).
- **`validation.md`**: server-side validation of submitted username/password (non-empty checks) before attempting the bcrypt compare; specific, non-leaking error message ("Invalid credentials" — does not reveal whether the username or password was wrong).
- **`minimal-implementation.md`**: no speculative methods — `SessionRepository.Revoke` is implemented (in scope per `scope-clarifications.md`) and has an immediate caller (`logout` handler), so it is not a stub. No TTL config surface added since `sessionTTL` mirrors the existing `banTime` hardcoded-constant precedent.
- **`conventions.md`**: `SessionRepository` accepted by `RequireLogin`/handlers/`Router` as an interface type (not concrete struct), matching the existing `IpRepository` dependency-injection convention.
- **`backend/api.md`**: `POST /logout` and `POST /l0g1n` use POST for state-changing actions; failed login returns HTTP 401 (not 200), consistent with proper status code usage.
- **`testing/test-writing.md`**: test names describe behavior and expectation (e.g. `TestRequireLogin_RedirectsWhenSessionMissing`, `TestRequireLogin_ExemptsLoginPath`); external dependencies (Redis) are not required for the in-memory repository tests, isolating them; Redis-backed tests should be skippable if no test Redis instance is available, consistent with "fast execution" and not blocking on infra.

## Test Requirements

| File under test | Test file | Cases (2-8) |
|---|---|---|
| `internal/repository/in_memory_session_repository.go` | `in_memory_session_repository_test.go` | Create then Validate returns true; Validate on unknown token returns false; Validate after `sessionTTL` elapses returns false (use an injectable/short TTL or time-travel hook for the test, not a real 24h sleep); Revoke then Validate returns false |
| `internal/repository/redis_session_repository.go` | `redis_session_repository_test.go` | Same 4 behavioral cases as in-memory, run against a real or fake Redis; skip cleanly (`t.Skip`) if no test Redis instance is reachable, consistent with "fast execution" / not blocking on infra |
| `internal/router/auth_middleware.go` | `auth_middleware_test.go` | Redirects 303 to `/l0g1n` when no session cookie present; redirects 303 when cookie present but `Validate` returns false; calls `next.ServeHTTP` when `Validate` returns true; exempts `GET /l0g1n`, `POST /l0g1n`, `POST /logout`, `/robots.txt` without checking the session at all |
| `internal/router/auth_handler.go` (login) | `auth_handler_test.go` | `GET /l0g1n` renders with empty error; `POST /l0g1n` with correct credentials sets cookie + redirects 303 to `/`; `POST /l0g1n` with wrong password re-renders with "Invalid credentials" + HTTP 401, password field not echoed; `POST /l0g1n` with wrong username behaves identically (no leak of which field was wrong) |
| `internal/router/auth_handler.go` (logout) | `auth_handler_test.go` (shared file) | `POST /logout` calls `Revoke`, clears cookie (`MaxAge: -1`), redirects 303 to `/l0g1n` |
| `internal/router/routing.go` (regression) | `routing_test.go` or appended to an existing router test file | `POST /l0g1n` and `POST /logout` reach their real handlers, not the catch-all `POST /*` ban handler (the Chi-precedence claim verified during spec audit — guard it with a test so a future refactor can't silently regress it) |

All tests use stdlib `testing` + `net/http/httptest`; `SessionRepository` is faked with a small hand-rolled struct in middleware/handler tests (no mocking library, none exists in `go.mod`).

## Out of Scope
- User registration or multi-user support (single hardcoded user via env vars only).
- Password reset flow.
- Rate-limiting or lockout on repeated failed login attempts.
- CSRF token implementation (relying on `SameSite=Strict` per the resolved scope decision).
- Fixing any other pre-existing bugs not directly blocking this feature.
- Registering the existing but currently-unwired `robotsTxt` handler as a live route (it remains unregistered; `RequireLogin` defensively exempts `/robots.txt` by path string only, in case it's wired up later).
- A `?next=` redirect-back-to-original-page parameter after login (the app has exactly one protected page, `/`, today — this would be speculative complexity).
- Any CSS framework, styling system, or responsive-design work beyond plain semantic HTML.

## Success Criteria
- An unauthenticated `GET /` (or any non-exempt path) returns a 303 redirect to `/l0g1n`.
- `GET /l0g1n` renders the login form with no error message on first visit.
- `POST /l0g1n` with correct `LOGIN_USER`/password (matching `LOGIN_PASS_HASH`) creates a session, sets a cookie with `HttpOnly`/`Secure`/`SameSite=Strict`, and redirects 303 to `/`.
- `POST /l0g1n` with incorrect credentials re-renders the login form with "Invalid credentials" (HTTP 401), username preserved, password field empty.
- An authenticated `GET /` renders the landing page with a visible Logout control, and `base.gohtml`'s `{{ template "header" . }}` no longer fails/no-ops (pre-existing bug resolved as a byproduct).
- `POST /logout` revokes the session, clears the cookie, and redirects 303 to `/l0g1n`; a subsequent `GET /` is redirected to `/l0g1n` again.
- `POST /l0g1n` and `POST /logout` are not intercepted by the catch-all `POST /*` IP-ban route.
- All new files (`session_repository.go`, `redis_session_repository.go`, `in_memory_session_repository.go`, `auth_middleware.go`, `auth_handler.go`) have passing tests (2-8 per file/group) covering their stated behaviors.
- `go build ./...` succeeds with the new `golang.org/x/crypto/bcrypt` dependency declared as direct in `go.mod`/`go.sum`.
