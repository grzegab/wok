# Design Context Index

| ID | Type | Source | Description |
|----|------|--------|-------------|
| screen:login | screen | analysis/design-context/ascii/ui-mockups.md#login-page | Login page at GET/POST /l0g1n — username/password form, error region, no CSRF token |
| screen:login-error | screen | analysis/design-context/ascii/ui-mockups.md#login-error-state | Login page failed-attempt state showing "Invalid credentials" error banner |
| screen:landing-authenticated | screen | analysis/design-context/ascii/ui-mockups.md#landing-with-logout | Existing landing page ("/") enhanced with header partial and Logout affordance |
| component:logout-link | component | analysis/design-context/ascii/ui-mockups.md#landing-with-logout | Logout button (POST form, not a link) in new header partial, top-right of landing page |
| component:login-form | component | analysis/design-context/ascii/ui-mockups.md#login-page | Username/password form on the login page, POSTs to /l0g1n |
| component:login-error-banner | component | analysis/design-context/ascii/ui-mockups.md#login-error-state | Error message region on login page shown after failed credential check |
