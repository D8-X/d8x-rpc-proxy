package methodallowlist

import (
	"bytes"
	"encoding/json"
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

// Check returns true if all methods in body are on the allowlist.
// Handles both single ({"method":...}) and batch ([{"method":...},...]) requests.
// Malformed bodies are denied.
func Check(body []byte) bool {
	trimmed := bytes.TrimSpace(body)
	if len(trimmed) == 0 {
		return false
	}
	if trimmed[0] == '[' {
		var batch []json.RawMessage
		if err := json.Unmarshal(trimmed, &batch); err != nil {
			return false
		}
		for _, raw := range batch {
			if !checkSingle(raw) {
				return false
			}
		}
		return true
	}
	return checkSingle(trimmed)
}

func checkSingle(raw json.RawMessage) bool {
	var req struct {
		Method string `json:"method"`
	}
	if err := json.Unmarshal(raw, &req); err != nil || req.Method == "" {
		return false
	}
	_, ok := allowed[req.Method]
	return ok
}
