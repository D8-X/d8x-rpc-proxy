package proxy

import "net/http"

func (p *Proxy) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/rpc", p.HandleRPC)
	mux.HandleFunc("/health", HandleHealth)
}
