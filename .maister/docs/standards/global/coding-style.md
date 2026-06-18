## Coding Style

### Naming Consistency
Follow established naming patterns for variables, functions, classes, and files throughout the project.

### Automatic Formatting
Use automated tools to enforce consistent indentation, spacing, and line breaks.

### Descriptive Names
Choose names that clearly communicate intent; avoid cryptic abbreviations or single-letter identifiers outside tight loops.

### Focused Functions
Write functions that do one thing well; smaller functions are easier to read, test, and maintain.

### Uniform Indentation
Standardize on spaces or tabs and enforce with editor/linter settings.

### No Dead Code
Remove unused imports, commented-out blocks, and orphaned functions instead of leaving them behind.

### No Backward Compatibility Unless Required
Avoid extra code paths for backward compatibility unless explicitly needed.

### DRY (Don't Repeat Yourself)
Extract repeated logic into reusable functions or modules.

### Package Naming (confidence 88, source: code patterns)
Lowercase, single-word package names matching their directory (e.g. `config`, `redis`, `repository`, `router`, `templates`). No underscores or mixedCase. Evidence: 14/14 sampled files.

### Import Grouping (confidence 86, source: code patterns)
Import blocks separate the standard library group from third-party/internal (`github.com/grzegab/wok/...`) imports with a blank line; no further split between third-party and internal. Evidence: 7/8 multi-import files.
```go
import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)
```

### Type Naming (confidence 85, source: code patterns)
Exported struct/interface types use PascalCase (`Config`, `Router`, `IpRepository`); unexported fields use camelCase (`ipRepo`, `rdb`). Evidence: 6/6 exported types, 4/4 unexported fields sampled.

### Constructor Pattern (confidence 82, source: code patterns)
Types requiring initialization expose a `New()`-prefixed factory function returning a pointer, rather than exposing struct literals directly. Evidence: 4/4 structs requiring construction (`NewClient`, `NewInMemoryIpRepository`, `NewRedisIpRepository`, `router.New`).
Example: `func NewRedisIpRepository(rdb *redis.Client) *RedisIpRepository { return &RedisIpRepository{rdb: rdb} }`

### File Naming (confidence 78, source: code patterns)
Go source files use snake_case (e.g. `in_memory_ip_repository.go`, `check_middleware.go`, `ban_handler.go`). Evidence: 14/14 sampled files.
