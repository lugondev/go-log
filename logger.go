package logger

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"go.opentelemetry.io/otel/log"
	"go.opentelemetry.io/otel/log/global"
	"go.opentelemetry.io/otel/log/noop"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var (
	_ Logger = (*_logger)(nil) // Compile-time check
	_ Logger = (*_loggerWithFields)(nil)

	emptyAttrs = make([]log.KeyValue, 0)
	recordPool = sync.Pool{
		New: func() any {
			return &log.Record{}
		},
	}
	// Pool for attribute slices to reduce allocations
	attrsPool = sync.Pool{
		New: func() any {
			return make([]log.KeyValue, 0, 8) // Pre-allocate capacity for common case
		},
	}
	// Pool for string builders to reduce allocations in console logging
	builderPool = sync.Pool{
		New: func() any {
			return &strings.Builder{}
		},
	}
	// Pool for zap fields slices to reduce allocations
	zapFieldsPool = sync.Pool{
		New: func() any {
			return make([]zap.Field, 0, 8) // Pre-allocate capacity for common case
		},
	}
	// Pool for runtime.Callers slices to reduce allocations
	callersPool = sync.Pool{
		New: func() any {
			return make([]uintptr, 1) // Only need 1 frame for our use case
		},
	}

	// Mutex to protect console output to prevent interleaved logs
	consoleMutex = &sync.Mutex{}
)

var (
	instrumentationScopeName    = "github.com/lugondev/go-log"
	instrumentationScopeVersion = "0.1.0"
)

type _logger struct {
	logger    log.Logger
	provider  log.LoggerProvider
	zapLogger *zap.Logger
	otp       *Option
	fatalHook func()
	level     zapcore.Level // Current log level for early filtering
}

type _loggerWithFields struct {
	*_logger
	fields []log.KeyValue
}

type flusher interface {
	ForceFlush(context.Context) error
}

type Option struct {
	Format       string
	ScopeName    string
	ScopeVersion string
	Level        zapcore.Level // Log level for filtering (default: Debug)
}

// NewLogger creates a new _logger.
func NewLogger(opt *Option) (Logger, error) {
	// Initialize Zap logger for stdout
	encoderConfig := zap.NewProductionEncoderConfig()
	encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	encoderConfig.TimeKey = "timestamp"
	encoderConfig.LevelKey = "level"
	encoderConfig.NameKey = "logger"
	encoderConfig.CallerKey = "caller"
	encoderConfig.MessageKey = "message"
	encoderConfig.StacktraceKey = "stacktrace"

	var encoder zapcore.Encoder
	if strings.Contains(strings.ToLower(opt.Format), "json") {
		encoder = zapcore.NewJSONEncoder(encoderConfig)
		opt.Format = "json"
	} else {
		// Enable colors for console format
		encoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
		encoder = zapcore.NewConsoleEncoder(encoderConfig)
		opt.Format = "console"
	}

	// Set default level to Debug if not specified
	if opt.Level == 0 {
		opt.Level = zapcore.DebugLevel
	}

	// Create atomic level for dynamic level changes
	atomicLevel := zap.NewAtomicLevelAt(opt.Level)

	core := zapcore.NewCore(
		encoder,
		zapcore.AddSync(zapcore.Lock(zapcore.AddSync(os.Stdout))),
		atomicLevel,
	)
	zapLogger := zap.New(core, zap.AddCaller(), zap.AddCallerSkip(2))

	// Initialize OpenTelemetry logger
	provider := global.GetLoggerProvider()
	if _, ok := provider.(*noop.LoggerProvider); ok || provider == nil {
		zapLogger.Info("No global OpenTelemetry LoggerProvider configured. Using no-op logger.")
		provider = noop.NewLoggerProvider()
	}
	if opt.ScopeName != "" {
		instrumentationScopeName = opt.ScopeName
	}
	if opt.ScopeVersion != "" {
		instrumentationScopeVersion = opt.ScopeVersion
	}

	logger := provider.Logger(
		instrumentationScopeName,
		log.WithInstrumentationVersion(instrumentationScopeVersion),
	)

	otelLogger := &_logger{
		logger:    logger,
		provider:  provider,
		zapLogger: zapLogger,
		otp:       opt,
		level:     opt.Level, // Initialize level for early filtering
	}

	otelLogger.Info(context.Background(), "Logger initialized successfully")
	return otelLogger, nil
}

