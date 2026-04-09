package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewSettings(t *testing.T) {
	envVars := []string{
		"IS_DEVELOPMENT", "GO_ENV", "CLOUD_MODE",
		"DB_CONNECTION_STRING", "LOG_LEVEL", "ACCESS_TOKEN_EXPIRY", "REFRESH_TOKEN_EXPIRY",
		"GOOGLE_OAUTH_ENABLED", "CORS_ALLOWED_ORIGINS", "ENABLE_SECURITY_HEADERS",
		"AI_PROVIDER", "AI_KEY", "GEMINI_API_KEY", "CACHE_MAX_MEMORY_MB", "CACHE_DEFAULT_TTL",
	}
	for _, env := range envVars {
		os.Unsetenv(env)
	}

	tests := []struct {
		name     string
		setup    func()
		validate func(t *testing.T, s Settings)
	}{
		{
			name:  "should_use_default_values_when_no_env_vars",
			setup: func() {},
			validate: func(t *testing.T, s Settings) {
				assert.Equal(t, "vega", s.AppName)
				assert.Equal(t, ":8765", s.ServerPort)
				assert.Equal(t, "info", s.LogLevel)
				assert.False(t, s.IsDevelopment)
				assert.False(t, s.IsTest)
				assert.False(t, s.IsCloudMode)
				assert.Equal(t, 60*time.Minute, s.AccessTokenExpiry)
				assert.Equal(t, 72*time.Hour, s.RefreshTokenExpiry)
				assert.Equal(t, 10, s.DBMaxOpenConns)
				assert.Equal(t, 256, s.CacheMaxMemoryMB)
			},
		},
		{
			name: "should_set_development_mode_when_env_true",
			setup: func() {
				os.Setenv("IS_DEVELOPMENT", "true")
			},
			validate: func(t *testing.T, s Settings) {
				assert.True(t, s.IsDevelopment)
				assert.Equal(t, "debug", s.LogLevel)
			},
		},
		{
			name: "should_set_test_mode_when_go_env_test",
			setup: func() {
				os.Setenv("GO_ENV", "test")
			},
			validate: func(t *testing.T, s Settings) {
				assert.True(t, s.IsTest)
			},
		},
		{
			name: "should_set_cloud_mode_when_env_true",
			setup: func() {
				os.Setenv("CLOUD_MODE", "true")
			},
			validate: func(t *testing.T, s Settings) {
				assert.True(t, s.IsCloudMode)
			},
		},
		{
			name: "should_parse_access_token_expiry_from_env",
			setup: func() {
				os.Setenv("ACCESS_TOKEN_EXPIRY", "30")
			},
			validate: func(t *testing.T, s Settings) {
				assert.Equal(t, 30*time.Minute, s.AccessTokenExpiry)
			},
		},
		{
			name: "should_parse_refresh_token_expiry_from_env",
			setup: func() {
				os.Setenv("REFRESH_TOKEN_EXPIRY", "24")
			},
			validate: func(t *testing.T, s Settings) {
				assert.Equal(t, 24*time.Hour, s.RefreshTokenExpiry)
			},
		},
		{
			name: "should_parse_cors_origins_from_env",
			setup: func() {
				os.Setenv("CORS_ALLOWED_ORIGINS", "http://localhost:3000,https://example.com")
			},
			validate: func(t *testing.T, s Settings) {
				assert.Equal(t, []string{"http://localhost:3000", "https://example.com"}, s.CORSAllowedOrigins)
			},
		},
		{
			name: "should_parse_cache_settings_from_env",
			setup: func() {
				os.Setenv("CACHE_MAX_MEMORY_MB", "256")
				os.Setenv("CACHE_DEFAULT_TTL", "30m")
			},
			validate: func(t *testing.T, s Settings) {
				assert.Equal(t, 256, s.CacheMaxMemoryMB)
				assert.Equal(t, 30*time.Minute, s.CacheDefaultTTL)
			},
		},
		{
			name: "should_handle_invalid_numeric_values",
			setup: func() {
				os.Setenv("ACCESS_TOKEN_EXPIRY", "invalid")
				os.Setenv("CACHE_MAX_MEMORY_MB", "not-a-number")
			},
			validate: func(t *testing.T, s Settings) {
				assert.Equal(t, 60*time.Minute, s.AccessTokenExpiry)
				assert.Equal(t, 256, s.CacheMaxMemoryMB)
			},
		},
		{
			name: "should_set_google_oauth_settings",
			setup: func() {
				os.Setenv("CLOUD_MODE", "true") // Google OAuth is enabled in cloud mode
				os.Setenv("GOOGLE_CLIENT_ID", "test-client-id")
				os.Setenv("GOOGLE_CLIENT_SECRET", "test-secret")
			},
			validate: func(t *testing.T, s Settings) {
				assert.True(t, s.GoogleOAuthEnabled)
				assert.Equal(t, "test-client-id", s.GoogleClientID)
				assert.Equal(t, "test-secret", s.GoogleClientSecret)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, env := range envVars {
				os.Unsetenv(env)
			}

			tt.setup()
			settings := NewSettings()
			tt.validate(t, settings)

			for _, env := range envVars {
				os.Unsetenv(env)
			}
		})
	}
}

