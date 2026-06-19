package auth

import (
	"context"
	"fmt"

	"github.com/benakaben10/sns/internal/config"
)

// Verifier validates JWT tokens and returns parsed claims.
type Verifier interface {
	Verify(ctx context.Context, tokenString string) (*Claims, error)
}

type verifyConfig struct {
	Issuer   string
	Audience string
}

// NewVerifier creates the appropriate Verifier based on JWT configuration.
func NewVerifier(cfg config.JWTConfig) (Verifier, error) {
	vc := verifyConfig{
		Issuer:   cfg.Issuer,
		Audience: cfg.Audience,
	}

	switch cfg.AuthMode {
	case "symmetric":
		return NewHMACVerifier(cfg.HMACSecret, vc), nil
	case "jwks":
		return NewJWKSVerifier(cfg.JWKSURL, vc)
	default:
		return nil, fmt.Errorf("unsupported auth mode: %s", cfg.AuthMode)
	}
}