func (l *_logger) SetFatalHook(hook func()) {
	l.fatalHook = hook
}

// SetLevel sets the minimum log level that will be processed
func (l *_logger) SetLevel(level string) error {
	switch strings.ToLower(level) {
	case "debug":
		l.level = zapcore.DebugLevel
	case "info":
		l.level = zapcore.InfoLevel
	case "warn", "warning":
		l.level = zapcore.WarnLevel
	case "error":
		l.level = zapcore.ErrorLevel
	case "fatal":
		l.level = zapcore.FatalLevel
	case "panic":
		l.level = zapcore.PanicLevel
	default:
		return fmt.Errorf("invalid log level: %s", level)
	}

	// Also update the Zap logger's level
	if atomicLevel, ok := l.zapLogger.Core().(zapcore.LevelEnabler); ok {
		if al, ok := atomicLevel.(*zap.AtomicLevel); ok {
			al.SetLevel(l.level)
		}
	}

	return nil
}

func getRecordFromPool() *log.Record {
	return recordPool.Get().(*log.Record)
}

func putRecordToPool(record *log.Record) {
	recordPool.Put(record)
}

// getAttrsFromPool retrieves a pre-allocated attribute slice from the pool
func getAttrsFromPool() []log.KeyValue {
	return attrsPool.Get().([]log.KeyValue)[:0] // Reset length but keep capacity
}

// putAttrsToPool returns an attribute slice to the pool
func putAttrsToPool(attrs []log.KeyValue) {
	// Only return reasonably sized slices to the pool to prevent memory leaks
	if cap(attrs) <= 64 {
		attrsPool.Put(attrs[:0]) // Clear the slice before returning to pool
	}
}

// getBuilderFromPool retrieves a pre-allocated strings.Builder from the pool
func getBuilderFromPool() *strings.Builder {
	builder := builderPool.Get().(*strings.Builder)
	builder.Reset() // Ensure the builder is empty
	return builder
}

// putBuilderToPool returns a strings.Builder to the pool
func putBuilderToPool(builder *strings.Builder) {
	// Reset the builder before returning it to the pool
	builder.Reset()
	builderPool.Put(builder)
}

// getZapFieldsFromPool retrieves a pre-allocated zap.Field slice from the pool
func getZapFieldsFromPool() []zap.Field {
	return zapFieldsPool.Get().([]zap.Field)[:0] // Reset length but keep capacity
}

// putZapFieldsToPool returns a zap.Field slice to the pool
func putZapFieldsToPool(fields []zap.Field) {
	// Only return reasonably sized slices to the pool to prevent memory leaks
	if cap(fields) <= 64 {
		zapFieldsPool.Put(fields[:0]) // Clear the slice before returning to pool
	}
}

// getCallersFromPool retrieves a pre-allocated uintptr slice from the pool
func getCallersFromPool() []uintptr {
	return callersPool.Get().([]uintptr)
}

// putCallersToPool returns a uintptr slice to the pool
func putCallersToPool(callers []uintptr) {
	callersPool.Put(callers)
}

func getCallerFrame(skip int) (pc uintptr, file string, line int, ok bool) {
	// Get a pre-allocated slice from the pool
	rpc := getCallersFromPool()
	defer putCallersToPool(rpc)

	n := runtime.Callers(skip+2, rpc[:])
	if n < 1 {
		return
	}
	frame, _ := runtime.CallersFrames(rpc).Next()
	return frame.PC, frame.File, frame.Line, frame.PC != 0
}

