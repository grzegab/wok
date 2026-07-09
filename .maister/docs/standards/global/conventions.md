## Development Conventions

### Predictable Structure
Organize files and directories in a logical, navigable layout.

### Up-to-Date Documentation
Keep README files current with setup steps, architecture overview, and contribution guidelines.

### Clean Version Control
Write clear commit messages, use feature branches, and add meaningful descriptions to pull requests.

### Environment Variables
Store configuration in environment variables; never commit secrets or API keys.

### Minimal Dependencies
Keep dependencies lean and up-to-date; document why major ones are included.

### Consistent Reviews
Follow a defined code review process with clear expectations for reviewers and authors.

### Testing Standards
Define required test coverage (unit, integration, etc.) before merging.

### Feature Flags
Use flags for incomplete features instead of long-lived branches.

### Changelog Updates
Maintain a changelog or release notes for significant changes.

### Build What's Needed
Avoid speculative code and "just in case" additions (see minimal-implementation.md).

### Interface-Based Dependency Injection for Repositories (confidence 80, source: code patterns)
Repository implementations are accepted by handlers/middleware/router as the `IpRepository` interface type rather than concrete structs, enabling swappable in-memory/Redis backends decided at startup in `main()`. Evidence: 4/4 consumer sites.

### Runtime Debug Flag (confidence 70, source: code patterns)
Replaces the previous build-tag approach (twin `_on`/`_off` files): debug mode is controlled by a `-D` boolean CLI flag parsed in `main()` and threaded as a plain `bool` parameter into constructors, rather than build constraints or environment variables. Consumers conditionally attach tracing/logging hooks when the flag is true. Evidence: cmd/web/main.go (`flag.Bool("D", false, "enable debug mode (verbose logging, SQL and Redis command tracing)")`, passed to `postgres.NewClient(dsn, debug)` and `redis.NewClient(addr, debug)`); internal/postgres/client.go (`NewClient(dsn string, debug bool)` attaches a `pgxpool` `tracelog.TraceLog` tracer when `debug` is true); internal/redis/client.go (`NewClient(addr string, debug bool)` attaches a `debugHook` implementing `redis.Hook` when `debug` is true).
