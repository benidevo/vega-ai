package home

import (
	"net/http"

	"github.com/benidevo/vega/internal/common/alerts"
	ctxutil "github.com/benidevo/vega/internal/common/context"
	"github.com/benidevo/vega/internal/common/render"
	"github.com/benidevo/vega/internal/config"
	"github.com/gin-gonic/gin"
)

// Handler manages home page related HTTP requests.
type Handler struct {
	cfg      *config.Settings
	service  *Service
	renderer *render.HTMLRenderer
}

// NewHandler creates and returns a new Handler.
func NewHandler(cfg *config.Settings, service *Service) *Handler {
	return &Handler{
		cfg:      cfg,
		service:  service,
		renderer: render.NewHTMLRenderer(cfg),
	}
}

// GetHomePage renders the home page template with dynamic user data.
// Note: Despite the name, this renders the dashboard (templates/home/index.html), not the landing page.
func (h *Handler) GetHomePage(c *gin.Context) {
	// In cloud mode, show landing page only for "/" route
	if h.cfg.IsCloudMode && c.Request.URL.Path == "/" {
		username, _ := c.Get("username")

		h.renderer.HTML(c, http.StatusOK, "landing/index.html", gin.H{
			"username": username,
		})
		return
	}

	// In self-hosted mode, show dashboard
	userIDValue, exists := c.Get("userID")
	if !exists {
		// Show dashboard with onboarding for non-authenticated users
		emptyHomeData := NewHomePageData(0, "")
		h.renderer.HTML(c, http.StatusOK, "layouts/base.html", gin.H{
			"title":              emptyHomeData.Title,
			"page":               emptyHomeData.Page,
			"activeNav":          "dashboard",
			"pageTitle":          "Dashboard",
			"googleOAuthEnabled": h.cfg.GoogleOAuthEnabled,
			"isCloudMode":        h.cfg.IsCloudMode,
			"showOnboarding":     emptyHomeData.ShowOnboarding,
			"stats":              emptyHomeData.Stats,
			"recentJobs":         emptyHomeData.RecentJobs,
			"TopMatches":         emptyHomeData.TopMatches,
			"hasJobs":            emptyHomeData.HasJobs,
			"quotaStatus":        emptyHomeData.QuotaStatus,
		})
		return
	}

	userID := userIDValue.(int)
	username, _ := c.Get("username")
	usernameStr := ""
	if username != nil {
		if str, ok := username.(string); ok {
			usernameStr = str
		}
	}

	ctx := c.Request.Context()
	if roleValue, exists := c.Get("role"); exists {
		if role, ok := roleValue.(string); ok {
			ctx = ctxutil.WithRole(ctx, role)
		}
	}

	homeData, err := h.service.GetHomePageData(ctx, userID, usernameStr)
	if err != nil {
		alerts.RenderError(c, http.StatusInternalServerError, "Failed to load homepage data", alerts.ContextGeneral)
		return
	}

	h.renderer.HTML(c, http.StatusOK, "layouts/base.html", gin.H{
		"title":          homeData.Title,
		"page":           homeData.Page,
		"activeNav":      "dashboard",
		"pageTitle":      "Dashboard",
		"username":       usernameStr,
		"stats":          homeData.Stats,
		"recentJobs":     homeData.RecentJobs,
		"TopMatches":     homeData.TopMatches,
		"hasJobs":        homeData.HasJobs,
		"showOnboarding": homeData.ShowOnboarding,
		"quotaStatus":    homeData.QuotaStatus,
	})
}
