package auth

import (
	"errors"
	"log/slog"
	"net/http"
	"strings"

	httpresponse "github.com/benakaben10/sns/internal/http/response"
)

// Middleware returns an HTTP middleware that validates Bearer JWT tokens.
func Middleware(verifier Verifier, logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tokenString, err := extractBearerToken(r)
			if err != nil {
				httpresponse.WriteError(w, http.StatusUnauthorized, "unauthorized", "missing or invalid token")
				return
			}

			claims, err := verifier.Verify(r.Context(), tokenString)
			if err != nil {
				switch {
				case errors.Is(err, ErrExpiredToken):
					httpresponse.WriteError(w, http.StatusUnauthorized, "unauthorized", "token has expired")
				default:
					httpresponse.WriteError(w, http.StatusUnauthorized, "unauthorized", "missing or invalid token")
				}
				return
			}

			info := UserInfo{
				UserID:   claims.UserID(),
				Email:    claims.Email,
				Username: claims.PreferredUsername,
				Claims:   claims,
			}

			logger.Debug("authenticated request",
				"user_id", info.UserID,
				"username", info.Username,
			)

			ctx := withUserInfo(r.Context(), info)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequireAnyPermission returns middleware that allows the request if any of the permission
// check functions returns true. Returns 403 if none pass.
func RequireAnyPermission(permissions ...func(*Claims) bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			info, ok := GetUserInfo(r.Context())
			if !ok {
				httpresponse.WriteError(w, http.StatusUnauthorized, "unauthorized", "missing or invalid token")
				return
			}

			for _, check := range permissions {
				if check(info.Claims) {
					next.ServeHTTP(w, r)
					return
				}
			}

			httpresponse.WriteError(w, http.StatusForbidden, "forbidden", "permission denied")
		})
	}
}

func extractBearerToken(r *http.Request) (string, error) {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return "", ErrMissingToken
	}

	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return "", ErrInvalidToken
	}

	token := strings.TrimSpace(parts[1])
	if token == "" {
		return "", ErrInvalidToken
	}

	return token, nil
}
