package proxy

const (
	JsonRpcErrInvalidRequest = -32600 // standard json-rpc code
	JsonRpcErrMethodNotFound = -32601 // standard json-rpc code
	JsonRpcErrInternal       = -32603 // standard json-rpc code
	JsonRpcErrServerGeneric  = -32000 // standard json-rpc code
	JsonRpcErrUnauthorized   = -32001 // custom for auth failures when auth is expected
	JsonRpcErrMethodBlocked  = -32004 // custom for disallowed methods
	JsonRpcErrRateLimit      = -32005 // custom for rate limit exceeded
)
