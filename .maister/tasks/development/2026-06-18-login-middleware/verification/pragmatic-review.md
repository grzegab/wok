# Pragmatic Code Review: Login Authentication Middleware

## Executive Summary

**Status: Appropriate, with minor deviations**

**Overall complexity: Low** — appropriately matched to a single-user personal app. The implementation does not introduce enterprise patterns, unnecessary infrastructure, or speculative abstractions. It correctly mirrors the existing `IpRepository`/`RedisIpRepository`/`InMemoryIpRepository` pattern for sessions, uses plain HTML with no JS/CSS framework, and avoids all five items the spec explicitly excluded (registration, password reset, rate-limiting, CSRF tokens, `?next=` redirect parameter).

Findings by severity:
- **Critical: 0**
- **High: 1** (login template bypasses the shared layout, duplicating markup — contradicts an explicit spec instruction)
- **Medium: 1** (`config.Load()` called a second time inside `router.New()`, breaking the project's dependency-injection convention and re-parsing `.env` redundantly)
- **Low: 2** (duplicate `sessionTTL` constant across two packages; non-fatal pre-existing `fmt.Println`/Printf vet warning unrelated to this feature)

No over-engineering was found. The few issues below are consistency/correctness deviations from the codebase's own stated patterns, not added complexity.

---

## Complexity Assessment

Project scale: single-user personal app, Go stdlib-first, no frontend framework, two engineers' worth of conventions already established by `IpRepository`/`CheckIP`.

The session feature matches that scale:
- `SessionRepository` interface has exactly 3 verb methods (`Create`, `Validate`, `Revoke`) — no speculative methods, no generic `Repository[T]` abstraction.
- `RedisSessionRepository` / `InMemorySessionRepository` mirror the existing IP-ban repository pair line-for-line in style (key prefixing, hardcoded TTL constant, `%w` error wrapping). No new infra (no separate session store, no JWT library, no cache layer).
- `RequireLogin` middleware is a single function matching `CheckIP`'s exact shape (`func(repo) func(next) http.Handler`, non-fatal `fmt.Printf` logging, write-and-return on the blocked branch). No middleware-chaining framework, no `chi.Group`-scoped route trees.
- Session token: `crypto/rand` + hex encoding, no JWT/HMAC signing — correctly avoided as spec required.
- Cookie-based session id checked against a key-value store is the simplest correct mechanism here; nothing fancier was introduced.

This is exactly proportional to the stated need.

---

## Over-Engineering Patterns

**None found.** Specifically checked and absent:
- No Redis/Kafka/Elasticsearch added beyond the already-present Redis dependency (reused, not duplicated — confirmed `main.go` passes the same `*redis.Client` instance to both `ipRepo` and `sessionRepo`).
- No Repository/Service/Factory/Strategy layering beyond the existing repository-interface convention.
- No caching, connection pooling, or load-balancing additions.
- No new config files, feature flags, or multi-environment setup — `LoginUser`/`LoginPassHash` were added as two plain fields on the existing `Config` struct, loaded via `os.Getenv`, exactly like the three pre-existing fields.
- No abstraction was added to share the `main.go` Redis-or-fallback selection block between `ipRepo` and `sessionRepo`, consistent with the spec's explicit instruction not to introduce a shared helper for two call sites.

---

## Key Issues Found

### High: `login` template bypasses the shared layout wrapper, duplicating `base.gohtml`'s shell

- **Evidence**: `internal/templates/partials/login.gohtml` opens its own `<!DOCTYPE html><html lang="en">`, calls `{{ template "header" . }}`, and closes with its own `</body></html>`. Compare `internal/templates/base.gohtml`:
  ```
  {{ define "base" }}
  <!DOCTYPE html>
  <html lang="en">
  {{ template "header" . }}
  {{ block "content" . }}{{ end }}
  </body>
  </html>
  {{ end }}
  ```
  and `internal/router/auth_handler.go`'s `renderLogin`:
  ```go
  err = tmpl.ExecuteTemplate(w, "login", data)
  ```
  versus `pages_handler.go`'s `landing`:
  ```go
  err = tmpl.ExecuteTemplate(w, "base", nil)
  ```
- **Problem**: The login handler executes the `"login"` template directly instead of `"base"` with a `"content"` block override. This means the login page never goes through `base.gohtml` at all — it's a second, hand-duplicated copy of the document shell (DOCTYPE, `<html>`, header include, `</body></html>`).
- **Why this matters**: The spec is explicit on this point — "Layout guideline: both new templates render through the existing single layout wrapper, `base.gohtml`, unmodified, via the same `template.ParseFS(...)` + `ExecuteTemplate(w, "base", data)` pattern already used by `landing`." This was not followed. It works today only because `login.gohtml`'s hand-copied shell happens to match `base.gohtml`'s — the moment someone edits the layout (adds a meta tag, a script include, anything) the login page will silently drift out of sync, since there's no single source of truth for the page shell. This is the opposite of the project's "no dead/duplicate code" and DRY standards (`coding-style.md`).
- **Recommendation**: Change `login.gohtml` to use `{{ define "content" }}` instead of `{{ define "login" }}{{ ... }}<!DOCTYPE html>...`, dropping the duplicated `<!DOCTYPE html><html>`/`{{ template "header" }}`/`</body></html>` wrapper, and change `renderLogin` to call `tmpl.ExecuteTemplate(w, "base", data)` instead of `"login"`. This is a small, mechanical fix (delete ~5 lines, change one string literal) — not a redesign.
- **Estimated effort**: 10-15 minutes including re-running the existing template-rendering tests.

### Medium: `config.Load()` called twice — once in `main.go`, once inside `router.New()`

- **Evidence**:
  - `cmd/web/main.go:26`: `conf := config.Load()`
  - `internal/router/routing.go:65`: `authHdlr := newAuthHandler(sessionRepo, config.Load())`
  - `config.Load()` does real I/O and logging on every call: `log.Println("Reading config")`, `godotenv.Load()` (reads `.env` from disk), and reads 5 env vars.
- **Problem**: `Router.New(ipRepo, sessionRepo)` silently re-reads and re-parses configuration from the environment instead of accepting it as a parameter, even though `main.go` already loaded it once and has it in scope. This is inconsistent with the project's own stated convention ("interface-based dependency injection for repositories" in `conventions.md`) — config should flow in explicitly like `ipRepo`/`sessionRepo` do, not be re-derived from global process state inside a constructor.
- **Why this matters**: It's a correctness/testability smell more than a complexity one — `TestRouting_FullLoginLifecycle` in `routing_test.go` has to use `t.Setenv("LOGIN_USER", ...)` / `t.Setenv("LOGIN_PASS_HASH", ...)` to influence behavior, because `New()` has no way to receive config directly. `t.Setenv` mutates process-global environment state, which is fragile if tests ever run in parallel (`t.Parallel()`), and it duplicates the "Reading config" log line and `.env` file I/O on every router construction (harmless today at single-instance scale, but it's the kind of inconsistency that compounds — e.g. a future test or call site could get a different config object than the one `main.go` validated/logged).
- **Recommendation**: Add `conf *config.Config` as a third parameter to `router.New(ipRepo, sessionRepo, conf)`, pass `conf` from `main.go` (already loaded), and delete the `config.Load()` call inside `routing.go`. Update the one call site and the routing tests to construct a `*config.Config` directly instead of relying on `t.Setenv`.
- **Estimated effort**: 15-20 minutes.

