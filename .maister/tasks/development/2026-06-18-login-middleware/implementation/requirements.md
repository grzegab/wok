# Requirements: Login Authentication Middleware

## Initial Description
Add login authentication to the WOK app: a middleware that checks if the user is logged in before allowing access to any page (redirect to login page if not), a login page served at `/l0g1n`, and tests for all new files.

## Q&A

**Q: Existing real `.env` secrets to update, or just `.env.default` template?**
A: Just `.env.default` as a documented template (placeholder values). The real `.env` (untracked) is the user's responsibility.

**Q: Should `RequireLogin` exclude static/SEO routes (robots.txt, assets) from the login check?**
A: Yes, exclude `/robots.txt` and asset paths conceptually — though investigation found neither has a currently *registered* route (the `robotsTxt` handler in `pages_handler.go:25` exists but is never wired into `routing.go`; `internal/templates/assets/*` is embedded but has no HTTP route serving it). Decision: add `/robots.txt` to the middleware's exemption list defensively (cheap, future-proof, matches user intent) but do NOT register new routes for it — that's pre-existing, out-of-scope dead code, not part of this task.

## Existing Code Reuse
- `internal/repository/ip_repository.go` + `redis_ip_repository.go` + `in_memory_ip_repository.go` — exact pattern to mirror for `SessionRepository`.
- `internal/router/check_middleware.go` (`CheckIP`) — exact factory-function pattern to mirror for `RequireLogin`.
- `internal/router/pages_handler.go` (`landing`) — exact `template.ParseFS` + `ExecuteTemplate(w, "base", data)` pattern to mirror for the new `login`/`logout` handlers.
- `internal/redis/client.go` — reuse the same `*redis.Client` instance already created in `main.go`, no new Redis connection.

## Important routing.go discovery
- `routing.go` already has commented-out `//r.Get("/l0g1n", landing)` (line 65) confirming intentional prior planning for this exact path.
- `routing.go` registers catch-all bans on `POST /*`, `PUT /*`, `DELETE /*` (lines 79-90). Chi's router matches the most specific route over a wildcard, so explicitly registering `r.Post("/l0g1n", ...)` and `r.Post("/logout", ...)` will correctly take precedence over the `POST /*` ban-everything catch-all — this must be verified working (not just assumed) during implementation/testing, since the ban behavior calls `ban(ipRepo, ...)` which would otherwise IP-ban legitimate login attempts.

## Visual Assets
ASCII mockups generated in Phase 4: `analysis/design-context/ascii/ui-mockups.md`, indexed in `analysis/design-context/INDEX.md` (`screen:login`, `screen:login-error`, `screen:landing-authenticated`, `component:logout-link`, `component:login-form`, `component:login-error-banner`). Binding inputs for implementation:
- Login page reuses `base.gohtml` unmodified, new `partials/login.gohtml` form (username, password, submit, error region using `role="alert"` or equivalent).
- New `partials/header.gohtml` resolves the pre-existing bug where `base.gohtml:5` references an undefined `"header"` template, and houses the Logout `<form method="POST" action="/logout">` (must be a form/button, not a link/GET, since logout is state-changing and there's no JS in this stdlib SSR app).
- Password field never repopulated on failed login; username may be.

## Functional Requirements Summary
1. `SessionRepository` interface (`Create`, `Validate`, `Revoke`) with `RedisSessionRepository` and `InMemorySessionRepository` implementations, selected in `main.go` the same way `ipRepo` is (Redis-or-fallback), passed into `router.New(ipRepo, sessionRepo)`.
2. Session token: random `crypto/rand`-generated token (32 bytes), TTL = hardcoded `sessionTTL = 24 * time.Hour` constant (mirrors `banTime` precedent), stored via cookie with `HttpOnly`, `Secure`, `SameSite=Strict`.
3. `RequireLogin` middleware (`internal/router/auth_middleware.go`): global (`r.Use(...)`), self-exempting `/l0g1n` (GET+POST), `/logout` (POST), and `/robots.txt` (defensive, even though currently unregistered). Validates session cookie via `sessionRepo.Validate`; on failure, redirect 303 to `/l0g1n`.
4. `login` handler (`internal/router/auth_handler.go`): GET renders `partials/login.gohtml`; POST validates `LOGIN_USER` + bcrypt-compares `LOGIN_PASS_HASH` against submitted password, on success creates session + sets cookie + redirects to `/` (303), on failure re-renders the same template with `Error: "Invalid credentials"` (password field not repopulated).
5. `logout` handler (`internal/router/auth_handler.go`): POST calls `sessionRepo.Revoke`, clears the cookie, redirects to `/l0g1n` (303).
6. New `partials/header.gohtml`: brand/title + logout form, satisfies `base.gohtml`'s `{{ template "header" . }}` for the first time across the whole app (fixes pre-existing latent bug as a byproduct, scoped narrowly to this need).
7. Config additions (`internal/config/loader.go`): `LoginUser`, `LoginPassHash` string fields, loaded via `os.Getenv("LOGIN_USER")` / `os.Getenv("LOGIN_PASS_HASH")`.
8. `.env.default` additions: `LOGIN_USER=` and `LOGIN_PASS_HASH=` placeholder entries (template only, no real secret committed).
9. New dependency: `golang.org/x/crypto/bcrypt` via `go get`.
10. Routes registered in `routing.go`: `r.Get("/l0g1n", login)`, `r.Post("/l0g1n", login)`, `r.Post("/logout", logout)` — replacing the commented-out line 65.
11. Tests for every new file: `session_repository` (both impls), `auth_middleware` (redirect-when-unauthenticated, pass-through-when-authenticated, self-exemption for `/l0g1n`/`/logout`), `auth_handler` (login success/failure, logout). Use stdlib `testing` + `net/http/httptest`, hand-rolled mocks for `SessionRepository` (no testify in `go.mod`).

## Reusability Opportunities
All new repository/middleware/handler code should mirror the IP-ban feature's structure exactly (interface → Redis impl → in-memory impl → factory-pattern middleware → simple handler), per `coding-style.md` and `conventions.md` standards.

## Scope Boundaries
- IN: login page, global auth middleware, session storage (Redis+fallback), logout, bcrypt credential check, tests.
- OUT: user registration/multi-user support, password reset, rate-limiting login attempts, CSRF tokens, fixing any other pre-existing bugs not directly blocking this feature, registering the dead `robotsTxt` handler as a live route.

## Technical Considerations
- Mirror `%w` error wrapping and non-fatal logging conventions from `check_middleware.go`/`redis/client.go`.
- `LOGIN_PASS_HASH` must never be logged or echoed in error messages.
