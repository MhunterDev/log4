package log4

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// Test helper functions
func createTempDir(t *testing.T) string {
	dir, err := os.MkdirTemp("", "log4_test_")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	return dir
}

func cleanupTempDir(t *testing.T, dir string) {
	if err := os.RemoveAll(dir); err != nil {
		t.Errorf("Failed to cleanup temp dir: %v", err)
	}
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func readFile(t *testing.T, path string) string {
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("Failed to read file %s: %v", path, err)
	}
	return string(content)
}

func countLines(content string) int {
	if content == "" {
		return 0
	}
	return strings.Count(content, "\n")
}

// Test LogLevel functionality
func TestLogLevel(t *testing.T) {
	tests := []struct {
		level    LogLevel
		expected string
	}{
		{DEBUG, "DEBUG"},
		{INFO, "INFO"},
		{ERROR, "ERROR"},
		{LogLevel(999), "UNKNOWN"},
	}

	for _, test := range tests {
		if got := test.level.String(); got != test.expected {
			t.Errorf("LogLevel.String() = %s, want %s", got, test.expected)
		}
	}
}

func TestParseLogLevel(t *testing.T) {
	tests := []struct {
		input    string
		expected LogLevel
	}{
		{"DEBUG", DEBUG},
		{"debug", DEBUG},
		{"INFO", INFO},
		{"info", INFO},
		{"ERROR", ERROR},
		{"error", ERROR},
		{"INVALID", INFO}, // default fallback
		{"", INFO},        // default fallback
	}

	for _, test := range tests {
		if got := ParseLogLevel(test.input); got != test.expected {
			t.Errorf("ParseLogLevel(%s) = %v, want %v", test.input, got, test.expected)
		}
	}
}

// Test Config validation
func TestConfigValidation(t *testing.T) {
	t.Run("Valid config", func(t *testing.T) {
		config := DefaultConfig()
		if err := config.Validate(); err != nil {
			t.Errorf("Valid config should not return error: %v", err)
		}
	})

	t.Run("Invalid buffer size", func(t *testing.T) {
		config := DefaultConfig()
		config.BufferSize = 0
		if err := config.Validate(); err == nil {
			t.Error("Expected error for invalid buffer size")
		}
	})

	t.Run("Empty timestamp format", func(t *testing.T) {
		config := DefaultConfig()
		config.TimestampFormat = ""
		if err := config.Validate(); err == nil {
			t.Error("Expected error for empty timestamp format")
		}
	})

	t.Run("Auto-fix invalid values", func(t *testing.T) {
		config := DefaultConfig()
		config.MaxFileSize = 0
		config.MaxFiles = 0
		config.FileMode = 0
		config.DirMode = 0

		if err := config.Validate(); err != nil {
			t.Errorf("Config validation failed: %v", err)
		}

		if config.MaxFileSize != DefaultMaxFileSize {
			t.Errorf("MaxFileSize not auto-fixed: got %d, want %d", config.MaxFileSize, DefaultMaxFileSize)
		}
		if config.MaxFiles != DefaultMaxFiles {
			t.Errorf("MaxFiles not auto-fixed: got %d, want %d", config.MaxFiles, DefaultMaxFiles)
		}
		if config.FileMode != DefaultFileMode {
			t.Errorf("FileMode not auto-fixed: got %o, want %o", config.FileMode, DefaultFileMode)
		}
		if config.DirMode != DefaultDirMode {
			t.Errorf("DirMode not auto-fixed: got %o, want %o", config.DirMode, DefaultDirMode)
		}
	})
}

// Test package name sanitization
func TestSanitizePackageName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"", "default"},
		{"valid_package", "valid_package"},
		{"package-name", "package-name"},
		{"package/with/slashes", "package_with_slashes"},
		{"package with spaces", "package_with_spaces"},
		{"package@#$%", "package____"},
		{strings.Repeat("a", 150), strings.Repeat("a", MaxPackageNameLen)},
	}

	for _, test := range tests {
		if got := sanitizePackageName(test.input); got != test.expected {
			t.Errorf("sanitizePackageName(%s) = %s, want %s", test.input, got, test.expected)
		}
	}
}

