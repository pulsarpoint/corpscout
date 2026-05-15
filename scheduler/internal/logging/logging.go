package logging

import (
	"io"
	"log/slog"
	"os"
)

func ConfigureDefaultConsoleLogger() {
	slog.SetDefault(NewConsoleLogger(os.Stdout))
}

func NewConsoleLogger(output io.Writer) *slog.Logger {
	return slog.New(slog.NewJSONHandler(output, &slog.HandlerOptions{
		Level:     slog.LevelInfo,
		AddSource: true,
	}))
}
