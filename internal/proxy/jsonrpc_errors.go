package proxy

const (
	JsonRpcErrInvalidRequest = -32600 
	JsonRpcErrMethodNotFound = -32601 
	JsonRpcErrInternal       = -32603
	JsonRpcErrServerGeneric  = -32000 
	JsonRpcErrUnauthorized   = -32001 // cutom for auth failures when auth is expected
	JsonRpcErrMethodBlocked  = -32004 // custom for disallowed methods
	JsonRpcErrRateLimit      = -32005 // custom for rate limit exceeded
)
