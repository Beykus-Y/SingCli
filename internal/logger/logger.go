package logger

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const maxEntries = 200

type Entry struct {
	Time    time.Time
	Level   string
	Message string
}

type Logger struct {
	mu      sync.Mutex
	entries []Entry
	path    string
}

var Default *Logger

func Init(exePath string) {
	dir := filepath.Dir(exePath)
	Default = &Logger{
		path: filepath.Join(dir, "singcli.log"),
	}
}

func (l *Logger) log(level, msg string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.entries = append(l.entries, Entry{
		Time:    time.Now(),
		Level:   level,
		Message: msg,
	})
	if len(l.entries) > maxEntries {
		l.entries = l.entries[len(l.entries)-maxEntries:]
	}
	l.flush()
}

func (l *Logger) flush() {
	f, err := os.Create(l.path)
	if err != nil {
		return
	}
	defer f.Close()

	for _, e := range l.entries {
		fmt.Fprintf(f, "%s [%s] %s\n", e.Time.Format("2006-01-02 15:04:05"), e.Level, e.Message)
	}
}

func Info(msg string) {
	if Default != nil {
		Default.log("INFO", msg)
	}
}

func Infof(format string, args ...interface{}) {
	Info(fmt.Sprintf(format, args...))
}

func Error(msg string) {
	if Default != nil {
		Default.log("ERROR", msg)
	}
}

func Errorf(format string, args ...interface{}) {
	Error(fmt.Sprintf(format, args...))
}

func Warn(msg string) {
	if Default != nil {
		Default.log("WARN", msg)
	}
}

func Warnf(format string, args ...interface{}) {
	Warn(fmt.Sprintf(format, args...))
}