// Test basic logger creation and usage
func TestNewChannelLogger(t *testing.T) {
	tempDir := createTempDir(t)
	defer cleanupTempDir(t, tempDir)

	logger := NewChannelLogger(10, tempDir)
	defer logger.Close()

	if logger == nil {
		t.Fatal("NewChannelLogger returned nil")
	}

	// Test basic logging
	logger.Info("test", "Test message")
	logger.Error("test", "Error message")
	logger.Debug("test", "Debug message")

	// Give some time for async logging
	time.Sleep(100 * time.Millisecond)

	// Check if log file was created
	logFile := filepath.Join(tempDir, "test.log")
	if !fileExists(logFile) {
		t.Error("Log file was not created")
	}
}

func TestNewChannelLoggerWithConfig(t *testing.T) {
	tempDir := createTempDir(t)
	defer cleanupTempDir(t, tempDir)

	config := DefaultConfig()
	config.LogDir = tempDir
	config.MinLevel = ERROR
	config.BufferSize = 5

	logger := NewChannelLoggerWithConfig(config)
	defer logger.Close()

	// Test that debug and info messages are filtered out
	logger.Debug("test", "Debug message")
	logger.Info("test", "Info message")
	logger.Error("test", "Error message")

	time.Sleep(100 * time.Millisecond)

	logFile := filepath.Join(tempDir, "test.log")
	if !fileExists(logFile) {
		t.Error("Log file was not created")
	}

	content := readFile(t, logFile)
	if strings.Contains(content, "DEBUG") || strings.Contains(content, "INFO") {
		t.Error("Debug and Info messages should be filtered out")
	}
	if !strings.Contains(content, "ERROR") {
		t.Error("Error message should be present")
	}
}

// Test concurrent logging
func TestConcurrentLogging(t *testing.T) {
	tempDir := createTempDir(t)
	defer cleanupTempDir(t, tempDir)

	logger := NewChannelLogger(100, tempDir)
	defer logger.Close()

	const numGoroutines = 10
	const messagesPerGoroutine = 20

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			pkg := fmt.Sprintf("pkg%d", id)
			for j := 0; j < messagesPerGoroutine; j++ {
				logger.Info(pkg, fmt.Sprintf("Message %d from goroutine %d", j, id))
			}
		}(i)
	}

	wg.Wait()
	time.Sleep(200 * time.Millisecond) // Wait for async processing

	// Check that all package log files were created
	for i := 0; i < numGoroutines; i++ {
		logFile := filepath.Join(tempDir, fmt.Sprintf("pkg%d.log", i))
		if !fileExists(logFile) {
			t.Errorf("Log file for pkg%d was not created", i)
		}

		content := readFile(t, logFile)
		lineCount := countLines(content)
		if lineCount != messagesPerGoroutine {
			t.Errorf("Expected %d lines in pkg%d.log, got %d", messagesPerGoroutine, i, lineCount)
		}
	}
}

// Test structured logging with fields
func TestStructuredLogging(t *testing.T) {
	tempDir := createTempDir(t)
	defer cleanupTempDir(t, tempDir)

	logger := NewChannelLogger(10, tempDir)
	defer logger.Close()

	fields := map[string]interface{}{
		"user_id": 12345,
		"action":  "login",
		"ip":      "192.168.1.1",
	}

	logger.LogWithFields("auth", INFO, "User logged in", fields)
	time.Sleep(100 * time.Millisecond)

	logFile := filepath.Join(tempDir, "auth.log")
	content := readFile(t, logFile)

	expectedFields := []string{"user_id=12345", "action=login", "ip=192.168.1.1"}
	for _, field := range expectedFields {
		if !strings.Contains(content, field) {
			t.Errorf("Expected field %s not found in log content", field)
		}
	}
}

// Test context-aware logging
func TestContextLogging(t *testing.T) {
	tempDir := createTempDir(t)
	defer cleanupTempDir(t, tempDir)

	logger := NewChannelLogger(10, tempDir)
	defer logger.Close()

	t.Run("Valid context", func(t *testing.T) {
		ctx := context.Background()
		logger.LogWithContext(ctx, "test", "INFO", "Context message")
		time.Sleep(100 * time.Millisecond)

		logFile := filepath.Join(tempDir, "test.log")
		content := readFile(t, logFile)
		if !strings.Contains(content, "Context message") {
			t.Error("Context message not found in log")
		}
	})

	t.Run("Cancelled context", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		logger.LogWithContext(ctx, "test", "INFO", "Cancelled context message")
		time.Sleep(100 * time.Millisecond)

		logFile := filepath.Join(tempDir, "test.log")
		content := readFile(t, logFile)
		if strings.Contains(content, "Cancelled context message") {
			t.Error("Cancelled context message should not be logged")
		}
	})
}

