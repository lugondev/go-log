# go-log

A high-performance Go logging library that seamlessly integrates OpenTelemetry and Zap for structured, context-aware logging with distributed tracing support.

## Features

- **Structured Logging**: Rich field support with automatic type handling
- **Context-Aware**: Automatic trace correlation with OpenTelemetry spans
- **Multiple Log Levels**: Debug, Info, Warn, Error, Fatal, Panic
- **Dual Output Formats**: JSON for production, colored console for development
- **OpenTelemetry Integration**: Native support for distributed tracing
- **High Performance**: Built on Zap logger for optimal performance
- **Formatted Logging**: Printf-style formatted logging methods
- **Field Inheritance**: Create loggers with predefined fields using `WithFields`
- **Caller Information**: Automatic file, line, and function name tracking
- **Configurable Scope**: Custom instrumentation scope for OpenTelemetry

## Installation

```bash
go get github.com/lugondev/go-log
```

## Quick Start

```go
package main

import (
    "context"
    "github.com/lugondev/go-log"
)

func main() {
    // Create a new logger
    log, err := logger.NewLogger(&logger.Option{Format: "json"})
    if err != nil {
        panic(err)
    }
    defer log.Sync()

    ctx := context.Background()
    
    // Basic logging
    log.Info(ctx, "Application started")
    log.Infof(ctx, "Server listening on port %d", 8080)
}
```

## Usage Examples

### Basic Logging

```go
package main

import (
    "context"
    "github.com/lugondev/go-log"
)

func main() {
    // Create logger with JSON format (production)
    log, err := logger.NewLogger(&logger.Option{Format: "json"})
    if err != nil {
        panic(err)
    }
    defer log.Sync()

    ctx := context.Background()

    // Log at different levels
    log.Debug(ctx, "Debugging application flow")
    log.Info(ctx, "Application started successfully")
    log.Warn(ctx, "This is a warning message")
    log.Error(ctx, "An error occurred")

    // Formatted logging
    log.Infof(ctx, "User %s logged in from %s", "john", "192.168.1.1")
    log.Errorf(ctx, "Failed to process order %d: %v", 12345, err)
}
```

### Structured Logging with Fields

```go
package main

import (
    "context"
    "time"
    "github.com/lugondev/go-log"
)

func main() {
    log, err := logger.NewLogger(&logger.Option{Format: "json"})
    if err != nil {
        panic(err)
    }
    defer log.Sync()

    ctx := context.Background()

    // Log with structured fields
    log.Info(ctx, "User login attempt", map[string]any{
        "user_id":    12345,
        "username":   "john_doe",
        "ip_address": "192.168.1.100",
        "success":    true,
        "duration":   250 * time.Millisecond,
        "timestamp":  time.Now(),
    })

    // Supported field types
    log.Info(ctx, "Various field types", map[string]any{
        "string_field":  "hello world",
        "int_field":     42,
        "int64_field":   int64(123456789),
        "float_field":   3.14159,
        "bool_field":    true,
        "slice_field":   []string{"a", "b", "c"},
        "error_field":   fmt.Errorf("sample error"),
        "nil_field":     nil,
        "time_field":    time.Now(),
    })
}
```

### Logger with Predefined Fields

```go
package main

import (
    "context"
    "github.com/lugondev/go-log"
)

func main() {
    log, err := logger.NewLogger(&logger.Option{Format: "json"})
    if err != nil {
        panic(err)
    }
    defer log.Sync()

    ctx := context.Background()

    // Create service logger with common fields
    serviceLogger := log.WithFields(map[string]any{
        "service":     "user-service",
        "version":     "1.2.3",
        "environment": "production",
    })

    // All logs from serviceLogger will include the predefined fields
    serviceLogger.Info(ctx, "Service started")
    
    // Create user-specific logger
    userLogger := serviceLogger.WithFields(map[string]any{
        "user_id":  12345,
        "session":  "sess_abc123",
    })

    // This log will include service fields + user fields
    userLogger.Info(ctx, "User action performed")
    userLogger.Error(ctx, "Failed to save user data", map[string]any{
        "error_code": "DB_ERROR",
        "table":      "users",
    })

    // Create admin logger with additional fields
    adminLogger := userLogger.WithFields(map[string]any{
        "admin": true,
        "role":  "super_admin",
    })
    
    adminLogger.Warn(ctx, "Admin performed sensitive operation")
}
```

