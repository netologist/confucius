package confucius

import (
	"fmt"
	"io"
	"log"
	"runtime"
)

type LogCallback func(level LogLevel, message string, file string, line int)

type LogOption func(l *logger)

type LogLevel int

func (l LogLevel) String() string {
	switch l {
	case DebugLevel:
		return "DEBUG"
	case TraceLevel:
		return "TRACE"
	case InfoLevel:
		return "INFO"
	case WarningLevel:
		return "WARNING"
	case ErrorLevel:
		return "ERROR"
	case PanicLevel:
		return "PANIC"
	case FatalLevel:
		return "FATAL"
	}
	return "UNKNOWN"
}

const (
	DebugLevel LogLevel = iota
	TraceLevel
	InfoLevel
	WarningLevel
	ErrorLevel
	PanicLevel
	FatalLevel
)

func defaultLogger() *logger {
	return &logger{
		level:       DebugLevel,
		output:      io.Discard,
		useCallback: false,
		callback:    defaultCallback(io.Discard),
	}
}

func defaultCallback(output io.Writer) LogCallback {
	return func(level LogLevel, message string, file string, line int) {
		l := log.New(output, fmt.Sprintf("%-8s", level), log.LstdFlags)
		switch level {
		case PanicLevel:
			l.Panic(message)
		case FatalLevel:
			l.Fatal(message)
		default:
			l.Print(message)
		}
	}
}

func Callback(callback LogCallback) LogOption {
	return func(l *logger) {
		l.useCallback = true
		l.callback = callback
		l.output = io.Discard
	}
}

func SetLevel(level LogLevel) LogOption {
	return func(l *logger) {
		l.level = level
	}
}

func SetOutput(writer io.Writer) LogOption {
	return func(l *logger) {
		if !l.useCallback {
			l.output = writer
			l.callback = defaultCallback(writer)
		} else {
			l.Warn("log output feature is not usable when using callback")
		}
	}
}

type logger struct {
	useCallback bool
	callback    LogCallback
	level       LogLevel
	output      io.Writer
}

func (l *logger) Print(level LogLevel, message string, args ...interface{}) {
	if level < l.level {
		return
	}
	msg := fmt.Sprintf(message, args...)
	if _, file, line, ok := runtime.Caller(2); ok {
		l.callback(level, msg, file, line)
	} else {
		l.callback(level, msg, "n/a", -1)
	}
}

func (l *logger) Debug(message string, args ...interface{}) {
	l.Print(DebugLevel, message, args...)
}

func (l *logger) Trace(message string, args ...interface{}) {
	l.Print(TraceLevel, message, args...)
}

func (l *logger) Info(message string, args ...interface{}) {
	l.Print(InfoLevel, message, args...)
}

func (l *logger) Warn(message string, args ...interface{}) {
	l.Print(WarningLevel, message, args...)
}

func (l *logger) Error(message string, args ...interface{}) {
	l.Print(ErrorLevel, message, args...)
}

func (l *logger) Fatal(message string, args ...interface{}) {
	l.Print(FatalLevel, message, args...)
}

func (l *logger) Panic(message string, args ...interface{}) {
	l.Print(PanicLevel, message, args...)
}
