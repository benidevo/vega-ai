package config

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type Settings struct {
	AppName            string
	ServerPort         string
	DBConnectionString string
	DBDriver           string
	LogLevel           string
	IsDevelopment      bool
	IsTest             bool
	MigrationsDir      string

	DBMaxOpenConns    int
	DBMaxIdleConns    int
	DBConnMaxLifetime time.Duration

	TokenSecret        string
	AccessTokenExpiry  time.Duration
	RefreshTokenExpiry time.Duration
	CookieDomain       string
	CookieSecure       bool
	CookieSameSite     string

	GoogleOAuthEnabled      bool
	GoogleClientID          string
	GoogleClientSecret      string
	GoogleClientRedirectURL string
	GoogleAuthUserInfoURL   string
	GoogleAuthUserInfoScope string

	CORSAllowedOrigins   []string
	CORSAllowCredentials bool

	CreateAdminUser    bool
	AdminUsername      string
	AdminPassword      string
	ResetAdminPassword bool

	AIProvider string

	OpenAIAPIKey           string // AI_KEY
	OpenAIBaseURL          string // OPENAI_BASE_URL — empty uses OpenAI default; set to http://localhost:11434/v1 for Ollama
	OpenAIModel            string // OPENAI_MODEL
	OpenAIModelCVParsing   string // OPENAI_MODEL_CV_PARSING
	OpenAIModelJobAnalysis string // OPENAI_MODEL_JOB_ANALYSIS
	OpenAIModelCoverLetter string // OPENAI_MODEL_COVER_LETTER

	// Cloud mode - enables multi-tenant deployment with OAuth-only authentication
	IsCloudMode bool

	// Cache settings
	CachePath        string
	CacheMaxMemoryMB int
	CacheDefaultTTL  time.Duration

	// Security settings
	EnableSecurityHeaders bool
	EnableCSRF            bool

	AIOperationTimeout time.Duration
}

