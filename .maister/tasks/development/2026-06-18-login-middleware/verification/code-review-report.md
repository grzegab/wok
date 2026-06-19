# Code Review Report

**Date**: 2026-06-19
**Path**: internal/repository/{session_repository,redis_session_repository,in_memory_session_repository}.go (+tests), internal/router/{auth_middleware,auth_handler,routing}.go (+tests), internal/templates/partials/{login,header}.gohtml, cmd/web/main.go, internal/config/loader.go, .env.default, go.mod/go.sum
**Scope**: all
**Status**: ⚠️ Issues Found

## Summary
- **Critical**: 0 issues
- **Warnings**: 3 issues
- **Info**: 4 issues

`go build ./...` and `go test ./internal/repository/... ./internal/router/...` both pass cleanly. No hardcoded secrets, no injection vectors, no missing-auth gaps found. The cookie attributes, crypto/rand token generation, and bcrypt usage are all correct. Findings below are quality/robustness issues, not exploitable vulnerabilities — there's nothing rising to Critical.

## Warnings

### W1. `login.gohtml` bypasses `base.gohtml`, duplicating the document shell and double-invoking `"header"`
**Location**: `internal/router/auth_handler.go:90-98`, `internal/templates/partials/login.gohtml:1-25`, `internal/templates/base.gohtml:1-10`

Spec (line 52) requires both new templates to "render through the existing single layout wrapper, `base.gohtml`, unmodified," the same way `landing` does (`pages_handler.go:18`, `ExecuteTemplate(w, "base", nil)`). Instead, `renderLogin` calls `tmpl.ExecuteTemplate(w, "login", data)` directly — never invoking `"base"`. To compensate, `login.gohtml` re-declares its own `<!DOCTYPE html><html>...</html>` wrapper and calls `{{ template "header" . }}` itself, duplicating what `base.gohtml` already does (lines 1-7). This works today only because both templates happen to produce structurally-similar (if not identical) HTML, but it means:
- Any future change to `base.gohtml`'s shell (e.g. adding a `<meta>` tag, a footer, or analytics snippet) silently won't apply to the login page.
- The two templates' opening/closing tags are inconsistent: `header.gohtml` opens `<head>`/`<body>`/`<header>` and never closes `<header>` (relies on the caller closing it implicitly via no closing tag at all, then `login.gohtml` closes `</body></html>` while `base.gohtml` also closes `</body></html>` after its own `{{ block "content" }}`) — this only renders correctly because browsers are very tolerant of malformed/unclosed tags, not because the markup is actually well-formed HTML.

**Recommendation**: Make `login.gohtml` define a `"content"` block consumed by `base.gohtml` (matching the pattern implied by `base.gohtml:7`, `{{ block "content" . }}{{ end }}`), and change `renderLogin` to `ExecuteTemplate(w, "base", data)` like `landing` does. This removes the duplicated shell and the unclosed-tag fragility.

### W2. `RequireLogin` exemption list is a fixed map, but `/logout` is GET+POST exempt instead of POST-only
**Location**: `internal/router/auth_middleware.go:12-16`, `internal/router/routing.go:73`

`exemptPaths` matches on `r.URL.Path` only, not method+path (`auth_middleware.go:21`). `routing.go` only registers `r.Post("/logout", ...)` (no `GET /logout` route exists), so a stray `GET /logout` request currently falls through Chi's router to the catch-all-less default (chi 404), not to a real handler — it's exempted from the login check but there's no handler for it, so it 404s rather than redirecting to `/l0g1n`. Functionally harmless today (no information disclosure, no handler to reach), but it's an exemption that's broader than necessary. Minor inconsistency, not a security hole.

**Recommendation**: If method-specific exemption is cheap to add, scope `/logout` and `/l0g1n` (POST variant) to their actual methods for clarity; otherwise leave as-is and document the intentional path-only matching.

### W3. Known: redundant second config load in `routing.New`
**Location**: `internal/router/routing.go:49-65`

Already flagged in `work-log.md` as a non-blocking follow-up. Confirmed: `New(ipRepo, sessionRepo)` calls `config.Load()` internally (line 65) to build `authHandler`, while `main.go:26` already calls `config.Load()` once for the same process. This means `.env` is parsed twice and `godotenv.Load()` runs twice per process startup. Not a correctness bug (idempotent), but it's an avoidable I/O call and breaks the dependency-injection convention the rest of `Router` follows (`ipRepo`/`sessionRepo` are passed in, `conf` is not). Agree with the work-log's "non-blocking" assessment — severity here is Warning (code-quality/DI consistency), not Critical, since there's no functional impact in the current single-process deployment model.

**Recommendation**: Change `New(ipRepo, sessionRepo, conf *config.Config)` and have `main.go` pass the already-loaded `conf` through, eliminating the duplicate load.

## Informational

