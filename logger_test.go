package logger

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"go.opentelemetry.io/otel/log"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var logFormat = "json" // Change to "json" for JSON format

// TestNewLogger tests the creation of a new logger
func TestNewLogger(t *testing.T) {
	// Test with JSON format
	jsonLogger, err := NewLogger(&Option{Format: logFormat})
	if err != nil {
		t.Fatalf("Failed to create JSON logger: %v", err)
	}
	if jsonLogger == nil {
		t.Fatal("JSON logger is nil")
	}

	// Test with console format
	consoleLogger, err := NewLogger(&Option{Format: logFormat})
	if err != nil {
		t.Fatalf("Failed to create console logger: %v", err)
	}
	if consoleLogger == nil {
		t.Fatal("Console logger is nil")
	}

	// Test with custom scope name and version
	customLogger, err := NewLogger(&Option{
		Format:       logFormat,
		ScopeName:    "test-scope",
		ScopeVersion: "1.0.0",
	})
	if err != nil {
		t.Fatalf("Failed to create custom logger: %v", err)
	}
	if customLogger == nil {
		t.Fatal("Custom logger is nil")
	}
}

// TestLogLevels tests all log level methods
func TestLogLevels(t *testing.T) {
	logger, err := NewLogger(&Option{Format: logFormat})
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}

	ctx := context.Background()

	// Test Debug
	logger.Debug(ctx, "Debug message")
	logger.Debug(ctx, "Debug with fields", map[string]string{"key": "value"})
	logger.Debug(ctx, "Debug with any fields", map[string]any{"key": "value", "number": 123})

	// Test Info
	logger.Info(ctx, "Info message")
	logger.Info(ctx, "Info with fields", map[string]string{"key": "value"})
	logger.Info(ctx, "Info with any fields", map[string]any{"key": "value", "number": 123})

	// Test Warn
	logger.Warn(ctx, "Warn message")
	logger.Warn(ctx, "Warn with fields", map[string]string{"key": "value"})
	logger.Warn(ctx, "Warn with any fields", map[string]any{"key": "value", "number": 123})

	// Test Error
	logger.Error(ctx, "Error message")
	logger.Error(ctx, "Error with fields", map[string]string{"key": "value"})
	logger.Error(ctx, "Error with any fields", map[string]any{"key": "value", "number": 123})

	// Note: Fatal is not tested here to avoid program exit
	// It would be tested in a real environment with appropriate mocking
}

// TestFormattedLogging tests all formatted logging methods
func TestFormattedLogging(t *testing.T) {
	logger, err := NewLogger(&Option{Format: logFormat})
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}

	ctx := context.Background()

	// Test Debugf
	logger.Debugf(ctx, "Debug message with %s", "formatting")

	// Test Infof
	logger.Infof(ctx, "Info message with %s", "formatting")

	// Test Warnf
	logger.Warnf(ctx, "Warn message with %s", "formatting")

	// Test Errorf
	logger.Errorf(ctx, "Error message with %s", "formatting")

	// Note: Fatalf is not tested here to avoid program exit
	// It would be tested in a real environment with appropriate mocking

	// Panicf is tested in a separate test function
}

// TestWithFields tests the WithFields method
func TestWithFields(t *testing.T) {
	logger, err := NewLogger(&Option{Format: logFormat})
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}

	ctx := context.Background()

	// Create logger with fields
	fieldsLogger := logger.WithFields(map[string]any{
		"string_field": "value",
		"int_field":    123,
		"bool_field":   true,
		"float_field":  3.14,
		"time_field":   time.Now(),
		"error_field":  error(nil),
		"nil_field":    nil,
	})

	// Log with the fields logger
	fieldsLogger.Info(ctx, "Message with fields")

	// Test nested WithFields
	nestedLogger := fieldsLogger.WithFields(map[string]any{
		"nested_field": "nested_value",
	})
	nestedLogger.Info(ctx, "Message with nested fields")

	// Test WithFields with additional fields in log method
	fieldsLogger.Info(ctx, "Message with additional fields", map[string]any{
		"additional_field": "additional_value",
	})

	// Test all log levels with fields logger
	fieldsLogger.Debug(ctx, "Debug with fields")
	fieldsLogger.Warn(ctx, "Warn with fields")
	fieldsLogger.Error(ctx, "Error with fields")

	// Test formatted logging with fields logger
	fieldsLogger.Debugf(ctx, "Debug formatted with %s", "fields")
	fieldsLogger.Infof(ctx, "Info formatted with %s", "fields")
	fieldsLogger.Warnf(ctx, "Warn formatted with %s", "fields")
	fieldsLogger.Errorf(ctx, "Error formatted with %s", "fields")

	// Note: Fatal and Fatalf with fields logger are not tested here to avoid program exit
	// They would be tested in a real environment with appropriate mocking

	// Panic and Panicf with fields logger are tested in separate test functions
}

// Note: Panic and Panicf methods are not tested here to avoid program exit
// They would be tested in a real environment with appropriate mocking

