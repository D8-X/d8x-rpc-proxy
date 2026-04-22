package svc

import (
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"

	"github.com/D8-X/globalrpc"

	"github.com/D8-X/d8x-rpc-proxy/internal/env"
	"github.com/D8-X/d8x-rpc-proxy/internal/models"
	"github.com/D8-X/d8x-rpc-proxy/internal/proxy"
)

type Config struct {
	ConfigFile    string
	RedisAddr     string
	RedisPassword string
	ChainID       int
	ListenAddr    string
	PrivyAppID    string
	Mode          models.EnforceMode
	RateLimit     int
}

func ConfigFromEnv() (Config, error) {
	envName := strings.ToLower(os.Getenv(env.Env))
	isTest := envName == "test" || envName == "dev" || envName == "local"
	mode := models.Strict
	if isTest {
		slog.Info("ENV=test/dev/local detected, enabling 'log' mode")
		mode = models.Log
	} else {
		slog.Info("PROD")
	}

	chainIDStr := os.Getenv(env.ChainID)
	if chainIDStr == "" {
		return Config{}, fmt.Errorf("%s is required", env.ChainID)
	}
	chainID, err := strconv.Atoi(chainIDStr)
	if err != nil {
		return Config{}, fmt.Errorf("%s must be an integer: %w", env.ChainID, err)
	}

	appID := os.Getenv(env.PrivyAppID)
	if !isTest && appID == "" {
		return Config{}, fmt.Errorf("%s is required in prod", env.PrivyAppID)
	}

	rli := 0
	if rl := os.Getenv(env.RateLimit); rl != "" {
		rli, err = strconv.Atoi(rl)
		if err != nil {
			return Config{}, fmt.Errorf("invalid %s: %s", env.RateLimit, rl)
		}
	} else if !isTest {
		return Config{}, fmt.Errorf("%s is required in prod", env.RateLimit)
	}

	cfg := Config{
		ConfigFile:    envOr(env.RPCConfigFile, "rpc-config.json"),
		RedisAddr:     envOr(env.RedisAddr, "localhost:6379"),
		RedisPassword: os.Getenv(env.RedisPassword),
		ChainID:       chainID,
		ListenAddr:    envOr(env.ListenAddr, ":8080"),
		PrivyAppID:    appID,
		Mode:          mode,
		RateLimit:     rli,
	}
	return cfg, nil
}

func Run(cfg Config) error {
	grpc, err := globalrpc.NewGlobalRpc(cfg.ChainID, cfg.ConfigFile, cfg.RedisAddr, cfg.RedisPassword)
	if err != nil {
		return fmt.Errorf("failed to initialize globalrpc: %w", err)
	}
	slog.Info("initialized globalrpc", "chainID", cfg.ChainID, "rateLimit", cfg.RateLimit, "enforceMode", cfg.Mode.String())
	p, err := proxy.New(grpc,
		cfg.PrivyAppID,
		cfg.RateLimit,
		cfg.RedisAddr,
		cfg.RedisPassword,
		cfg.Mode,
	)
	if err != nil {
		return err
	}
	return p.Run(cfg.ListenAddr)
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
