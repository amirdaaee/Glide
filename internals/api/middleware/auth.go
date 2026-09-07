package middleware

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/gin-gonic/gin"
)

const (
	// AUTH_TGID_KEY is the Gin context key for the Telegram user ID.
	AUTH_TGID_KEY = "auth_tgid"
	// AUTH_TGNAME_KEY is the Gin context key for the Telegram display name.
	AUTH_TGNAME_KEY = "auth_tgname"
)

const telegramOIDCIssuer = "https://oauth.telegram.org"

// flexInt64 unmarshals Telegram claim IDs that may be JSON numbers or strings.
type flexInt64 int64

// UnmarshalJSON accepts a JSON number or string Telegram ID.
func (v *flexInt64) UnmarshalJSON(b []byte) error {
	if len(b) == 0 || string(b) == "null" {
		*v = 0
		return nil
	}
	if b[0] == '"' {
		var s string
		if err := json.Unmarshal(b, &s); err != nil {
			return err
		}
		n, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			return err
		}
		*v = flexInt64(n)
		return nil
	}
	var n int64
	if err := json.Unmarshal(b, &n); err != nil {
		return err
	}
	*v = flexInt64(n)
	return nil
}

// TgUserData is the Telegram user claims from an ID token.
type TgUserData struct {
	TgID   flexInt64 `json:"id"`
	TgName string    `json:"name"`
}

// TgAuthenticationMiddleware verifies Telegram OIDC bearer tokens.
type TgAuthenticationMiddleware struct {
	verifier *oidc.IDTokenVerifier
}

// MiddlewareFunc returns Gin middleware that requires a valid Telegram ID token.
func (m *TgAuthenticationMiddleware) MiddlewareFunc() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Authorization header missing"})
			return
		}

		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Invalid Authorization header format. Expected 'Bearer <token>'"})
			return
		}

		rawIDToken := parts[1]

		idToken, err := m.verifier.Verify(c.Request.Context(), rawIDToken)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired token"})
			return
		}
		var tgUser TgUserData
		if err := idToken.Claims(&tgUser); err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse token claims"})
			return
		}
		c.Set(AUTH_TGID_KEY, int64(tgUser.TgID))
		c.Set(AUTH_TGNAME_KEY, tgUser.TgName)

		c.Next()
	}
}

// JWTAuthenticationMiddlewareConfig configures Telegram OIDC token verification.
type JWTAuthenticationMiddlewareConfig struct {
	ClientID string
}

// NewJWTAuthenticationMiddleware returns Telegram OIDC auth middleware.
func NewJWTAuthenticationMiddleware(cfg *JWTAuthenticationMiddlewareConfig) (*TgAuthenticationMiddleware, error) {
	provider, err := oidc.NewProvider(context.Background(), telegramOIDCIssuer)
	if err != nil {
		return nil, fmt.Errorf("failed to create OIDC provider: %w", err)
	}
	verifier := provider.Verifier(&oidc.Config{ClientID: cfg.ClientID})
	return &TgAuthenticationMiddleware{verifier: verifier}, nil
}

// GetTgIDFromContext returns the authenticated Telegram user ID, or 0.
func GetTgIDFromContext(c *gin.Context) int64 {
	return c.GetInt64(AUTH_TGID_KEY)
}

// GetTgNameFromContext returns the authenticated Telegram display name.
func GetTgNameFromContext(c *gin.Context) string {
	return c.GetString(AUTH_TGNAME_KEY)
}
