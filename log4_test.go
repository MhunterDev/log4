package log4

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"
)

// TestLoggerBasicFunctionality tests the core logging functionality
func TestLoggerBasicFunctionality(t *testing.T) {
	// Clean up any existing test logs
	defer os.RemoveAll("./test-logs")

	logger := NewChannelLogger(10, "./test-logs")
	defer logger.Close()

	// Test different log levels
	logger.Info("test", "This is an info message")
	logger.Error("test", "This is an error message")
	logger.Debug("test", "This is a debug message")

	// Test with custom log level
	logger.LogLevel("test", ERROR, "Direct error log")

	// Test context logging
	ctx := context.Background()
	logger.LogWithContext(ctx, "test", "INFO", "Context-aware log")

	// Give time for async processing
	time.Sleep(100 * time.Millisecond)
}

// TestConfiguredLogger tests custom configuration options
func TestConfiguredLogger(t *testing.T) {
	// Clean up any existing test logs
	defer os.RemoveAll("./custom-test-logs")

	config := &Config{
		BufferSize:      50,
		LogDir:          "./custom-test-logs",
		TimestampFormat: "15:04:05.000",
		MinLevel:        INFO, // Only INFO and ERROR
	}

	logger := NewChannelLoggerWithConfig(config)
	defer logger.Close()

	// This should be logged (INFO >= INFO)
	logger.Info("configured-test", "This should appear")

	// This should NOT be logged (DEBUG < INFO)
	logger.Debug("configured-test", "This should NOT appear")

	// This should be logged (ERROR >= INFO)
	logger.Error("configured-test", "This error should appear")

	// Test runtime level change
	logger.SetMinLevel(ERROR)

	// This should NOT be logged now (INFO < ERROR)
	logger.Info("configured-test", "This info should NOT appear after level change")

	time.Sleep(100 * time.Millisecond)
}

// TestLogLevelParsing tests the log level parsing functionality
func TestLogLevelParsing(t *testing.T) {
	tests := []struct {
		input    string
		expected LogLevel
	}{
		{"DEBUG", DEBUG},
		{"INFO", INFO},
		{"ERROR", ERROR},
		{"INVALID", INFO}, // Should default to INFO
		{"", INFO},        // Should default to INFO
	}

	for _, test := range tests {
		result := ParseLogLevel(test.input)
		if result != test.expected {
			t.Errorf("ParseLogLevel(%s) = %v, expected %v", test.input, result, test.expected)
		}
	}
}

// TestLoggerConcurrency tests concurrent logging operations
func TestLoggerConcurrency(t *testing.T) {
	defer os.RemoveAll("./concurrent-test-logs")

	logger := NewChannelLogger(100, "./concurrent-test-logs")
	defer logger.Close()

	// Test concurrent logging from multiple goroutines
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func(id int) {
			logger.Info("concurrent", fmt.Sprintf("Message from goroutine %d", id))
			done <- true
		}(i)
	}

	// Wait for all goroutines to complete
	for i := 0; i < 10; i++ {
		<-done
	}

	time.Sleep(100 * time.Millisecond)
}

func ExampleChannelLogger() {
	// Create a logger with default settings
	logger := NewChannelLogger(100, "./example-logs")
	defer logger.Close()

	// Basic logging
	logger.Info("myapp", "Application started")
	logger.Error("database", "Connection failed")
	logger.Debug("auth", "User login attempt")

	// Create a logger with custom configuration
	config := &Config{
		BufferSize:      200,
		LogDir:          "./custom-logs",
		TimestampFormat: "15:04:05.000",
		MinLevel:        INFO, // Only show INFO and ERROR
	}
	customLogger := NewChannelLoggerWithConfig(config)
	defer customLogger.Close()

	// This will be logged (INFO >= INFO)
	customLogger.Info("api", "Request processed")

	// This will NOT be logged (DEBUG < INFO)
	customLogger.Debug("api", "Debug info")

	// Change log level at runtime
	customLogger.SetMinLevel(ERROR)

	// Now only ERROR messages will show
	customLogger.Info("api", "This won't show")
	customLogger.Error("api", "This will show")

	// Context-aware logging
	ctx := context.Background()
	customLogger.LogWithContext(ctx, "worker", "INFO", "Task completed")

	// Output (approximate):
	// [2006-01-02 15:04:05] INFO: Application started
	// [2006-01-02 15:04:05] ERROR: Connection failed
	// [2006-01-02 15:04:05] DEBUG: User login attempt
	// [15:04:05.000] INFO: Request processed
	// [15:04:05.000] ERROR: This will show
	// [15:04:05.000] INFO: Task completed
}

