# go-log

A Go logging library that integrates OpenTelemetry and Zap for structured, context-aware logging with tracing support.

## Features

- Structured logging with field support
- Context-aware logging with trace correlation
- Multiple log levels (Debug, Info, Warn, Error, Fatal, Panic)
- Formatted logging methods
- JSON and console output formats
- OpenTelemetry integration for distributed tracing
- High-performance logging via Zap

## Installation

```bash
go get github.com/lugondev/go-log
```

## Usage

### Basic Usage

```go
package main

import (
	"context"

	"github.com/lugondev/go-log"
)

func main() {
	// Create a new logger with JSON format
	log, err := logger.NewLogger(&logger.Option{Format: "json"})
	if err != nil {
		panic(err)
	}
	defer log.Sync()

	// Create a context
	ctx := context.Background()

	// Log messages at different levels
	log.Debug(ctx, "This is a debug message")
	log.Info(ctx, "This is an info message")
	log.Warn(ctx, "This is a warning message")
	log.Error(ctx, "This is an error message")
	
	// Formatted logging
	log.Infof(ctx, "Hello, %s!", "world")
}
```

### Structured Logging

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

	// Add fields to a log message
	log.Info(ctx, "User logged in", map[string]any{
		"user_id": 123,
		"email":   "user@example.com",
		"admin":   false,
	})

	// Create a logger with predefined fields
	userLogger := log.WithFields(map[string]any{
		"user_id": 123,
		"session": "abc-123",
	})

	// All logs from this logger will include the predefined fields
	userLogger.Info(ctx, "User performed an action")
	userLogger.Error(ctx, "Failed to process request", map[string]any{
		"error_code": 500,
		"duration":   time.Since(time.Now()),
	})

	// Create nested loggers with additional fields
	adminLogger := userLogger.WithFields(map[string]any{
		"admin": true,
	})
	adminLogger.Info(ctx, "Admin action performed")
}
```

### Console Output Format

```go
// Create a logger with console output format
log, err := logger.NewLogger(&logger.Option{Format: "console"})
if err != nil {
	panic(err)
}
```

### Custom Instrumentation Scope

```go
// Create a logger with custom instrumentation scope
log, err := logger.NewLogger(&logger.Option{
	Format:       "json",
	ScopeName:    "my-application",
	ScopeVersion: "1.0.0",
})
if err != nil {
	panic(err)
}
```

### Fatal Hook

```go
// Set a custom hook to be called before exiting on Fatal logs
log, err := logger.NewLogger(&logger.Option{Format: "json"})
if err != nil {
	panic(err)
}

// Set a custom hook to be called before exiting
log.SetFatalHook(func() {
	// Perform cleanup operations
	fmt.Println("Cleaning up before exit...")
})

// This will call the hook before exiting
log.Fatal(ctx, "Fatal error occurred")
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

## License

[MIT License](LICENSE)

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.