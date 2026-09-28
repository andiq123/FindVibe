package utils

import (
	"log/slog"
	"os"
)

var logger *slog.Logger

func init() {
	if os.Getenv("ENV") == "production" {
		logger = slog.New(slog.NewJSONHandler(os.Stdout, nil))
	} else {
		logger = slog.New(slog.NewTextHandler(os.Stdout, nil))
	}
}

func GetLogger() *slog.Logger {
	return logger
}
