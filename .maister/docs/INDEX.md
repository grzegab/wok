# Documentation Index

**IMPORTANT**: Read this file at the beginning of any development task to understand available documentation and standards.

## Quick Reference

### Project Documentation
Project-level documentation covering vision, goals, and technology choices. Vision and Tech Stack are documented; Roadmap and Architecture were skipped by the user during setup.

### Technical Standards
Coding standards, conventions, and best practices organized by domain. Initialized categories: global, backend, testing. Frontend standards were skipped during initialization.

---

## Project Documentation

Located in `.maister/docs/project/`

### Vision (`project/vision.md`)
WOK's pitch, problem statement, target user (single personal user), key features (company/alias tracking, CV versioning, status tracking with auto-expiry, blacklist marking, IP-based access blocking), success criteria, and differentiators.

### Tech Stack (`project/tech-stack.md`)
Go 1.26 stdlib-first backend, server-side rendered HTML via `html/template` (no frontend framework), Chi v5 router, Redis-backed IP-ban storage with in-memory fallback, Docker/Docker Compose setup, and key dependencies (chi, go-redis, godotenv, Sentry).

### Roadmap (`project/roadmap.md`)
*Skipped during setup.* Not generated — the user chose not to include a roadmap. Add it later via the docs-manager skill if needed.

### Architecture (`project/architecture.md`)
*Skipped during setup.* Not generated — the user chose not to include an architecture document. Add it later via the docs-manager skill if needed.

---

## Technical Standards

### Global Standards

Located in `.maister/docs/standards/global/`

#### Coding Style (`standards/global/coding-style.md`)
Naming consistency, automatic formatting, descriptive names, focused functions, uniform indentation, no dead code, no backward compatibility unless required, DRY (Don't Repeat Yourself), package naming, import grouping, type naming, constructor pattern, and file naming.

#### Commenting (`standards/global/commenting.md`)
Letting code speak for itself, commenting sparingly, and avoiding change-log style comments in code.

#### Development Conventions (`standards/global/conventions.md`)
Predictable project structure, up-to-date documentation, clean version control practices, environment variable handling, minimal dependencies, consistent code reviews, testing standards, feature flags, changelog updates, building only what's needed, interface-based dependency injection for repositories, and build-tag variant files.

#### Error Handling (`standards/global/error-handling.md`)
Clear user-facing messages, failing fast, typed exceptions, centralized error handling, graceful degradation, retry with backoff, resource cleanup, error wrapping with %w, and non-fatal error logging in handlers.

#### Minimal Implementation (`standards/global/minimal-implementation.md`)
Building only what you need, maintaining clear purpose, deleting exploration artifacts, avoiding future stubs and speculative abstractions, reviewing before commit, and treating unused code as debt.

#### Validation (`standards/global/validation.md`)
Server-side validation always required, client-side validation for UX feedback, validating early, specific error messages, allowlists over blocklists, type/format checks, input sanitization, business rule validation, and consistent enforcement across layers.

### Backend Standards

Located in `.maister/docs/standards/backend/`

#### API Design (`standards/backend/api.md`)
RESTful principles, consistent naming, versioning strategy, plural nouns for resources, limited nesting depth, query parameter usage, proper HTTP status codes, and rate limit headers.

#### Database Migrations (`standards/backend/migrations.md`)
Reversible migrations, small and focused changes, zero-downtime awareness, separating schema and data changes, careful indexing, descriptive migration names, and version control practices.

#### Models (`standards/backend/models.md`)
Clear naming, timestamps, database constraints, appropriate data types, indexing foreign keys, multi-layer validation, clear relationships, and practical normalization.

#### Database Queries (`standards/backend/queries.md`)
Parameterized queries, avoiding N+1 query patterns, selecting only needed columns, indexing strategic columns, use of transactions, query timeouts, and caching expensive queries.

#### Infrastructure (`standards/backend/infrastructure.md`)
Multi-stage Docker builds, building from the project root, non-root container user, static binaries via CGO_ENABLED=0, IP blocking for production access, Go toolchain version alignment, pinned module versions, dependency caching via layered COPY, environment variable naming convention, minimal curated dependency set, OS package upgrades in both build stages, `.env.default` as documented template, and Redis as the sole external service dependency.

### Frontend Standards

*Not initialized for this project.* If you need frontend standards, you can:
- *Add them manually using the docs-manager skill*
- *Run `/maister:standards-discover --scope=frontend` to auto-discover*

### Testing Standards

Located in `.maister/docs/standards/testing/`

#### Test Writing (`standards/testing/test-writing.md`)
Testing behavior rather than implementation, clear test names, mocking external dependencies, fast execution, risk-based testing, balancing coverage with velocity, critical path focus, and appropriate test depth.

---

## How to Use This Documentation

1. **Start Here**: Always read this INDEX.md first to understand what documentation exists
2. **Project Context**: Read relevant project documentation before starting work
3. **Standards**: This index only points to the standards — open and follow the specific standard files relevant to your task; don't rely on the index alone
4. **Keep Updated**: Update documentation when making significant changes
5. **Customize**: Adapt all documentation to your project's specific needs

## Updating Documentation

- Project documentation should be updated when goals, tech stack, or architecture changes
- Technical standards should be updated when team conventions evolve
- Always update INDEX.md when adding, removing, or significantly changing documentation