### Low: `sessionTTL` constant duplicated across two packages

- **Evidence**: `internal/repository/in_memory_session_repository.go:9`: `const sessionTTL = 24 * time.Hour`, and separately `internal/router/auth_handler.go:16`: `const sessionTTL = 24 * time.Hour`.
- **Problem**: The same 24-hour value is hardcoded twice in two different packages. If the TTL ever changes, both must be updated in lockstep or the cookie `MaxAge` and the actual session lifetime in the repository will silently diverge (cookie could expire before/after the session does).
- **Context**: This is a minor, low-severity issue — not over-engineering, the opposite problem (slight under-DRY) — and Go package boundaries make sharing an unexported constant across packages awkward without adding new exported surface area, which the spec's `minimal-implementation.md` would also discourage doing speculatively. Given the project's stated preference to hardcode constants over introducing new config surface (explicitly stated in the spec for `sessionTTL`/cookie name), this is a defensible trade-off, but worth flagging since it's the one place two independent literals must stay manually synchronized.
- **Recommendation**: Low priority. If addressed, the simplest fix consistent with project conventions is exporting `repository.SessionTTL` and importing it in `auth_handler.go`, rather than introducing a new shared `internal/constants` package (which would be the over-engineered fix).
- **Estimated effort**: 5 minutes, optional.

