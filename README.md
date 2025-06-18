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
    
    // Basic logging
    logger.Info("myapp", "Application started")
    logger.Error("database", "Connection failed") 
    logger.Debug("auth", "User login successful")
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

// Runtime level changes
logger.SetMinLevel(log4.ERROR) // Now only ERROR messages show

// Context-aware logging
ctx := context.Background()
logger.LogWithContext(ctx, "worker", "INFO", "Task completed")
```

## API Reference

### Log Levels
- `DEBUG`: Detailed information for diagnosing problems
- `INFO`: General information about program execution  
- `ERROR`: Error events that might still allow the application to continue

### Core Methods
- `Info(pkg, message string)`: Log info-level message
- `Error(pkg, message string)`: Log error-level message  
- `Debug(pkg, message string)`: Log debug-level message
- `Log(pkg, level, message string)`: Log with string level
- `LogLevel(pkg string, level LogLevel, message string)`: Log with typed level
- `LogWithContext(ctx, pkg, level, message string)`: Context-aware logging
- `SetMinLevel(level LogLevel)`: Change minimum log level at runtime
- `Close()`: Gracefully shutdown logger

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
