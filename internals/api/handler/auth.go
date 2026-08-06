package handler

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"net/http"

	"github.com/amirdaaee/Glide/internals/config"
	"github.com/amirdaaee/Glide/internals/domain"
	"github.com/amirdaaee/Glide/internals/repository"
	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/gin-gonic/gin"
	"golang.org/x/oauth2"
)

const (
	oauthStateCookie    = "oauth_state"
	oauthVerifierCookie = "oauth_verifier"
	oauthCookieMaxAge   = 600 // 10 minutes
	telegramOIDCIssuer  = "https://oauth.telegram.org"
)

type tgProfileClaims struct {
	ID                int64  `json:"id"`
	Name              string `json:"name"`
	GivenName         string `json:"given_name"`
	FamilyName        string `json:"family_name"`
	PreferredUsername string `json:"preferred_username"`
}

type AuthHandler struct {
	oauth2Config  *oauth2.Config
	verifier      *oidc.IDTokenVerifier
	userRepo      repository.IUserRepository
	secureCookies bool
}

var _ IApiHandler = (*AuthHandler)(nil)

func (h *AuthHandler) RegisterRoutes(router *gin.Engine) {
	auth := router.Group("/auth")
	{
		auth.GET("/login", h.Login)
		auth.GET("/callback", h.Callback)
	}
}

func (h *AuthHandler) Login(c *gin.Context) {
	state, err := randomString(32)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate state"})
		return
	}
	verifier := oauth2.GenerateVerifier()

	h.setOAuthCookie(c, oauthStateCookie, state)
	h.setOAuthCookie(c, oauthVerifierCookie, verifier)

	c.Redirect(http.StatusFound, h.oauth2Config.AuthCodeURL(state, oauth2.S256ChallengeOption(verifier)))
}

func (h *AuthHandler) Callback(c *gin.Context) {
	if errDesc := c.Query("error"); errDesc != "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":             errDesc,
			"error_description": c.Query("error_description"),
		})
		return
	}

	stateCookie, err := c.Cookie(oauthStateCookie)
	if err != nil || stateCookie == "" || stateCookie != c.Query("state") {
		h.clearOAuthCookies(c)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid OAuth state"})
		return
	}

	verifier, err := c.Cookie(oauthVerifierCookie)
	if err != nil || verifier == "" {
		h.clearOAuthCookies(c)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing PKCE verifier"})
		return
	}
	h.clearOAuthCookies(c)

	code := c.Query("code")
	if code == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing authorization code"})
		return
	}

	oauth2Token, err := h.oauth2Config.Exchange(c.Request.Context(), code, oauth2.VerifierOption(verifier))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to exchange token"})
		return
	}

	rawIDToken, ok := oauth2Token.Extra("id_token").(string)
	if !ok || rawIDToken == "" {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "No id_token field in oauth2 token"})
		return
	}

	idToken, err := h.verifier.Verify(c.Request.Context(), rawIDToken)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired id_token"})
		return
	}

	var claims tgProfileClaims
	if err := idToken.Claims(&claims); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse token claims"})
		return
	}
	if claims.ID == 0 {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Missing telegram user id in token claims"})
		return
	}

	user := &domain.User{
		TelegramID: claims.ID,
		Username:   claims.PreferredUsername,
		FirstName:  claims.GivenName,
		LastName:   claims.FamilyName,
	}
	if err := h.userRepo.UpsertByTelegramID(c.Request.Context(), user); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to upsert user"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Login successful",
		"token":   rawIDToken,
	})
}

func (h *AuthHandler) setOAuthCookie(c *gin.Context, name, value string) {
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(name, value, oauthCookieMaxAge, "/auth", "", h.secureCookies, true)
}

func (h *AuthHandler) clearOAuthCookies(c *gin.Context) {
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(oauthStateCookie, "", -1, "/auth", "", h.secureCookies, true)
	c.SetCookie(oauthVerifierCookie, "", -1, "/auth", "", h.secureCookies, true)
}
func NewAuthHandler(authCfg config.AuthConfigType, userRepo repository.IUserRepository) (*AuthHandler, error) {
	provider, err := oidc.NewProvider(context.Background(), telegramOIDCIssuer)
	if err != nil {
		return nil, fmt.Errorf("failed to create OIDC provider: %w", err)
	}

	return &AuthHandler{
		oauth2Config: &oauth2.Config{
			ClientID:     authCfg.ClientID,
			ClientSecret: authCfg.ClientSecret,
			RedirectURL:  authCfg.RedirectURL,
			Scopes:       []string{oidc.ScopeOpenID, "profile"},
			Endpoint: oauth2.Endpoint{
				AuthURL:  "https://oauth.telegram.org/auth",
				TokenURL: "https://oauth.telegram.org/token",
			},
		},
		verifier:      provider.Verifier(&oidc.Config{ClientID: authCfg.ClientID}),
		userRepo:      userRepo,
		secureCookies: authCfg.SecureCookies,
	}, nil
}
func randomString(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
