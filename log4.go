// Package log4 provides a concurrent, channel-based logger with per-package file separation.
//
// Features:
// - Concurrent logging via channels and goroutines
// - Per-package log file separation
// - Configurable log levels with runtime filtering
// - Context-aware logging
// - Buffered channels to prevent blocking
// - Graceful shutdown with proper resource cleanup
//
// Example usage:
//
//	logger := log4.NewChannelLogger(100, "./logs")
//	defer logger.Close()
//
//	logger.Info("myapp", "Application started")
//	logger.Error("database", "Connection failed")
package log4

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type LogLevel int

const (
	DEBUG LogLevel = iota
	INFO
	ERROR
)

func (l LogLevel) String() string {
	switch l {
	case DEBUG:
		return "DEBUG"
	case INFO:
		return "INFO"
	case ERROR:
		return "ERROR"
	default:
		return "UNKNOWN"
	}
}

type Config struct {
	BufferSize      int
	LogDir          string
	TimestampFormat string
	MinLevel        LogLevel
}

func DefaultConfig() *Config {
	return &Config{
		BufferSize:      100,
		LogDir:          "",
		TimestampFormat: "2006-01-02 15:04:05",
		MinLevel:        DEBUG,
	}
}

type LogMessage struct {
	Package   string
	Level     LogLevel
	Message   string
	Timestamp time.Time
}

type ChannelLogger struct {
	logChan chan LogMessage
	done    chan struct{}
	wg      sync.WaitGroup
	loggers map[string]*log.Logger // per-package loggers
	files   map[string]*os.File    // per-package files (fixed pointer issue)
	stdout  io.Writer
	config  *Config
	mu      sync.Mutex
}

func NewChannelLogger(bufferSize int, logDir string) *ChannelLogger {
	config := DefaultConfig()
	config.BufferSize = bufferSize
	config.LogDir = logDir
	return NewChannelLoggerWithConfig(config)
}

func NewChannelLoggerWithConfig(config *Config) *ChannelLogger {
	if config == nil {
		config = DefaultConfig()
	}
	cl := &ChannelLogger{
		logChan: make(chan LogMessage, config.BufferSize),
		done:    make(chan struct{}),
		loggers: make(map[string]*log.Logger),
		files:   make(map[string]*os.File),
		stdout:  os.Stdout,
		config:  config,
	}
	if config.LogDir != "" {
		if err := os.MkdirAll(config.LogDir, 0755); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to create log directory %s: %v\n", config.LogDir, err)
		}
	}
	cl.wg.Add(1)
	go cl.run()
	return cl
}

func (cl *ChannelLogger) getLogger(pkg string) *log.Logger {
	cl.mu.Lock()
	defer cl.mu.Unlock()
	logger, ok := cl.loggers[pkg]
	if ok {
		return logger
	}
	fileName := pkg + ".log"
	if cl.config.LogDir != "" {
		fileName = filepath.Join(cl.config.LogDir, fileName)
	}
	f, err := os.OpenFile(fileName, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	var writers []io.Writer
	writers = append(writers, cl.stdout)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to open log file %s: %v\n", fileName, err)
	} else {
		cl.files[pkg] = f
		writers = append(writers, f)
	}
	logger = log.New(io.MultiWriter(writers...), "", 0)
	cl.loggers[pkg] = logger
	return logger
}

func (cl *ChannelLogger) run() {
	defer cl.wg.Done()
	for {
		select {
		case msg := <-cl.logChan:
			if msg.Level >= cl.config.MinLevel {
				formatted := fmt.Sprintf("[%s] %s: %s",
					msg.Timestamp.Format(cl.config.TimestampFormat),
					msg.Level.String(),
					msg.Message)
				cl.getLogger(msg.Package).Println(formatted)
			}
		case <-cl.done:
			// Process remaining messages
			for len(cl.logChan) > 0 {
				msg := <-cl.logChan
				if msg.Level >= cl.config.MinLevel {
					formatted := fmt.Sprintf("[%s] %s: %s",
						msg.Timestamp.Format(cl.config.TimestampFormat),
						msg.Level.String(),
						msg.Message)
					cl.getLogger(msg.Package).Println(formatted)
				}
			}
			return
		}
	}
}

func ParseLogLevel(level string) LogLevel {
	switch level {
	case "DEBUG":
		return DEBUG
	case "INFO":
		return INFO
	case "ERROR":
		return ERROR
	default:
		return INFO // default fallback
	}
}

func (cl *ChannelLogger) Log(pkg, level, message string) {
	cl.LogLevel(pkg, ParseLogLevel(level), message)
}

func (cl *ChannelLogger) LogLevel(pkg string, level LogLevel, message string) {
	select {
	case cl.logChan <- LogMessage{
		Package:   pkg,
		Level:     level,
		Message:   message,
		Timestamp: time.Now(),
	}:
	default:
		fmt.Fprintf(os.Stderr, "Warning: Log channel full, dropping message: %s\n", message)
	}
}

func (cl *ChannelLogger) LogWithContext(ctx context.Context, pkg, level, message string) {
	if ctx.Err() != nil {
		return // Context cancelled/expired
	}
	cl.Log(pkg, level, message)
}