func (l *_logger) emitLog(ctx context.Context, severity log.Severity, body log.Value, attrs ...log.KeyValue) {
	if ctx == nil {
		ctx = context.Background()
		attrs = append(attrs, log.Bool("missing_context", true))
	}

	record := getRecordFromPool()
	defer putRecordToPool(record)

	record.SetTimestamp(time.Now())
	record.SetObservedTimestamp(record.Timestamp())
	record.SetSeverity(severity)
	record.SetBody(body)

	// Add caller information
	if pc, file, line, ok := getCallerFrame(3); ok {
		funcName := runtime.FuncForPC(pc).Name()
		record.AddAttributes(
			log.String(string(semconv.CodeFunctionKey), funcName),
			log.String(string(semconv.CodeFilepathKey), file),
			log.Int(string(semconv.CodeLineNumberKey), line),
		)
	}

	// Add trace context if available
	if spanCtx := trace.SpanContextFromContext(ctx); spanCtx.IsValid() {
		record.AddAttributes(
			log.String("trace_id", spanCtx.TraceID().String()),
			log.String("span_id", spanCtx.SpanID().String()),
		)
	}

	record.AddAttributes(attrs...)

	// Log to stdout using Zap
	zapLevel := otelSeverityToZapLevel(severity)
	fields := otelAttrsToZapFields(attrs)
	// Ensure fields are returned to the pool after use
	defer putZapFieldsToPool(fields)

	if l.otp.Format == "console" {
		// For console format, output in Key=Value format
		l.logConsoleFormat(zapLevel, body.AsString(), fields)
	} else {
		// Use JSON format
		l.zapLogger.Log(zapLevel, body.AsString(), fields...)
	}

	// Send to OpenTelemetry
	l.logger.Emit(ctx, *record)
}

func otelSeverityToZapLevel(severity log.Severity) zapcore.Level {
	switch severity {
	case log.SeverityDebug:
		return zapcore.DebugLevel
	case log.SeverityInfo:
		return zapcore.InfoLevel
	case log.SeverityWarn:
		return zapcore.WarnLevel
	case log.SeverityError:
		return zapcore.ErrorLevel
	case log.SeverityFatal:
		return zapcore.FatalLevel
	default:
		return zapcore.InfoLevel
	}
}

func (l *_logger) logConsoleFormat(level zapcore.Level, msg string, fields []zap.Field) {
	// Get a pooled builder
	output := getBuilderFromPool()
	defer putBuilderToPool(output)

	// Pre-allocate a reasonable size to reduce reallocations
	output.Grow(256)

	// Add timestamp
	output.WriteString(time.Now().Format("2006-01-02T15:04:05.000Z07:00"))
	output.WriteString(" ")

	// Add level
	levelStr := level.CapitalString()
	switch level {
	case zapcore.DebugLevel:
		levelStr = "\033[36mDEBUG\033[0m" // Cyan
	case zapcore.InfoLevel:
		levelStr = "\033[32mINFO\033[0m" // Green
	case zapcore.WarnLevel:
		levelStr = "\033[33mWARN\033[0m" // Yellow
	case zapcore.ErrorLevel:
		levelStr = "\033[31mERROR\033[0m" // Red
	case zapcore.FatalLevel:
		levelStr = "\033[35mFATAL\033[0m" // Magenta
	}
	output.WriteString(levelStr)
	output.WriteString(" ")

	// Add message
	output.WriteString(msg)

	// Add fields in Key=Value format (each on a new line with 2 tab indentation)
	for _, field := range fields {
		output.WriteString(" \n\t")
		output.WriteString(field.Key)
		output.WriteString("=")

		// Format field value based on type
		switch field.Type {
		case zapcore.StringType:
			output.WriteString(field.String)
		case zapcore.Int64Type:
			// Avoid fmt.Sprintf for integers
			output.WriteString(strconv.FormatInt(field.Integer, 10))
		case zapcore.Float64Type:
			if field.Interface != nil {
				if val, ok := field.Interface.(float64); ok {
					// Use strconv for float formatting
					output.WriteString(strconv.FormatFloat(val, 'g', -1, 64))
				} else {
					fmt.Fprintf(output, "%v", field.Interface)
				}
			} else {
				output.WriteString("<nil>")
			}
		case zapcore.BoolType:
			if field.Integer == 1 {
				output.WriteString("true")
			} else {
				output.WriteString("false")
			}
		default:
			if field.Interface != nil {
				fmt.Fprintf(output, "%v", field.Interface)
			} else {
				output.WriteString("<nil>")
			}
		}
	}

	// Add caller information
	if pc, file, line, ok := getCallerFrame(4); ok {
		output.WriteString(" \ncaller=")
		// Avoid fmt.Sprintf for file:line
		output.WriteString(file)
		output.WriteString(":")
		output.WriteString(strconv.Itoa(line))
		_ = pc // unused but needed for getCallerFrame
	}

	output.WriteString("\n")

	// Lock to prevent interleaved output from multiple goroutines
	consoleMutex.Lock()
	// Print directly from the builder without creating an intermediate string
	fmt.Print(output.String())
	consoleMutex.Unlock()
}

