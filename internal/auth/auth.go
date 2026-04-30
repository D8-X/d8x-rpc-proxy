// Package auth handles authentication
package auth

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/MicahParks/keyfunc/v3"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/time/rate"
)

var (
	ErrTokenExpired   = errors.New("token expired")
	ErrTokenSignature = errors.New("token signature invalid")
	ErrTokenClaims    = errors.New("token claims invalid")
	ErrTokenMalformed = errors.New("token malformed")
)

type PrivyVerifier struct {
	appID string
	jwks  keyfunc.Keyfunc
}

// NewPrivyVerifier initializes a JWKS client pointing at Privy's per-app JWKS.
// JWKS URL: <https://auth.privy.io/api/v1/apps/><appID>/jwks.json
// Configure keyfunc with:
//   - RefreshInterval: 1 * time.Hour
//   - RefreshRateLimit: 5 * time.Minute
//   - RefreshTimeout: 10 * time.Second
//   - RefreshUnknownKID: true  (refresh on unknown `kid` header)
//
// The library caches the last known-good JWKS — if Privy's endpoint is
// unreachable, verification must continue using the cached keys, not fail.
func NewPrivyVerifier(appID string) (*PrivyVerifier, error) {
	if appID == "" {
		return nil, fmt.Errorf("empty app id")
	}
	jwksURL := "https://auth.privy.io/api/v1/apps/" + appID + "/jwks.json"

	override := keyfunc.Override{
		RefreshInterval:   1 * time.Hour,
		HTTPTimeout:       10 * time.Second,
		RateLimitWaitMax:  5 * time.Minute,
		RefreshUnknownKID: rate.NewLimiter(rate.Every(5*time.Minute), 1),
		RefreshErrorHandlerFunc: func(u string) func(ctx context.Context, err error) {
			return func(ctx context.Context, err error) {
				slog.Warn("JWKS refresh failed, using cached keys", "url", u, "err", err)
			}
		},
	}

	jwks, err := keyfunc.NewDefaultOverrideCtx(context.Background(), []string{jwksURL}, override)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize JWKS client: %w", err)
	}

	return &PrivyVerifier{appID: appID, jwks: jwks}, nil
}

// Verify parses the JWT, validates the signature against the JWKS, and
// enforces registered claims:
//   - iss == "privy.io"
//   - aud contains appID  (aud can be string or []string per RFC 7519)
//   - exp > now  (allow 30s leeway for clock skew)
//   - nbf <= now  (if present)
//
// Returns the `sub` claim as userID.
//
// Errors must be typed so the caller can distinguish:
//   - ErrTokenExpired       (triggers retry hint in permissive mode log)
//   - ErrTokenSignature     (wrong app, forged, or bad JWKS)
//   - ErrTokenClaims        (bad iss/aud)
//   - ErrTokenMalformed
func (v *PrivyVerifier) Verify(token string) (userID string, err error) {
	parsed, parseErr := jwt.ParseWithClaims(
		token,
		&jwt.RegisteredClaims{},
		v.jwks.Keyfunc,
		jwt.WithIssuer("privy.io"),
		jwt.WithAudience(v.appID),
		jwt.WithLeeway(30*time.Second),
		jwt.WithExpirationRequired(),
	)
	if parseErr != nil {
		switch {
		case errors.Is(parseErr, jwt.ErrTokenExpired):
			return "", ErrTokenExpired
		case errors.Is(parseErr, jwt.ErrTokenSignatureInvalid),
			errors.Is(parseErr, jwt.ErrTokenUnverifiable):
			return "", ErrTokenSignature
		case errors.Is(parseErr, jwt.ErrTokenInvalidClaims):
			return "", ErrTokenClaims
		default:
			return "", ErrTokenMalformed
		}
	}

	claims, ok := parsed.Claims.(*jwt.RegisteredClaims)
	if !ok || claims.Subject == "" {
		return "", ErrTokenClaims
	}

	return claims.Subject, nil
}
