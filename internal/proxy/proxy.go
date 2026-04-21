package proxy

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/D8-X/d8x-rpc-proxy/internal/auth"
	"github.com/D8-X/globalrpc"
)

type Proxy struct {
	grpc      *globalrpc.GlobalRpc
	client    *http.Client
	privyAuth *auth.PrivyVerifier
}

func New(grpc *globalrpc.GlobalRpc, appID string) (*Proxy, error) {
	p, err := auth.NewPrivyVerifier(appID)
	if err != nil {
		return nil, err
	}
	return &Proxy{
		grpc:      grpc,
		client:    &http.Client{Timeout: 30 * time.Second},
		privyAuth: p,
	}, nil
}

func (p *Proxy) HandleRPC(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	kind, token := auth.Classify(r.Header.Get("Authorization"))
	switch kind {
	case auth.AuthUser:
		userID, err := p.privyAuth.Verify(token)
		if err != nil {
			writeJSONRPCError(w, r, err)
			return
		}
		slog.Info("user authenticated", "userID", userID)
	case auth.AuthNone:
		slog.Info("user request without authentication attempt")
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, 2<<20)) // 2 MiB limit
	if err != nil {
		http.Error(w, "failed to read request body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	receipt, err := p.grpc.GetAndLockRpc(ctx, globalrpc.TypeHTTPS, 10)
	if err != nil {
		slog.Error("failed to get RPC endpoint", "err", err)
		http.Error(w, "no RPC endpoint available", http.StatusServiceUnavailable)
		return
	}
	defer p.grpc.ReturnLock(receipt)

	req, err := http.NewRequestWithContext(r.Context(), http.MethodPost, receipt.Url, bytes.NewReader(body))
	if err != nil {
		slog.Error("failed to build upstream request", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		slog.Error("upstream request failed", "url", receipt.Url, "err", err)
		http.Error(w, "upstream error", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(resp.StatusCode)
	if _, err := io.Copy(w, resp.Body); err != nil {
		slog.Error("failed to write response", "err", err)
	}
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
func writeJSONRPCError(w http.ResponseWriter, r *http.Request, err error) {
	slog.Info("user authorization failed", "error", err)

	var reqID json.RawMessage = []byte("null")
	if r.Body != nil {
		var req struct {
			ID json.RawMessage `json:"id"`
		}
		if body, readErr := io.ReadAll(io.LimitReader(r.Body, 2<<20)); readErr == nil {
			r.Body.Close()
			if json.Unmarshal(body, &req) == nil && req.ID != nil {
				reqID = req.ID
			}
		}
	}

	message := "authentication required"
	if errors.Is(err, auth.ErrTokenExpired) {
		message = "token expired"
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
		Error:   struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		}{Code: -32001, Message: message},
		ID: reqID,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	if b, marshalErr := json.Marshal(resp); marshalErr == nil {
		w.Write(b)
	}
}
