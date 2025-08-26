package vega

import (
	"context"
	"database/sql"
	"html/template"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/benidevo/vega/internal/auth"
	"github.com/benidevo/vega/internal/cache"
	"github.com/benidevo/vega/internal/common/logger"
	"github.com/benidevo/vega/internal/common/render"
	"github.com/benidevo/vega/internal/config"
	"github.com/benidevo/vega/internal/db"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
	_ "modernc.org/sqlite"
)

// App represents the core application structure, encapsulating configuration,
// HTTP router, database connection, cache, HTTP server, and a channel for handling OS signals.
type App struct {
	config   config.Settings
	router   *gin.Engine
	db       *sql.DB
	cache    cache.Cache
	server   *http.Server
	done     chan os.Signal
	renderer *render.HTMLRenderer
}

// loadTemplates walks the templates directory and loads all HTML files
func loadTemplates(router *gin.Engine) error {
	var files []string
	err := filepath.Walk("templates", func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && filepath.Ext(path) == ".html" {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		return err
	}

	if len(files) > 0 {
		router.LoadHTMLFiles(files...)
	}
	return nil
}

// New creates and returns a new instance of App with the provided configuration.
// It initializes the router using the Gin framework and sets up a channel to handle OS signals.
func New(cfg config.Settings) *App {
	router := gin.Default()

	corsConfig := cors.DefaultConfig()
	corsConfig.AllowOrigins = cfg.CORSAllowedOrigins
	corsConfig.AllowCredentials = cfg.CORSAllowCredentials
	corsConfig.AllowMethods = []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"}
	corsConfig.AllowHeaders = []string{"Origin", "Content-Type", "Accept", "Authorization"}

	router.Use(cors.New(corsConfig))

	if !cfg.IsTest {
		router.SetFuncMap(templateFuncMap())
		if err := loadTemplates(router); err != nil {
			log.Fatal().Err(err).Msg("Failed to load templates")
		}
	}

	return &App{
		config:   cfg,
		router:   router,
		done:     make(chan os.Signal, 1),
		renderer: render.NewHTMLRenderer(&cfg),
	}
}

// Setup initializes the application by setting up dependencies and routes.
func (a *App) Setup() error {
	logger.Initialize(
		a.config.IsDevelopment,
		a.config.LogLevel,
	)

	log.Info().Msg("Starting application setup")

	if err := a.setupDependencies(); err != nil {
		log.Error().Err(err).Msg("Failed to setup dependencies")
		return err
	}
	SetupRoutes(a)

	log.Info().Msg("Application setup completed successfully")
	return nil
}

// Run initializes the application, sets up the HTTP server, and starts it in a separate goroutine.
// It also listens for system interrupt signals to handle graceful shutdown.
func (a *App) Run() error {
	if err := a.Setup(); err != nil {
		return err
	}

	a.server = &http.Server{
		Addr:    a.config.ServerPort,
		Handler: a.router,
	}

	signal.Notify(a.done, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		log.Info().Str("port", a.config.ServerPort).Msg("Starting server")

		if !a.config.IsCloudMode {
			log.Info().Msgf("🚀 Vega AI is running at http://localhost%s", a.config.ServerPort)
			log.Info().Msg("📋 Default login: Username 'admin', Password 'VegaAdmin'")
			log.Info().Msgf("🔒 Change your password at http://localhost%s/settings/security", a.config.ServerPort)
		} else {
			log.Info().Str("port", a.config.ServerPort).Msg("🚀 Vega AI is running in cloud mode")
		}

		if err := a.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error().Err(err).Msg("Error starting server")
		}
	}()

	return nil
}

// WaitForShutdown waits for a shutdown signal, gracefully shuts down the server,
// It uses a context with a 10-second timeout
// to ensure the shutdown completes within the specified time.
func (a *App) WaitForShutdown() {
	<-a.done
	log.Info().Msg("Received shutdown signal, shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := a.Shutdown(ctx); err != nil {
		log.Error().Err(err).Msg("Error during shutdown")
	}

	log.Info().Msg("Server shut down gracefully")
}

// Shutdown gracefully shuts down the application by stopping the server
// and closing the database connection and cache.
func (a *App) Shutdown(ctx context.Context) error {
	var err error

	if a.server != nil {
		err = a.server.Shutdown(ctx)
	}

	if a.db != nil {
		dbErr := a.db.Close()
		if err == nil {
			err = dbErr
		}
	}

	if a.cache != nil {
		cacheErr := a.cache.Close()
		if err == nil {
			err = cacheErr
		}
	}

	a.server = nil
	a.db = nil
	a.cache = nil

	return err
}

