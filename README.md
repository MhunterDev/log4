# 4logs - Concurrent Channel-Based Logger

A high-performance, concurrent logging library for Go with per-package file separation and configurable log levels.

## Features

✅ **Concurrent Logging**: Channel-based architecture with background goroutine processing  
✅ **Per-Package Files**: Automatic separation of logs into individual files per package  
✅ **Log Level Filtering**: Runtime configurable minimum log levels (DEBUG, INFO, ERROR)  
✅ **Context Support**: Context-aware logging with cancellation support  
✅ **Dual Output**: Simultaneous logging to stdout and files  
✅ **Graceful Shutdown**: Proper resource cleanup and message draining  
✅ **Buffered Channels**: Configurable buffer sizes to prevent blocking  
✅ **Custom Configuration**: Flexible timestamp formats and directory structures  
✅ **Package-Scoped Loggers**: Clean API without package name repetition  
✅ **Formatted Logging**: Printf-style formatting support  

## Quick Start

```go
package main

import (
    "4logs"
    "context"
)

func main() {
    // Create logger with default settings
    logger := log4.NewChannelLogger(100, "./logs")
    defer logger.Close()
    
    // Traditional approach (still supported)
    logger.Info("myapp", "Application started")
    logger.Error("database", "Connection failed") 
    logger.Debug("auth", "User login successful")
    
    // NEW: Package-scoped loggers (recommended)
    appLogger := logger.Package("myapp")
    dbLogger := logger.Package("database")
    
    // Much cleaner API - no package name repetition
    appLogger.Info("Application started successfully")
    dbLogger.Error("Connection failed")
    appLogger.InfoF("Server listening on port %d", 8080)
}
```

## Advanced Configuration

```go
// Custom configuration
config := &log4.Config{
    BufferSize:      200,
    LogDir:          "./custom-logs",
    TimestampFormat: "2006-01-02 15:04:05.000",
    MinLevel:        log4.INFO, // Only show INFO and ERROR
}

logger := log4.NewChannelLoggerWithConfig(config)
defer logger.Close()

// Create package-scoped loggers for cleaner API
appLogger := logger.Package("myapp")
dbLogger := logger.Package("database")

// Clean logging without package name repetition
appLogger.Info("Application configured successfully")
dbLogger.Error("Database connection failed")

// Formatted logging
appLogger.InfoF("Server started on port %d with %d workers", 8080, 4)
dbLogger.ErrorF("Connection failed after %d attempts", 3)

// Runtime level changes
logger.SetMinLevel(log4.ERROR) // Now only ERROR messages show

// Context-aware logging
ctx := context.Background()
appLogger.LogWithContext(ctx, "INFO", "Task completed")
```

## API Reference

### Log Levels
- `DEBUG`: Detailed information for diagnosing problems
- `INFO`: General information about program execution  
- `ERROR`: Error events that might still allow the application to continue

### Core Methods

**ChannelLogger Methods:**
- `Info(pkg, message string)`: Log info-level message
- `Error(pkg, message string)`: Log error-level message  
- `Debug(pkg, message string)`: Log debug-level message
- `Log(pkg, level, message string)`: Log with string level
- `LogLevel(pkg string, level LogLevel, message string)`: Log with typed level
- `LogWithContext(ctx, pkg, level, message string)`: Context-aware logging
- `Package(pkg string) *PackageLogger`: Create package-scoped logger
- `SetMinLevel(level LogLevel)`: Change minimum log level at runtime
- `Close()`: Gracefully shutdown logger

**PackageLogger Methods (Recommended):**
- `Info(message string)`: Log info-level message
- `Error(message string)`: Log error-level message
- `Debug(message string)`: Log debug-level message
- `InfoF(format string, args ...interface{})`: Formatted info logging
- `ErrorF(format string, args ...interface{})`: Formatted error logging
- `DebugF(format string, args ...interface{})`: Formatted debug logging
- `LogWithContext(ctx, level, message string)`: Context-aware logging
- `GetPackageName() string`: Get associated package name

### Configuration Options
- `BufferSize`: Channel buffer size (default: 100)
- `LogDir`: Directory for log files (default: current directory)
- `TimestampFormat`: Go time format string (default: "2006-01-02 15:04:05")
- `MinLevel`: Minimum log level to display (default: DEBUG)

## File Organization

The logger automatically creates separate log files for each package:
```
logs/
├── myapp.log      # Logs from "myapp" package
├── database.log   # Logs from "database" package  
└── auth.log       # Logs from "auth" package
```

## API Ergonomics

The package-scoped logger provides a much cleaner API:

```go
// Before: Repetitive package names
logger.Info("myapp", "Server started")
logger.Error("myapp", "Server failed")
logger.InfoF("myapp", "Listening on port %d", 8080) // No direct support

// After: Clean package-scoped loggers
appLogger := logger.Package("myapp")
appLogger.Info("Server started")
appLogger.Error("Server failed") 
appLogger.InfoF("Listening on port %d", 8080) // Built-in formatting
```

**Benefits:**
- ✅ No package name repetition
- ✅ Built-in formatted logging (`InfoF`, `ErrorF`, `DebugF`)
- ✅ Cleaner, more readable code
- ✅ Backward compatibility maintained

## Performance Features

- **Non-blocking**: Buffered channels prevent goroutine blocking
- **Concurrent**: Background processing doesn't slow down your application
- **Memory efficient**: Proper cleanup prevents resource leaks
- **Graceful shutdown**: Ensures all messages are written before exit

## Testing

Run the test suite:
```bash
go test -v
```

## Improvements Made

This version includes major improvements over the original:

1. **Fixed Memory Issues**: Corrected file handle storage and cleanup
2. **Added Log Levels**: Type-safe log level system with filtering
3. **Better Error Handling**: Proper error reporting for file operations
4. **Enhanced Configuration**: Flexible config system with defaults
5. **Context Support**: Modern Go context integration
6. **Improved Shutdown**: WaitGroup ensures proper goroutine cleanup
7. **Runtime Configuration**: Dynamic log level changes
8. **Better Documentation**: Comprehensive examples and API docs

## License

This project is provided as-is for educational and development purposes.