func NewSettings() Settings {
	isDevelopment := getEnv("IS_DEVELOPMENT", "false") == "true"
	isTest := getEnv("GO_ENV", "") == "test"
	isCloudMode := getEnv("CLOUD_MODE", "false") == "true"
	aiProvider := getEnv("AI_PROVIDER", "gemini")

	accessTokenExpiry := 60 * time.Minute
	refreshTokenExpiry := 72 * time.Hour
	dbMaxOpenConns := 10
	dbMaxIdleConns := 5
	dbConnMaxLifetime := 5 * time.Minute

	if envVal := getEnv("ACCESS_TOKEN_EXPIRY", ""); envVal != "" {
		if mins, err := strconv.Atoi(envVal); err == nil {
			accessTokenExpiry = time.Duration(mins) * time.Minute
		}
	}

	if envVal := getEnv("REFRESH_TOKEN_EXPIRY", ""); envVal != "" {
		if hours, err := strconv.Atoi(envVal); err == nil {
			refreshTokenExpiry = time.Duration(hours) * time.Hour
		}
	}

	dbConnectionString := "/app/data/vega.db"
	if envDB := getEnv("DB_CONNECTION_STRING", ""); envDB != "" {
		dbConnectionString = envDB
	}
	if isTest && getEnv("DB_CONNECTION_STRING", "") == "" {
		dbConnectionString = ":memory:"
	}

	cookieSecure := !isDevelopment
	if envSecure := getEnv("COOKIE_SECURE", ""); envSecure != "" {
		cookieSecure = envSecure == "true"
	}

	var corsOrigins []string
	if envCORS := getEnv("CORS_ALLOWED_ORIGINS", ""); envCORS != "" {
		corsOrigins = strings.Split(envCORS, ",")
		for i, origin := range corsOrigins {
			corsOrigins[i] = strings.TrimSpace(origin)
		}
	} else {
		corsOrigins = []string{"*"}
	}

	return Settings{
		AppName:            "vega",
		ServerPort:         ":8765",
		DBConnectionString: dbConnectionString,
		DBDriver:           "sqlite",
		LogLevel:           getEnv("LOG_LEVEL", getDefaultLogLevel(isDevelopment)),
		IsDevelopment:      isDevelopment,
		TokenSecret:        getEnv("TOKEN_SECRET", "default-secret-key"),
		IsTest:             isTest,
		MigrationsDir:      "migrations/sqlite",

		DBMaxOpenConns:    dbMaxOpenConns,
		DBMaxIdleConns:    dbMaxIdleConns,
		DBConnMaxLifetime: dbConnMaxLifetime,

		AccessTokenExpiry:  accessTokenExpiry,
		RefreshTokenExpiry: refreshTokenExpiry,
		CookieDomain:       getEnv("COOKIE_DOMAIN", ""),
		CookieSecure:       cookieSecure,
		CookieSameSite:     "lax",

		GoogleOAuthEnabled:      isCloudMode, // Google OAuth is required in cloud mode
		GoogleClientID:          getEnv("GOOGLE_CLIENT_ID", ""),
		GoogleClientSecret:      getEnv("GOOGLE_CLIENT_SECRET", ""),
		GoogleClientRedirectURL: getEnv("GOOGLE_CLIENT_REDIRECT_URL", "http://localhost:8765/auth/google/callback"),
		GoogleAuthUserInfoURL:   "https://www.googleapis.com/oauth2/v3/userinfo",
		GoogleAuthUserInfoScope: getEnv("GOOGLE_AUTH_USER_INFO_SCOPE", "https://www.googleapis.com/auth/userinfo.email"),

		CORSAllowedOrigins:   corsOrigins,
		CORSAllowCredentials: false,

		// In self-hosted mode, always create admin user if it doesn't exist
		// In cloud mode, admin user creation is disabled
		CreateAdminUser:    !isCloudMode,
		AdminUsername:      getEnv("ADMIN_USERNAME", "admin"),
		AdminPassword:      getEnv("ADMIN_PASSWORD", "VegaAdmin"),
		ResetAdminPassword: getEnv("RESET_ADMIN_PASSWORD", "false") == "true",

		AIProvider:             aiProvider,
		OpenAIAPIKey:           getEnv("AI_KEY", getEnv("GEMINI_API_KEY", "")),
		OpenAIBaseURL:          getEnv("AI_BASE_URL", resolveDefaultBaseURL(aiProvider)),
		OpenAIModel:            getEnv("AI_MODEL", getEnv("GEMINI_MODEL", resolveDefaultModel(aiProvider))),
		OpenAIModelCVParsing:   getEnv("AI_MODEL_CV_PARSING", getEnv("GEMINI_MODEL_CV_PARSING", "")),
		OpenAIModelJobAnalysis: getEnv("AI_MODEL_JOB_ANALYSIS", getEnv("GEMINI_MODEL_JOB_ANALYSIS", "")),
		OpenAIModelCoverLetter: getEnv("AI_MODEL_COVER_LETTER", getEnv("GEMINI_MODEL_COVER_LETTER", "")),

		IsCloudMode: isCloudMode,

		CachePath:        getEnv("CACHE_PATH", "./data/cache"),
		CacheMaxMemoryMB: getCacheMaxMemoryMB(),
		CacheDefaultTTL:  getCacheDefaultTTL(),

		EnableSecurityHeaders: getEnv("ENABLE_SECURITY_HEADERS", "true") == "true",
		EnableCSRF:            getEnv("ENABLE_CSRF", "true") == "true",

		AIOperationTimeout: getAIOperationTimeout(),
	}
}

// NewTestSettings creates settings optimized for testing with in-memory database
func NewTestSettings() Settings {
	settings := NewSettings()
	settings.IsTest = true
	settings.DBConnectionString = ":memory:"
	settings.ServerPort = ":0" // Use random available port
	return settings
}

// NewTestSettingsWithTempDB creates settings for testing with a temporary file database
// This is useful for tests that need persistence or migration testing
// The caller is responsible for cleaning up the returned temp file path
func NewTestSettingsWithTempDB() (Settings, string) {
	tempDir := os.TempDir()
	tempFile := filepath.Join(tempDir, fmt.Sprintf("vega_test_%d.db", time.Now().UnixNano()))

	settings := NewSettings()
	settings.IsTest = true
	settings.DBConnectionString = tempFile + "?_journal_mode=WAL"
	settings.ServerPort = ":0"

	return settings, tempFile
}