### I1. `Secure: true` cookie attribute will block login over plain HTTP in local/dev environments
**Location**: `internal/router/auth_handler.go:76`, `internal/router/auth_handler.go:115`

Both the login-setting and logout-clearing cookies set `Secure: true` unconditionally. This is correct and required for production (per spec, line 81), but browsers refuse to send `Secure` cookies over plain `http://`. `.env.default` (`WEB_ADDR=":80"`) and the absence of any TLS-termination setup in this repo suggest local/dev runs may be over HTTP, in which case the cookie would never be sent back by the browser on subsequent requests, causing an infinite redirect-to-login loop that's confusing to debug. This is consistent with the spec's explicit requirement and is not a defect to fix, but worth documenting (e.g. in `.env.default` comments or `tech-stack.md`) that local development requires either HTTPS or a reverse proxy terminating TLS in front of the app — otherwise a future contributor will spend time debugging a "broken" login that's actually working as specified.

### I2. `LOGIN_USER` comparison uses non-constant-time string equality
**Location**: `internal/router/auth_handler.go:50`

`username != h.conf.LoginUser` is a standard Go `!=` comparison (not constant-time). The spec explicitly calls this out as intentional (spec line 82: "compared with plain string equality (it's a username, not a secret)"), and the password path correctly uses `bcrypt.CompareHashAndPassword` (which is internally constant-time-relevant via bcrypt's design) at line 55. For a single-user app with no username enumeration value (there's only one valid username, and the error message doesn't distinguish bad-username from bad-password — confirmed by `TestLogin_Post_WrongUsername_BehavesIdenticallyToWrongPassword`), this is an accepted, documented trade-off, not a fresh finding. Noting for completeness only.

### I3. `fmt.Printf`-style logging in `RequireLogin` and `CheckIP` is unstructured and inconsistent with `log` package usage elsewhere
**Location**: `internal/router/auth_middleware.go:34`, `internal/router/check_middleware.go:23` (pre-existing pattern)

`RequireLogin` logs `sessionRepo.Validate` errors via `fmt.Printf` rather than the `log` package used in `main.go` (`log.Printf`, `log.Println`). This mirrors the pre-existing `CheckIP` pattern exactly (per spec guidance, line 60, "non-fatal `fmt.Printf`-style logging on repository errors"), so it's a deliberate consistency choice, not a new deviation. Flagging only because `fmt.Printf` to stdout bypasses Sentry/structured logging that `main.go` sets up — a session-validation error (e.g. Redis down) won't be visible in Sentry. Low priority since `CheckIP` already has this gap and fixing only the new file would create inconsistency.

### I4. Minor duplication between login-cookie-set and logout-cookie-clear attribute literals
**Location**: `internal/router/auth_handler.go:71-79` and `internal/router/auth_handler.go:109-117`

Both `loginPost` and `logout` construct an `http.Cookie{Name: sessionCookieName, Path: "/", HttpOnly: true, Secure: true, SameSite: http.SameSiteStrictMode, ...}` literal, differing only in `Value`/`MaxAge`. `TestLogout_ClearedCookie_MatchesLoginCookieNameAndPath` exists specifically to guard against this duplication drifting out of sync. A small `newSessionCookie(value string, maxAge int) *http.Cookie` helper would remove the duplication and make the symmetry the test checks for structurally guaranteed rather than test-enforced. Low priority — both call sites are short and the test already covers regression risk.

## Metrics
- Max function length: ~25 lines (`loginPost`, `RequireLogin`'s returned handler) — well under 50
- Max nesting depth: 3 levels (middleware closures) — fine
- `go build ./...`: clean (exit 0)
- `go vet ./...`: clean (exit 0) against actual repo files
- `go test ./internal/repository/... ./internal/router/...`: all pass (Redis-backed tests skip cleanly without infra)
- Potential vulnerabilities: 0 (no secrets, no injection, no auth bypass found)
- N+1 query risks: none (Redis ops are single-key `Set`/`Exists`/`Del`, no loops)
- Hardcoded secrets: none — `.env.default` correctly ships empty `LOGIN_USER=`/`LOGIN_PASS_HASH=` placeholders

## Prioritized Recommendations
1. Fix W1 — route `login.gohtml` through `base.gohtml`'s `"content"` block instead of duplicating the document shell and calling `"header"` a second time independently of `base`. This is the most likely to cause real drift/bugs as the templates evolve.
2. Address W3 (already known/tracked) — pass `*config.Config` into `router.New` instead of loading it a second time internally, for DI consistency and to avoid the duplicate `.env` parse.
3. Consider I4 — extract a small cookie-builder helper to make login/logout cookie symmetry structural rather than test-enforced.
4. Document I1 (Secure-cookie-requires-HTTPS) in `tech-stack.md` or `.env.default` comments so local dev setup isn't a surprise.
5. W2 and I2-I3 are informational/no-action-needed; retain as-is given they're either intentional (I2) or pre-existing convention (I3).
