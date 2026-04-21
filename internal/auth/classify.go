package auth

import "strings"

type AuthKind int32

const (
	AuthNone AuthKind = iota
	AuthUser
)

// Classify parses the Authorization header
// Returns (kind, token) where token is the Bearer value (prefix stripped).
//
// Rules:
//   - empty or non-"Bearer " header       → AuthNone, ""
//   - Bearer token              → AuthUser, token
//
// Case-insensitive on "Bearer". Reject a Bearer with empty token as AuthNone.
func Classify(header string) (AuthKind, string) {
	if len(header) < 7 || !strings.EqualFold(header[:7], "bearer ") {
		return AuthNone, ""
	}
	tkn := strings.TrimSpace(header[7:])
	if tkn == "" {
		return AuthNone, ""
	}
	return AuthUser, tkn
}