func otelAttrsToZapFields(attrs []log.KeyValue) []zap.Field {
	if len(attrs) == 0 {
		return nil
	}

	// Get a pre-allocated slice from the pool
	fields := getZapFieldsFromPool()
	// Ensure the slice has enough capacity
	if cap(fields) < len(attrs) {
		// If the pooled slice is too small, resize it
		fields = append(fields, make([]zap.Field, 0, len(attrs)-cap(fields))...)
	}

	// Pre-allocate the fields slice to avoid repeated slice growth
	fields = fields[:0]
	if cap(fields) < len(attrs) {
		fields = make([]zap.Field, 0, len(attrs))
	}

	for _, attr := range attrs {
		// Handle different kinds of values appropriately
		key := string(attr.Key)
		switch attr.Value.Kind() {
		case log.KindInt64:
			// For Int64, use zap.Int64 instead of converting to string
			fields = append(fields, zap.Int64(key, attr.Value.AsInt64()))
		case log.KindFloat64:
			// For Float64, use zap.Float64 instead of converting to string
			fields = append(fields, zap.Float64(key, attr.Value.AsFloat64()))
		case log.KindBool:
			// For Bool, use zap.Bool instead of converting to string
			fields = append(fields, zap.Bool(key, attr.Value.AsBool()))
		case log.KindString:
			// For String, use the string value directly without AsString()
			fields = append(fields, zap.String(key, attr.Value.AsString()))
		default:
			// For other types, use AsString as before
			fields = append(fields, zap.String(key, attr.Value.AsString()))
		}
	}
	return fields
}

func argsToValue(args ...any) log.Value {
	switch len(args) {
	case 0:
		return log.StringValue("")
	case 1:
		if str, ok := args[0].(string); ok {
			return log.StringValue(str)
		}
		return log.StringValue(fmt.Sprint(args[0]))
	default:
		// Get a pooled builder
		sb := getBuilderFromPool()
		defer putBuilderToPool(sb)

		// Pre-allocate a reasonable size
		sb.Grow(64)

		for i, arg := range args {
			if i > 0 {
				sb.WriteByte(' ')
			}
			_, err := fmt.Fprint(sb, arg)
			if err != nil {
				return log.Value{}
			}
		}
		return log.StringValue(sb.String())
	}
}

func formatToValue(template string, args ...any) log.Value {
	if len(args) == 0 {
		return log.StringValue(template)
	}

	// For simple cases, use fmt.Sprintf directly
	if len(template) < 64 && len(args) <= 2 {
		return log.StringValue(fmt.Sprintf(template, args...))
	}

	// For more complex cases, use a pooled builder
	sb := getBuilderFromPool()
	defer putBuilderToPool(sb)

	// Pre-allocate a reasonable size based on template length and number of args
	sb.Grow(len(template) + 32*len(args))

	// Use Fprintf to write directly to the builder
	fmt.Fprintf(sb, template, args...)
	return log.StringValue(sb.String())
}

func extractMessageAndFields(args []any) (string, []log.KeyValue, bool) {
	if len(args) < 2 {
		return "", nil, false
	}

	msg, ok := args[0].(string)
	if !ok {
		return "", nil, false
	}

	switch fields := args[1].(type) {
	case map[string]string:
		if len(fields) == 0 {
			return msg, emptyAttrs, true
		}
		// Use pooled attribute slice
		attrs := getAttrsFromPool()
		// Ensure capacity
		if cap(attrs) < len(fields) {
			attrs = append(attrs, make([]log.KeyValue, 0, len(fields)-cap(attrs))...)
		}
		for k, v := range fields {
			attrs = append(attrs, log.String(k, v))
		}
		return msg, attrs, true
	case map[string]any:
		return msg, mapToAttributes(fields), true
	default:
		return "", nil, false
	}
}

