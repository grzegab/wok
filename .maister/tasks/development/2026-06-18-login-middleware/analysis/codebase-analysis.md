# Codebase Analysis Report

**Date**: 2026-06-18
**Task**: Add login authentication middleware to WOK
**Description**: Add a login authentication middleware to the WOK app (Go 1.26 stdlib-first, html/template SSR, Chi v5 router, Redis-backed storage with in-memory fallback). Need: (1) middleware checking if user is logged in before any page access, redirecting to login page if not; (2) login page served at `/l0g1n`; (3) tests for new files. Single-user personal app per project vision.
**Analyzer**: codebase-analyzer skill (3 Explore agents: File Discovery, Code Analysis, Context Discovery)

---

## Summary

WOK has an established interface-based repository pattern (`IpRepository` + Redis/in-memory implementations) and a matching factory-style middleware pattern (`CheckIP`) that the new auth feature should mirror exactly: a `SessionRepository` interface with Redis and in-memory implementations, an `auth_middleware.go` modeled on `check_middleware.go`, and a login handler modeled on `pages_handler.go`. The codebase already contains a deliberate, currently-commented-out hint (`//r.Get("/l0g1n", landing)` in `routing.go:65`) and a banned-URI entry for `/login` (`routing.go:20`), confirming `/l0g1n` is an intentional obfuscated path to dodge bot scanners, not a typo. The repo has zero existing tests, so this feature also establishes the project's first test files and pattern.

---

## Files Identified

### Primary Files (to create)

**internal/repository/session_repository.go** (new, ~8 lines expected)
- Defines `SessionRepository` interface, mirroring `IpRepository` (`internal/repository/ip_repository.go`, 8 lines): `Create(ctx, ...) (token string, err error)`, `Validate(ctx, token) (valid bool/userID, err error)`, `Revoke(ctx, token) error`.

**internal/repository/redis_session_repository.go** (new)
- Redis-backed implementation, mirrors `redis_ip_repository.go` (27 lines): key-prefixed storage with TTL via Redis SETEX/EXPIRE semantics.

**internal/repository/in_memory_session_repository.go** (new)
- In-memory fallback, mirrors `in_memory_ip_repository.go` (30 lines): map-based storage, used when Redis is unavailable.

**internal/router/auth_middleware.go** (new)
- Factory middleware mirroring `check_middleware.go` (36 lines): reads session cookie, validates via `SessionRepository`, redirects to `/l0g1n` (303 See Other) if invalid/missing, otherwise calls `next.ServeHTTP`.

**internal/router/auth_handler.go** (new)
- GET handler renders login form (mirrors `landing` in `pages_handler.go`); POST handler validates credentials, creates session, sets cookie, redirects to `/`.

**internal/templates/partials/login.gohtml** (new)
- Login form partial. Must live under `partials/` — confirmed by reading `internal/templates/embed.go` (`//go:embed base.gohtml partials/* other/*`), which is the only template glob currently embedded. A `pages/` directory (as one raw finding speculated) would NOT be picked up without also editing `embed.go`.

### Test files (new, currently zero tests exist in repo)

- internal/repository/redis_session_repository_test.go
- internal/repository/in_memory_session_repository_test.go
- internal/router/auth_middleware_test.go
- internal/router/auth_handler_test.go

### Related Files (to modify)

**internal/router/routing.go** (97 lines, verified)
- Line 13-16: `Router` struct holds `chi *chi.Mux` and `ipRepo repository.IpRepository` as unexported fields — add `sessionRepo repository.SessionRepository` the same way.
- Line 20: `banUri` list already contains `"/login"` — confirms `/l0g1n` is the deliberately obfuscated real path, distinct from the banned decoy.
- Line 47: `func New(ipRepo repository.IpRepository) *Router` — needs a `sessionRepo repository.SessionRepository` parameter.
- Lines 49-54: middleware chain — `RealIP` → `CheckIP(ipRepo)` → `Timeout` → `Logger` → `Recoverer` → `SentryHandler()`. New auth middleware must be inserted after `CheckIP` (IP bans checked first) and must NOT protect `/l0g1n` itself (avoid redirect loop).
- Line 64: only existing route, `r.Get("/", landing)`.
- Line 65: `//r.Get("/l0g1n", landing)` — commented-out line already present, strong signal of intended route registration.

