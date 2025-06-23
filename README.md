# log4 - Concurrent Logging

A high-performance, production-ready concurrent logging library for Go with advanced features for enterprise applications.

## 🚀 Key Features

✅ **High-Performance Concurrent Logging**: Channel-based architecture with background goroutine processing  
✅ **Per-Package File Separation**: Automatic separation of logs into individual files per package  
✅ **Runtime Log Level Control**: Dynamic filtering with thread-safe level changes (DEBUG, INFO, ERROR)  
✅ **Structured Logging**: Key-value field support for better log analysis  
✅ **Context-Aware Logging**: Full Go context integration with cancellation support  
✅ **Automatic Log Rotation**: Size-based rotation with configurable file retention  
✅ **Memory Pool Optimization**: Object pooling reduces GC pressure for high-throughput scenarios  
✅ **Package Sanitization**: Safe handling of package names with invalid characters  
✅ **Dual Output**: Simultaneous logging to stdout and files  
✅ **Graceful Shutdown**: Proper resource cleanup and message draining  
✅ **Smart Buffer Management**: Adaptive timeout strategies prevent message loss  
✅ **Custom Error Handling**: Optional error callbacks for monitoring  
✅ **Production Hardened**: Comprehensive test suite with 100% pass rate  

## 📦 Installation

```bash
go get github.com/MhunterDev/log4
```

## 🔥 Quick Start

```go
package main

import (
    "context"
    "github.com/MhunterDev/log4"
)

func main() {
    // Create logger with default settings
    logger := log4.NewChannelLogger(100, "./logs")
    defer logger.Close()
    
    // Traditional approach (still supported)
    logger.Info("myapp", "Application started")
    logger.Error("database", "Connection failed") 
    logger.Debug("auth", "User login successful")
    
    // 🌟 NEW: Package-scoped loggers (recommended)
    appLogger := logger.Package("myapp")
    dbLogger := logger.Package("database")
    
    // Much cleaner API - no package name repetition
    appLogger.Info("Application started successfully")
    dbLogger.Error("Connection failed")
    appLogger.InfoF("Server listening on port %d", 8080)
    
    // Structured logging with fields
    appLogger.InfoWithFields("User logged in", map[string]interface{}{
        "user_id": 12345,
        "ip": "192.168.1.1",
        "action": "login",
    })
}
```

## ⚙️ Advanced Configuration

```go
// Custom configuration with all available options
config := &log4.Config{
    BufferSize:      200,                    // Channel buffer size
    LogDir:          "./logs",               // Log directory
    TimestampFormat: "2006-01-02 15:04:05.000", // Custom timestamp
    MinLevel:        log4.INFO,              // Minimum log level
    FileMode:        0644,                   // Log file permissions
    DirMode:         0755,                   // Directory permissions
    MaxFileSize:     50 * 1024 * 1024,      // 50MB max file size
    MaxFiles:        10,                     // Keep 10 rotated files
    ErrorHandler: func(err error) {          // Custom error handling
        fmt.Printf("Logger error: %v\n", err)
    },
}

logger := log4.NewChannelLoggerWithConfig(config)
defer logger.Close()

// Create package-scoped loggers for clean API
appLogger := logger.Package("myapp")
dbLogger := logger.Package("database")

// All logging methods available
appLogger.Info("Application configured successfully")
appLogger.InfoF("Server started on port %d with %d workers", 8080, 4)
appLogger.InfoWithFields("Request processed", map[string]interface{}{
    "duration_ms": 45,
    "status_code": 200,
    "user_id": 12345,
})

// Runtime level changes (thread-safe)
logger.SetMinLevel(log4.ERROR) // Now only ERROR messages show

// Context-aware logging
ctx := context.Background()
appLogger.LogWithContext(ctx, "INFO", "Task completed successfully")
```

## 📊 Structured Logging

The logger supports rich structured logging for better log analysis:

```go
logger := log4.NewChannelLogger(100, "./logs")
defer logger.Close()

appLogger := logger.Package("ecommerce")

// Traditional logging
appLogger.Info("Order processed")

// Enhanced with structured fields
appLogger.InfoWithFields("Order processed", map[string]interface{}{
    "order_id":     "ORD-12345",
    "customer_id":  67890,
    "amount":       99.99,
    "currency":     "USD",
    "payment_method": "credit_card",
    "processing_time_ms": 234,
})

// Output: [2025-06-23 18:10:15] INFO: Order processed | order_id=ORD-12345, customer_id=67890, amount=99.99, currency=USD, payment_method=credit_card, processing_time_ms=234
```