const (
	// maxSecretFileSize is the maximum allowed size for secret files (1MB)
	maxSecretFileSize = 1024 * 1024
)

// getEnv reads environment variable with _FILE suffix support
// If KEY_FILE is set, it reads the content from that file
// Otherwise falls back to KEY, then to defaultValue
func getEnv(key string, defaultValue string) (value string) {
	if filePath := os.Getenv(key + "_FILE"); filePath != "" {
		if !filepath.IsAbs(filePath) {
			fmt.Fprintf(os.Stderr, "Warning: %s_FILE must be an absolute path, got %s\n", key, filePath)
		} else if strings.Contains(filePath, "..") {
			fmt.Fprintf(os.Stderr, "Warning: %s_FILE path contains '..', refusing to read %s\n", key, filePath)
		} else {
			fileInfo, err := os.Lstat(filePath)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Warning: Failed to check %s_FILE at %s: %v\n", key, filePath, err)
			} else if fileInfo.Mode()&os.ModeSymlink != 0 {
				fmt.Fprintf(os.Stderr, "Warning: %s_FILE at %s is a symlink, refusing to read for security\n", key, filePath)
			} else {
				file, err := os.Open(filePath)
				if err != nil {
					fmt.Fprintf(os.Stderr, "Warning: Failed to open %s_FILE at %s: %v\n", key, filePath, err)
				} else {
					defer file.Close()

					stat, err := file.Stat()
					if err != nil {
						fmt.Fprintf(os.Stderr, "Warning: Failed to stat %s_FILE at %s: %v\n", key, filePath, err)
					} else if stat.Size() > maxSecretFileSize {
						fmt.Fprintf(os.Stderr, "Warning: %s_FILE at %s is too large (%d bytes, max %d)\n", key, filePath, stat.Size(), maxSecretFileSize)
					} else {
						content := make([]byte, stat.Size())
						_, err = io.ReadFull(file, content)
						if err != nil {
							fmt.Fprintf(os.Stderr, "Warning: Failed to read %s_FILE from %s: %v\n", key, filePath, err)
						} else {
							return strings.TrimSpace(string(content))
						}
					}
				}
			}
		}
	}

	value = os.Getenv(key)
	if value == "" {
		value = defaultValue
	}
	return
}

// resolveDefaultBaseURL returns the default OpenAI-compatible base URL for the given provider.
// When provider is "gemini", points to Google's OpenAI-compatible endpoint so existing
// GEMINI_API_KEY users work without any config changes.
func resolveDefaultBaseURL(provider string) string {
	if provider == "gemini" {
		return "https://generativelanguage.googleapis.com/v1beta/openai/"
	}
	return "" // empty = official OpenAI endpoint
}

// resolveDefaultModel returns a sensible default model for the given provider.
func resolveDefaultModel(provider string) string {
	if provider == "gemini" {
		return "gemini-2.5-flash"
	}
	return "gpt-4o-mini"
}

func getDefaultLogLevel(isDevelopment bool) string {
	if isDevelopment {
		return "debug"
	}
	return "info"
}

func getCacheMaxMemoryMB() int {
	if envVal := getEnv("CACHE_MAX_MEMORY_MB", ""); envVal != "" {
		if mb, err := strconv.Atoi(envVal); err == nil {
			return mb
		}
	}
	return 256
}

func getCacheDefaultTTL() time.Duration {
	if envVal := getEnv("CACHE_DEFAULT_TTL", ""); envVal != "" {
		if duration, err := time.ParseDuration(envVal); err == nil {
			return duration
		}
	}
	return time.Hour
}

func getAIOperationTimeout() time.Duration {
	if envVal := getEnv("AI_OPERATION_TIMEOUT", ""); envVal != "" {
		if d, err := time.ParseDuration(envVal); err == nil && d > 0 {
			return d
		}
	}
	return 120 * time.Second
}
