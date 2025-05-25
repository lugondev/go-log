package logger

import (
	"context"
	"fmt"
	"testing"
)

func TestConsoleFormat(t *testing.T) {
	// Create logger with console format
	logger, err := NewLogger(&Option{Format: "console"})
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}

	ctx := context.Background()

	t.Log("=== Testing Console Format (Key=Value) ===")

	// Test basic logging
	logger.Info(ctx, "Basic info message")

	// Test logging with fields
	logger.Info(ctx, "Message with fields", map[string]any{
		"user_id":  12345,
		"action":   "login",
		"success":  true,
		"duration": 1.23,
	})

	// Test with WithFields
	userLogger := logger.WithFields(map[string]any{
		"service": "auth-service",
		"version": "1.0.0",
	})

	userLogger.Info(ctx, "User authenticated successfully", map[string]any{
		"username": "john_doe",
		"role":     "admin",
	})

	// Test different log levels
	logger.Debug(ctx, "Debug message", map[string]any{"debug_flag": true})
	logger.Warn(ctx, "Warning message", map[string]any{"warning_code": 404})
	logger.Error(ctx, "Error message", map[string]any{"error_code": 500, "retries": 3})

	// Test formatted logging
	logger.Infof(ctx, "Formatted message: user %s logged in", "alice")

	fmt.Println("=== Console Format Test Complete ===")
}
