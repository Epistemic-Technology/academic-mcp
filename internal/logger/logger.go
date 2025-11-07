package logger

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
)

// Level represents the logging level
type Level int

const (
	DebugLevel Level = iota
	InfoLevel
	WarnLevel
	ErrorLevel
	FatalLevel
)

// String returns the string representation of the log level
func (l Level) String() string {
	switch l {
	case DebugLevel:
		return "DEBUG"
	case InfoLevel:
		return "INFO"
	case WarnLevel:
		return "WARN"
	case ErrorLevel:
		return "ERROR"
	case FatalLevel:
		return "FATAL"
	default:
		return "UNKNOWN"
	}
}

// Logger is the interface for logging operations
type Logger interface {
	Debug(format string, v ...any)
	Info(format string, v ...any)
	Warn(format string, v ...any)
	Error(format string, v ...any)
	Fatal(format string, v ...any)
	SetLevel(level Level)
}

// LogConfig holds configuration for the logger
type LogConfig struct {
	// Output destination: "file" or "stderr"
	Output string
	// Log level: "debug", "info", "warn", "error", "fatal"
	Level string
	// FilePath for file output (only used when Output is "file")
	FilePath string
}

// standardLogger implements the Logger interface using Go's standard log package
type standardLogger struct {
	logger *log.Logger
	level  Level
}

// NewLogger creates a new logger based on the provided configuration
func NewLogger(config LogConfig) (Logger, error) {
	var writer io.Writer

	// Determine output destination
	output := config.Output
	if output == "" {
		output = os.Getenv("LOG_OUTPUT")
	}
	if output == "" {
		// Auto-detect: if running in container, use stderr; otherwise use file
		output = detectEnvironment()
	}

	switch output {
	case "stderr":
		writer = os.Stderr
	case "file":
		filePath := config.FilePath
		if filePath == "" {
			filePath = os.Getenv("LOG_FILE_PATH")
		}
		if filePath == "" {
			// Default to ~/.academic-mcp/academic.log
			homeDir, err := os.UserHomeDir()
			if err != nil {
				return nil, fmt.Errorf("failed to get user home directory: %w", err)
			}
			logDir := filepath.Join(homeDir, ".academic-mcp")
			if err := os.MkdirAll(logDir, 0755); err != nil {
				return nil, fmt.Errorf("failed to create log directory: %w", err)
			}
			filePath = filepath.Join(logDir, "academic.log")
		}

		// Open log file in append mode
		file, err := os.OpenFile(filePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			return nil, fmt.Errorf("failed to open log file: %w", err)
		}
		writer = file
	default:
		return nil, fmt.Errorf("invalid log output: %s (expected 'file' or 'stderr')", output)
	}

	// Parse log level
	levelStr := config.Level
	if levelStr == "" {
		levelStr = os.Getenv("LOG_LEVEL")
	}
	if levelStr == "" {
		levelStr = "info" // default level
	}

	level := parseLevel(levelStr)

	// Create standard logger with timestamp
	stdLog := log.New(writer, "", log.LstdFlags)

	return &standardLogger{
		logger: stdLog,
		level:  level,
	}, nil
}

// NewNoOpLogger creates a logger that discards all output (useful for tests)
func NewNoOpLogger() Logger {
	return &standardLogger{
		logger: log.New(io.Discard, "", 0),
		level:  FatalLevel, // Only log fatals (essentially nothing)
	}
}

// detectEnvironment determines the appropriate output based on the environment
func detectEnvironment() string {
	// Check if running in a container
	if _, err := os.Stat("/.dockerenv"); err == nil {
		return "stderr"
	}

	// Check for Kubernetes environment
	if os.Getenv("KUBERNETES_SERVICE_HOST") != "" {
		return "stderr"
	}

	// Default to file for local development
	return "file"
}

// parseLevel converts a string to a Level
func parseLevel(level string) Level {
	switch strings.ToLower(level) {
	case "debug":
		return DebugLevel
	case "info":
		return InfoLevel
	case "warn", "warning":
		return WarnLevel
	case "error":
		return ErrorLevel
	case "fatal":
		return FatalLevel
	default:
		return InfoLevel
	}
}

// SetLevel sets the minimum log level
func (l *standardLogger) SetLevel(level Level) {
	l.level = level
}

// Debug logs a debug message
func (l *standardLogger) Debug(format string, v ...any) {
	if l.level <= DebugLevel {
		l.log(DebugLevel, format, v...)
	}
}

// Info logs an info message
func (l *standardLogger) Info(format string, v ...any) {
	if l.level <= InfoLevel {
		l.log(InfoLevel, format, v...)
	}
}

// Warn logs a warning message
func (l *standardLogger) Warn(format string, v ...any) {
	if l.level <= WarnLevel {
		l.log(WarnLevel, format, v...)
	}
}

// Error logs an error message
func (l *standardLogger) Error(format string, v ...any) {
	if l.level <= ErrorLevel {
		l.log(ErrorLevel, format, v...)
	}
}

// Fatal logs a fatal message and exits
func (l *standardLogger) Fatal(format string, v ...any) {
	l.log(FatalLevel, format, v...)
	os.Exit(1)
}

// log performs the actual logging
func (l *standardLogger) log(level Level, format string, v ...any) {
	message := fmt.Sprintf(format, v...)
	l.logger.Printf("[%s] %s", level.String(), message)
}