// TestPackageLogger tests the package-scoped logger functionality
func TestPackageLogger(t *testing.T) {
	// Clean up any existing test logs
	defer os.RemoveAll("./package-test-logs")

	logger := NewChannelLogger(10, "./package-test-logs")
	defer logger.Close()

	// Create package-scoped loggers
	appLogger := logger.Package("myapp")
	dbLogger := logger.Package("database")

	// Test basic logging
	appLogger.Info("Application started")
	dbLogger.Error("Connection failed")
	appLogger.Debug("Debug message")

	// Test formatted logging
	appLogger.InfoF("Server listening on port %d", 8080)
	dbLogger.ErrorF("Failed to connect after %d attempts", 3)
	appLogger.DebugF("Processing %d items", 42)

	// Test log levels
	appLogger.LogLevel(INFO, "Info via LogLevel")
	dbLogger.LogLevel(ERROR, "Error via LogLevel")

	// Test context logging
	ctx := context.Background()
	appLogger.LogWithContext(ctx, "INFO", "Context-aware message")

	// Test that package name is correctly stored
	if appLogger.GetPackageName() != "myapp" {
		t.Errorf("Expected package name 'myapp', got '%s'", appLogger.GetPackageName())
	}

	if dbLogger.GetPackageName() != "database" {
		t.Errorf("Expected package name 'database', got '%s'", dbLogger.GetPackageName())
	}

	// Give time for async processing
	time.Sleep(100 * time.Millisecond)
}

// TestPackageLoggerWithMinLevel tests package logger with minimum level filtering
func TestPackageLoggerWithMinLevel(t *testing.T) {
	defer os.RemoveAll("./package-level-test-logs")

	config := &Config{
		BufferSize: 10,
		LogDir:     "./package-level-test-logs",
		MinLevel:   INFO, // Only INFO and ERROR
	}

	logger := NewChannelLoggerWithConfig(config)
	defer logger.Close()

	appLogger := logger.Package("testapp")

	// This should be logged (INFO >= INFO)
	appLogger.Info("This should appear")

	// This should NOT be logged (DEBUG < INFO)
	appLogger.Debug("This should NOT appear")

	// This should be logged (ERROR >= INFO)
	appLogger.Error("This error should appear")

	// Test runtime level change
	logger.SetMinLevel(ERROR)

	// This should NOT be logged now (INFO < ERROR)
	appLogger.Info("This info should NOT appear after level change")

	// This should still be logged (ERROR >= ERROR)
	appLogger.Error("This error should still appear")

	time.Sleep(100 * time.Millisecond)
}

// ExamplePackageLogger demonstrates the package-scoped logger usage
func ExamplePackageLogger() {
	// Create a logger
	logger := NewChannelLogger(100, "./example-package-logs")
	defer logger.Close()

	// Create package-scoped loggers
	appLogger := logger.Package("myapp")
	dbLogger := logger.Package("database")
	authLogger := logger.Package("auth")

	// Clean API - no package name repetition
	appLogger.Info("Application started successfully")
	dbLogger.Error("Failed to connect to database")
	authLogger.Debug("User authentication attempt")

	// Formatted logging
	appLogger.InfoF("Server started on port %d", 8080)
	dbLogger.ErrorF("Connection failed after %d retries", 3)

	// Context-aware logging
	ctx := context.Background()
	appLogger.LogWithContext(ctx, "INFO", "Request processed")

	// Output will be written to separate files:
	// - ./example-package-logs/myapp.log
	// - ./example-package-logs/database.log
	// - ./example-package-logs/auth.log
}
