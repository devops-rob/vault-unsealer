package main

import (
	"context"
	"strings"

	logger "github.com/sirupsen/logrus"
)

func main() {
	cfg, err := newConfig()
	if err != nil {
		logger.Fatal(err)
	}

	setLogLevel(cfg.LogLevel)
	logger.SetFormatter(&logger.JSONFormatter{
		PrettyPrint: true,
	})

	provider, err := newKeyProvider(cfg.KeySource)
	if err != nil {
		logger.Fatal(err)
	}

	client, err := newVaultClient(cfg.TLS)
	if err != nil {
		logger.Fatal(err)
	}

	logger.Debug("Vault Unsealer starting...")

	monitorAndUnsealVaults(context.Background(), client, cfg.Nodes, provider, cfg.ProbeInterval)
}

func setLogLevel(level string) {
	switch strings.ToLower(level) {
	case "info":
		logger.SetLevel(logger.InfoLevel)
	case "warn":
		logger.SetLevel(logger.WarnLevel)
	case "error":
		logger.SetLevel(logger.ErrorLevel)
	case "fatal":
		logger.SetLevel(logger.FatalLevel)
	case "panic":
		logger.SetLevel(logger.PanicLevel)
	case "trace":
		logger.SetLevel(logger.TraceLevel)
	case "debug":
		logger.SetLevel(logger.DebugLevel)
	default:
		logger.SetLevel(logger.InfoLevel)
	}
}