// Test minimum level changes
func TestMinLevelChanges(t *testing.T) {
	tempDir := createTempDir(t)
	defer cleanupTempDir(t, tempDir)

	logger := NewChannelLogger(10, tempDir)
	defer logger.Close()

	// Initially set to DEBUG
	logger.SetMinLevel(DEBUG)
	if logger.GetMinLevel() != DEBUG {
		t.Errorf("Expected min level DEBUG, got %v", logger.GetMinLevel())
	}

	logger.Debug("test", "Debug message 1")
	logger.Info("test", "Info message 1")

	// Change to INFO
	logger.SetMinLevel(INFO)
	if logger.GetMinLevel() != INFO {
		t.Errorf("Expected min level INFO, got %v", logger.GetMinLevel())
	}

	logger.Debug("test", "Debug message 2")
	logger.Info("test", "Info message 2")

	time.Sleep(100 * time.Millisecond)

	logFile := filepath.Join(tempDir, "test.log")
	content := readFile(t, logFile)

	if !strings.Contains(content, "Debug message 1") {
		t.Error("First debug message should be present")
	}
	if strings.Contains(content, "Debug message 2") {
		t.Error("Second debug message should be filtered out")
	}
	if !strings.Contains(content, "Info message 1") || !strings.Contains(content, "Info message 2") {
		t.Error("Both info messages should be present")
	}
}

// Test PackageLogger functionality
func TestPackageLogger(t *testing.T) {
	tempDir := createTempDir(t)
	defer cleanupTempDir(t, tempDir)

	logger := NewChannelLogger(10, tempDir)
	defer logger.Close()

	// Test panic on empty package name
	defer func() {
		if r := recover(); r == nil {
			t.Error("Expected panic for empty package name")
		}
	}()
	_ = logger.Package("")
}

func TestPackageLoggerMethods(t *testing.T) {
	tempDir := createTempDir(t)
	defer cleanupTempDir(t, tempDir)

	logger := NewChannelLogger(10, tempDir)
	defer logger.Close()

	pkgLogger := logger.Package("mypackage")

	if pkgLogger.GetPackageName() != "mypackage" {
		t.Errorf("Expected package name 'mypackage', got '%s'", pkgLogger.GetPackageName())
	}

	// Test all methods
	pkgLogger.Info("Info message")
	pkgLogger.Error("Error message")
	pkgLogger.Debug("Debug message")
	pkgLogger.InfoF("Formatted info: %d", 42)
	pkgLogger.ErrorF("Formatted error: %s", "test")
	pkgLogger.DebugF("Formatted debug: %v", true)

	fields := map[string]interface{}{"key": "value"}
	pkgLogger.InfoWithFields("Info with fields", fields)
	pkgLogger.ErrorWithFields("Error with fields", fields)
	pkgLogger.DebugWithFields("Debug with fields", fields)

	ctx := context.Background()
	pkgLogger.LogWithContext(ctx, "INFO", "Context message")

	time.Sleep(100 * time.Millisecond)

	logFile := filepath.Join(tempDir, "mypackage.log")
	if !fileExists(logFile) {
		t.Error("Package log file was not created")
	}

	content := readFile(t, logFile)
	expectedMessages := []string{
		"Info message",
		"Error message", 
		"Debug message",
		"Formatted info: 42",
		"Formatted error: test",
		"Formatted debug: true",
		"Info with fields",
		"Error with fields",
		"Debug with fields",
		"Context message",
		"key=value",
	}

	for _, msg := range expectedMessages {
		if !strings.Contains(content, msg) {
			t.Errorf("Expected message '%s' not found in log", msg)
		}
	}
}

// Test log rotation
func TestLogRotation(t *testing.T) {
	tempDir := createTempDir(t)
	defer cleanupTempDir(t, tempDir)

	config := DefaultConfig()
	config.LogDir = tempDir
	config.MaxFileSize = 100 // Very small size to trigger rotation
	config.MaxFiles = 3

	logger := NewChannelLoggerWithConfig(config)
	defer logger.Close()

	// Write enough messages to trigger rotation
	for i := 0; i < 50; i++ {
		logger.Info("test", fmt.Sprintf("This is a longer message to fill up the log file quickly - message %d", i))
	}

	time.Sleep(200 * time.Millisecond)

	// Check if rotation files exist
	baseFile := filepath.Join(tempDir, "test.log")
	rotatedFile1 := filepath.Join(tempDir, "test.log.1")

	if !fileExists(baseFile) {
		t.Error("Base log file should exist")
	}

	// At least one rotation should have occurred
	if !fileExists(rotatedFile1) {
		t.Error("At least one rotated file should exist")
	}
}