func (a *App) setupDependencies() error {
	database, err := sql.Open(a.config.DBDriver, a.config.DBConnectionString)
	if err != nil {
		return err
	}

	database.SetMaxOpenConns(a.config.DBMaxOpenConns)
	database.SetMaxIdleConns(a.config.DBMaxIdleConns)
	database.SetConnMaxLifetime(a.config.DBConnMaxLifetime)

	if err := database.Ping(); err != nil {
		return err
	}
	a.db = database

	// Initialize cache
	if a.config.IsTest {
		// Use NoOpCache for tests to avoid file system operations
		a.cache = cache.NewNoOpCache()
	} else {
		cacheInstance, err := cache.NewBadgerCache(a.config.CachePath, a.config.CacheMaxMemoryMB)
		if err != nil {
			log.Warn().Err(err).Msg("Failed to initialize cache, using no-op cache")
			a.cache = cache.NewNoOpCache()
		} else {
			a.cache = cacheInstance
			log.Info().Str("path", a.config.CachePath).Int("maxMemoryMB", a.config.CacheMaxMemoryMB).Msg("Cache initialized successfully")
		}
	}

	if !a.config.IsTest {
		if err := a.runMigrations(); err != nil {
			return err
		}

		if err := auth.CreateAdminUserIfRequired(a.db, &a.config); err != nil {
			return err
		}
	}

	return nil
}

// runMigrations applies database migrations from the configured migrations directory
func (a *App) runMigrations() error {
	dbPath := a.config.DBConnectionString
	for i, char := range dbPath {
		if char == '?' {
			dbPath = dbPath[:i]
			break
		}
	}

	if err := db.MigrateDatabase(dbPath, a.config.MigrationsDir); err != nil {
		log.Error().Err(err).Msg("Database migration failed")
		return err
	}

	return nil
}

// templateFuncMap returns a map of custom template functions
func templateFuncMap() template.FuncMap {
	return template.FuncMap{
		"safeHTML": func(s string) template.HTML {
			return template.HTML(s)
		},
		"dict": func(values ...interface{}) (map[string]interface{}, error) {
			if len(values)%2 != 0 {
				return nil, nil
			}
			dict := make(map[string]interface{}, len(values)/2)
			for i := 0; i < len(values); i += 2 {
				key, ok := values[i].(string)
				if !ok {
					return nil, nil
				}
				dict[key] = values[i+1]
			}
			return dict, nil
		},
		"default": func(def interface{}, val interface{}) interface{} {
			if val == nil || val == "" {
				return def
			}
			return val
		},
		"add": func(a, b int) int {
			return a + b
		},
		"sub": func(a, b int) int {
			return a - b
		},
		"mul": func(a, b int) int {
			return a * b
		},
		"min": func(a, b int) int {
			if a < b {
				return a
			}
			return b
		},
		"max": func(a, b int) int {
			if a > b {
				return a
			}
			return b
		},
		"pageRange": func(current, total int) []int {
			// Show at most 5 page numbers around current page
			start := current - 2
			end := current + 2

			if start < 1 {
				start = 1
				end = start + 4
			}
			if end > total {
				end = total
				start = end - 4
			}
			if start < 1 {
				start = 1
			}

			pages := make([]int, 0)
			for i := start; i <= end; i++ {
				pages = append(pages, i)
			}
			return pages
		},
		"matchColors": func(score *int) string {
			if score == nil {
				return "bg-gray-500 bg-opacity-20 text-gray-400"
			}
			s := *score
			switch {
			case s >= 85:
				return "bg-green-600 bg-opacity-20 text-primary"
			case s >= 70:
				return "bg-green-500 bg-opacity-20 text-green-400"
			case s >= 55:
				return "bg-yellow-500 bg-opacity-20 text-yellow-400"
			case s >= 40:
				return "bg-orange-500 bg-opacity-20 text-orange-400"
			case s >= 25:
				return "bg-red-500 bg-opacity-20 text-red-400"
			default:
				return "bg-red-600 bg-opacity-20 text-red-400"
			}
		},
		"cappedQuota": func(used, limit int) int {
			// If limit is -1 (unlimited), return the actual usage
			if limit == -1 {
				return used
			}
			// Otherwise, cap the usage at the limit
			if used > limit {
				return limit
			}
			return used
		},
		"quotaProgressClass": func(remaining int) string {
			if remaining <= 0 {
				return "bg-red-500"
			} else if remaining <= 1 {
				return "bg-yellow-500"
			}
			return "bg-primary"
		},
	}
}
