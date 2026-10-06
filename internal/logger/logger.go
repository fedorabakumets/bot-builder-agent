package logger

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	levelDebug = iota
	levelInfo
	levelWarn
	levelError
)

// Logger пишет в файл и в stderr.
type Logger struct {
	mu    sync.Mutex
	out   io.Writer
	file  *os.File
	level int
}

// New открывает файл лога. Пустой path оставляет только stderr.
func New(level, path string) (*Logger, error) {
	l := &Logger{level: parseLevel(level), out: os.Stderr}
	if strings.TrimSpace(path) == "" {
		return l, nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("каталог логов: %w", err)
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, fmt.Errorf("файл лога: %w", err)
	}
	l.file = f
	l.out = io.MultiWriter(os.Stderr, f)
	return l, nil
}

// Close закрывает файл лога.
func (l *Logger) Close() error {
	if l == nil || l.file == nil {
		return nil
	}
	return l.file.Close()
}

// Info пишет информационную строку.
func (l *Logger) Info(format string, args ...any) { l.write(levelInfo, "INFO", format, args...) }

// Warn пишет предупреждение.
func (l *Logger) Warn(format string, args ...any) { l.write(levelWarn, "WARN", format, args...) }

// Error пишет ошибку.
func (l *Logger) Error(format string, args ...any) { l.write(levelError, "ERROR", format, args...) }

func (l *Logger) write(lvl int, tag, format string, args ...any) {
	if l == nil || lvl < l.level {
		return
	}
	line := fmt.Sprintf("%s %s %s\n", time.Now().Format(time.RFC3339), tag, fmt.Sprintf(format, args...))
	l.mu.Lock()
	defer l.mu.Unlock()
	_, _ = io.WriteString(l.out, line)
}

func parseLevel(s string) int {
	switch strings.ToUpper(strings.TrimSpace(s)) {
	case "DEBUG":
		return levelDebug
	case "WARN", "WARNING":
		return levelWarn
	case "ERROR":
		return levelError
	default:
		return levelInfo
	}
}
