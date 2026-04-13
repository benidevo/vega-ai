package vega

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/benidevo/vega/internal/ai"
	authapi "github.com/benidevo/vega/internal/api/auth"
	jobapi "github.com/benidevo/vega/internal/api/job"
	"github.com/benidevo/vega/internal/auth"
	"github.com/benidevo/vega/internal/common/middleware"
	"github.com/benidevo/vega/internal/common/render"
	"github.com/benidevo/vega/internal/documents"
	"github.com/benidevo/vega/internal/home"
	"github.com/benidevo/vega/internal/job"
	localmiddleware "github.com/benidevo/vega/internal/middleware"
	"github.com/benidevo/vega/internal/pages"
	"github.com/benidevo/vega/internal/quota"
	"github.com/benidevo/vega/internal/settings"
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
)

// SetupRoutes configures all application routes and middleware
func SetupRoutes(a *App) {
	a.router.Use(localmiddleware.RequestID())
	a.router.Use(globalErrorHandler(a.renderer))
	a.router.Use(localmiddleware.RequestTimeout(a.config.AIOperationTimeout))

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

	jobHandler, err := job.NewJobHandler(jobService, &a.config)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to initialize job handler: missing or invalid templates")
	}

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

	// Always initialize rate limiter for auth endpoints (API + page routes)
	authLimiter := localmiddleware.NewAuthRateLimiter()
	a.authLimiter = authLimiter

	// Register auth routes
	if !a.config.IsCloudMode {
		auth.RegisterPublicRoutes(authGroup, authHandler, authLimiter)
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

	a.router.GET("/", authHandler.OptionalAuthMiddleware(), homeHandler.GetHomePage)
	a.router.GET("/dashboard", authHandler.OptionalAuthMiddleware(), homeHandler.GetHomePage)

	jobGroup := a.router.Group("/jobs")
	jobGroup.Use(authHandler.AuthMiddleware())
	job.RegisterRoutes(jobGroup, jobHandler)

	settingsGroup := a.router.Group("/settings")
	settingsGroup.Use(authHandler.AuthMiddleware())
	settings.RegisterRoutes(settingsGroup, settingsHandler)

	documents.RegisterRoutes(&a.router.RouterGroup, documentHandler, authHandler.AuthMiddleware())

	authAPIGroup := a.router.Group("/api/auth")
	authAPIGroup.Use(a.authLimiter.Middleware())
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

// globalErrorHandler recovers from panics and renders a 500 page for unhandled errors.
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

				// Try to render error page, fallback to minimal HTML if it fails (e.g. template issues)
				defer func() {
					if r2 := recover(); r2 != nil {
						log.Error().Interface("panic", r2).Msg("Panic during error rendering")
						if !c.Writer.Written() {
							accept := c.GetHeader("Accept")
							if strings.Contains(accept, "text/html") {
								c.Data(http.StatusInternalServerError, "text/html; charset=utf-8", []byte(`
									<!DOCTYPE html>
									<html>
									<head>
										<title>Something Went Wrong</title>
											<style>
											body { font-family: 'Plus Jakarta Sans', -apple-system, system-ui, sans-serif; background: #f9fafb; display: flex; align-items: center; justify-content: center; min-height: 100vh; margin: 0; color: #111827; }
											.card { background: white; padding: 3rem 2rem; border-radius: 1rem; box-shadow: 0 10px 15px -3px rgba(0, 0, 0, 0.1); border: 1px solid #f3f4f6; max-width: 28rem; text-align: center; }
											.icon { color: #0d9488; margin-bottom: 1.5rem; display: inline-block; }
											p { color: #4b5563; line-height: 1.6; margin-bottom: 2rem; font-size: 1.05rem; }
											a { display: inline-block; background: #0d9488; color: white; padding: 0.75rem 2rem; border-radius: 0.5rem; text-decoration: none; font-weight: 600; transition: all 0.2s; }
											a:hover { background: #0f766e; transform: translateY(-1px); }
										</style>
									</head>
									<body>
										<div class="card">
											<div class="icon">
												<svg xmlns="http://www.w3.org/2000/svg" width="80" height="80" fill="none" viewBox="0 0 24 24" stroke="currentColor">
													<path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
												</svg>
											</div>
											<p style="font-size: 1.25rem; color: #111827; margin-bottom: 2rem;">Something went wrong. Please try again later.</p>
											<a href="/">Go Home</a>
										</div>
									</body>
									</html>
								`))
							} else {
								c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal Server Error"})
							}
						}
					}
				}()

				if !c.Writer.Written() {
					renderer.Error(c, http.StatusInternalServerError, "Something Went Wrong")
				}
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
