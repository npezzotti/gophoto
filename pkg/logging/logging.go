package logging

import (
	"fmt"
	"io"
	"log"
)

type Logger struct {
	base  *log.Logger
	debug bool
}

func NewLogger(out io.Writer, debug bool) *Logger {
	return &Logger{base: log.New(out, "", log.LstdFlags|log.Lshortfile), debug: debug}
}

func (l *Logger) Info(format string, args ...any) {
	l.logf("INFO", format, args...)
}

func (l *Logger) Warn(format string, args ...any) {
	l.logf("WARN", format, args...)
}

func (l *Logger) Error(format string, args ...any) {
	l.logf("ERROR", format, args...)
}

func (l *Logger) Debug(format string, args ...any) {
	if l.debug {
		l.logf("DEBUG", format, args...)
	}
}

func (l *Logger) logf(level string, format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	_ = l.base.Output(2, fmt.Sprintf("[%s] %s", level, msg))
}