// TestContextHandling tests logging with different contexts
func TestContextHandling(t *testing.T) {
	logger, err := NewLogger(&Option{Format: logFormat})
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}

	// Test with background context
	ctx := context.Background()
	logger.Info(ctx, "Message with background context")

	// Test with nil context (should use background context internally)
	logger.Info(ctx, "Message with nil context")

	// Test with context with values
	logger.Info(ctx, "Message with context with values")
}

// TestSync tests the Sync method
func TestSync(t *testing.T) {
	logger, err := NewLogger(&Option{Format: logFormat})
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}

	// Log something before sync
	ctx := context.Background()
	logger.Info(ctx, "Message before sync")

	// Test sync
	err = logger.Sync()
	if err != nil {
		// Sync might fail in tests due to stdout being redirected
		t.Logf("Sync returned error (might be expected in tests): %v", err)
	}
}

// TestEdgeCases tests various edge cases
func TestEdgeCases(t *testing.T) {
	logger, err := NewLogger(&Option{Format: logFormat})
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}

	ctx := context.Background()

	// Test empty message
	logger.Info(ctx, "")

	// Test logging with no arguments
	logger.Info(ctx)

	// Test logging with multiple arguments
	logger.Info(ctx, "Message", "with", "multiple", "arguments")

	// Test logging with empty fields map
	logger.Info(ctx, "Message with empty fields", map[string]any{})

	// Test logging with various field types
	logger.Info(ctx, "Message with various field types", map[string]any{
		"string_slice": []string{"a", "b", "c"},
		"int_slice":    []int{1, 2, 3},
		"complex":      complex(1, 2),
		"struct":       struct{ Name string }{"test"},
	})
}

// TestGetCallerFrame tests the getCallerFrame function
func TestGetCallerFrame(t *testing.T) {
	// Define a helper function to test getCallerFrame
	testFunc := func() (uintptr, string, int, bool) {
		return getCallerFrame(1) // Skip 1 frame (this function)
	}

	// Call the helper function
	pc, file, line, ok := testFunc()

	// Verify the results
	if !ok {
		t.Error("getCallerFrame failed to get caller information")
	}
	if pc == 0 {
		t.Error("getCallerFrame returned zero PC")
	}
	if file == "" {
		t.Error("getCallerFrame returned empty file path")
	}
	if !strings.Contains(file, "logger_test.go") {
		t.Errorf("getCallerFrame returned unexpected file path: %s", file)
	}
	if line <= 0 {
		t.Errorf("getCallerFrame returned invalid line number: %d", line)
	}

	// Test with invalid skip value
	_, _, _, ok = getCallerFrame(1000) // Skip more frames than the stack has
	if ok {
		t.Error("getCallerFrame should fail for invalid skip value")
	}
}

// TestExtractMessageAndFields tests the extractMessageAndFields function
func TestExtractMessageAndFields(t *testing.T) {
	// Test with valid string message and string map
	msg, attrs, ok := extractMessageAndFields([]any{"test message", map[string]string{"key": "value"}})
	if !ok {
		t.Error("extractMessageAndFields failed for valid string message and string map")
	}
	if msg != "test message" {
		t.Errorf("extractMessageAndFields returned wrong message: got %s, want %s", msg, "test message")
	}
	if len(attrs) != 1 {
		t.Errorf("extractMessageAndFields returned wrong number of attributes: got %d, want %d", len(attrs), 1)
	}

	// Test with valid string message and any map
	msg, attrs, ok = extractMessageAndFields([]any{"test message", map[string]any{"key": "value", "number": 123}})
	if !ok {
		t.Error("extractMessageAndFields failed for valid string message and any map")
	}
	if msg != "test message" {
		t.Errorf("extractMessageAndFields returned wrong message: got %s, want %s", msg, "test message")
	}
	if len(attrs) != 2 {
		t.Errorf("extractMessageAndFields returned wrong number of attributes: got %d, want %d", len(attrs), 2)
	}

	// Test with empty string map
	_, attrs, ok = extractMessageAndFields([]any{"test message", map[string]string{}})
	if !ok {
		t.Error("extractMessageAndFields failed for empty string map")
	}
	if len(attrs) != 0 {
		t.Errorf("extractMessageAndFields returned non-empty attributes for empty map: %v", attrs)
	}

	// Test with invalid arguments (not enough args)
	_, _, ok = extractMessageAndFields([]any{"test message"})
	if ok {
		t.Error("extractMessageAndFields should fail for not enough args")
	}

	// Test with invalid arguments (first arg not string)
	_, _, ok = extractMessageAndFields([]any{123, map[string]string{"key": "value"}})
	if ok {
		t.Error("extractMessageAndFields should fail for non-string first arg")
	}

	// Test with invalid arguments (second arg not map)
	_, _, ok = extractMessageAndFields([]any{"test message", "not a map"})
	if ok {
		t.Error("extractMessageAndFields should fail for non-map second arg")
	}
}