### Low: Pre-existing `fmt.Println` vet warning blocks `go test ./...` at the root package

- **Evidence**: `go vet` reports `main.go:13:2: fmt.Println call has possible Printf formatting directive %s` in the root-level `main.go` (not `cmd/web/main.go`, which is the actual entrypoint used by this feature).
- **Problem**: This causes `go test ./...` to report `FAIL github.com/grzegab/wok [build failed]` at the root package, even though all packages touched by this feature (`internal/repository`, `internal/router`) pass cleanly.
- **Context**: Confirmed via `git`/spec context this is unrelated to and predates the login-middleware change — the spec's "Out of Scope" section explicitly excludes "Fixing any other pre-existing bugs not directly blocking this feature," so leaving this alone was correct per spec, but it is worth flagging since it makes the top-level `go test ./...` output look like the new feature has a failure when it does not.
- **Recommendation**: No action needed for this task. Worth a one-line fix in a separate, unrelated commit.

---

## Developer Experience

- **Setup**: No new external services. Redis is reused (single connection shared between `ipRepo` and `sessionRepo`, as instructed). `.env.default` gained two empty placeholder lines (`LOGIN_USER=`, `LOGIN_PASS_HASH=`), consistent with existing onboarding flow.
- **Feedback loop**: Tests run fast (`internal/repository` and `internal/router` packages pass in well under a second from cache). No new build steps, no codegen.
- **Test patterns**: Hand-rolled fakes (`fakeSessionRepository`, `fakeAuthSessionRepository`) instead of a mocking library, consistent with project convention (no mock library in `go.mod`). Test names are behavior-descriptive (`TestRequireLogin_NoCookie_Redirects303ToLogin`, etc.), matching `testing/test-writing.md`.
- **Friction point**: The `t.Setenv` usage in `TestRouting_FullLoginLifecycle` (forced by the `config.Load()` issue above) is the one spot where a future developer adding a parallel test could get surprising cross-test env pollution. Fixing the Medium issue above removes this friction.

---

## Requirements Alignment

Compared against `implementation/spec.md`:

