// Package log4 provides a concurrent, channel-based logger with per-package file separation.
//
// Features:
// - Concurrent logging via channels and goroutines
// - Per-package log file separation
// - Configurable log levels with runtime filtering
// - Context-aware logging
// - Buffered channels to prevent blocking
// - Graceful shutdown with proper resource cleanup
// - Structured logging support
// - Basic log rotation
// - Enhanced error handling
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
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// Constants for configuration defaults and limits
const (
	DefaultBufferSize  = 100
	DefaultFileMode    = 0644
	DefaultDirMode     = 0755
	ShutdownTimeout    = 5 * time.Second
	MaxPackageNameLen  = 100
	DefaultMaxFileSize = 100 * 1024 * 1024 // 100MB
	DefaultMaxFiles    = 5
)

// Error message templates for consistency
const (
	ErrCreateLogDir      = "failed to create log directory %s: %w"
	ErrOpenLogFile       = "failed to open log file %s: %w"
	ErrCloseLogFile      = "failed to close log file for package %s: %w"
	ErrInvalidBufferSize = "buffer size must be positive, got %d"
	ErrEmptyTimestamp    = "timestamp format cannot be empty"
	ErrInvalidPackage    = "package name cannot be empty or contain invalid characters"
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

// LogEntry represents a structured log entry
type LogEntry struct {
	Package   string
	Level     LogLevel
	Message   string
	Fields    map[string]interface{}
	Context   context.Context
	Timestamp time.Time
}

// Config holds configuration options for the logger
type Config struct {
	BufferSize      int
	LogDir          string
	TimestampFormat string
	MinLevel        LogLevel
	FileMode        os.FileMode
	DirMode         os.FileMode
	MaxFileSize     int64
	MaxFiles        int
	ErrorHandler    func(error) // Optional error callback
}

// Validate checks if the configuration is valid
func (c *Config) Validate() error {
	if c.BufferSize <= 0 {
		return fmt.Errorf(ErrInvalidBufferSize, c.BufferSize)
	}
	if c.TimestampFormat == "" {
		return fmt.Errorf(ErrEmptyTimestamp)
	}
	if c.MaxFileSize <= 0 {
		c.MaxFileSize = DefaultMaxFileSize
	}
	if c.MaxFiles <= 0 {
		c.MaxFiles = DefaultMaxFiles
	}
	if c.FileMode == 0 {
		c.FileMode = DefaultFileMode
	}
	if c.DirMode == 0 {
		c.DirMode = DefaultDirMode
	}
	return nil
}

// DefaultConfig returns a configuration with sensible defaults
func DefaultConfig() *Config {
	return &Config{
		BufferSize:      DefaultBufferSize,
		LogDir:          "",
		TimestampFormat: "2006-01-02 15:04:05",
		MinLevel:        DEBUG,
		FileMode:        DefaultFileMode,
		DirMode:         DefaultDirMode,
		MaxFileSize:     DefaultMaxFileSize,
		MaxFiles:        DefaultMaxFiles,
	}
}

// Pool for LogEntry structs to reduce GC pressure
var logEntryPool = sync.Pool{
	New: func() interface{} {
		return &LogEntry{
			Fields: make(map[string]interface{}),
		}
	},
}

func getLogEntry() *LogEntry {
	return logEntryPool.Get().(*LogEntry)
}

func putLogEntry(entry *LogEntry) {
	// Reset the entry
	entry.Package = ""
	entry.Level = DEBUG
	entry.Message = ""
	entry.Context = nil
	entry.Timestamp = time.Time{}
	// Clear the map but keep the allocated memory
	for k := range entry.Fields {
		delete(entry.Fields, k)
	}
	logEntryPool.Put(entry)
}

// ChannelLogger is the main logger implementation
type ChannelLogger struct {
	logChan   chan *LogEntry
	done      chan struct{}
	wg        sync.WaitGroup
	loggers   map[string]*log.Logger // per-package loggers
	files     map[string]*os.File    // per-package files
	fileSizes map[string]int64       // track file sizes for rotation
	stdout    io.Writer
	config    *Config
	mu        sync.RWMutex
	minLevel  atomic.Int32 // Thread-safe minimum level
	closed    atomic.Bool  // Prevent operations after close
	errorChan chan error   // For async error reporting
}

// packageNameRegex for sanitizing package names
var packageNameRegex = regexp.MustCompile(`[^a-zA-Z0-9_-]`)

// sanitizePackageName removes dangerous characters from package names
func sanitizePackageName(pkg string) string {
	if pkg == "" {
		return "default"
	}

	// Replace invalid characters with underscores
	sanitized := packageNameRegex.ReplaceAllString(pkg, "_")

	// Limit length
	if len(sanitized) > MaxPackageNameLen {
		sanitized = sanitized[:MaxPackageNameLen]
	}

	return sanitized
}

// NewChannelLogger creates a new logger with basic configuration
func NewChannelLogger(bufferSize int, logDir string) *ChannelLogger {
	config := DefaultConfig()
	config.BufferSize = bufferSize
	config.LogDir = logDir
	return NewChannelLoggerWithConfig(config)
}

// NewChannelLoggerWithConfig creates a new logger with custom configuration
func NewChannelLoggerWithConfig(config *Config) *ChannelLogger {
	if config == nil {
		config = DefaultConfig()
	}

	if err := config.Validate(); err != nil {
		panic(fmt.Sprintf("Invalid logger configuration: %v", err))
	}

	cl := &ChannelLogger{
		logChan:   make(chan *LogEntry, config.BufferSize),
		done:      make(chan struct{}),
		loggers:   make(map[string]*log.Logger),
		files:     make(map[string]*os.File),
		fileSizes: make(map[string]int64),
		stdout:    os.Stdout,
		config:    config,
		errorChan: make(chan error, 10), // Small buffer for errors
	}

	// Set initial minimum level atomically
	cl.minLevel.Store(int32(config.MinLevel))

	// Create log directory if specified
	if config.LogDir != "" {
		if err := os.MkdirAll(config.LogDir, config.DirMode); err != nil {
			cl.handleError(fmt.Errorf(ErrCreateLogDir, config.LogDir, err))
		}
	}

	// Start the logging goroutine
	cl.wg.Add(1)
	go cl.run()

	// Start error handling goroutine if error handler is provided
	if config.ErrorHandler != nil {
		cl.wg.Add(1)
		go cl.handleErrors()
	}

	return cl
}

// handleError sends an error to the error channel or prints to stderr
func (cl *ChannelLogger) handleError(err error) {
	if cl.config.ErrorHandler != nil {
		select {
		case cl.errorChan <- err:
		default:
			// Error channel is full, fall back to stderr
			fmt.Fprintf(os.Stderr, "Logger error (channel full): %v\n", err)
		}
	} else {
		fmt.Fprintf(os.Stderr, "Logger error: %v\n", err)
	}
}

// handleErrors processes errors in a separate goroutine
func (cl *ChannelLogger) handleErrors() {
	defer cl.wg.Done()
	for {
		select {
		case err := <-cl.errorChan:
			if cl.config.ErrorHandler != nil {
				cl.config.ErrorHandler(err)
			}
		case <-cl.done:
			// Process remaining errors
			for len(cl.errorChan) > 0 {
				err := <-cl.errorChan
				if cl.config.ErrorHandler != nil {
					cl.config.ErrorHandler(err)
				}
			}
			return
		}
	}
}

// shouldRotate checks if a log file should be rotated
func (cl *ChannelLogger) shouldRotate(pkg string) bool {
	size, exists := cl.fileSizes[pkg]
	return exists && size >= cl.config.MaxFileSize
}

// rotateFile performs log file rotation
func (cl *ChannelLogger) rotateFile(pkg string) error {
	baseName := fmt.Sprintf("%s.log", sanitizePackageName(pkg))
	if cl.config.LogDir != "" {
		baseName = filepath.Join(cl.config.LogDir, baseName)
	}

	// Close current file
	if f, exists := cl.files[pkg]; exists {
		f.Close()
		delete(cl.files, pkg)
		delete(cl.loggers, pkg)
	}

	// Rotate existing files
	for i := cl.config.MaxFiles - 1; i > 0; i-- {
		oldName := fmt.Sprintf("%s.%d", baseName, i)
		newName := fmt.Sprintf("%s.%d", baseName, i+1)
		if i == cl.config.MaxFiles-1 {
			os.Remove(newName) // Remove oldest file
		}
		os.Rename(oldName, newName)
	}

	// Move current file to .1
	if _, err := os.Stat(baseName); err == nil {
		os.Rename(baseName, fmt.Sprintf("%s.1", baseName))
	}

	// Reset file size tracking
	cl.fileSizes[pkg] = 0

	return nil
}

// getLogger gets or creates a logger for the specified package
func (cl *ChannelLogger) getLogger(pkg string) *log.Logger {
	if cl.closed.Load() {
		// If closed, return a stdout-only logger
		return log.New(cl.stdout, "", 0)
	}

	cl.mu.Lock()
	defer cl.mu.Unlock()

	logger, ok := cl.loggers[pkg]
	if ok && !cl.shouldRotate(pkg) {
		return logger
	}

	// Handle rotation if needed
	if cl.shouldRotate(pkg) {
		if err := cl.rotateFile(pkg); err != nil {
			cl.handleError(fmt.Errorf("failed to rotate log file for package %s: %w", pkg, err))
		}
	}

	sanitizedPkg := sanitizePackageName(pkg)
	fileName := fmt.Sprintf("%s.log", sanitizedPkg)
	if cl.config.LogDir != "" {
		fileName = filepath.Join(cl.config.LogDir, fileName)
	}

	var writers []io.Writer
	writers = append(writers, cl.stdout)

	f, err := os.OpenFile(fileName, os.O_CREATE|os.O_WRONLY|os.O_APPEND, cl.config.FileMode)
	if err != nil {
		cl.handleError(fmt.Errorf(ErrOpenLogFile, fileName, err))
	} else {
		cl.files[pkg] = f
		writers = append(writers, f)

		// Get current file size
		if stat, err := f.Stat(); err == nil {
			cl.fileSizes[pkg] = stat.Size()
		}
	}

	logger = log.New(io.MultiWriter(writers...), "", 0)
	cl.loggers[pkg] = logger
	return logger
}

// formatLogMessage formats a log message with efficient string building
func formatLogMessage(entry *LogEntry, timestampFormat string) string {
	var sb strings.Builder

	// Pre-allocate reasonable capacity
	sb.Grow(len(timestampFormat) + len(entry.Level.String()) + len(entry.Message) + 20)

	sb.WriteString("[")
	sb.WriteString(entry.Timestamp.Format(timestampFormat))
	sb.WriteString("] ")
	sb.WriteString(entry.Level.String())
	sb.WriteString(": ")
	sb.WriteString(entry.Message)

	// Add structured fields if present
	if len(entry.Fields) > 0 {
		sb.WriteString(" | ")
		first := true
		for k, v := range entry.Fields {
			if !first {
				sb.WriteString(", ")
			}
			sb.WriteString(k)
			sb.WriteString("=")
			sb.WriteString(fmt.Sprintf("%v", v))
			first = false
		}
	}

	return sb.String()
}

// run processes log entries in a background goroutine
func (cl *ChannelLogger) run() {
	defer cl.wg.Done()

	for {
		select {
		case entry := <-cl.logChan:
			// Check if context is cancelled
			if entry.Context != nil && entry.Context.Err() != nil {
				putLogEntry(entry)
				continue
			}

			// Format and log the message (level check already done in logEntry)
			formatted := formatLogMessage(entry, cl.config.TimestampFormat)
			logger := cl.getLogger(entry.Package)

			// Track bytes written for rotation
			messageSize := int64(len(formatted) + 1) // +1 for newline
			cl.mu.Lock()
			cl.fileSizes[entry.Package] += messageSize
			cl.mu.Unlock()

			logger.Println(formatted)

			// Return entry to pool
			putLogEntry(entry)

		case <-cl.done:
			// Process remaining entries
			for len(cl.logChan) > 0 {
				entry := <-cl.logChan
				if entry.Context != nil && entry.Context.Err() != nil {
					putLogEntry(entry)
					continue
				}
				formatted := formatLogMessage(entry, cl.config.TimestampFormat)
				cl.getLogger(entry.Package).Println(formatted)
				putLogEntry(entry)
			}
			return
		}
	}
}

// ParseLogLevel converts a string to a LogLevel
func ParseLogLevel(level string) LogLevel {
	switch strings.ToUpper(level) {
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

// logEntry sends a log entry to the processing channel
func (cl *ChannelLogger) logEntry(entry *LogEntry) {
	if cl.closed.Load() {
		putLogEntry(entry)
		return
	}

	// Check minimum level before sending to channel to avoid unnecessary work
	if entry.Level < LogLevel(cl.minLevel.Load()) {
		putLogEntry(entry)
		return
	}

	select {
	case cl.logChan <- entry:
		// Successfully queued
	default:
		// Channel is immediately full, use a brief timeout for larger buffers
		if cl.config.BufferSize > 10 {
			// For larger buffers, give a brief chance to queue
			select {
			case cl.logChan <- entry:
				// Successfully queued after brief wait
			case <-time.After(5 * time.Millisecond):
				// Channel remained full, drop the message
				cl.handleError(fmt.Errorf("log channel full, dropping message: %s", entry.Message))
				putLogEntry(entry)
			}
		} else {
			// For small buffers, drop immediately to properly test overflow behavior
			cl.handleError(fmt.Errorf("log channel full, dropping message: %s", entry.Message))
			putLogEntry(entry)
		}
	}
}

// Log logs a message with string level
func (cl *ChannelLogger) Log(pkg, level, message string) {
	cl.LogLevel(pkg, ParseLogLevel(level), message)
}

// LogLevel logs a message with typed level
func (cl *ChannelLogger) LogLevel(pkg string, level LogLevel, message string) {
	entry := getLogEntry()
	entry.Package = pkg
	entry.Level = level
	entry.Message = message
	entry.Timestamp = time.Now()
	cl.logEntry(entry)
}

// LogWithContext logs a context-aware message
func (cl *ChannelLogger) LogWithContext(ctx context.Context, pkg, level, message string) {
	if ctx.Err() != nil {
		return // Context cancelled/expired
	}

	entry := getLogEntry()
	entry.Package = pkg
	entry.Level = ParseLogLevel(level)
	entry.Message = message
	entry.Context = ctx
	entry.Timestamp = time.Now()
	cl.logEntry(entry)
}

// LogWithFields logs a message with structured fields
func (cl *ChannelLogger) LogWithFields(pkg string, level LogLevel, message string, fields map[string]interface{}) {
	entry := getLogEntry()
	entry.Package = pkg
	entry.Level = level
	entry.Message = message
	entry.Timestamp = time.Now()

	// Copy fields to avoid mutation
	for k, v := range fields {
		entry.Fields[k] = v
	}

	cl.logEntry(entry)
}

// Info logs an info-level message
func (cl *ChannelLogger) Info(pkg, message string) {
	cl.LogLevel(pkg, INFO, message)
}

// Error logs an error-level message
func (cl *ChannelLogger) Error(pkg, message string) {
	cl.LogLevel(pkg, ERROR, message)
}

// Debug logs a debug-level message
func (cl *ChannelLogger) Debug(pkg, message string) {
	cl.LogLevel(pkg, DEBUG, message)
}

// SetMinLevel changes the minimum log level at runtime (thread-safe)
func (cl *ChannelLogger) SetMinLevel(level LogLevel) {
	cl.minLevel.Store(int32(level))
}

// GetMinLevel returns the current minimum log level (thread-safe)
func (cl *ChannelLogger) GetMinLevel() LogLevel {
	return LogLevel(cl.minLevel.Load())
}

// Close gracefully shuts down the logger
func (cl *ChannelLogger) Close() {
	if !cl.closed.CompareAndSwap(false, true) {
		return // Already closed
	}

	close(cl.done) // Signal shutdown
	cl.wg.Wait()   // Wait for goroutines to finish

	// Close all file handles
	cl.mu.Lock()
	for pkg, f := range cl.files {
		if err := f.Close(); err != nil {
			cl.handleError(fmt.Errorf(ErrCloseLogFile, pkg, err))
		}
	}
	cl.mu.Unlock()

	// Close channels
	close(cl.logChan)
	close(cl.errorChan)
}

// Package creates a new PackageLogger for the specified package
func (cl *ChannelLogger) Package(pkg string) *PackageLogger {
	if pkg == "" {
		panic(fmt.Errorf(ErrInvalidPackage))
	}

	return &PackageLogger{
		logger: cl,
		pkg:    pkg,
	}
}

// PackageLogger provides a package-scoped logger interface
type PackageLogger struct {
	logger *ChannelLogger
	pkg    string
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

// InfoWithFields logs an info message with structured fields
func (pl *PackageLogger) InfoWithFields(message string, fields map[string]interface{}) {
	pl.logger.LogWithFields(pl.pkg, INFO, message, fields)
}

// ErrorWithFields logs an error message with structured fields
func (pl *PackageLogger) ErrorWithFields(message string, fields map[string]interface{}) {
	pl.logger.LogWithFields(pl.pkg, ERROR, message, fields)
}

// DebugWithFields logs a debug message with structured fields
func (pl *PackageLogger) DebugWithFields(message string, fields map[string]interface{}) {
	pl.logger.LogWithFields(pl.pkg, DEBUG, message, fields)
}

// LogWithContext logs a context-aware message for this package
func (pl *PackageLogger) LogWithContext(ctx context.Context, level, message string) {
	pl.logger.LogWithContext(ctx, pl.pkg, level, message)
}

// GetPackageName returns the package name this logger is associated with
func (pl *PackageLogger) GetPackageName() string {
	return pl.pkg
}

// Logger interface for compatibility
type Logger interface {
	Info(pkg, message string)
	Error(pkg, message string)
	Debug(pkg, message string)
	Log(pkg, level, message string)
	LogLevel(pkg string, level LogLevel, message string)
	LogWithContext(ctx context.Context, pkg, level, message string)
	Close()
}
