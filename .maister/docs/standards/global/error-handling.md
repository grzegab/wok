## Error Handling

### Clear User Messages
Show helpful, actionable messages without exposing internal details or security-sensitive information.

### Fail Fast
Validate inputs and check preconditions early; reject invalid data before it causes deeper issues.

### Typed Exceptions
Use specific exception types instead of generic ones to enable precise error handling.

### Centralized Handling
Catch and process errors at appropriate boundaries (controllers, API layers) rather than scattering try-catch throughout.

### Graceful Degradation
When non-critical services fail, continue operating with reduced functionality rather than crashing entirely.

### Retry with Backoff
Use exponential backoff for transient failures when calling external services.

### Resource Cleanup
Always release resources (file handles, connections) in finally blocks or equivalent cleanup mechanisms.

### Error Wrapping with %w (confidence 65, source: code patterns)
Errors from external dependencies (e.g. Redis ping) are wrapped with `fmt.Errorf` and `%w` to preserve the error chain; errors originating locally are returned directly. Example: `return nil, fmt.Errorf("could not connect to redis: %w", err)`. Evidence: internal/redis/client.go.

### Non-Fatal Error Logging in Handlers (confidence 70, source: code patterns)
In HTTP handlers/middleware, errors from repository calls are logged but do not abort the request — the handler continues with degraded/default behavior rather than returning an HTTP error. Evidence: internal/router/ban_handler.go, internal/router/check_middleware.go.
