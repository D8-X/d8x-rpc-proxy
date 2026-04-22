package proxy

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/D8-X/d8x-rpc-proxy/internal/auth"
	"github.com/D8-X/d8x-rpc-proxy/internal/methodallowlist"
	"github.com/D8-X/d8x-rpc-proxy/internal/models"
	"github.com/D8-X/d8x-rpc-proxy/internal/ratelimit"
	"github.com/D8-X/globalrpc"
)

type Proxy struct {
	grpc        *globalrpc.GlobalRpc
	client      *http.Client
	privyAuth   *auth.PrivyVerifier
	rateLimiter *ratelimit.RateLimiter
	enforceMode models.EnforceMode // log rate limits or enforce
}

func New(
	grpc *globalrpc.GlobalRpc,
	appID string,
	rateLimit int,
	redisAddr, redisPw string,
	enforceMode models.EnforceMode,
) (*Proxy, error) {
	p, err := auth.NewPrivyVerifier(appID)
	if err != nil {
		return nil, err
	}
	rl, err := ratelimit.NewRateLimiter(redisAddr, redisPw, rateLimit)
	if err != nil {
		return nil, fmt.Errorf("unable to create ratelimiter: %v", err)
	}
	return &Proxy{
		grpc:        grpc,
		client:      &http.Client{Timeout: 30 * time.Second},
		privyAuth:   p,
		rateLimiter: rl,
		enforceMode: enforceMode,
	}, nil
}

func (p *Proxy) HandleRPC(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	userID := "anon"
	kind, token := auth.Classify(r.Header.Get("Authorization"))
	switch kind {
	case auth.AuthUser:
		var err error
		userID, err = p.privyAuth.Verify(token)
		if err != nil {
			msg := "authentication required"
			if errors.Is(err, auth.ErrTokenExpired) {
				msg = "token expired"
			}
			writeJSONRPCError(w, r, http.StatusUnauthorized, msg)
			return
		}
		slog.Info("user authenticated", "userID", userID)
	case auth.AuthNone:
		slog.Info("user request without authentication attempt")
		if p.enforceMode == models.Strict {
			writeJSONRPCError(w, r, http.StatusUnauthorized, "no authorization provided")
			return
		}
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, 2<<20)) // 2 MiB limit
	if err != nil {
		http.Error(w, "failed to read request body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	if !methodallowlist.Check(body) {
		writeJSONRPCError(w, r, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	if !p.rateLimiter.Allow(ctx, userID) {
		if p.enforceMode == models.Strict {
			w.Header().Set("Retry-After", "60")
			writeJSONRPCError(w, r, http.StatusTooManyRequests, "rate limit exceeded")
			return
		} else {
			slog.Info("rate limit exceeded (mode log)", "userID", userID)
		}
	}

	tried := make(map[string]struct{})
	poolSize := len(p.grpc.Config.Https)
	maxAttempts := max(poolSize, 3)
	var lastStatus int
	var lastBody []byte
	var lastUrl string

	for attempts := 0; attempts < maxAttempts; {
		if r.Context().Err() != nil {
			http.Error(w, "client canceled", http.StatusServiceUnavailable)
			return
		}

		getCtx, getCancel := context.WithTimeout(r.Context(), 10*time.Second)
		_, cleanup, upstreamUrl, err := globalrpc.RpcDial(getCtx, p.grpc, globalrpc.TypeHTTPS)
		getCancel()
		if err != nil {
			slog.Error("failed to get RPC endpoint", "attempt", attempts, "err", err)
			if lastStatus != 0 {
				respondWithLast(w, lastStatus, lastBody, tried)
				return
			}
			http.Error(w, "no RPC endpoint available", http.StatusServiceUnavailable)
			return
		}

		if _, seen := tried[upstreamUrl]; seen {
			cleanup()
			if poolSize > 0 && len(tried) >= poolSize {
				break
			}
			time.Sleep(100 * time.Millisecond)
			continue
		}
		tried[upstreamUrl] = struct{}{}
		attempts++

		status, respBody, retry := p.forward(r.Context(), upstreamUrl, body)
		cleanup()

		if !retry {
			w.Header().Set("X-RPC-Upstream", upstreamUrl)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(status)
			_, _ = w.Write(respBody)
			return
		}

		slog.Warn("upstream returned retryable status, trying another",
			"url", upstreamUrl, "status", status, "attempt", attempts)
		lastStatus, lastBody, lastUrl = status, respBody, upstreamUrl

		if attempts < maxAttempts {
			time.Sleep(100 * time.Millisecond)
		}
	}

	slog.Error("all RPC upstreams exhausted",
		"last_url", lastUrl,
		"last_status", lastStatus,
		"tried", triedList(tried))
	respondWithLast(w, lastStatus, lastBody, tried)
}

func respondWithLast(w http.ResponseWriter, status int, body []byte, tried map[string]struct{}) {
	w.Header().Set("X-RPC-Tried", triedList(tried))
	w.Header().Set("Content-Type", "application/json")
	if status == 0 {
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte(`{"error":"all upstream RPCs failed"}`))
		return
	}
	w.WriteHeader(status)
	if len(body) == 0 {
		_, _ = w.Write([]byte(`{"error":"all upstream RPCs failed"}`))
		return
	}
	_, _ = w.Write(body)
}

func triedList(tried map[string]struct{}) string {
	urls := make([]string, 0, len(tried))
	for u := range tried {
		urls = append(urls, u)
	}
	return strings.Join(urls, ",")
}

func (p *Proxy) forward(parent context.Context, url string, body []byte) (int, []byte, bool) {
	req, err := http.NewRequestWithContext(parent, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return 0, []byte(`{"error":"internal error"}`), false
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		slog.Error("upstream request failed", "url", url, "err", err)
		return http.StatusBadGateway, []byte(`{"error":"upstream transport error"}`), true
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	return resp.StatusCode, respBody, isRetryable(resp.StatusCode)
}

func isRetryable(status int) bool {
	switch status {
	case http.StatusTooManyRequests, // 429
		http.StatusInternalServerError, // 500
		http.StatusBadGateway,          // 502
		http.StatusServiceUnavailable,  // 503 
		http.StatusGatewayTimeout,      // 504
		http.StatusNotFound:            // 404 under load 
		return true
	}
	// not retryable ones like 403 or 401 
	return false
}

func HandleHealth(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok"))
}

func (p *Proxy) Run(listenAddr string) error {
	mux := http.NewServeMux()
	p.RegisterRoutes(mux)
	slog.Info("starting RPC proxy", "listen", listenAddr)
	return http.ListenAndServe(listenAddr, mux)
}

// writeJSONRPCError sends an error to the user of the form
// {"jsonrpc":"2.0","error":{"code":-32001,"message":"authentication required"},"id":null}
func writeJSONRPCError(w http.ResponseWriter, r *http.Request, statusCode int, message string) {
	var reqID json.RawMessage = []byte("null")
	if r.Body != nil {
		var req struct {
			ID json.RawMessage `json:"id"`
		}
		if body, readErr := io.ReadAll(io.LimitReader(r.Body, 2<<20)); readErr == nil {
			_ = r.Body.Close()
			if json.Unmarshal(body, &req) == nil && req.ID != nil {
				reqID = req.ID
			}
		}
	}

	resp := struct {
		JSONRPC string `json:"jsonrpc"`
		Error   struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
		ID json.RawMessage `json:"id"`
	}{
		JSONRPC: "2.0",
		Error: struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		}{Code: -32001, Message: message},
		ID: reqID,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	if b, marshalErr := json.Marshal(resp); marshalErr == nil {
		_, _ = w.Write(b)
	}
}