## 🔄 Automatic Log Rotation

Built-in log rotation prevents disk space issues:

```go
config := log4.DefaultConfig()
config.LogDir = "./logs"
config.MaxFileSize = 100 * 1024 * 1024  // 100MB per file
config.MaxFiles = 5                      // Keep 5 rotated files

logger := log4.NewChannelLoggerWithConfig(config)
defer logger.Close()

// Files automatically rotate when they reach MaxFileSize:
// myapp.log      (current)
// myapp.log.1    (previous)
// myapp.log.2    (older)
// myapp.log.3    (older)
// myapp.log.4    (oldest)
```

## 🎯 API Reference

### Core Logger Methods

**ChannelLogger:**
```go
// Basic logging
Info(pkg, message string)
Error(pkg, message string)  
Debug(pkg, message string)
Log(pkg, level, message string)
LogLevel(pkg string, level LogLevel, message string)

// Advanced logging
LogWithContext(ctx context.Context, pkg, level, message string)
LogWithFields(pkg string, level LogLevel, message string, fields map[string]interface{})

// Configuration
SetMinLevel(level LogLevel)        // Thread-safe runtime level changes
GetMinLevel() LogLevel             // Get current minimum level
Package(pkg string) *PackageLogger // Create package-scoped logger
Close()                            // Graceful shutdown
```

**PackageLogger (Recommended):**
```go
// Clean API without package repetition
Info(message string)
Error(message string)
Debug(message string)

// Formatted logging
InfoF(format string, args ...interface{})
ErrorF(format string, args ...interface{})
DebugF(format string, args ...interface{})

// Structured logging
InfoWithFields(message string, fields map[string]interface{})
ErrorWithFields(message string, fields map[string]interface{})
DebugWithFields(message string, fields map[string]interface{})

// Context support
LogWithContext(ctx context.Context, level, message string)
GetPackageName() string
```

### Log Levels
- **`DEBUG`**: Detailed diagnostic information
- **`INFO`**: General informational messages  
- **`ERROR`**: Error events that may allow continued execution

### Configuration Options

```go
type Config struct {
    BufferSize      int           // Channel buffer size (default: 100)
    LogDir          string        // Log directory (default: current dir)
    TimestampFormat string        // Time format (default: "2006-01-02 15:04:05")
    MinLevel        LogLevel      // Minimum level (default: DEBUG)
    FileMode        os.FileMode   // File permissions (default: 0644)
    DirMode         os.FileMode   // Directory permissions (default: 0755)
    MaxFileSize     int64         // Max file size in bytes (default: 100MB)
    MaxFiles        int           // Number of rotated files to keep (default: 5)
    ErrorHandler    func(error)   // Optional error callback
}
```

## 📁 File Organization

The logger automatically creates separate log files for each package:

```
logs/
├── myapp.log           # Logs from "myapp" package
├── myapp.log.1         # Previous rotation
├── database.log        # Logs from "database" package  
├── database.log.1      # Previous rotation
├── auth.log           # Logs from "auth" package
└── monitoring.log     # Logs from "monitoring" package
```

## 🎨 API Evolution

The package-scoped logger provides a superior developer experience:

```go
// ❌ Before: Repetitive and error-prone
logger.Info("myapp", "Server started")
logger.Error("myapp", "Server failed")
logger.Info("myapp", fmt.Sprintf("Listening on port %d", 8080)) // Manual formatting

// ✅ After: Clean and intuitive
appLogger := logger.Package("myapp")
appLogger.Info("Server started")
appLogger.Error("Server failed") 
appLogger.InfoF("Listening on port %d", 8080) // Built-in formatting
appLogger.InfoWithFields("Request handled", map[string]interface{}{
    "method": "GET",
    "path": "/api/users",
    "duration_ms": 45,
}) // Rich structured logging
```

**Key Benefits:**
- ✅ No package name repetition
- ✅ Built-in formatted logging (`InfoF`, `ErrorF`, `DebugF`)
- ✅ Structured logging with fields
- ✅ Cleaner, more maintainable code
- ✅ Full backward compatibility