func (l *_logger) log(ctx context.Context, severity log.Severity, args ...any) {
	span := trace.SpanFromContext(ctx)
	traceId := span.SpanContext().TraceID().String()
	spanId := span.SpanContext().SpanID().String()

	// Only add trace fields if they contain actual tracing information (not zero values)
	var traceFields []log.KeyValue
	if traceId != "00000000000000000000000000000000" {
		traceFields = append(traceFields, log.String("trace_id", traceId))
	}
	if spanId != "0000000000000000" {
		traceFields = append(traceFields, log.String("span_id", spanId))
	}

	if len(args) >= 2 {
		if msg, fields, ok := extractMessageAndFields(args); ok {
			l.emitLog(ctx, severity, log.StringValue(msg), append(fields, traceFields...)...)
			return
		}
	}
	l.emitLog(ctx, severity, argsToValue(args...), traceFields...)
}

func (l *_logger) Debug(ctx context.Context, args ...any) {
	// Early level check to avoid unnecessary processing
	if l.level > zapcore.DebugLevel {
		return
	}
	l.log(ctx, log.SeverityDebug, args...)
}

func (l *_logger) Info(ctx context.Context, args ...any) {
	// Early level check to avoid unnecessary processing
	if l.level > zapcore.InfoLevel {
		return
	}
	l.log(ctx, log.SeverityInfo, args...)
}

func (l *_logger) Warn(ctx context.Context, args ...any) {
	// Early level check to avoid unnecessary processing
	if l.level > zapcore.WarnLevel {
		return
	}
	l.log(ctx, log.SeverityWarn, args...)
}

func (l *_logger) Error(ctx context.Context, args ...any) {
	// Early level check to avoid unnecessary processing
	if l.level > zapcore.ErrorLevel {
		return
	}
	l.log(ctx, log.SeverityError, args...)
}

func (l *_logger) Fatal(ctx context.Context, args ...any) {
	// Fatal logs are always processed regardless of level
	l.log(ctx, log.SeverityFatal, args...)
	if l.fatalHook != nil {
		l.fatalHook()
	}
}

func (l *_logger) Panic(ctx context.Context, args ...any) {
	var msgValue log.Value
	if len(args) >= 2 {
		if msg, fields, ok := extractMessageAndFields(args); ok {
			msgValue = log.StringValue(msg)
			l.emitLog(ctx, log.SeverityFatal, msgValue, fields...)
		} else {
			msgValue = argsToValue(args...)
			l.emitLog(ctx, log.SeverityFatal, msgValue)
		}
	} else {
		msgValue = argsToValue(args...)
		l.emitLog(ctx, log.SeverityFatal, msgValue)
	}
	panic(msgValue.AsString())
}

func (l *_logger) Debugf(ctx context.Context, template string, args ...any) {
	// Early level check to avoid unnecessary processing
	if l.level > zapcore.DebugLevel {
		return
	}
	l.emitLog(ctx, log.SeverityDebug, formatToValue(template, args...))
}

func (l *_logger) Infof(ctx context.Context, template string, args ...any) {
	// Early level check to avoid unnecessary processing
	if l.level > zapcore.InfoLevel {
		return
	}
	l.emitLog(ctx, log.SeverityInfo, formatToValue(template, args...))
}

func (l *_logger) Warnf(ctx context.Context, template string, args ...any) {
	// Early level check to avoid unnecessary processing
	if l.level > zapcore.WarnLevel {
		return
	}
	l.emitLog(ctx, log.SeverityWarn, formatToValue(template, args...))
}

func (l *_logger) Errorf(ctx context.Context, template string, args ...any) {
	// Early level check to avoid unnecessary processing
	if l.level > zapcore.ErrorLevel {
		return
	}
	l.emitLog(ctx, log.SeverityError, formatToValue(template, args...))
}

func (l *_logger) Fatalf(ctx context.Context, template string, args ...any) {
	// Fatal logs are always processed regardless of level
	l.emitLog(ctx, log.SeverityFatal, formatToValue(template, args...))
	if l.fatalHook != nil {
		l.fatalHook()
	}
}

func (l *_logger) Panicf(ctx context.Context, template string, args ...any) {
	// Panic logs are always processed regardless of level
	msg := formatToValue(template, args...)
	l.emitLog(ctx, log.SeverityFatal, msg)
	panic(msg.AsString())
}

