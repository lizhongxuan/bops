package logging

import (
	"encoding/json"
	"fmt"
	"io"
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

type Options struct {
	Level  string
	Format string
	Out    io.Writer
}

type Logger struct {
	mu     sync.Mutex
	level  Level
	format string
	out    io.Writer
}

type entry struct {
	TS     string         `json:"ts"`
	Level  string         `json:"level"`
	Msg    string         `json:"msg"`
	Fields map[string]any `json:"fields,omitempty"`
}

func New(opts Options) *Logger {
	lvl := parseLevel(opts.Level)
	format := strings.ToLower(strings.TrimSpace(opts.Format))
	if format == "" {
		format = "json"
	}
	out := opts.Out
	if out == nil {
		out = os.Stdout
	}

	return &Logger{
		level:  lvl,
		format: format,
		out:    out,
	}
}

func (l *Logger) Debug(msg string, fields map[string]any) {
	l.log(LevelDebug, msg, fields)
}

func (l *Logger) Info(msg string, fields map[string]any) {
	l.log(LevelInfo, msg, fields)
}

func (l *Logger) Warn(msg string, fields map[string]any) {
	l.log(LevelWarn, msg, fields)
}

func (l *Logger) Error(msg string, fields map[string]any) {
	l.log(LevelError, msg, fields)
}

func (l *Logger) log(level Level, msg string, fields map[string]any) {
	if level < l.level {
		return
	}

	ts := time.Now().UTC().Format(time.RFC3339Nano)
	levelStr := levelString(level)

	l.mu.Lock()
	defer l.mu.Unlock()

	if l.format == "text" {
		fmt.Fprintf(l.out, "%s %s %s", ts, levelStr, msg)
		for k, v := range fields {
			fmt.Fprintf(l.out, " %s=%v", k, v)
		}
		fmt.Fprintln(l.out)
		return
	}

	enc := json.NewEncoder(l.out)
	_ = enc.Encode(entry{
		TS:     ts,
		Level:  levelStr,
		Msg:    msg,
		Fields: fields,
	})
}

func parseLevel(raw string) Level {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "debug":
		return LevelDebug
	case "warn", "warning":
		return LevelWarn
	case "error":
		return LevelError
	default:
		return LevelInfo
	}
}

func levelString(level Level) string {
	switch level {
	case LevelDebug:
		return "debug"
	case LevelWarn:
		return "warn"
	case LevelError:
		return "error"
	default:
		return "info"
	}
}
