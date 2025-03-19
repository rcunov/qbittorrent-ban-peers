package main

import (
	"errors"
	"log/slog"
	"os"
	"strings"
)

// maps peer information together
type peerInfo struct {
	Addr      string `json:"addr"`
	Hash      string `json:"hash"`
	Id        string `json:"id"`
	UserAgent string `json:"useragent"`
}

// NewJSONHandler creates a JSON-based slog handler with a configurable log level.
func NewJSONHandler(logLevel slog.Level) *slog.Logger {
	// logLevel := GetLogLevel()
	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: logLevel,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			if a.Key == slog.TimeKey {
				return slog.String(slog.TimeKey, a.Value.Time().UTC().Format("2006-01-02T15:04:05,999Z"))
			}
			return a
		},
	})
	return slog.New(handler)
}

// GetLogLevel retrieves the log level from the environment variable logLevel.
// Defaults to slog.LevelInfo if the variable is not set or invalid.
// Returns an error if logLevel is not a valid logging level.
func GetLogLevel() (slog.Level, error) {
	logLevelEnv := os.Getenv("logLevel")
	logLevelEnv = strings.ToLower(logLevelEnv)
	switch logLevelEnv {
	case "debug":
		return slog.LevelDebug, nil
	case "info":
		return slog.LevelInfo, nil
	case "error":
		return slog.LevelError, nil
	case "":
		return slog.LevelInfo, errors.New("log level not set. defaulting to info")
	default:
		return slog.LevelInfo, errors.New("log level " + logLevelEnv + " invalid. defaulting to info")
	}
}

var logger *slog.Logger

func InitializeLogging() {
	logLevel, err := GetLogLevel()
	logger = NewJSONHandler(logLevel)
	if err != nil {
		logger.Info(err.Error())
	}
}