func mapToAttributes(fields map[string]any) []log.KeyValue {
	if len(fields) == 0 {
		return emptyAttrs
	}

	// Get a pre-allocated slice from the pool
	attrs := getAttrsFromPool()
	// Ensure the slice has enough capacity
	if cap(attrs) < len(fields) {
		// If the pooled slice is too small, resize it
		attrs = append(attrs, make([]log.KeyValue, 0, len(fields)-cap(attrs))...)
	}

	for k, v := range fields {
		switch val := v.(type) {
		case string:
			attrs = append(attrs, log.String(k, val))
		case int:
			attrs = append(attrs, log.Int(k, val))
		case int64:
			// Use Int64 directly instead of converting to string
			attrs = append(attrs, log.Int64(k, val))
		case float64:
			attrs = append(attrs, log.Float64(k, val))
		case bool:
			attrs = append(attrs, log.Bool(k, val))
		case []string, []int:
			attrs = append(attrs, log.String(k, fmt.Sprintf("%v", val)))
		case time.Time:
			attrs = append(attrs, log.String(k, val.Format(time.RFC3339Nano)))
		case error:
			if val != nil {
				attrs = append(attrs, log.String(k, val.Error()))
			} else {
				attrs = append(attrs, log.String(k, "<nil>"))
			}
		case nil:
			attrs = append(attrs, log.String(k, "<nil>"))
		default:
			attrs = append(attrs, log.String(k, fmt.Sprintf("%+v", v)))
		}
	}
	return attrs
}

func (l *_logger) Sync() error {
	if err := l.zapLogger.Sync(); err != nil {
		return fmt.Errorf("failed to sync zap logger: %w", err)
	}
	if p, ok := l.provider.(flusher); ok {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := p.ForceFlush(ctx); err != nil {
			return fmt.Errorf("failed to flush otel provider: %w", err)
		}
	}
	return nil
}

func (l *_logger) WithFields(fields map[string]any) Logger {
	return &_loggerWithFields{
		_logger: l,
		fields:  mapToAttributes(fields),
	}
}

func (l *_loggerWithFields) emitLog(ctx context.Context, severity log.Severity, body log.Value, attrs ...log.KeyValue) {
	if len(attrs) == 0 {
		l._logger.emitLog(ctx, severity, body, l.fields...)
		return
	}

	combinedAttrs := make([]log.KeyValue, 0, len(l.fields)+len(attrs))
	combinedAttrs = append(combinedAttrs, l.fields...)
	combinedAttrs = append(combinedAttrs, attrs...)
	l._logger.emitLog(ctx, severity, body, combinedAttrs...)
}

func (l *_loggerWithFields) Debug(ctx context.Context, args ...any) {
	// Early level check to avoid unnecessary processing
	if l._logger.level > zapcore.DebugLevel {
		return
	}
	if len(args) >= 2 {
		if msg, fields, ok := extractMessageAndFields(args); ok {
			l.emitLog(ctx, log.SeverityDebug, log.StringValue(msg), fields...)
			return
		}
	}
	l.emitLog(ctx, log.SeverityDebug, argsToValue(args...))
}

func (l *_loggerWithFields) Info(ctx context.Context, args ...any) {
	// Early level check to avoid unnecessary processing
	if l._logger.level > zapcore.InfoLevel {
		return
	}
	if len(args) >= 2 {
		if msg, fields, ok := extractMessageAndFields(args); ok {
			l.emitLog(ctx, log.SeverityInfo, log.StringValue(msg), fields...)
			return
		}
	}
	l.emitLog(ctx, log.SeverityInfo, argsToValue(args...))
}

func (l *_loggerWithFields) Warn(ctx context.Context, args ...any) {
	// Early level check to avoid unnecessary processing
	if l._logger.level > zapcore.WarnLevel {
		return
	}
	if len(args) >= 2 {
		if msg, fields, ok := extractMessageAndFields(args); ok {
			l.emitLog(ctx, log.SeverityWarn, log.StringValue(msg), fields...)
			return
		}
	}
	l.emitLog(ctx, log.SeverityWarn, argsToValue(args...))
}

