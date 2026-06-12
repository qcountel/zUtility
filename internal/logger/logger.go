// Package logger represents a set of utilities for loggers.
package logger

import (
	"io"
	"log/slog"
	"os"

	"github.com/Marlliton/slogpretty"
)

func NewPrettySlogger(out io.Writer, level slog.Level) *slog.Logger {
	if out == nil {
		out = os.Stdout
	}
	return slog.New(slogpretty.New(out, &slogpretty.Options{
		Level:      level,
		AddSource:  true,
		Colorful:   true,
		Multiline:  true,
		TimeFormat: slogpretty.DefaultTimeFormat,
	}))
}

func Level(debug bool) slog.Level {
	if debug {
		return slog.LevelDebug
	}
	return slog.LevelInfo
}
