package auth

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/benidevo/vega/internal/auth/models"
	"github.com/benidevo/vega/internal/auth/services"
	"github.com/benidevo/vega/internal/common/alerts"
	"github.com/benidevo/vega/internal/common/render"
	"github.com/benidevo/vega/internal/config"
	"github.com/gin-gonic/gin"
)

type authService interface {
	Login(ctx context.Context, username, password string) (string, string, int64, error)
	VerifyToken(token string) (*services.Claims, error)
	RefreshAccessToken(ctx context.Context, refreshToken string) (string, string, int64, error)
	LogError(err error)
}

// AuthHandler handles authentication-related HTTP requests.
type AuthHandler struct {
	service  authService
	cfg      *config.Settings
	renderer *render.HTMLRenderer
}

// NewAuthHandler creates and returns a new AuthHandler with the provided AuthService.
func NewAuthHandler(service authService, cfg *config.Settings) *AuthHandler {
	return &AuthHandler{
		service:  service,
		cfg:      cfg,
		renderer: render.NewHTMLRenderer(cfg),
	}
}

// GetLoginPage renders the login page template.
func (h *AuthHandler) GetLoginPage(c *gin.Context) {
	data := gin.H{
		"title":              "Login",
		"page":               "login",
		"googleOAuthEnabled": h.cfg.GoogleOAuthEnabled,
		"isCloudMode":        h.cfg.IsCloudMode,
	}

	h.renderer.HTML(c, http.StatusOK, "layouts/base.html", data)
}

// Login authenticates user credentials and sets JWT cookies on success
func (h *AuthHandler) Login(c *gin.Context) {
	var req models.LoginRequest
	if err := c.ShouldBind(&req); err != nil {
		h.service.LogError(err)
		c.Header("X-Toast-Message", models.ErrInvalidCredentials.Error())
		c.Header("X-Toast-Type", string(alerts.TypeError))
		alerts.TriggerToast(c, models.ErrInvalidCredentials.Error(), alerts.TypeError)
		c.String(http.StatusUnauthorized, "")
		return
	}

	accessToken, refreshToken, _, loginErr := h.service.Login(c.Request.Context(), req.Username, req.Password)
	if loginErr != nil {
		h.service.LogError(loginErr)
		c.Header("X-Toast-Message", loginErr.Error())
		c.Header("X-Toast-Type", string(alerts.TypeError))
		alerts.TriggerToast(c, loginErr.Error(), alerts.TypeError)
		c.String(http.StatusUnauthorized, "")
		return
	}

	sameSite := parseSameSiteMode(h.cfg.CookieSameSite)

	c.SetSameSite(sameSite)
	c.SetCookie(
		"token",
		accessToken,
		int(h.cfg.AccessTokenExpiry.Seconds()),
		"/",
		h.cfg.CookieDomain,
		h.cfg.CookieSecure,
		true,
	)

	c.SetCookie(
		"refresh_token",
		refreshToken,
		int(h.cfg.RefreshTokenExpiry.Seconds()),
		"/",
		h.cfg.CookieDomain,
		h.cfg.CookieSecure,
		true,
	)

	c.Header("HX-Redirect", "/jobs")
	c.Status(http.StatusOK)
}

// Logout logs out the current user by clearing the authentication cookies and redirecting to the home page.
func (h *AuthHandler) Logout(c *gin.Context) {
	h.clearAuthCookies(c)
	c.Redirect(http.StatusFound, "/")
}

// RefreshToken validates a refresh token and issues a new access token
func (h *AuthHandler) RefreshToken(c *gin.Context) {
	refreshToken, err := c.Cookie("refresh_token")
	if err != nil {
		h.clearAuthCookies(c)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Missing refresh token"})
		return
	}

	accessToken, _, _, err := h.service.RefreshAccessToken(c.Request.Context(), refreshToken)
	if err != nil {
		h.service.LogError(fmt.Errorf("refresh token validation failed: %w", err))
		h.clearAuthCookies(c)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid refresh token"})
		return
	}

	sameSite := parseSameSiteMode(h.cfg.CookieSameSite)

	c.SetSameSite(sameSite)
	c.SetCookie(
		"token",
		accessToken,
		int(h.cfg.AccessTokenExpiry.Seconds()),
		"/",
		h.cfg.CookieDomain,
		h.cfg.CookieSecure,
		true,
	)

	c.JSON(http.StatusOK, gin.H{"success": true})
}

