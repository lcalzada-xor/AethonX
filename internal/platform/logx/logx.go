// internal/platform/logx/log.go
package logx

import (
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"
)

type Level int

const (
	LevelDebug Level = iota
	LevelInfo
	LevelWarn
	LevelError
)

type Logger interface {
	Debug(msg string, kv ...any)
	Info(msg string, kv ...any)
	Warn(msg string, kv ...any)
	Err(err error, kv ...any)
	With(kv ...any) Logger
	SetLevel(lvl Level)
}

type simpleLogger struct {
	mu    sync.Mutex
	lvl   Level
	scope []string // pares key=value fijos
	lg    *log.Logger
}

func New() Logger {
	l := &simpleLogger{
		lvl: parseLevel(os.Getenv("AETHONX_LOG_LEVEL")),
		lg:  log.New(os.Stderr, "", 0),
	}
	return l
}

// NewWithLevel creates a logger with a specific log level
func NewWithLevel(lvl Level) Logger {
	l := &simpleLogger{
		lvl: lvl,
		lg:  log.New(os.Stderr, "", 0),
	}
	return l
}

// NewSilent creates a logger that only outputs errors (silent mode for UI)
func NewSilent() Logger {
	return NewWithLevel(LevelError)
}

func (s *simpleLogger) With(kv ...any) Logger {
	clone := *s
	clone.scope = append(append([]string{}, s.scope...), kvPairs(kv...)...)
	return &clone
}

func (s *simpleLogger) SetLevel(lvl Level) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lvl = lvl
}

func (s *simpleLogger) Debug(msg string, kv ...any) { s.log(LevelDebug, "DBG", msg, kv...) }
func (s *simpleLogger) Info(msg string, kv ...any)  { s.log(LevelInfo, "INF", msg, kv...) }
func (s *simpleLogger) Warn(msg string, kv ...any)  { s.log(LevelWarn, "WRN", msg, kv...) }
func (s *simpleLogger) Err(err error, kv ...any) {
	if err == nil {
		return
	}
	kv = append([]any{"error", err.Error()}, kv...)
	s.log(LevelError, "ERR", "", kv...)
}

func (s *simpleLogger) log(l Level, tag, msg string, kv ...any) {
	if l < s.lvl {
		return
	}
	ts := time.Now().Format("15:04:05")
	fields := append([]string{}, s.scope...)
	fields = append(fields, kvPairs(kv...)...)
	line := fmt.Sprintf("%s %s %s", ts, tag, msg)
	if len(strings.TrimSpace(msg)) == 0 && len(fields) > 0 {
		// si no hay msg y solo campos (e.g., Err), evita doble espacio
		line = fmt.Sprintf("%s %s", ts, tag)
	}
	if len(fields) > 0 {
		line = fmt.Sprintf("%s %s", line, strings.Join(fields, " "))
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lg.Println(line)
}

func kvPairs(kv ...any) []string {
	out := make([]string, 0, len(kv))
	for i := 0; i < len(kv); i += 2 {
		var k, v any
		k = kv[i]
		if i+1 < len(kv) {
			v = kv[i+1]
		} else {
			v = "(missing)"
		}
		out = append(out, fmt.Sprintf("%v=%v", k, v))
	}
	return out
}

func parseLevel(s string) Level {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "debug", "dbg":
		return LevelDebug
	case "info", "inf", "":
		return LevelInfo
	case "warn", "warning":
		return LevelWarn
	case "err", "error":
		return LevelError
	default:
		return LevelInfo
	}
}
