// Package ratelimit is a simple
// rate limit package with fixed window limits
package ratelimit

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/redis/rueidis"
)

type RateLimiter struct {
	ruedi rueidis.Client
	limit int
}

func NewRateLimiter(redisAddr, redisPw string, limit int) (*RateLimiter, error) {
	var rl RateLimiter
	var err error
	rl.ruedi, err = rueidis.NewClient(
		rueidis.ClientOption{InitAddress: []string{redisAddr}, Password: redisPw},
	)
	if err != nil {
		return nil, fmt.Errorf("unable to create Redis instance: %v", err)
	}
	if limit <= 10 {
		return nil, fmt.Errorf("limit per minute unreasonable: %d", limit)
	}
	rl.limit = limit
	return &rl, nil
}


func (rl *RateLimiter) Allow(ctx context.Context, id string) (bool, int, int64) {
	now := time.Now().Unix()
	minute := now / 60
	resetUnix := (minute + 1) * 60
	key := fmt.Sprintf("rl:%s:%d", id, minute)
	results := rl.ruedi.DoMulti(ctx,
		rl.ruedi.B().Incr().Key(key).Build(),
		rl.ruedi.B().Expire().Key(key).Seconds(90).Build(),
	)
	count, err := results[0].AsInt64()
	if err != nil {
		slog.Error("Allow ratelimit failed: redis error", "error", err)
		return true, rl.limit, resetUnix
	}
	if err := results[1].Error(); err != nil {
		slog.Error("Allow ratelimit: expire", "id", id, "error", err)
	}
	remaining := max(rl.limit - int(count), 0)
	return count <= int64(rl.limit), remaining, resetUnix
}

func (rl *RateLimiter) Limit() int { return rl.limit }