## 🚀 Performance Features

- **Non-blocking Design**: Buffered channels prevent goroutine blocking
- **Concurrent Processing**: Background processing doesn't slow your application
- **Memory Optimization**: Object pooling reduces GC pressure
- **Smart Buffer Management**: Adaptive timeouts handle load spikes gracefully
- **Efficient String Building**: Pre-allocated buffers for message formatting
- **Thread-Safe Operations**: Atomic operations for runtime configuration changes

## 🧪 Testing & Quality

```bash
# Run the complete test suite
go test -v

# Run with race detection
go test -race -v

# Run benchmarks
go test -bench=. -benchmem
```

**Test Coverage:**
- ✅ Concurrent logging scenarios
- ✅ Log level filtering and runtime changes
- ✅ Structured logging with fields
- ✅ Context cancellation handling
- ✅ File rotation mechanics
- ✅ Error handling and recovery
- ✅ Memory pool efficiency
- ✅ Package name sanitization
- ✅ Graceful shutdown behavior
- ✅ Channel overflow scenarios

## 🔧 Advanced Use Cases

### Enterprise Monitoring
```go
config := &log4.Config{
    BufferSize:   1000,                    // High throughput
    LogDir:       "/var/log/myapp",        // Standard location
    MinLevel:     log4.INFO,               // Production level
    MaxFileSize:  500 * 1024 * 1024,      // 500MB files
    MaxFiles:     20,                      // 10GB total retention
    ErrorHandler: func(err error) {
        // Send to monitoring system
        monitoring.AlertLoggerError(err)
    },
}

logger := log4.NewChannelLoggerWithConfig(config)
defer logger.Close()
```

### Microservice Architecture
```go
// Each service component gets its own logger
apiLogger := logger.Package("api")
dbLogger := logger.Package("database") 
cacheLogger := logger.Package("cache")
queueLogger := logger.Package("message-queue")

// Structured logging for observability
apiLogger.InfoWithFields("Request processed", map[string]interface{}{
    "trace_id": "abc123",
    "user_id": 12345,
    "endpoint": "/api/v1/users",
    "method": "GET",
    "status": 200,
    "duration_ms": 45,
})
```

### Development vs Production
```go
var config *log4.Config
if os.Getenv("ENV") == "production" {
    config = &log4.Config{
        MinLevel: log4.INFO,        // Hide debug logs
        LogDir:   "/var/log/app",   // Centralized location
    }
} else {
    config = &log4.Config{
        MinLevel: log4.DEBUG,       // Show all logs
        LogDir:   "./dev-logs",     // Local development
    }
}

logger := log4.NewChannelLoggerWithConfig(config)
```

## 📈 Migration Guide

### From Standard Library
```go
// Old: Standard log
import "log"
log.Println("Something happened")

// New: log4
import "github.com/MhunterDev/log4"
logger := log4.NewChannelLogger(100, "./logs")
appLogger := logger.Package("myapp")
appLogger.Info("Something happened")
```

### From Other Loggers
```go
// From logrus, zap, etc.
// 1. Replace initialization
logger := log4.NewChannelLogger(100, "./logs")
defer logger.Close()

// 2. Create package-scoped loggers
appLogger := logger.Package("myapp")

// 3. Use familiar methods
appLogger.Info("message")
appLogger.Error("error message")
appLogger.InfoWithFields("event", map[string]interface{}{
    "key": "value",
})
```

## 🐛 Troubleshooting

**Common Issues:**

1. **Messages being dropped**: Increase `BufferSize` for high-throughput scenarios
2. **Permission errors**: Check `FileMode` and `DirMode` settings
3. **Disk space**: Monitor log rotation with `MaxFileSize` and `MaxFiles`
4. **Performance**: Use package-scoped loggers and structured logging appropriately

**Debug Mode:**
```go
config := log4.DefaultConfig()
config.ErrorHandler = func(err error) {
    fmt.Printf("DEBUG: Logger error: %v\n", err)
}
```

## 📄 License

This project is open source and available under the MIT License.

## 🤝 Contributing

Contributions are welcome! Please ensure all tests pass and maintain the existing code quality standards.

---

**Built for Production** | **Battle-Tested** | **Developer Friendly**
