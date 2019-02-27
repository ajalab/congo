package log

import (
	"log"
	"os"
	"strings"
	"sync"
)

// Logger is the interface for logging.
type Logger interface {
	// Printf prints a formated message to the log.
	Printf(format string, v ...interface{})

	// Print prints a message to the log.
	Print(v ...interface{})

	// Fatalf
	Fatalf(format string, v ...interface{})

	// Fatal
	Fatal(v ...interface{})

	// Level returns the logging level.
	Level() Level
}

// Level represents the log level.
type Level int

const (
	// DebugLevel represents the debug-level.
	DebugLevel Level = iota
	// InfoLevel represents the info-level.
	InfoLevel
	// ErrorLevel represents the error-level.
	ErrorLevel
	// DisabledLevel represents that the logger is defabled.
	DisabledLevel
)

var (
	// Debug is a debug-level logger.
	Debug = &logger{DebugLevel}
	// Info is an info-level logger.
	Info = &logger{InfoLevel}
	// Error is an error-level logger.
	Error = &logger{ErrorLevel}
)

var mu sync.RWMutex

var currentLogger = &defaultLogger{
	level:  InfoLevel,
	Logger: log.New(os.Stderr, "", log.Ldate|log.Ltime|log.LUTC),
}

type logger struct {
	level Level
}

func getCurrentLogger() Logger {
	mu.RLock()
	defer mu.RUnlock()
	return currentLogger
}

func (l logger) Printf(format string, v ...interface{}) {
	cLogger := getCurrentLogger()
	if l.level >= cLogger.Level() {
		cLogger.Printf(format, v...)
	}
}

func (l logger) Print(v ...interface{}) {
	cLogger := getCurrentLogger()
	if l.level >= cLogger.Level() {
		cLogger.Print(v...)
	}
}

func (l logger) Fatalf(format string, v ...interface{}) {
	cLogger := getCurrentLogger()
	if l.level >= cLogger.Level() {
		cLogger.Fatalf(format, v...)
	}
}

func (l logger) Fatal(v ...interface{}) {
	cLogger := getCurrentLogger()
	if l.level >= cLogger.Level() {
		cLogger.Fatal(v...)
	}
}

func (l logger) Level() Level {
	return l.level
}

type defaultLogger struct {
	level Level
	*log.Logger
}

func (l *defaultLogger) Level() Level {
	return l.level
}

// SetLevel sets the current logging level.
func SetLevel(level Level) {
	mu.Lock()
	currentLogger.level = level
	mu.Unlock()
}

// SetLevelByName sets the current logging level with a name.
func SetLevelByName(level string) {
	switch strings.ToLower(level) {
	case "debug":
		SetLevel(DebugLevel)
	case "info":
		SetLevel(InfoLevel)
	case "error":
		SetLevel(ErrorLevel)
	case "disabled":
		SetLevel(DisabledLevel)
	}
}
