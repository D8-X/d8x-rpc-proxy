package svc

import (
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strconv"

	"github.com/D8-X/globalrpc"

	"github.com/D8-X/d8x-rpc-proxy/internal/env"
	"github.com/D8-X/d8x-rpc-proxy/internal/proxy"
)

type Config struct {
	ConfigFile    string
	RedisAddr     string
	RedisPassword string
	ChainID       int
	ListenAddr    string
}

func ConfigFromEnv() (Config, error) {
	chainIDStr := os.Getenv(env.ChainID)
	if chainIDStr == "" {
		return Config{}, fmt.Errorf("%s is required", env.ChainID)
	}
	chainID, err := strconv.Atoi(chainIDStr)
	if err != nil {
		return Config{}, fmt.Errorf("%s must be an integer: %w", env.ChainID, err)
	}

	cfg := Config{
		ConfigFile:    envOr(env.RPCConfigFile, "rpc-config.json"),
		RedisAddr:     envOr(env.RedisAddr, "localhost:6379"),
		RedisPassword: os.Getenv(env.RedisPassword),
		ChainID:       chainID,
		ListenAddr:    envOr(env.ListenAddr, ":8080"),
	}
	return cfg, nil
}

func Run(cfg Config) error {
	grpc, err := globalrpc.NewGlobalRpc(cfg.ChainID, cfg.ConfigFile, cfg.RedisAddr, cfg.RedisPassword)
	if err != nil {
		return fmt.Errorf("failed to initialize globalrpc: %w", err)
	}

	p := proxy.New(grpc)

	mux := http.NewServeMux()
	mux.HandleFunc("/rpc", p.HandleRPC)
	mux.HandleFunc("/health", proxy.HandleHealth)

	slog.Info("starting RPC proxy", "listen", cfg.ListenAddr, "chainID", cfg.ChainID)
	return http.ListenAndServe(cfg.ListenAddr, mux)
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