**cmd/web/main.go** (83 lines, verified)
- Lines 27-38: fallback-selection pattern for `ipRepo` (try Redis, log warning + fall back to in-memory on error) — replicate identically for `sessionRepo`.
- Line 49: `router.New(ipRepo)` — needs second argument once `sessionRepo` exists.

**internal/config/loader.go** (34 lines, verified)
- `Config` struct currently has only `RedisAddr`, `WebAddr`, `SentryDsn` (lines 10-14), populated via `os.Getenv` after `godotenv.Load()`. Needs new fields for session/credential config (exact names are an open decision, see below).

**.env.default**
- Currently: `REDIS_ADDR`, `SENTRY_DSN`, `WEB_ADDR` only (per Context Discovery agent). Needs new auth/session vars added as documented template; real secret values stay in untracked `.env`.

**internal/templates/embed.go** (9 lines, verified)
- No change needed — `partials/*` glob already covers the new login template.

---

## Current Functionality

The app is a single-user personal tool with no authentication today. All routes are public except for an IP-ban gate (`CheckIP` middleware) that blocks known-bad IPs with HTTP 418. There is no session, cookie, or user-credential concept anywhere in the codebase yet — this feature introduces all of it from scratch.

### Key Components/Functions

- **`repository.IpRepository`** (interface, `Add`/`Exists`): pattern to mirror for `SessionRepository`.
- **`router.CheckIP`** (factory middleware): pattern to mirror for the new auth middleware. Notably, on a "bad" condition it writes a response and returns without calling `next.ServeHTTP` — the auth middleware should do the same but with a redirect instead of a 418.
- **`router.landing`** (handler): pattern to mirror for the login GET handler — `template.ParseFS(templates.FS, "base.gohtml", "partials/*.gohtml")` then `ExecuteTemplate(w, "base", data)`.
- **`redis.NewClient`**: existing single Redis client/connection; session storage should reuse this same client instance, no new infra needed (`docker-compose.yaml` only runs Redis already).

### Data Flow

Request → `RealIP` → `CheckIP` (IP ban check) → [new: auth check] → `Timeout`/`Logger`/`Recoverer`/`SentryHandler` → route handler. For protected routes, a missing/invalid session cookie should short-circuit with a redirect to `/l0g1n` before reaching the handler.

---

## Dependencies

### Imports (What This Depends On)

- `github.com/go-chi/chi/v5` and `.../middleware`: routing and chain registration.
- `github.com/grzegab/wok/internal/repository`: new `SessionRepository` interface lives here.
- `internal/redis`: existing Redis client, reused for session storage (no new connection).
- `html/template`: SSR rendering for the login page, same as existing handlers.
- stdlib `net/http`, `crypto/rand` (or similar) for session token generation — new dependency surface but stdlib-only, consistent with the project's stdlib-first tech stack.

### Consumers (What Depends On This)

- **internal/router/routing.go**: registers the new middleware and `/l0g1n` route; will hold `sessionRepo` field.
- **cmd/web/main.go**: wires `sessionRepo` construction and passes it into `router.New`.
- **internal/config/loader.go**: supplies new config fields (session TTL, credentials) consumed by `main.go`.

**Consumer Count**: 3 files directly touched (`routing.go`, `main.go`, `loader.go`), plus `.env.default`.
**Impact Scope**: Medium — touches the app's bootstrap and core router wiring, but the existing `IpRepository`/`CheckIP` pattern provides a near-identical template to copy, reducing actual design risk.

---

## Test Coverage

### Test Files

None exist yet anywhere in the repository (no `*_test.go` files found). This feature will establish the project's first tests.

### Coverage Assessment

- **Test count**: 0 (current), target 4+ new test files per task requirement ("tests for new files").
- **Gaps**: No test library beyond stdlib is present in `go.mod` (no testify). Plan should use stdlib `testing` + `httptest.NewRequest`/`NewRecorder`, with small hand-rolled fakes for `SessionRepository` rather than introducing a mocking library.

---

## Coding Patterns

### Naming Conventions

