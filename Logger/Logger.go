// Package Logger is a thin leveled wrapper over the standard logger, shared by
// every package in the bot so all output lands in one place with one format.
package Logger

import (
	"io"
	"log"
	"strings"
)

// Level is a logging severity. Messages below the configured level are dropped.
type Level int

const (
	LevelDebug Level = iota
	LevelInfo
	LevelWarn
	LevelError
)

var levelNames = map[Level]string{
	LevelDebug: "DEBUG",
	LevelInfo:  "INFO ",
	LevelWarn:  "WARN ",
	LevelError: "ERROR",
}

var currentLevel = LevelInfo

// ParseLevel maps a config string ("debug", "INFO", ...) to a Level,
// falling back to INFO for anything unrecognised.
func ParseLevel(s string) Level {
	switch strings.ToUpper(strings.TrimSpace(s)) {
	case "DEBUG":
		return LevelDebug
	case "WARN", "WARNING":
		return LevelWarn
	case "ERROR":
		return LevelError
	default:
		return LevelInfo
	}
}

// Setup points the standard logger at out and sets the minimum level.
func Setup(out io.Writer, level Level) {
	currentLevel = level
	log.SetOutput(out)
	log.SetFlags(log.Ldate | log.Ltime)
}

func logAt(level Level, format string, args ...any) {
	if level < currentLevel {
		return
	}
	log.Printf("| "+levelNames[level]+" | "+format, args...)
}

func Debug(format string, args ...any) { logAt(LevelDebug, format, args...) }
func Info(format string, args ...any)  { logAt(LevelInfo, format, args...) }
func Warn(format string, args ...any)  { logAt(LevelWarn, format, args...) }
func Error(format string, args ...any) { logAt(LevelError, format, args...) }