func TestNewTestSettings(t *testing.T) {
	settings := NewTestSettings()

	assert.Equal(t, ":0", settings.ServerPort)
	assert.True(t, settings.IsTest)
	assert.Equal(t, ":memory:", settings.DBConnectionString)
}

func TestNewTestSettingsWithTempDB(t *testing.T) {
	settings, tempFile := NewTestSettingsWithTempDB()

	assert.True(t, settings.IsTest)
	assert.Contains(t, settings.DBConnectionString, ".db")
	assert.Contains(t, settings.DBConnectionString, "_journal_mode=WAL")
	assert.NotEmpty(t, tempFile)
}

func TestGetDefaultLogLevel(t *testing.T) {
	tests := []struct {
		name          string
		isDevelopment bool
		expected      string
	}{
		{
			name:          "should_return_debug_when_development",
			isDevelopment: true,
			expected:      "debug",
		},
		{
			name:          "should_return_info_when_production",
			isDevelopment: false,
			expected:      "info",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getDefaultLogLevel(tt.isDevelopment)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetCacheMaxMemoryMB(t *testing.T) {
	tests := []struct {
		name     string
		envValue string
		expected int
	}{
		{
			name:     "should_return_default_when_no_env",
			envValue: "",
			expected: 256,
		},
		{
			name:     "should_parse_valid_value",
			envValue: "512",
			expected: 512,
		},
		{
			name:     "should_return_default_when_invalid",
			envValue: "not-a-number",
			expected: 256,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envValue != "" {
				os.Setenv("CACHE_MAX_MEMORY_MB", tt.envValue)
				defer os.Unsetenv("CACHE_MAX_MEMORY_MB")
			}

			result := getCacheMaxMemoryMB()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetCacheDefaultTTL(t *testing.T) {
	tests := []struct {
		name     string
		envValue string
		expected time.Duration
	}{
		{
			name:     "should_return_default_when_no_env",
			envValue: "",
			expected: 1 * time.Hour,
		},
		{
			name:     "should_parse_valid_duration",
			envValue: "30m",
			expected: 30 * time.Minute,
		},
		{
			name:     "should_return_default_when_invalid",
			envValue: "invalid",
			expected: 1 * time.Hour,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envValue != "" {
				os.Setenv("CACHE_DEFAULT_TTL", tt.envValue)
				defer os.Unsetenv("CACHE_DEFAULT_TTL")
			}

			result := getCacheDefaultTTL()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSettingsWithFileEnvVars(t *testing.T) {
	tempDir := t.TempDir()

	tests := []struct {
		name     string
		setup    func() string
		cleanup  func()
		envKey   string
		validate func(t *testing.T, settings Settings)
	}{
		{
			name: "should_read_secrets_from_files_when_file_env_vars_set",
			setup: func() string {
				tokenFile := filepath.Join(tempDir, "token-secret")
				require.NoError(t, os.WriteFile(tokenFile, []byte("file-based-token-secret"), 0600))

				apiKeyFile := filepath.Join(tempDir, "api-key")
				require.NoError(t, os.WriteFile(apiKeyFile, []byte("file-based-api-key"), 0600))

				os.Setenv("TOKEN_SECRET_FILE", tokenFile)
				os.Setenv("AI_KEY_FILE", apiKeyFile)

				return tokenFile
			},
			cleanup: func() {
				os.Unsetenv("TOKEN_SECRET_FILE")
				os.Unsetenv("AI_KEY_FILE")
			},
			validate: func(t *testing.T, settings Settings) {
				assert.Equal(t, "file-based-token-secret", settings.TokenSecret)
				assert.Equal(t, "file-based-api-key", settings.OpenAIAPIKey)
			},
		},
		{
			name: "should_ignore_relative_path_when_not_absolute",
			setup: func() string {
				os.Setenv("TOKEN_SECRET_FILE", "relative/path/token")
				os.Unsetenv("TOKEN_SECRET") // Clear any existing env var
				return ""
			},
			cleanup: func() {
				os.Unsetenv("TOKEN_SECRET_FILE")
			},
			validate: func(t *testing.T, settings Settings) {
				expectedSecret := getEnv("TOKEN_SECRET", "default-secret-key")
				assert.Equal(t, expectedSecret, settings.TokenSecret)
			},
		},
		{
			name: "should_ignore_path_with_parent_directory_when_contains_dotdot",
			setup: func() string {
				os.Setenv("TOKEN_SECRET_FILE", "/path/../token")
				return ""
			},
			cleanup: func() {
				os.Unsetenv("TOKEN_SECRET_FILE")
			},
			validate: func(t *testing.T, settings Settings) {
				expectedSecret := getEnv("TOKEN_SECRET", "default-secret-key")
				assert.Equal(t, expectedSecret, settings.TokenSecret)
			},
		},
		{
			name: "should_ignore_symlink_when_file_is_symlink",
			setup: func() string {
				realFile := filepath.Join(tempDir, "real-token")
				symlinkFile := filepath.Join(tempDir, "symlink-token")
				require.NoError(t, os.WriteFile(realFile, []byte("symlink-secret"), 0600))
				require.NoError(t, os.Symlink(realFile, symlinkFile))

				os.Setenv("TOKEN_SECRET_FILE", symlinkFile)
				return symlinkFile
			},
			cleanup: func() {
				os.Unsetenv("TOKEN_SECRET_FILE")
			},
			validate: func(t *testing.T, settings Settings) {
				expectedSecret := getEnv("TOKEN_SECRET", "default-secret-key")
				assert.Equal(t, expectedSecret, settings.TokenSecret)
			},
		},
		{
			name: "should_ignore_nonexistent_file_when_file_not_found",
			setup: func() string {
				os.Setenv("TOKEN_SECRET_FILE", filepath.Join(tempDir, "nonexistent"))
				return ""
			},
			cleanup: func() {
				os.Unsetenv("TOKEN_SECRET_FILE")
			},
			validate: func(t *testing.T, settings Settings) {
				expectedSecret := getEnv("TOKEN_SECRET", "default-secret-key")
				assert.Equal(t, expectedSecret, settings.TokenSecret)
			},
		},
		{
			name: "should_ignore_large_file_when_exceeds_max_size",
			setup: func() string {
				largeFile := filepath.Join(tempDir, "large-token")
				largeContent := make([]byte, 1024*1024+1)
				for i := range largeContent {
					largeContent[i] = 'A'
				}
				require.NoError(t, os.WriteFile(largeFile, largeContent, 0600))

				os.Setenv("TOKEN_SECRET_FILE", largeFile)
				return largeFile
			},
			cleanup: func() {
				os.Unsetenv("TOKEN_SECRET_FILE")
			},
			validate: func(t *testing.T, settings Settings) {
				expectedSecret := getEnv("TOKEN_SECRET", "default-secret-key")
				assert.Equal(t, expectedSecret, settings.TokenSecret)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setup()
			defer tt.cleanup()

			settings := NewSettings()
			tt.validate(t, settings)
		})
	}
}
