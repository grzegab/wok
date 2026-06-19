# UI Mockups: Login Middleware (`/l0g1n` + Logout)

**Generated**: 2026-06-18
**Task Path**: `.maister/tasks/development/2026-06-18-login-middleware`
**Feature Type**: New Feature (login page) + Enhancement (logout affordance on existing landing page)

## Overview

### UI Requirements
- A login page at `GET /l0g1n` with a username field, password field, submit button, and a visible area for an error message (e.g. "Invalid credentials"). Form `POST`s to `/l0g1n`. No CSRF token (per `scope-clarifications.md`).
- A small "Logout" affordance on the existing landing page (`/`), visible only once a session exists, that `POST`s to `/logout`.

### Integration Strategy
**Decision**: Render the login page as a new partial (`internal/templates/partials/login.gohtml`) plumbed through the *same* `base.gohtml` + `ExecuteTemplate(w, "base", data)` pattern already used by `landing` (`internal/router/pages_handler.go`). The logout link is added directly into the existing landing content (or into `base.gohtml`'s currently-broken `header` block, see note below), gated on whether the request reached the handler at all — since the new global auth middleware (`r.Use(RequireLogin(sessionRepo))`) guarantees that any request inside `landing` is already authenticated, the logout link can be unconditional within that handler's rendered output.

**Rationale**: The codebase has exactly one rendering pattern (`base.gohtml` + `partials/*.gohtml` glob via `embed.go`) and exactly one existing page. There is no component library, no CSS framework, and no JS — so "reuse" here means reusing the *template plumbing and `ParseFS` call pattern*, not visual components (none exist yet). Both new templates should sit in `partials/` since that's the only embedded glob (`//go:embed base.gohtml partials/* other/*"`).

## Existing Layout Analysis

### Application Structure
The app currently has a single page (`/`, handled by `landing` in `internal/router/pages_handler.go:10`) rendered through:

```
base.gohtml  (defines "base", template.template "header" . , block "content")
  └─ partials/*.gohtml  (currently EMPTY directory — no existing content/header partial)
```

**Key Components**:
- Base layout: `internal/templates/base.gohtml`
- Partials directory (empty today): `internal/templates/partials/`
- Embed glob: `internal/templates/embed.go` (`//go:embed base.gohtml partials/* other/*`)
- Landing handler: `internal/router/pages_handler.go:10` (`landing`)
- Router/route table: `internal/router/routing.go`

### Identified Patterns
- **Render pattern**: `template.ParseFS(templates.FS, "base.gohtml", "partials/*.gohtml")` then `tmpl.ExecuteTemplate(w, "base", data)` — see `pages_handler.go:11-19`. New login handler should follow this exactly.
- **Handler naming**: lowercase verb/noun (`landing`, `robotsTxt`) — new handlers should be named e.g. `login`, `logout` (or similar) per `codebase-analysis.md` naming conventions.
- **KNOWN BUG (pre-existing, out of scope unless blocking)**: `base.gohtml` line 5 references `{{ template "header" . }}` but no `"header"` template is defined anywhere — `partials/` is empty. Per `scope-clarifications.md`, this is **not** to be fixed unless it blocks the login page from rendering. Since `landing` already calls this same `base.gohtml` today, either (a) the app currently errors on `/` and nobody has hit it broken-path, or (b) Go's `html/template` silently no-ops on a missing `{{ template }}` invocation in some configurations — implementer should verify which, and only patch minimally if the login page's `ExecuteTemplate(w, "base", ...)` call errors out the same way `landing` would.

## Mockups

<a id="login-page"></a>
### Mockup 1: Login Page — `GET /l0g1n`

**Context**: Unauthenticated request to any protected route gets redirected (303 See Other) here by the new `RequireLogin` middleware. Also reachable directly.

```
┌──────────────────────────────────────────────────────────┐
│ /l0g1n  (NEW)                                             │
│ base.gohtml (EXISTING, reused)                            │
│  └─ partials/login.gohtml (NEW)                           │
│                                                            │
│   ┌────────────────────────────────────────────────┐     │
│   │                  WOK — Sign in                  │     │
│   │                                                  │     │
│   │   [!] Invalid credentials            ← NEW      │     │
│   │       (error region, EMPTY unless POST failed)  │     │
│   │                                                  │     │
│   │   Username                                       │     │
│   │   ┌──────────────────────────────────────────┐  │     │
│   │   │ ____________________________________      │  │     │
│   │   └──────────────────────────────────────────┘  │     │
│   │                                                  │     │
│   │   Password                                       │     │
│   │   ┌──────────────────────────────────────────┐  │     │
│   │   │ ********                                  │  │     │
│   │   └──────────────────────────────────────────┘  │     │
│   │                                                  │     │
│   │              ┌────────────────┐                 │     │
│   │              │   Log in   →   │  (submit)        │     │
│   │              └────────────────┘                 │     │
│   │                                                  │     │
│   │   <form method="POST" action="/l0g1n">           │     │
│   │     (no CSRF token — SameSite=Strict instead)    │     │
│   │   </form>                                        │     │
│   └────────────────────────────────────────────────┘     │
└──────────────────────────────────────────────────────────┘
```

**Integration Points**:
- Served via new `login` GET handler in `internal/router/auth_handler.go`, following the exact `template.ParseFS(...)` / `ExecuteTemplate(w, "base", data)` pattern as `landing` (`pages_handler.go:10-23`).
- Template lives at `internal/templates/partials/login.gohtml` — picked up automatically by the existing `partials/*` embed glob, **no `embed.go` change needed**.
- Error message is conditionally rendered from handler-passed data (e.g. `data.Error = "Invalid credentials"`), set only on a failed `POST /l0g1n` before re-rendering the same template (PRG-style: redirect-free re-render so the message shows, or 303-redirect-to-self-with-flash — implementer's call, but a simple "re-render with error" is simplest and matches "minimal implementation" standard).
- Route registered in `internal/router/routing.go`, replacing the commented-out `//r.Get("/l0g1n", landing)` (line 65) with both `r.Get("/l0g1n", login)` and `r.Post("/l0g1n", login)` (or split GET/POST handlers), registered **outside** the `RequireLogin` middleware group so it doesn't redirect-loop on itself.

**Component Reuse**:
- `base.gohtml` (`internal/templates/base.gohtml`) — the only layout wrapper in the app; login page reuses it unmodified.
- Render pattern from `landing` (`internal/router/pages_handler.go:10-23`) — copy the `ParseFS`/`ExecuteTemplate` call shape for the new `login` handler.
- No existing form/input/button components exist to reuse (`partials/` is empty) — this is the first form in the app, so its markup becomes the de facto pattern for any future forms.

<a id="login-error-state"></a>
### Mockup 1b: Login Page — Failed Attempt State

**Context**: Same template/route as Mockup 1, rendered after a `POST /l0g1n` with bad credentials (bcrypt compare fails).

```
┌────────────────────────────────────────────────┐
│                  WOK — Sign in                  │
│                                                  │
│  ┌────────────────────────────────────────┐    │
│  │ ⚠ Invalid credentials                   │ ← NEW: error banner,
│  └────────────────────────────────────────┘    │   visible only here
│                                                  │
│   Username                                      │
│   ┌──────────────────────────────────────┐     │
│   │ alice                                 │     │  (value preserved,
│   └──────────────────────────────────────┘     │   per UX convention —
│                                                  │   optional, implementer's
│   Password                                      │   call given minimal-
│   ┌──────────────────────────────────────┐     │   implementation standard)
│   │                                        │     │
│   └──────────────────────────────────────┘     │
│                                                  │
│              ┌────────────────┐                │
│              │   Log in   →   │                 │
│              └────────────────┘                │
└────────────────────────────────────────────────┘
```

**Interaction Details**:
1. User submits wrong username/password to `POST /l0g1n`.
2. Handler: bcrypt compare fails → re-render `partials/login.gohtml` with `Error: "Invalid credentials"` in template data, HTTP 401 (or 200 — implementer's call; 401 is more correct REST-wise per `standards/backend/api.md`).
3. Password field is NOT repopulated (never echo passwords back); username MAY be repopulated for UX convenience.

<a id="landing-with-logout"></a>
### Mockup 2: Landing Page (`/`) — With Logout Affordance

**Context**: Authenticated request to `/`, after the new `RequireLogin` middleware has validated the session cookie and called `next.ServeHTTP`.

```
┌──────────────────────────────────────────────────────────┐
│ /  (EXISTING route, ENHANCED)                              │
│ base.gohtml (EXISTING)                                     │
│  └─ {{ template "header" . }}   ← pre-existing bug:        │
│      no "header" template defined (partials/ was empty)    │
│      NOW DEFINED as part of this feature, see note below   │
│                                                              │
│   ┌────────────────────────────────────────────────┐      │
│   │  WOK                                 [Logout] ← NEW │  │
│   │  └─ existing app title/brand   └─ NEW affordance    │  │
│   │                                    (small link/button)│ │
│   │ ──────────────────────────────────────────────────  │  │
│   │                                                      │  │
│   │   (existing landing page content — unchanged)        │  │
│   │                                                      │  │
│   └────────────────────────────────────────────────┘      │
│                                                              │
│   <form method="POST" action="/logout">                     │
│     <button type="submit">Logout</button>                   │
│   </form>                                                    │
│   (a real <a href> can't safely trigger a state-changing     │
│    POST — must be a form/button per HTTP semantics)          │
└──────────────────────────────────────────────────────────┘
```

**Integration Points**:
- Placement: top-right corner of the page header/nav area — the conventional location for session/account actions, maximizing discoverability without competing with primary content.
- Because `{{ template "header" . }}` in `base.gohtml` is currently undefined (empty `partials/`), this feature is the natural point to **finally define** a minimal `partials/header.gohtml` containing the brand/title plus the logout form — this directly addresses the pre-existing bug as a side effect of needing *somewhere* to put the logout button, consistent with `scope-clarifications.md`'s guidance ("fix minimally...since the login page must render" — here it's the landing page header that needs it).
  - **Alternative considered**: inline the logout form directly inside the landing partial's content block instead of fixing `header`. Rejected — it would leave the "header" bug untouched while bolting an account-action into content markup, which is less conventional than a header/nav slot and would need redoing once `header` is eventually fixed anyway.
- Must be implemented as a `<form method="POST">` (not a plain link), since logout is a state-changing action (session revocation) and the app has no JS to intercept a link click and convert it to a POST.
- Wired to new `POST /logout` route in `routing.go`, calling `sessionRepo.Revoke(token)` and clearing the session cookie, then redirecting to `/l0g1n` (303).

**Component Reuse**:
- Same `base.gohtml` wrapper, now with `partials/header.gohtml` finally supplying the `"header"` template it expects.
- Logout button styling/markup should match whatever minimal button style is established by the login page's submit button (Mockup 1) — first form elements in the app, so consistency between the two is self-referential (there's nothing older to match).

## Reusable Components

### Layout
- **base.gohtml**: `internal/templates/base.gohtml` — only layout wrapper; defines `"base"`, expects a `"header"` template and a `"content"` block. Reused unmodified by both `/` and `/l0g1n`.

### Templates (new, this feature)
- **partials/login.gohtml** (NEW): `internal/templates/partials/login.gohtml` — login form, error region. Picked up automatically by `embed.go`'s `partials/*` glob.
- **partials/header.gohtml** (NEW, resolves pre-existing bug as a byproduct): houses brand/title + logout form, satisfies `base.gohtml`'s `{{ template "header" . }}` call for the first time.

### Handlers (new, this feature)
- **login**: `internal/router/auth_handler.go` (new) — mirrors `landing`'s `ParseFS`/`ExecuteTemplate` shape (`internal/router/pages_handler.go:10-23`).
- **logout**: `internal/router/auth_handler.go` (new) — calls `sessionRepo.Revoke`, clears cookie, redirects.

### Middleware (new, this feature)
- **RequireLogin**: `internal/router/auth_middleware.go` (new) — mirrors `CheckIP` factory pattern (`internal/router/check_middleware.go`).

## Implementation Notes

### Consistency Checklist
- Login handler uses the identical `template.ParseFS` + `ExecuteTemplate(w, "base", data)` call shape as `landing`.
- File/package naming follows existing snake_case file / lowercase handler conventions (`auth_handler.go`, `auth_middleware.go`, `login`, `logout`).
- Logout is a `POST` form, not a `GET` link, per HTTP semantics for state-changing actions (consistent with `standards/backend/api.md`'s "proper HTTP status codes/verbs" guidance).
- No CSRF token on either form, per `scope-clarifications.md` decision #4 — relies on `SameSite=Strict` cookie attribute instead.

### Accessibility Considerations
- Login form: `<label for="username">`/`<label for="password">` explicitly associated with inputs (not placeholder-only labels).
- Error message region should use a semantic element (e.g. role="alert" or a clearly visible text block) so the failure is announced, not just colored text.
- Logout control should be a real `<button>` inside a `<form>`, keyboard-operable by default (no JS-only click handlers, consistent with the project's no-JS-framework stdlib-first approach).

### Responsive Behavior
- No CSS framework or breakpoints exist in the codebase today (stdlib-first, server-rendered, no frontend framework per `tech-stack.md`). Both new templates should use plain, minimal HTML that degrades gracefully without explicit responsive work — out of scope for this feature unless a CSS file already exists (none was found).

## Alternatives Considered

### Option 1: Render login as a standalone HTML document, bypassing `base.gohtml` (Rejected)
**Why rejected**: Would create a second rendering pattern in a codebase that currently has exactly one. Increases inconsistency for zero benefit — the existing `base.gohtml` wrapper is generic enough (title/head/body shell) to host a login form without modification.

### Option 2: Inline logout link directly in landing page content instead of a `header` partial (Considered, not selected)
**Why not selected**: Leaves the pre-existing `"header"` template bug unaddressed while placing an account-level action in content markup rather than a conventional nav/header slot. Defining `partials/header.gohtml` kills two birds: satisfies `base.gohtml`'s existing (currently broken) expectation AND gives logout a conventional, discoverable home.

### Option 3: JS-based logout (fetch + redirect) (Rejected)
**Why rejected**: Project is explicitly stdlib-first SSR with no frontend framework or JS build step (`tech-stack.md`). A plain `<form method="POST">` achieves the same outcome with zero new dependencies or complexity, consistent with `minimal-implementation.md`.

---

*Generated by ui-mockup-generator subagent*