func (l *_loggerWithFields) Error(ctx context.Context, args ...any) {
	// Early level check to avoid unnecessary processing
	if l._logger.level > zapcore.ErrorLevel {
		return
	}
	if len(args) >= 2 {
		if msg, fields, ok := extractMessageAndFields(args); ok {
			l.emitLog(ctx, log.SeverityError, log.StringValue(msg), fields...)
			return
		}
	}
	l.emitLog(ctx, log.SeverityError, argsToValue(args...))
}

func (l *_loggerWithFields) Fatal(ctx context.Context, args ...any) {
	// Fatal logs are always processed regardless of level
	if len(args) >= 2 {
		if msg, fields, ok := extractMessageAndFields(args); ok {
			l.emitLog(ctx, log.SeverityFatal, log.StringValue(msg), fields...)
			if l._logger.fatalHook != nil {
				l._logger.fatalHook()
			}
			return
		}
	}
	l.emitLog(ctx, log.SeverityFatal, argsToValue(args...))
	if l._logger.fatalHook != nil {
		l._logger.fatalHook()
	}
}

func (l *_loggerWithFields) Panic(ctx context.Context, args ...any) {
	// Panic logs are always processed regardless of level
	var msgValue log.Value
	if len(args) >= 2 {
		if msg, fields, ok := extractMessageAndFields(args); ok {
			msgValue = log.StringValue(msg)
			l.emitLog(ctx, log.SeverityFatal, msgValue, fields...)
		} else {
			msgValue = argsToValue(args...)
			l.emitLog(ctx, log.SeverityFatal, msgValue)
		}
	} else {
		msgValue = argsToValue(args...)
		l.emitLog(ctx, log.SeverityFatal, msgValue)
	}
	panic(msgValue.AsString())
}

func (l *_loggerWithFields) Debugf(ctx context.Context, template string, args ...any) {
	// Early level check to avoid unnecessary processing
	if l._logger.level > zapcore.DebugLevel {
		return
	}
	l.emitLog(ctx, log.SeverityDebug, formatToValue(template, args...))
}

func (l *_loggerWithFields) Infof(ctx context.Context, template string, args ...any) {
	// Early level check to avoid unnecessary processing
	if l._logger.level > zapcore.InfoLevel {
		return
	}
	l.emitLog(ctx, log.SeverityInfo, formatToValue(template, args...))
}

func (l *_loggerWithFields) Warnf(ctx context.Context, template string, args ...any) {
	// Early level check to avoid unnecessary processing
	if l._logger.level > zapcore.WarnLevel {
		return
	}
	l.emitLog(ctx, log.SeverityWarn, formatToValue(template, args...))
}

func (l *_loggerWithFields) Errorf(ctx context.Context, template string, args ...any) {
	// Early level check to avoid unnecessary processing
	if l._logger.level > zapcore.ErrorLevel {
		return
	}
	l.emitLog(ctx, log.SeverityError, formatToValue(template, args...))
}

func (l *_loggerWithFields) Fatalf(ctx context.Context, template string, args ...any) {
	// Fatal logs are always processed regardless of level
	l.emitLog(ctx, log.SeverityFatal, formatToValue(template, args...))
	if l._logger.fatalHook != nil {
		l._logger.fatalHook()
	}
}

func (l *_loggerWithFields) Panicf(ctx context.Context, template string, args ...any) {
	// Panic logs are always processed regardless of level
	msg := formatToValue(template, args...)
	l.emitLog(ctx, log.SeverityFatal, msg)
	panic(msg.AsString())
}

func (l *_loggerWithFields) SetLevel(level string) error {
	return l._logger.SetLevel(level)
}

func (l *_loggerWithFields) Sync() error {
	return l._logger.Sync()
}

func (l *_loggerWithFields) WithFields(fields map[string]any) Logger {
	newAttrs := mapToAttributes(fields)
	combinedAttrs := make([]log.KeyValue, 0, len(l.fields)+len(newAttrs))
	combinedAttrs = append(combinedAttrs, l.fields...)
	combinedAttrs = append(combinedAttrs, newAttrs...)

	return &_loggerWithFields{
		_logger: l._logger,
		fields:  combinedAttrs,
	}
}
