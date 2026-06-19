# Phase 2 Scope Decisions

1. **Logout**: IN SCOPE. Add `POST /logout` route + `Revoke` method on `SessionRepository`, plus a logout link/button in the UI (rendered only when logged in — likely in a base template header/nav once a session exists).
2. **Session TTL**: Hardcoded constant `sessionTTL = 24 * time.Hour` in code, mirroring the existing `banTime = 30*time.Minute` precedent in `ip_repository.go`. No new env var.
3. **bcrypt dependency**: Accepted. Add `golang.org/x/crypto` via `go get` for password hashing (`bcrypt.GenerateFromPassword` / `bcrypt.CompareHashAndPassword`).
4. **CSRF**: No dedicated CSRF token on the login form. Rely on `SameSite=Strict` cookie attribute, consistent with minimal single-user app scope.
5. **Post-login redirect**: Redirect to `/` on successful login.

## Carried over from Phase 1 clarifications
- Credentials: bcrypt-hashed password via `LOGIN_USER` + `LOGIN_PASS_HASH` env vars.
- Middleware: global `r.Use(RequireLogin(sessionRepo))`, middleware itself exempts `/l0g1n` (GET/POST) and now also `/logout`.
- Session token: random `crypto/rand` token, `SessionRepository` (Redis+TTL / in-memory fallback), httpOnly+Secure+SameSite=Strict cookie.

## Noted but out of scope for this task
- Pre-existing latent bug: `base.gohtml` references an undefined `"header"` template — flagged for awareness, not fixed here unless it blocks rendering the login page (in which case implementer should fix minimally as part of this work, since the login page must render).
