package logger

import "context"

// Logger defines the interface for logging operations, requiring context for trace correlation.
// It allows for different logging implementations (e.g., Zap, Logrus, OpenTelemetry).
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
