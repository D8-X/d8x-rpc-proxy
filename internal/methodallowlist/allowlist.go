package methodallowlist

import (
	"bytes"
	"encoding/json"
	"errors"
)

var allowed = map[string]struct{}{
	"eth_chainId":               {},
	"eth_blockNumber":           {},
	"eth_call":                  {},
	"eth_estimateGas":           {},
	"eth_gasPrice":              {},
	"eth_getBalance":            {},
	"eth_getTransactionCount":   {},
	"eth_getTransactionReceipt": {},
	"eth_getTransactionByHash":  {},
	"eth_getBlockByNumber":      {},
	"eth_getBlockByHash":        {},
	"eth_sendRawTransaction":    {},
	"net_version":               {},
	"web3_clientVersion":        {},
	"eth_maxPriorityFeePerGas":  {},
	"eth_feeHistory":            {},
}

// canonical is the only JSON-RPC 2.0 envelope shape we forward upstream.
// Anything else from the client is stripped before forwarding.
// Malformed bodies are denied.
type canonical struct {
	JSONRPC string          `json:"jsonrpc"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
	ID      json.RawMessage `json:"id,omitempty"`
}

// Check returns true if all methods in body are on the allowlist.
// Malformed bodies are denied
// Kept for backwards compat with our callers that don't need the sanitized output
func Check(body []byte) bool {
	_, ok := Sanitize(body)
	return ok
}

// Sanitize validates the JSON-RPC body and returns a reserialized version
// containing only the canonical fields
// Handles single and batch requests.
// Returns (nil, false) for malformed bodies, disallowed methods, or anything other than a JSON-RPC envelope.
func Sanitize(body []byte) ([]byte, bool) {
	trimmed := bytes.TrimSpace(body)
	if len(trimmed) == 0 {
		return nil, false
	}
	if trimmed[0] == '[' {
		var batch []json.RawMessage
		if err := json.Unmarshal(trimmed, &batch); err != nil || len(batch) == 0 {
			return nil, false
		}
		out := make([]canonical, 0, len(batch))
		for _, raw := range batch {
			c, err := sanitizeOne(raw)
			if err != nil {
				return nil, false
			}
			out = append(out, c)
		}
		buf, err := json.Marshal(out)
		if err != nil {
			return nil, false
		}
		return buf, true
	}
	c, err := sanitizeOne(trimmed)
	if err != nil {
		return nil, false
	}
	buf, err := json.Marshal(c)
	if err != nil {
		return nil, false
	}
	return buf, true
}

func sanitizeOne(raw json.RawMessage) (canonical, error) {
	var c canonical
	if err := json.Unmarshal(raw, &c); err != nil {
		return canonical{}, err
	}
	if c.Method == "" {
		return canonical{}, errors.New("missing method")
	}
	if _, ok := allowed[c.Method]; !ok {
		return canonical{}, errors.New("method not allowed")
	}
	if c.JSONRPC == "" {
		c.JSONRPC = "2.0"
	}
	return c, nil
}

type canonicalResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   json.RawMessage `json:"error,omitempty"`
	ID      json.RawMessage `json:"id"`
}

func SanitizeResponse(body []byte) []byte {
	trimmed := bytes.TrimSpace(body)
	if len(trimmed) == 0 {
		return body
	}
	if trimmed[0] == '[' {
		var batch []json.RawMessage
		if err := json.Unmarshal(trimmed, &batch); err != nil {
			return body
		}
		out := make([]canonicalResponse, 0, len(batch))
		for _, raw := range batch {
			c, err := sanitizeResponseOne(raw)
			if err != nil {
				return body
			}
			out = append(out, c)
		}
		buf, err := json.Marshal(out)
		if err != nil {
			return body
		}
		return buf
	}
	c, err := sanitizeResponseOne(trimmed)
	if err != nil {
		return body
	}
	buf, err := json.Marshal(c)
	if err != nil {
		return body
	}
	return buf
}

func sanitizeResponseOne(raw json.RawMessage) (canonicalResponse, error) {
	var c canonicalResponse
	if err := json.Unmarshal(raw, &c); err != nil {
		return canonicalResponse{}, err
	}
	if c.JSONRPC == "" {
		c.JSONRPC = "2.0"
	}
	return c, nil
}
