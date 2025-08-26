package vega

import (
	"fmt"
	"net/http"

	"github.com/benidevo/vega/internal/ai"
	authapi "github.com/benidevo/vega/internal/api/auth"
	jobapi "github.com/benidevo/vega/internal/api/job"
	"github.com/benidevo/vega/internal/auth"
	"github.com/benidevo/vega/internal/common/middleware"
	"github.com/benidevo/vega/internal/common/render"
	"github.com/benidevo/vega/internal/documents"
	"github.com/benidevo/vega/internal/home"
	"github.com/benidevo/vega/internal/job"
	"github.com/benidevo/vega/internal/pages"
	"github.com/benidevo/vega/internal/quota"
	"github.com/benidevo/vega/internal/settings"
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
)

// SetupRoutes configures all application routes and middleware
func SetupRoutes(a *App) {
	a.router.Use(globalErrorHandler(a.renderer))

	if a.config.EnableSecurityHeaders {
		a.router.Use(middleware.SecurityHeaders())
	}

	if a.config.EnableCSRF {
		a.router.Use(middleware.CSRF(&a.config))
	}

	a.router.Static("/static", "./static")

	// health check
	a.router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	aiService, err := ai.Setup(&a.config)
	if err != nil {
		log.Warn().Err(err).Msg("AI service initialization failed, AI features will be disabled")
		aiService = nil
	}

	authHandler, authService := auth.SetupAuthWithService(a.db, &a.config)
	jobService := job.SetupService(a.db, &a.config, a.cache)
	jobHandler := job.NewJobHandler(jobService, &a.config)

	// Setup unified quota service
	jobRepo := job.SetupJobRepository(a.db, a.cache)
	quotaAdapter := quota.NewJobRepositoryAdapter(jobRepo)
	unifiedQuotaService := quota.NewUnifiedService(a.db, quotaAdapter, a.config.IsCloudMode)

	settingsHandler, _ := settings.SetupWithService(&a.config, a.db, aiService, unifiedQuotaService, authService)
	authAPIHandler := authapi.Setup(a.db, &a.config)
	jobAPIHandler := jobapi.Setup(a.db, &a.config, a.cache, unifiedQuotaService)

	homeHandler := home.Setup(a.db, &a.config, a.cache, jobService)

	// Setup document handler
	documentHandler := documents.Setup(a.db, &a.config, a.cache, a.renderer)

	authGroup := a.router.Group("/auth")

	// Register auth routes
	if !a.config.IsCloudMode {
		// In non-cloud mode, register all password-based auth routes
		auth.RegisterPublicRoutes(authGroup, authHandler)
	} else {
		// In cloud mode, only register the login page route which will redirect to Google OAuth
		authGroup.GET("/login", authHandler.GetLoginPage)
	}

	// Setup and register Google Auth routes only if enabled
	if a.config.GoogleOAuthEnabled {
		googleAuthHandler, err := auth.SetupGoogleAuth(&a.config, a.db)
		if err != nil {
			log.Error().Err(err).Msg("Failed to setup Google Auth")
		} else {
			auth.RegisterGoogleAuthRoutes(authGroup, googleAuthHandler)
		}
	}

	authGroup.Use(authHandler.AuthMiddleware())
	auth.RegisterPrivateRoutes(authGroup, authHandler)

	// Homepage route - register different handlers based on mode
	a.router.GET("/", authHandler.OptionalAuthMiddleware(), homeHandler.GetHomePage)

	// Dashboard route - always shows dashboard
	a.router.GET("/dashboard", authHandler.OptionalAuthMiddleware(), homeHandler.GetHomePage)

	jobGroup := a.router.Group("/jobs")
	jobGroup.Use(authHandler.AuthMiddleware())
	job.RegisterRoutes(jobGroup, jobHandler)

	settingsGroup := a.router.Group("/settings")
	settingsGroup.Use(authHandler.AuthMiddleware())
	settings.RegisterRoutes(settingsGroup, settingsHandler)

	// Register document routes
	csrfMiddleware := middleware.CSRF(&a.config)
	documents.RegisterRoutes(&a.router.RouterGroup, documentHandler, authHandler.AuthMiddleware(), csrfMiddleware)

	authAPIGroup := a.router.Group("/api/auth")
	authapi.RegisterRoutes(authAPIGroup, authAPIHandler)

	jobAPIGroup := a.router.Group("/api/jobs")
	jobAPIGroup.Use(authHandler.APIAuthMiddleware())
	jobapi.RegisterRoutes(jobAPIGroup, jobAPIHandler)

	pagesHandler := pages.NewHandler(&a.config)
	if a.config.IsCloudMode {
		a.router.GET("/privacy", pagesHandler.GetPrivacyPage)
		a.router.GET("/extension/download", pagesHandler.GetExtensionDownload)
	}

	a.router.NoRoute(func(c *gin.Context) {
		a.renderer.Error(c, http.StatusNotFound, "Page Not Found")
	})
}

// globalErrorHandler is a Gin middleware that recovers from panics and handles internal server errors
// by rendering a generic 500 error page.
//
// It ensures that any unhandled errors or panics result in a
// consistent error response to the client.
func globalErrorHandler(renderer *render.HTMLRenderer) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if r := recover(); r != nil {
				var err error
				switch v := r.(type) {
				case error:
					err = v
				case string:
					err = fmt.Errorf("%s", v)
				default:
					err = fmt.Errorf("%v", v)
				}
				log.Error().Err(err).Msg("Recovered from panic")

				renderer.Error(c, http.StatusInternalServerError, "Something Went Wrong")
				c.Abort()
			}
		}()

		c.Next()

		// Only handle errors if no response has been written yet
		if !c.Writer.Written() && (len(c.Errors) > 0 || c.Writer.Status() == http.StatusInternalServerError) {
			renderer.Error(c, http.StatusInternalServerError, "Something Went Wrong")
			c.Abort()
		}
	}
}