| Spec item | Status |
|---|---|
| `SessionRepository` interface shape | Matches exactly (3 methods, `Validate` returns `(bool, error)`) |
| `RedisSessionRepository`/`InMemorySessionRepository` mirroring IP-ban pattern | Matches; in-memory impl correctly enforces TTL (deviates from `InMemoryIpRepository`'s known TTL gap, as required) |
| Token: 32-byte `crypto/rand`, hex-encoded, no JWT | Matches |
| Cookie attributes (`HttpOnly`, `Secure`, `SameSite=Strict`, `MaxAge`) | Matches |
| `RequireLogin` self-exempting middleware, 303 redirect | Matches, including exemption test with `panicOnCall` guard ensuring `Validate` is never even called for exempt paths |
| Login/logout handlers, bcrypt compare, generic "Invalid credentials" message, no field-leak | Matches; verified by `TestLogin_Post_WrongUsername_BehavesIdenticallyToWrongPassword` |
| `header.gohtml`/`login.gohtml` markup, labeled inputs, `role="alert"` | Matches structurally; **deviates** in render path (see High finding above — login template doesn't actually render through `base.gohtml` as instructed) |
| `Config.LoginUser`/`LoginPassHash` fields | Matches |
| `.env.default` placeholders | Matches |
| `bcrypt` as direct dependency | Confirmed used directly in `auth_handler.go`, not vendored/reimplemented |
| `routing.go` registration order ahead of catch-all | Matches; explicitly tested in `TestRouting_LoginAndLogoutNotInterceptedByCatchAllBan` |
| `main.go` Redis-or-fallback duplication for `sessionRepo` | Matches; correctly reuses the same `*redis.Client`, not a second connection |

**Out-of-Scope verification** — grepped the full diff surface (`internal/`, `cmd/`) for registration, password-reset, rate-limiting, CSRF, and `?next=` indicators: **zero matches**. None of the five explicitly-excluded items were sneaked in. This is a clean, disciplined adherence to the spec's scope boundary.

**One deviation found**: the render-path issue (High finding) is a deviation from an explicit spec instruction ("render through the existing single layout wrapper... unmodified"), not scope creep — it under-delivers on a stated requirement rather than over-delivering with unrequested complexity. Worth distinguishing from over-engineering: this is the opposite failure mode (a shortcut/duplication that should be cleaned up), not added complexity.

---

## Context Consistency

- No dead code, no unused imports detected in the reviewed files.
- No abandoned/half-implemented patterns.
- No unused private methods — every handler method and helper (`generateSessionToken`, `renderLoginUnauthorized`, `renderLogin`) has a live caller.
- Error handling is consistent with `CheckIP`'s established pattern (non-fatal `fmt.Printf` logging on repository validate errors, falling through to "not authenticated").
- The one inconsistency is the `config.Load()` double-call noted above — it doesn't create dead code, but it does create two independent "facts" about config in the process (same values today, but no structural guarantee they stay that way if `New()`'s signature or call sites diverge in the future).

---

## Recommended Simplifications

No simplifications are needed — the implementation is already minimal. The three "top priority" actions are correctness/consistency fixes, not complexity reductions:

1. **Fix login template to route through `base.gohtml`** (High, ~15 min) — replace `{{ define "login" }}` + hand-copied shell with `{{ define "content" }}`, and change `ExecuteTemplate(w, "login", data)` to `ExecuteTemplate(w, "base", data)` in `auth_handler.go`.
2. **Pass `*config.Config` into `router.New()` instead of calling `config.Load()` twice** (Medium, ~15-20 min) — add a third parameter, update `main.go`'s call site and `routing_test.go`'s `t.Setenv` usage.
3. **(Optional, Low) Export `sessionTTL` from `repository` package and reuse it in `auth_handler.go`** instead of a second hardcoded constant — only if convenient; not urgent given Go's package-boundary trade-offs.

---

## Summary Statistics

| Metric | Current |
|---|---|
| New files | 7 (interface, 2 repo impls + 2 test files, middleware + test, handler + test, 2 templates) |
| New dependencies | 1 (`golang.org/x/crypto/bcrypt`, direct) |
| New config fields | 2 (`LoginUser`, `LoginPassHash`) |
| New infra components | 0 (Redis connection reused) |
| Abstraction layers added | 0 beyond existing repository-interface convention |
| Out-of-scope items leaked in | 0 of 5 |
| `go build ./...` | Succeeds |
| `go test ./internal/...` | Passes |

## Conclusion

This is a well-scoped, appropriately simple implementation for a single-user app. It correctly resists the temptation to add CSRF tokens, rate-limiting, multi-user support, or a redirect-back parameter — exactly the discipline the spec asked for. The two real issues (login template bypassing `base.gohtml`, duplicate `config.Load()` call) are not over-engineering; they're small consistency/correctness gaps relative to the codebase's own stated patterns, both fixable in under an hour combined. No architectural rework is warranted.
