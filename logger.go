package logger

import (
	"context"
	"fmt"
	"os"
	"runtime"
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
)

var (
	instrumentationScopeName    = "github.com/lugondev/go-log"
	instrumentationScopeVersion = "0.1.0"
)

type _logger struct {
	logger    log.Logger
	provider  log.LoggerProvider
	zapLogger *zap.Logger
	fatalHook func()
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
	} else {
		encoder = zapcore.NewConsoleEncoder(encoderConfig)
	}

	core := zapcore.NewCore(
		encoder,
		zapcore.AddSync(zapcore.Lock(zapcore.AddSync(os.Stdout))),
		zap.NewAtomicLevelAt(zapcore.DebugLevel),
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
	}

	otelLogger.Info(context.Background(), "Logger initialized successfully")
	return otelLogger, nil
}

func (l *_logger) SetFatalHook(hook func()) {
	l.fatalHook = hook
}

func getRecordFromPool() *log.Record {
	return recordPool.Get().(*log.Record)
}

func putRecordToPool(record *log.Record) {
	recordPool.Put(record)
}

func getCallerFrame(skip int) (pc uintptr, file string, line int, ok bool) {
	rpc := make([]uintptr, 1)
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
	fmt.Println("zapLevel:", zapLevel)
	fmt.Println("body:", body)
	l.zapLogger.Log(zapLevel, body.AsString(), fields...)

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

func otelAttrsToZapFields(attrs []log.KeyValue) []zap.Field {
	if len(attrs) == 0 {
		return nil
	}
	fields := make([]zap.Field, 0, len(attrs))
	for _, attr := range attrs {
		// Handle different kinds of values appropriately
		switch attr.Value.Kind() {
		case log.KindInt64:
			// For Int64, use zap.Int64 instead of converting to string
			val := attr.Value.AsInt64()
			fields = append(fields, zap.Int64(string(attr.Key), val))
		case log.KindFloat64:
			// For Float64, use zap.Float64 instead of converting to string
			val := attr.Value.AsFloat64()
			fields = append(fields, zap.Float64(string(attr.Key), val))
		case log.KindBool:
			// For Bool, use zap.Bool instead of converting to string
			val := attr.Value.AsBool()
			fields = append(fields, zap.Bool(string(attr.Key), val))
		default:
			// For other types, use AsString as before
			fields = append(fields, zap.String(string(attr.Key), attr.Value.AsString()))
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
		var sb strings.Builder
		for i, arg := range args {
			if i > 0 {
				sb.WriteByte(' ')
			}
			_, err := fmt.Fprint(&sb, arg)
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
	return log.StringValue(fmt.Sprintf(template, args...))
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
		attrs := make([]log.KeyValue, 0, len(fields))
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

	if len(args) >= 2 {
		if msg, fields, ok := extractMessageAndFields(args); ok {
			l.emitLog(ctx, severity, log.StringValue(msg), append(fields,
				log.String("trace_id", traceId),
				log.String("span_id", spanId),
			)...)
			return
		}
	}
	l.emitLog(ctx, severity, argsToValue(args...),
		log.String("trace_id", traceId),
		log.String("span_id", spanId),
	)
}

func (l *_logger) Debug(ctx context.Context, args ...any) {
	l.log(ctx, log.SeverityDebug, args...)
}

func (l *_logger) Info(ctx context.Context, args ...any) {
	l.log(ctx, log.SeverityInfo, args...)
}

func (l *_logger) Warn(ctx context.Context, args ...any) {
	l.log(ctx, log.SeverityWarn, args...)
}

func (l *_logger) Error(ctx context.Context, args ...any) {
	l.log(ctx, log.SeverityError, args...)
}

func (l *_logger) Fatal(ctx context.Context, args ...any) {
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
	l.emitLog(ctx, log.SeverityDebug, formatToValue(template, args...))
}

func (l *_logger) Infof(ctx context.Context, template string, args ...any) {
	l.emitLog(ctx, log.SeverityInfo, formatToValue(template, args...))
}

func (l *_logger) Warnf(ctx context.Context, template string, args ...any) {
	l.emitLog(ctx, log.SeverityWarn, formatToValue(template, args...))
}

func (l *_logger) Errorf(ctx context.Context, template string, args ...any) {
	l.emitLog(ctx, log.SeverityError, formatToValue(template, args...))
}

func (l *_logger) Fatalf(ctx context.Context, template string, args ...any) {
	l.emitLog(ctx, log.SeverityFatal, formatToValue(template, args...))
	if l.fatalHook != nil {
		l.fatalHook()
	}
}

func (l *_logger) Panicf(ctx context.Context, template string, args ...any) {
	msg := formatToValue(template, args...)
	l.emitLog(ctx, log.SeverityFatal, msg)
	panic(msg.AsString())
}

func mapToAttributes(fields map[string]any) []log.KeyValue {
	if len(fields) == 0 {
		return emptyAttrs
	}

	attrs := make([]log.KeyValue, 0, len(fields))
	for k, v := range fields {
		switch val := v.(type) {
		case string:
			attrs = append(attrs, log.String(k, val))
		case int:
			attrs = append(attrs, log.Int(k, val))
		case int64:
			attrs = append(attrs, log.String(k, fmt.Sprintf("%d", val)))
		case float64:
			attrs = append(attrs, log.Float64(k, val))
		case bool:
			attrs = append(attrs, log.Bool(k, val))
		case []string, []int:
			attrs = append(attrs, log.String(k, fmt.Sprintf("%v", val)))
		case time.Time:
			attrs = append(attrs, log.String(k, val.Format(time.RFC3339Nano)))
		case error:
			attrs = append(attrs, log.String(k, val.Error()))
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
	if len(args) >= 2 {
		if msg, fields, ok := extractMessageAndFields(args); ok {
			l.emitLog(ctx, log.SeverityDebug, log.StringValue(msg), fields...)
			return
		}
	}
	l.emitLog(ctx, log.SeverityDebug, argsToValue(args...))
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