- **Packages**: lowercase, single word (`router`, `repository`, `config`).
- **Files**: snake_case (`check_middleware.go`, `redis_ip_repository.go`).
- **Types**: PascalCase exported (`IpRepository`, `RedisIpRepository`), camelCase unexported fields (`ipRepo`, `chi`).
- **Constructors**: `New[TypeName]` returning concrete pointer (`NewRedisIpRepository`, `NewInMemoryIpRepository`), stored as interface-typed variables at call sites.
- **Middleware**: named for what they check, factory pattern returning `func(next http.Handler) http.Handler` (`CheckIP`).
- **Handlers**: lowercase verb/noun (`landing`, `ban`, `robotsTxt`).
- **Interfaces**: named for capability (`IpRepository`), methods are verbs (`Add`, `Exists`).
- **Errors**: variable named `err`, wrapped with `%w` (see `internal/redis/client.go`), non-fatal errors in middleware logged via `fmt.Printf`/`log.Printf` without aborting the request (see `check_middleware.go:23-25`).
- **Imports**: grouped stdlib / third-party / internal, as seen in every file read.

### Architecture Patterns

- **Style**: procedural/functional Go, no classes; structs used for grouping state (`Router`), interfaces for DI.
- **State Management**: no global state; dependencies passed explicitly through constructors (`router.New(ipRepo)`), stored as unexported struct fields.
- **Fallback selection**: Redis-or-in-memory selection happens once, in `main.go`, at startup — not lazily inside the repository implementations.

---

## Complexity Assessment

| Factor | Value | Level |
|--------|-------|-------|
| New/modified files | 9 new + 3 modified | Medium |
| Dependencies | stdlib + existing chi/redis (0 new third-party deps) | Low |
| Consumers | 3 files directly wired (routing.go, main.go, loader.go) | Low-Medium |
| Test coverage | 0 existing tests in repo; 4 new test files required | High (gap, but expected — first tests in project) |

### Overall: Moderate

The implementation pattern is already fully established in the codebase (IpRepository/CheckIP), which significantly de-risks the design work. Complexity comes from: (a) this being the project's first test suite, (b) several genuinely open design decisions (below) that affect security posture (credential storage, cookie attributes) rather than from sheer file count or dependency depth.

---

## Key Findings

### Strengths
- Strong existing pattern to mirror (`IpRepository`/`CheckIP`/`landing`) reduces design ambiguity to near zero for the mechanical parts.
- Single Redis instance already provisioned; no infra changes needed.
- The `/l0g1n` requirement is already partially anticipated in the code (commented route, banned `/login` decoy) — confirms this is a deliberate, previously-planned feature, not a one-off ad hoc request.
- `embed.go` glob already covers `partials/*`, so the login template needs no build/embed changes.

### Concerns
- No tests exist anywhere yet — the test patterns for this feature will become the de facto project standard going forward; worth getting naming/structure right (`TestXxxRedirectsWhenY` style, table-driven where useful).
- Credential storage approach is unresolved: plain env-var comparison vs hashed (bcrypt). For a single hardcoded user this matters for the threat model even though the app is "personal."
- Cookie/session token generation approach (length, entropy source, `Secure`/`HttpOnly`/`SameSite` attributes) is unresolved — directly affects session-hijack risk.

### Opportunities
- Reusing the exact `IpRepository` interface shape for `SessionRepository` keeps the codebase highly consistent and makes future maintenance (e.g., adding a third repository type) trivial.
- This is a natural point to also add `SESSION_TTL` and credential vars to `.env.default` as documented template entries, following existing convention.

---

## Impact Assessment

- **Primary changes**: new files in `internal/repository/` (interface + 2 impls), new files in `internal/router/` (middleware + handler), new template partial.
- **Related changes**: `routing.go` (Router struct, `New()` signature, middleware chain, route registration), `main.go` (sessionRepo wiring), `config/loader.go` (new config fields), `.env.default` (new documented vars).
- **Test updates**: 4 new test files; this is the project's first test infrastructure, so test setup conventions established here will likely be referenced by future features.

### Risk Level: Low-Medium

Risk is contained because the blast radius is limited to: route registration (additive, won't break the existing `/` route), the `Router`/`New()` constructor signature (a breaking change to call sites, but there's only one call site — `main.go`), and config loading (additive fields). The main residual risk is security-design risk (credential/cookie choices) rather than implementation/integration risk.

---

## Recommendations

This is a **new capability** (no existing auth implementation to modify) for a **single-user personal app**.

