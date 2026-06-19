# Phase 1 Clarifications

1. **Credential storage**: bcrypt-hashed password in env var (`LOGIN_USER` + `LOGIN_PASS_HASH`), compared via `bcrypt.CompareHashAndPassword`. No plain-text secrets.
2. **Middleware scope**: Global middleware (`r.Use(RequireLogin(sessionRepo))`) applied to the whole router; the middleware itself exempts `/l0g1n` (GET and POST) from the login check, mirroring the `CheckIP` pattern.
3. **Session token**: Random `crypto/rand`-generated 32-byte token, stored via `SessionRepository` (Redis with TTL, in-memory fallback), set in an httpOnly + Secure + SameSite=Strict cookie. No JWT/HMAC-signed stateless approach.