// clearAccessTokenCookie clears only the access token cookie
func (h *AuthHandler) clearAccessTokenCookie(c *gin.Context) {
	sameSite := parseSameSiteMode(h.cfg.CookieSameSite)
	c.SetSameSite(sameSite)
	c.SetCookie("token", "", -1, "/", h.cfg.CookieDomain, h.cfg.CookieSecure, true)
}

// clearAuthCookies clears both access and refresh token cookies
func (h *AuthHandler) clearAuthCookies(c *gin.Context) {
	sameSite := parseSameSiteMode(h.cfg.CookieSameSite)
	c.SetSameSite(sameSite)
	c.SetCookie("token", "", -1, "/", h.cfg.CookieDomain, h.cfg.CookieSecure, true)
	c.SetCookie("refresh_token", "", -1, "/", h.cfg.CookieDomain, h.cfg.CookieSecure, true)
}

// authMiddleware is the core authentication logic shared by different middleware functions.
// It attempts to get and verify a token, refresh if needed, and populate context.
func (h *AuthHandler) authMiddleware(c *gin.Context) (*services.Claims, error) {
	tokenString, err := c.Cookie("token")

	if err != nil || tokenString == "" {
		refreshToken, refreshErr := c.Cookie("refresh_token")
		if refreshErr == nil && refreshToken != "" {
			newAccessToken, _, _, refreshErr := h.service.RefreshAccessToken(c.Request.Context(), refreshToken)
			if refreshErr == nil {
				sameSite := parseSameSiteMode(h.cfg.CookieSameSite)
				c.SetSameSite(sameSite)
				c.SetCookie(
					"token",
					newAccessToken,
					int(h.cfg.AccessTokenExpiry.Seconds()),
					"/",
					h.cfg.CookieDomain,
					h.cfg.CookieSecure,
					true,
				)
				tokenString = newAccessToken
			} else {
				// Refresh token is invalid/expired, clear all cookies
				h.service.LogError(fmt.Errorf("refresh token failed: %w", refreshErr))
				h.clearAuthCookies(c)
				return nil, fmt.Errorf("refresh token invalid")
			}
		}
	}

	if tokenString == "" {
		return nil, fmt.Errorf("no token found")
	}

	claims, err := h.service.VerifyToken(tokenString)
	if err != nil {
		// Only clear the access token, not the refresh token
		h.service.LogError(fmt.Errorf("access token verification failed: %w", err))
		h.clearAccessTokenCookie(c)

		// Try to refresh with the existing refresh token
		refreshToken, refreshErr := c.Cookie("refresh_token")
		if refreshErr == nil && refreshToken != "" {
			newAccessToken, _, _, refreshErr := h.service.RefreshAccessToken(c.Request.Context(), refreshToken)
			if refreshErr == nil {
				sameSite := parseSameSiteMode(h.cfg.CookieSameSite)
				c.SetSameSite(sameSite)
				c.SetCookie(
					"token",
					newAccessToken,
					int(h.cfg.AccessTokenExpiry.Seconds()),
					"/",
					h.cfg.CookieDomain,
					h.cfg.CookieSecure,
					true,
				)

				claims, err := h.service.VerifyToken(newAccessToken)
				if err != nil {
					h.service.LogError(fmt.Errorf("newly refreshed token verification failed: %w", err))
					h.clearAuthCookies(c)
					return nil, fmt.Errorf("token refresh failed")
				}
				c.Set("userID", claims.UserID)
				c.Set("username", claims.Username)
				c.Set("role", claims.Role)
				return claims, nil
			} else {
				h.service.LogError(fmt.Errorf("refresh token failed after access token failure: %w", refreshErr))
				h.clearAuthCookies(c)
				return nil, fmt.Errorf("both tokens invalid")
			}
		}
		return nil, fmt.Errorf("invalid token: %w", err)
	}

	c.Set("userID", claims.UserID)
	c.Set("username", claims.Username)
	c.Set("role", claims.Role)

	return claims, nil
}

// AuthMiddleware requires authentication and redirects to login if not authenticated.
func (h *AuthHandler) AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		_, err := h.authMiddleware(c)
		if err != nil {
			c.Redirect(http.StatusFound, "/auth/login")
			c.Abort()
			return
		}
		c.Next()
	}
}

// OptionalAuthMiddleware attempts authentication but continues regardless.
// Useful for pages accessible to both authenticated and unauthenticated users.
func (h *AuthHandler) OptionalAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		h.authMiddleware(c) // Ignore error
		c.Next()
	}
}

