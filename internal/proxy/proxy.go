package proxy

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/D8-X/globalrpc"
)

type Proxy struct {
	grpc   *globalrpc.GlobalRpc
	client *http.Client
}

func New(grpc *globalrpc.GlobalRpc) *Proxy {
	return &Proxy{
		grpc:   grpc,
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

func (p *Proxy) HandleRPC(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
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
