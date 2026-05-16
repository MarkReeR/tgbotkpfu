package logger

import (
	"io"
	"log"
	"os"
)

// Logger простой логгер с уровнями
type Logger struct {
*log.Logger
level LogLevel
}

type LogLevel int

const (
DEBUG LogLevel = iota
INFO
WARN
ERROR
)

// New создает новый логгер
func New(w io.Writer, level LogLevel) *Logger {
return &Logger{
Logger: log.New(w, "", log.LstdFlags|log.Lmicroseconds),
level:  level,
}
}

func (l *Logger) Debug(msg string, args ...interface{}) {
if l.level <= DEBUG {
l.Printf("DEBUG "+msg, args...)
}
}

func (l *Logger) Info(msg string, args ...interface{}) {
if l.level <= INFO {
l.Printf("INFO "+msg, args...)
}
}

func (l *Logger) Warn(msg string, args ...interface{}) {
if l.level <= WARN {
l.Printf("WARN "+msg, args...)
}
}

func (l *Logger) Error(msg string, args ...interface{}) {
if l.level <= ERROR {
l.Printf("ERROR "+msg, args...)
}
}

// DefaultLogger возвращает логгер по умолчанию
func Default() *Logger {
return New(os.Stdout, INFO)
}

// ParseLevel парсит строку в уровень логирования
func ParseLevel(s string) LogLevel {
switch s {
case "debug":
return DEBUG
case "info":
return INFO
case "warn", "warning":
return WARN
case "error":
return ERROR
default:
return INFO
}
}

// Fatal фатальная ошибка
func (l *Logger) Fatal(msg string, args ...interface{}) {
l.Printf("FATAL "+msg, args...)
os.Exit(1)
}