func (cl *ChannelLogger) Info(pkg, message string) {
	cl.LogLevel(pkg, INFO, message)
}

func (cl *ChannelLogger) Error(pkg, message string) {
	cl.LogLevel(pkg, ERROR, message)
}

func (cl *ChannelLogger) Debug(pkg, message string) {
	cl.LogLevel(pkg, DEBUG, message)
}

func (cl *ChannelLogger) Close() {
	close(cl.done) // Signal shutdown
	cl.wg.Wait()   // Wait for goroutine to finish

	// Close all file handles
	cl.mu.Lock()
	for pkg, f := range cl.files {
		if err := f.Close(); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to close log file for package %s: %v\n", pkg, err)
		}
	}
	cl.mu.Unlock()

	// Close the channel
	close(cl.logChan)
}

// SetMinLevel allows changing the minimum log level at runtime
func (cl *ChannelLogger) SetMinLevel(level LogLevel) {
	cl.mu.Lock()
	defer cl.mu.Unlock()
	cl.config.MinLevel = level
}

// GetMinLevel returns the current minimum log level
func (cl *ChannelLogger) GetMinLevel() LogLevel {
	cl.mu.Lock()
	defer cl.mu.Unlock()
	return cl.config.MinLevel
}

// Example usage function
func ExampleUsage() {
	// Create logger with default configuration
	logger := NewChannelLogger(100, "./logs")

	// Or create with custom configuration
	config := &Config{
		BufferSize:      200,
		LogDir:          "./custom-logs",
		TimestampFormat: "2006-01-02 15:04:05.000",
		MinLevel:        INFO, // Only log INFO and ERROR
	}
	advancedLogger := NewChannelLoggerWithConfig(config)

	// Traditional usage (still supported)
	logger.Info("mypackage", "Application started")
	logger.Error("database", "Connection failed")
	logger.Debug("auth", "User authenticated") // Won't show if MinLevel > DEBUG

	// NEW: Package-scoped loggers (recommended approach)
	appLogger := logger.Package("myapp")
	dbLogger := logger.Package("database")
	authLogger := logger.Package("auth")

	// Much cleaner API - no package name repetition
	appLogger.Info("Application started successfully")
	dbLogger.Error("Connection failed to primary database")
	authLogger.Debug("User authentication attempt")

	// Formatted logging
	appLogger.InfoF("Server started on port %d", 8080)
	dbLogger.ErrorF("Query failed after %d retries", 3)

	// Using context with package logger
	ctx := context.Background()
	appLogger.LogWithContext(ctx, "INFO", "Request processed successfully")

	// Change log level at runtime
	logger.SetMinLevel(ERROR) // Now only ERROR messages will be logged

	// Always close when done
	defer logger.Close()
	defer advancedLogger.Close()
}

type Logger interface {
	Info(pkg, message string)
	Error(pkg, message string)
	Debug(pkg, message string)
	Log(pkg, level, message string)
	LogLevel(pkg string, level LogLevel, message string)
	LogWithContext(ctx context.Context, pkg, level, message string)
	Close()
}

// PackageLogger provides a package-scoped logger interface
// This eliminates the need to repeat package names in every log call
type PackageLogger struct {
	logger *ChannelLogger
	pkg    string
}

// Package creates a new PackageLogger for the specified package
// This provides a more ergonomic API by eliminating package name repetition
func (cl *ChannelLogger) Package(pkg string) *PackageLogger {
	return &PackageLogger{
		logger: cl,
		pkg:    pkg,
	}
}

// Info logs an info-level message for this package
func (pl *PackageLogger) Info(message string) {
	pl.logger.Info(pl.pkg, message)
}

// Error logs an error-level message for this package
func (pl *PackageLogger) Error(message string) {
	pl.logger.Error(pl.pkg, message)
}

// Debug logs a debug-level message for this package
func (pl *PackageLogger) Debug(message string) {
	pl.logger.Debug(pl.pkg, message)
}

// Log logs a message with string level for this package
func (pl *PackageLogger) Log(level, message string) {
	pl.logger.Log(pl.pkg, level, message)
}

// LogLevel logs a message with typed level for this package
func (pl *PackageLogger) LogLevel(level LogLevel, message string) {
	pl.logger.LogLevel(pl.pkg, level, message)
}

// LogWithContext logs a context-aware message for this package
func (pl *PackageLogger) LogWithContext(ctx context.Context, level, message string) {
	pl.logger.LogWithContext(ctx, pl.pkg, level, message)
}

// InfoF logs a formatted info-level message for this package
func (pl *PackageLogger) InfoF(format string, args ...interface{}) {
	pl.logger.Info(pl.pkg, fmt.Sprintf(format, args...))
}

// ErrorF logs a formatted error-level message for this package
func (pl *PackageLogger) ErrorF(format string, args ...interface{}) {
	pl.logger.Error(pl.pkg, fmt.Sprintf(format, args...))
}

// DebugF logs a formatted debug-level message for this package
func (pl *PackageLogger) DebugF(format string, args ...interface{}) {
	pl.logger.Debug(pl.pkg, fmt.Sprintf(format, args...))
}

// GetPackageName returns the package name this logger is associated with
func (pl *PackageLogger) GetPackageName() string {
	return pl.pkg
}