### Console Format for Development

```go
package main

import (
    "context"
    "github.com/lugondev/go-log"
)

func main() {
    // Create logger with colored console output
    log, err := logger.NewLogger(&logger.Option{Format: "console"})
    if err != nil {
        panic(err)
    }
    defer log.Sync()

    ctx := context.Background()

    // Console format provides colored, human-readable output
    log.Debug(ctx, "Debug message", map[string]any{"debug_flag": true})
    log.Info(ctx, "Info message", map[string]any{"status": "ok"})
    log.Warn(ctx, "Warning message", map[string]any{"warning_code": 404})
    log.Error(ctx, "Error message", map[string]any{"error_code": 500})
}
```

### OpenTelemetry Integration

```go
package main

import (
    "context"
    "github.com/lugondev/go-log"
)

func main() {
    // Create logger with custom instrumentation scope
    log, err := logger.NewLogger(&logger.Option{
        Format:       "json",
        ScopeName:    "my-application",
        ScopeVersion: "1.0.0",
    })
    if err != nil {
        panic(err)
    }
    defer log.Sync()

    // When used with OpenTelemetry tracing, logs automatically include:
    // - trace_id: Links logs to distributed traces
    // - span_id: Links logs to specific spans
    // - Caller information (file, line, function)
    
    ctx := context.Background()
    log.Info(ctx, "Operation completed successfully")
}
```

### Fatal Hook for Cleanup

```go
package main

import (
    "context"
    "fmt"
    "github.com/lugondev/go-log"
)

func main() {
    log, err := logger.NewLogger(&logger.Option{Format: "json"})
    if err != nil {
        panic(err)
    }
    defer log.Sync()

    // Set cleanup hook for fatal errors
    log.SetFatalHook(func() {
        fmt.Println("Performing cleanup before exit...")
        // Close database connections, flush caches, etc.
    })

    ctx := context.Background()
    
    // This will call the hook (but won't actually exit in this implementation)
    // log.Fatal(ctx, "Critical system failure")
    
    // For actual panics that terminate the program
    // log.Panic(ctx, "Unrecoverable error occurred")
}
```

## API Reference

### Logger Interface

```go
type Logger interface {
	Debug(ctx context.Context, args ...any)
	Info(ctx context.Context, args ...any)
	Warn(ctx context.Context, args ...any)
	Error(ctx context.Context, args ...any)
	Fatal(ctx context.Context, args ...any) // Note: Implementations might not exit
	Panic(ctx context.Context, args ...any) // Note: Implementations should panic

	Debugf(ctx context.Context, template string, args ...any)
	Infof(ctx context.Context, template string, args ...any)
	Warnf(ctx context.Context, template string, args ...any)
	Errorf(ctx context.Context, template string, args ...any)
	Fatalf(ctx context.Context, template string, args ...any) // Note: Implementations might not exit
	Panicf(ctx context.Context, template string, args ...any) // Note: Implementations should panic

	// WithFields returns a new Logger instance with the provided fields added
	// to subsequent log entries. Context must still be passed to the methods
	// of the returned logger.
	WithFields(fields map[string]any) Logger

	// Sync flushes any buffered log entries
	Sync() error
}
```

### Option Struct

```go
type Option struct {
	Format       string // "json" or "console"
	ScopeName    string // OpenTelemetry instrumentation scope name
	ScopeVersion string // OpenTelemetry instrumentation scope version
}
```

### Additional Methods

The logger also provides a `SetFatalHook` method for cleanup operations:

```go
func (l *_logger) SetFatalHook(hook func())
```