// Test error handling
func TestErrorHandling(t *testing.T) {
	var capturedErrors []error
	var mu sync.Mutex

	config := DefaultConfig()
	config.LogDir = "/invalid/path/that/does/not/exist"
	config.ErrorHandler = func(err error) {
		mu.Lock()
		capturedErrors = append(capturedErrors, err)
		mu.Unlock()
	}

	logger := NewChannelLoggerWithConfig(config)
	defer logger.Close()

	logger.Info("test", "Test message")
	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	errorCount := len(capturedErrors)
	mu.Unlock()

	if errorCount == 0 {
		t.Error("Expected at least one error to be captured")
	}
}

// Test graceful shutdown
func TestGracefulShutdown(t *testing.T) {
	tempDir := createTempDir(t)
	defer cleanupTempDir(t, tempDir)

	logger := NewChannelLogger(10, tempDir)

	// Send some messages
	for i := 0; i < 5; i++ {
		logger.Info("test", fmt.Sprintf("Message %d", i))
	}

	// Close should not hang
	done := make(chan bool)
	go func() {
		logger.Close()
		done <- true
	}()

	select {
	case <-done:
		// Success
	case <-time.After(2 * time.Second):
		t.Error("Close() took too long, possible deadlock")
	}

	// Verify that messages after close are handled gracefully
	logger.Info("test", "Message after close")
	time.Sleep(50 * time.Millisecond)
}

// Test channel full scenario
func TestChannelFull(t *testing.T) {
	tempDir := createTempDir(t)
	defer cleanupTempDir(t, tempDir)

	var errorCount int
	var mu sync.Mutex

	config := DefaultConfig()
	config.LogDir = tempDir
	config.BufferSize = 1 // Very small buffer
	config.ErrorHandler = func(err error) {
		mu.Lock()
		if strings.Contains(err.Error(), "log channel full") {
			errorCount++
		}
		mu.Unlock()
	}

	logger := NewChannelLoggerWithConfig(config)
	defer logger.Close()

	// Flood the logger to fill the channel
	for i := 0; i < 10; i++ {
		logger.Info("test", fmt.Sprintf("Message %d", i))
	}

	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	hasChannelFullErrors := errorCount > 0
	mu.Unlock()

	if !hasChannelFullErrors {
		t.Error("Expected channel full errors when flooding small buffer")
	}
}

// Test memory pool efficiency
func TestMemoryPool(t *testing.T) {
	// This test ensures the memory pool is working by checking that
	// we can get and put entries without issues
	entry1 := getLogEntry()
	entry2 := getLogEntry()

	if entry1 == nil || entry2 == nil {
		t.Error("getLogEntry() should not return nil")
	}

	// Modify entries
	entry1.Package = "test1"
	entry1.Message = "message1"
	entry1.Fields["key1"] = "value1"

	entry2.Package = "test2"
	entry2.Message = "message2"
	entry2.Fields["key2"] = "value2"

	// Put them back
	putLogEntry(entry1)
	putLogEntry(entry2)

	// Get new entries and verify they're reset
	entry3 := getLogEntry()
	if entry3.Package != "" || entry3.Message != "" || len(entry3.Fields) != 0 {
		t.Error("LogEntry should be reset when returned from pool")
	}

	putLogEntry(entry3)
}

// Benchmark tests
func BenchmarkChannelLogger(b *testing.B) {
	tempDir := createTempDir(&testing.T{})
	defer cleanupTempDir(&testing.T{}, tempDir)

	logger := NewChannelLogger(1000, tempDir)
	defer logger.Close()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			logger.Info("benchmark", fmt.Sprintf("Benchmark message %d", i))
			i++
		}
	})
}

func BenchmarkStructuredLogging(b *testing.B) {
	tempDir := createTempDir(&testing.T{})
	defer cleanupTempDir(&testing.T{}, tempDir)

	logger := NewChannelLogger(1000, tempDir)
	defer logger.Close()

	fields := map[string]interface{}{
		"user_id": 12345,
		"action":  "test",
		"count":   1,
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			logger.LogWithFields("benchmark", INFO, fmt.Sprintf("Structured message %d", i), fields)
			i++
		}
	})
}