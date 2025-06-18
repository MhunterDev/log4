package main

import (
	"context"
	"time"

	"log4"
)

func main() {
	logger := log4.NewChannelLogger(100, "./demo-logs")
	defer logger.Close()

	logger.Info("demo", "This is an info message from the demo!")
	logger.Error("demo", "This is an error message from the demo!")
	logger.Debug("demo", "This is a debug message from the demo!")

	ctx := context.Background()
	logger.LogWithContext(ctx, "demo", "INFO", "This is a context-aware log message!")

	time.Sleep(100 * time.Millisecond)
}
