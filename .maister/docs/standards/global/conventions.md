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

### Build-Tag Variant Files (confidence 60, source: code patterns)
Conditional compilation uses Go build constraints in twin files sharing a function signature, named with an `_on`/`_off` suffix (e.g. `debug_on.go`/`debug_off.go`), rather than runtime flags. Evidence: cmd/web/debug_on.go, cmd/web/debug_off.go.
