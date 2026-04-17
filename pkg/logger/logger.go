package logger

import (
	"log/slog"
	"os"
)

func InitLogger() *slog.Logger {

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	
	slog.SetDefault(logger)
	
	return logger
}