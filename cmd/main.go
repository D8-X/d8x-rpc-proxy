package main

import (
	"log/slog"
	"os"

	"github.com/D8-X/d8x-rpc-proxy/internal/svc"
)

func main() {
	cfg, err := svc.ConfigFromEnv()
	if err != nil {
		slog.Error("invalid configuration", "err", err)
		os.Exit(1)
	}

	if err := svc.Run(cfg); err != nil {
		slog.Error("server error", "err", err)
		os.Exit(1)
	}
}
