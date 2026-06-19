package auth

import (
	"context"
	"errors"
	"fmt"

	"github.com/MicahParks/keyfunc/v3"
	"github.com/golang-jwt/jwt/v5"
)

// JWKSVerifier verifies JWT tokens using a remote JWKS endpoint (Keycloak, Auth0, OIDC).
type JWKSVerifier struct {
	jwks keyfunc.Keyfunc
	cfg  verifyConfig
}

// NewJWKSVerifier creates a JWKS verifier that fetches and caches public keys from the given URL.
func NewJWKSVerifier(jwksURL string, cfg verifyConfig) (*JWKSVerifier, error) {
	jwks, err := keyfunc.NewDefaultCtx(context.Background(), []string{jwksURL})
	if err != nil {
		return nil, fmt.Errorf("initialize JWKS from %s: %w", jwksURL, err)
	}
	return &JWKSVerifier{jwks: jwks, cfg: cfg}, nil
}

func (v *JWKSVerifier) Verify(_ context.Context, tokenString string) (*Claims, error) {
	opts := []jwt.ParserOption{
		jwt.WithExpirationRequired(),
	}
	if v.cfg.Issuer != "" {
		opts = append(opts, jwt.WithIssuer(v.cfg.Issuer))
	}
	if v.cfg.Audience != "" {
		opts = append(opts, jwt.WithAudience(v.cfg.Audience))
	}

	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, v.jwks.Keyfunc, opts...)
	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, ErrExpiredToken
		}
		return nil, ErrInvalidToken
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, ErrInvalidToken
	}

	return claims, nil
}
