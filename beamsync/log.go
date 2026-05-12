package beamsync

import (
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"time"
)

type LogLevel int

const (
	LevelDebug LogLevel = iota
	LevelInfo
	LevelWarn
	LevelError
)

var (
	currentLevel = LevelInfo
	logger       = log.New(os.Stdout, "", 0)
)

func SetLogLevel(l LogLevel) {
	currentLevel = l
}

func SetLogOutput(w io.Writer) {
	logger = log.New(w, "", 0)
}

func logf(level LogLevel, tag string, format string, args ...interface{}) {
	if level < currentLevel {
		return
	}
	msg := fmt.Sprintf(format, args...)
	timestamp := time.Now().Format("15:04:05.000")
	logger.Printf("%s [%s] %s", timestamp, tag, msg)
}

func Debug(format string, args ...interface{}) {
	logf(LevelDebug, "DBG", format, args...)
}

func Info(format string, args ...interface{}) {
	logf(LevelInfo, "INF", format, args...)
}

func Warn(format string, args ...interface{}) {
	logf(LevelWarn, "WRN", format, args...)
}

func Error(format string, args ...interface{}) {
	logf(LevelError, "ERR", format, args...)
}

func Fatal(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	timestamp := time.Now().Format("15:04:05.000")
	logger.Printf("%s [FTL] %s", timestamp, msg)
	os.Exit(1)
}

// LevelFromString parses a log level string (case-insensitive).
func LevelFromString(s string) LogLevel {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "debug":
		return LevelDebug
	case "info":
		return LevelInfo
	case "warn":
		return LevelWarn
	case "error":
		return LevelError
	default:
		return LevelInfo
	}
}