Set a function to be called before the application exits when `Fatal` or `Fatalf` is called.

### Creating a Logger

```go
func NewLogger(opt *Option) (Logger, error)
```

Creates a new logger instance with the specified options.

## Supported Field Types

The logger automatically handles various Go types when logging structured fields:

| Go Type | OpenTelemetry Type | Example |
|---------|-------------------|---------|
| `string` | String | `"hello world"` |
| `int`, `int32`, `int64` | Int64 | `42`, `int64(123)` |
| `float32`, `float64` | Float64 | `3.14159` |
| `bool` | Bool | `true`, `false` |
| `time.Time` | String (RFC3339Nano) | `time.Now()` |
| `error` | String (error message) | `fmt.Errorf("failed")` |
| `[]string`, `[]int` | String (formatted) | `[]string{"a", "b"}` |
| `nil` | String (`"<nil>"`) | `nil` |
| Other types | String (formatted with %+v) | Any struct or type |

## Output Formats

### JSON Format (`"json"`)

Structured JSON output suitable for production environments and log aggregation systems:

```json
{
  "timestamp": "2024-01-15T10:30:45.123Z",
  "level": "INFO",
  "message": "User login attempt",
  "user_id": 12345,
  "username": "john_doe",
  "success": true,
  "caller": "main.go:45",
  "trace_id": "abc123...",
  "span_id": "def456..."
}
```

### Console Format (`"console"`)

Human-readable colored output for development:

```
2024-01-15T10:30:45.123Z INFO User login attempt 
	user_id=12345
	username=john_doe
	success=true
caller=main.go:45
```

## OpenTelemetry Integration

When OpenTelemetry tracing is configured in your application, the logger automatically:

- **Extracts trace context** from the provided context
- **Adds trace_id and span_id** to all log entries
- **Links logs to traces** for distributed debugging
- **Sends logs to OpenTelemetry collectors** when configured

The logger uses the global OpenTelemetry LoggerProvider. If none is configured, it falls back to a no-op provider.

## Performance Considerations

- **High Performance**: Built on Zap, one of the fastest Go logging libraries
- **Memory Efficient**: Uses object pooling for log records
- **Minimal Allocations**: Optimized field handling and string building
- **Concurrent Safe**: Thread-safe for use across multiple goroutines
- **Buffered Output**: Use `Sync()` to ensure all logs are flushed

## Best Practices

### Context Usage
Always pass a valid context to logging methods for proper trace correlation:

```go
// Good
log.Info(ctx, "Operation completed")

// Avoid - will work but won't have trace correlation
log.Info(context.Background(), "Operation completed")
```

### Field Organization
Use `WithFields` for common fields to avoid repetition:

```go
// Good - create logger with common fields
requestLogger := log.WithFields(map[string]any{
    "request_id": requestID,
    "user_id": userID,
})
requestLogger.Info(ctx, "Processing request")
requestLogger.Info(ctx, "Request completed")

// Less efficient - repeating fields
log.Info(ctx, "Processing request", map[string]any{"request_id": requestID, "user_id": userID})
log.Info(ctx, "Request completed", map[string]any{"request_id": requestID, "user_id": userID})
```

### Error Handling
Include structured error information:

```go
// Good
log.Error(ctx, "Database operation failed", map[string]any{
    "operation": "INSERT",
    "table": "users",
    "error": err.Error(),
    "retry_count": retries,
})

// Less informative
log.Errorf(ctx, "Database error: %v", err)
```

### Production vs Development
Use different formats for different environments:

```go
// Production
log, _ := logger.NewLogger(&logger.Option{Format: "json"})

// Development
log, _ := logger.NewLogger(&logger.Option{Format: "console"})
```

## Requirements

- Go 1.21 or later
- OpenTelemetry Go SDK v1.36.0+
- Zap v1.27.0+

## License

[MIT License](LICENSE)

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

For bug reports and feature requests, please use the [GitHub Issues](https://github.com/lugondev/go-log/issues).