// GoogleAuthHandler handles authentication requests using Google OAuth.
type GoogleAuthHandler struct {
	service  *services.GoogleAuthService
	cfg      *config.Settings
	renderer *render.HTMLRenderer
}

// NewGoogleAuthHandler creates and returns a new GoogleAuthHandler with the provided service
func NewGoogleAuthHandler(service *services.GoogleAuthService, cfg *config.Settings) *GoogleAuthHandler {
	return &GoogleAuthHandler{
		service:  service,
		cfg:      cfg,
		renderer: render.NewHTMLRenderer(cfg),
	}
}

// HandleLogin initiates the Google OAuth login flow by redirecting the user to the authentication URL.
func (h *GoogleAuthHandler) HandleLogin(c *gin.Context) {
	authURL, state := h.service.GetAuthURL()

	// Store state in secure, httpOnly cookie for CSRF protection
	sameSite := parseSameSiteMode(h.cfg.CookieSameSite)
	c.SetSameSite(sameSite)
	c.SetCookie(
		"oauth_state",
		state,
		300, // 5 minutes expiry
		"/",
		h.cfg.CookieDomain,
		h.cfg.CookieSecure,
		true, // httpOnly
	)

	c.Redirect(http.StatusFound, authURL)
}

// HandleCallback processes the OAuth2 callback from Google, exchanges the code for a token,
// sets the authentication cookie, and redirects the user to the jobs page.
// For errors, redirects to login page with appropriate error messages.
func (h *GoogleAuthHandler) HandleCallback(c *gin.Context) {
	code := c.Query("code")
	state := c.Query("state")

	// Validate state parameter for CSRF protection
	storedState, err := c.Cookie("oauth_state")
	if err != nil || storedState == "" || storedState != state {
		h.service.LogError(fmt.Errorf("invalid oauth state: stored=%s, received=%s, error=%v", storedState, state, err))
		h.renderer.HTML(c, http.StatusBadRequest, "layouts/base.html", gin.H{
			"title":              "Login",
			"page":               "login",
			"googleOAuthEnabled": h.cfg.GoogleOAuthEnabled,
			"isCloudMode":        h.cfg.IsCloudMode,
			"error":              "Invalid authentication request. Please try again.",
		})
		return
	}

	// Clear state cookie after validation
	c.SetCookie("oauth_state", "", -1, "/", h.cfg.CookieDomain, h.cfg.CookieSecure, true)

	if code == "" {
		h.service.LogError(fmt.Errorf("missing code in callback"))
		// Redirect to login page with error message
		h.renderer.HTML(c, http.StatusBadRequest, "layouts/base.html", gin.H{
			"title":              "Login",
			"page":               "login",
			"googleOAuthEnabled": h.cfg.GoogleOAuthEnabled,
			"isCloudMode":        h.cfg.IsCloudMode,
			"error":              "Invalid authentication request. Please try again.",
		})
		return
	}

	accessToken, refreshToken, err := h.service.Authenticate(c.Request.Context(), code, "")
	if err != nil {
		h.service.LogError(err)

		errorMessage := "Authentication failed. Please try again later."

		h.renderer.HTML(c, http.StatusOK, "layouts/base.html", gin.H{
			"title":              "Login",
			"page":               "login",
			"googleOAuthEnabled": h.cfg.GoogleOAuthEnabled,
			"isCloudMode":        h.cfg.IsCloudMode,
			"error":              errorMessage,
		})
		return
	}

	sameSite := parseSameSiteMode(h.cfg.CookieSameSite)

	c.SetSameSite(sameSite)
	c.SetCookie(
		"token",
		accessToken,
		int(h.cfg.AccessTokenExpiry.Seconds()),
		"/",
		h.cfg.CookieDomain,
		h.cfg.CookieSecure,
		true,
	)

	c.SetCookie(
		"refresh_token",
		refreshToken,
		int(h.cfg.RefreshTokenExpiry.Seconds()),
		"/",
		h.cfg.CookieDomain,
		h.cfg.CookieSecure,
		true,
	)

	c.Redirect(http.StatusFound, "/jobs")
}

// Helper function to parse SameSite mode from string
func parseSameSiteMode(sameSiteStr string) http.SameSite {
	switch strings.ToLower(sameSiteStr) {
	case "strict":
		return http.SameSiteStrictMode
	case "none":
		return http.SameSiteNoneMode
	default:
		return http.SameSiteLaxMode
	}
}
