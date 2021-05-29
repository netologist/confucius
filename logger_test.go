package confucius

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"
	"time"
)

func Test_defaultLogger(t *testing.T) {
	l := defaultLogger()
	if l.level != DebugLevel {
		t.Fatal("default level should be DEBUG")
	}
	if l.output != io.Discard {
		t.Fatal("default writer should be io.Discard")
	}

	if !l.defaultCallback {
		t.Fatal("should set default callback")
	}
}

func Test_defaultCallback(t *testing.T) {
	old := os.Stdout // keep backup of the real stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	l := defaultLogger()
	l.callback = defaultCallback(os.Stdout)

	expected := fmt.Sprintf("%-7s %s message", DebugLevel, time.Now().Format("2006/01/02 15:04:05"))
	l.Debug("message")

	outC := make(chan string)
	// copy the output in a separate goroutine so printing can't block indefinitely
	go func() {
		var buf bytes.Buffer
		io.Copy(&buf, r)
		outC <- buf.String()
	}()

	// back to normal state
	w.Close()
	os.Stdout = old // restoring the real stdout
	actual := <-outC

	if !strings.Contains(actual, expected) {
		t.Fatalf("unexpected log entry: %+v", actual)
	}
}

func Test_defaultCallback_Panic(t *testing.T) {
	msg := "message"
	defer func() {
		if r := recover(); r != nil {
			if r != msg {
				t.Error("it should be throw panic")
			}
		}
	}()

	l := defaultLogger()
	l.callback = defaultCallback(os.Stdout)
	l.Panic(msg)
}

func Test_LogLevel_String(t *testing.T) {
	level := LogLevel(999)
	if level.String() != "UNKNOWN" {
		t.Error("unexpected level string")
	}
}

func Test_logger(t *testing.T) {
	levels := []LogLevel{
		DebugLevel,
		TraceLevel,
		InfoLevel,
		WarningLevel,
		ErrorLevel,
		PanicLevel,
		FatalLevel,
	}

	for _, level := range levels {
		t.Run(level.String(), func(t *testing.T) {
			l := defaultLogger()
			var actual LogLevel
			l.callback = func(level LogLevel, message, file string, line int) {
				actual = level
			}

			switch level {
			case DebugLevel:
				l.Debug("message")
			case TraceLevel:
				l.Trace("message")
			case InfoLevel:
				l.Info("message")
			case WarningLevel:
				l.Warn("message")
			case ErrorLevel:
				l.Error("message")
			case FatalLevel:
				l.Fatal("message")
			case PanicLevel:
				l.Panic("message")
			}

			if actual != level {
				t.Errorf("unexpected log entry: %+v", actual)
			}
		})
	}
}