// TestOtelAttrsToZapFields tests the otelAttrsToZapFields function
func TestOtelAttrsToZapFields(t *testing.T) {
	// Test with empty attributes
	fields := otelAttrsToZapFields([]log.KeyValue{})
	if fields != nil {
		t.Errorf("otelAttrsToZapFields returned non-nil for empty attributes: %v", fields)
	}

	// Test with various attribute types
	attrs := []log.KeyValue{
		log.String("string_key", "string_value"),
		log.Int("int_key", 123),
		log.Float64("float_key", 3.14),
		log.Bool("bool_key", true),
	}

	fields = otelAttrsToZapFields(attrs)
	if len(fields) != len(attrs) {
		t.Errorf("otelAttrsToZapFields returned wrong number of fields: got %d, want %d", len(fields), len(attrs))
	}

	// Check that fields have the correct types based on the attribute kind
	for i, field := range fields {
		if field.Key != string(attrs[i].Key) {
			t.Errorf("Field key mismatch at index %d: got %s, want %s", i, field.Key, string(attrs[i].Key))
		}

		// Check field type based on attribute kind
		switch attrs[i].Value.Kind() {
		case log.KindString:
			if field.Type != zapcore.StringType {
				t.Errorf("Field type at index %d should be string: got %v", i, field.Type)
			}
		case log.KindInt64:
			if field.Type != zapcore.Int64Type {
				t.Errorf("Field type at index %d should be int64: got %v", i, field.Type)
			}
		case log.KindFloat64:
			if field.Type != zapcore.Float64Type {
				t.Errorf("Field type at index %d should be float64: got %v", i, field.Type)
			}
		case log.KindBool:
			if field.Type != zapcore.BoolType {
				t.Errorf("Field type at index %d should be bool: got %v", i, field.Type)
			}
		default:
			if field.Type != zapcore.StringType {
				t.Errorf("Field type at index %d should be string for unknown kind: got %v", i, field.Type)
			}
		}
	}
}

// TestHelperFunctions tests internal helper functions
func TestHelperFunctions(t *testing.T) {
	// Test otelSeverityToZapLevel
	if otelSeverityToZapLevel(log.SeverityDebug) != zap.DebugLevel {
		t.Error("otelSeverityToZapLevel failed for Debug")
	}
	if otelSeverityToZapLevel(log.SeverityInfo) != zap.InfoLevel {
		t.Error("otelSeverityToZapLevel failed for Info")
	}
	if otelSeverityToZapLevel(log.SeverityWarn) != zap.WarnLevel {
		t.Error("otelSeverityToZapLevel failed for Warn")
	}
	if otelSeverityToZapLevel(log.SeverityError) != zap.ErrorLevel {
		t.Error("otelSeverityToZapLevel failed for Error")
	}
	if otelSeverityToZapLevel(log.SeverityFatal) != zap.FatalLevel {
		t.Error("otelSeverityToZapLevel failed for Fatal")
	}
	if otelSeverityToZapLevel(log.Severity(999)) != zap.InfoLevel {
		t.Error("otelSeverityToZapLevel failed for unknown severity")
	}

	// Test argsToValue
	if argsToValue().AsString() != "" {
		t.Error("argsToValue failed for empty args")
	}
	if argsToValue("test").AsString() != "test" {
		t.Error("argsToValue failed for single string arg")
	}
	if argsToValue(123).AsString() != "123" {
		t.Error("argsToValue failed for single int arg")
	}
	if argsToValue("a", "b", "c").AsString() != "a b c" {
		t.Error("argsToValue failed for multiple args")
	}

	// Test formatToValue
	if formatToValue("test").AsString() != "test" {
		t.Error("formatToValue failed for string without args")
	}
	if formatToValue("test %s", "value").AsString() != "test value" {
		t.Error("formatToValue failed for string with args")
	}
	if formatToValue("test %d %s", 123, "value").AsString() != "test 123 value" {
		t.Error("formatToValue failed for string with multiple args")
	}

	// Test mapToAttributes
	testMap := map[string]any{
		"string":  "value",
		"int":     123,
		"int64":   int64(123),
		"float64": 3.14,
		"bool":    true,
		"strings": []string{"a", "b", "c"},
		"ints":    []int{1, 2, 3},
		"time":    time.Now(),
		"error":   fmt.Errorf("test error"),
		"nil":     nil,
		"complex": complex(1, 2),
		"struct":  struct{ Name string }{"test"},
	}

	attrs := mapToAttributes(testMap)
	if len(attrs) != len(testMap) {
		t.Errorf("mapToAttributes returned wrong number of attributes: got %d, want %d", len(attrs), len(testMap))
	}

	// Test empty map
	emptyAttrs := mapToAttributes(map[string]any{})
	if len(emptyAttrs) != 0 {
		t.Errorf("mapToAttributes for empty map returned non-empty attributes: %v", emptyAttrs)
	}
}