**Recommended architecture** (mirroring existing patterns):
1. `internal/repository/session_repository.go`: `SessionRepository` interface — e.g. `Create(ctx, userID string) (token string, err error)`, `Validate(ctx, token string) (bool, error)`, `Revoke(ctx, token string) error`. Mirror `IpRepository`'s minimal two/three-method shape.
2. `internal/repository/redis_session_repository.go` + `internal/repository/in_memory_session_repository.go`: mirror `redis_ip_repository.go` / `in_memory_ip_repository.go` exactly (key prefix convention, TTL-based expiry for sessions).
3. `internal/router/auth_middleware.go`: factory middleware mirroring `CheckIP` — read session cookie, call `sessionRepo.Validate`, on failure log non-fatally and `http.Redirect(w, r, "/l0g1n", http.StatusSeeOther)` (matches PRG pattern), on success call `next.ServeHTTP`.
4. `internal/router/auth_handler.go`: GET renders `partials/login.gohtml` via the same `template.ParseFS` pattern as `landing`; POST validates submitted credentials against config-sourced single-user credentials, creates a session via `sessionRepo.Create`, sets an `HttpOnly`+`Secure`+`SameSite=Strict` cookie, redirects to `/`.
5. `internal/templates/partials/login.gohtml`: simple form partial, no new `embed.go` changes required.
6. Wire into `routing.go`: add `sessionRepo repository.SessionRepository` to `Router` struct and `New()` signature; insert auth middleware after `CheckIP` in the chain but apply it via `chi.Group` (scoped to protected routes) rather than a blanket `r.Use`, so `/l0g1n` itself and static assets remain naturally excluded without needing path-exclusion logic inside the middleware. Register `r.Get("/l0g1n", ...)` and `r.Post("/l0g1n", ...)` outside the protected group, replacing the commented-out line at `routing.go:65`.
7. Wire into `main.go`: replicate the Redis-or-fallback selection block (lines 27-38) for `sessionRepo`, pass into `router.New(ipRepo, sessionRepo)`.
8. Add config fields to `loader.go` and corresponding entries to `.env.default`.
9. Tests: `auth_middleware_test.go` (redirect-when-no-cookie, redirect-when-invalid-token, pass-through-when-valid), `auth_handler_test.go` (GET renders form, POST with correct/incorrect credentials), `in_memory_session_repository_test.go` (create/validate/revoke/expiry), `redis_session_repository_test.go` (same, against a test Redis or skipped if unavailable — follow whatever pattern, if any, existing `redis_ip_repository.go` testing intends, though none currently exists).

**Open Decisions / Risks (require user/orchestrator decision before specification):**

1. **Credential storage approach** — hardcoded single env-var username/plaintext-password compared at request time, vs. bcrypt-hashed password stored in env var. Plaintext-in-env is simpler and matches the project's "minimal implementation" standard for a personal single-user app, but bcrypt is safer if `.env` could ever leak. Raw findings disagreed (`AUTH_SECRET`+`PASSWORD_HASH_ALGORITHM`+`LOGIN_ATTEMPTS_MAX` vs. simpler `LOGIN_USER`+`LOGIN_PASS`+`SESSION_TTL`) — recommend the simpler `LOGIN_USER`/`LOGIN_PASS` plaintext-env approach as consistent with project scale and the `minimal-implementation.md` standard, but this should be confirmed with the user given it's a security-relevant choice.
2. **Exact env var names** — depends on decision 1. Suggested: `LOGIN_USER`, `LOGIN_PASS`, `SESSION_TTL`.
3. **Global middleware vs. `chi.Group` route-scoping** — recommend `chi.Group` (option B from raw findings) to avoid needing path-exclusion logic for `/l0g1n` inside the middleware itself, keeping the middleware simpler and more aligned with `CheckIP`'s unconditional style.
4. **Login template location** — confirmed via direct read of `embed.go`: must be `internal/templates/partials/login.gohtml`, not a new `pages/` directory, unless `embed.go` is also updated. No further decision needed here — resolved.
5. **Cookie attributes/session token generation** — recommend `HttpOnly`, `Secure`, `SameSite=Strict`, and token generated via `crypto/rand` (stdlib, no new dependency) producing a sufficiently long random string (e.g., 32 bytes, base64/hex-encoded). This should be confirmed but is lower-risk than decisions 1-3 since it follows well-established stdlib practice.

---

## Next Steps

Proceed to gap-analysis (or directly to specification, given the open decisions above are narrow enough to resolve inline) to formalize: the `SessionRepository` interface method signatures, the exact config field names, and the middleware-scoping approach, before generating the implementation plan.
