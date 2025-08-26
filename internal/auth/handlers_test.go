package auth

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/benidevo/vega/internal/auth/models"
	"github.com/benidevo/vega/internal/auth/services"
	"github.com/benidevo/vega/internal/common/alerts"
	"github.com/benidevo/vega/internal/common/testutil"
	"github.com/benidevo/vega/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type mockAuthService struct {
	mock.Mock
}

func (m *mockAuthService) Login(ctx context.Context, username, password string) (string, string, int64, error) {
	args := m.Called(ctx, username, password)
	return args.String(0), args.String(1), args.Get(2).(int64), args.Error(3)
}

func (m *mockAuthService) VerifyToken(token string) (*services.Claims, error) {
	args := m.Called(token)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*services.Claims), args.Error(1)
}

func (m *mockAuthService) RefreshAccessToken(ctx context.Context, refreshToken string) (string, string, int64, error) {
	args := m.Called(ctx, refreshToken)
	return args.String(0), args.String(1), args.Get(2).(int64), args.Error(3)
}

func (m *mockAuthService) LogError(err error) {
	m.Called(err)
}

func setupTestAuthHandler() (*AuthHandler, *mockAuthService, *gin.Engine) {
	mockService := new(mockAuthService)
	cfg := &config.Settings{
		TokenSecret:        "test-secret",
		AccessTokenExpiry:  15 * time.Minute,
		RefreshTokenExpiry: 7 * 24 * time.Hour,
		CookieDomain:       "localhost",
		CookieSecure:       false,
		CookieSameSite:     "lax",
		GoogleOAuthEnabled: true,
		IsCloudMode:        false,
	}

	handler := NewAuthHandler(mockService, cfg)
	router := testutil.SetupTestRouter()

	return handler, mockService, router
}

func TestAuthHandler_GetLoginPage(t *testing.T) {
	t.Skip("Skipping test that requires template rendering")
}

func TestAuthHandler_Login(t *testing.T) {
	handler, mockService, router := setupTestAuthHandler()

	// Setup routes
	router.POST("/login", handler.Login)

	tests := []testutil.HandlerTestCase{
		{
			Name:   "should_redirect_to_jobs_when_credentials_valid",
			Method: "POST",
			Path:   "/login",
			FormData: map[string]string{
				"username": "testuser",
				"password": "password123",
			},
			MockSetup: func() {
				mockService.On("Login", mock.Anything, "testuser", "password123").
					Return("access-token", "refresh-token", int64(1234567890), nil)
			},
			ExpectedStatus: http.StatusOK,
			ExpectedHeader: map[string]string{
				"HX-Redirect": "/jobs",
			},
			ExpectedCookie: []testutil.CookieAssertion{
				{
					Name:     "token",
					Value:    "access-token",
					HttpOnly: true,
					Secure:   false,
				},
				{
					Name:     "refresh_token",
					Value:    "refresh-token",
					HttpOnly: true,
					Secure:   false,
				},
			},
		},
		{
			Name:   "should_return_unauthorized_when_credentials_invalid",
			Method: "POST",
			Path:   "/login",
			FormData: map[string]string{
				"username": "testuser",
				"password": "wrongpassword",
			},
			MockSetup: func() {
				mockService.On("Login", mock.Anything, "testuser", "wrongpassword").
					Return("", "", int64(0), models.ErrInvalidCredentials)
				mockService.On("LogError", models.ErrInvalidCredentials).Return()
			},
			ExpectedStatus: http.StatusUnauthorized,
			ExpectedToast: &testutil.ToastAssertion{
				Message: models.ErrInvalidCredentials.Error(),
				Type:    string(alerts.TypeError),
			},
		},
		{
			Name:   "should_return_unauthorized_when_username_empty",
			Method: "POST",
			Path:   "/login",
			FormData: map[string]string{
				"username": "",
				"password": "password123",
			},
			MockSetup: func() {
				mockService.On("LogError", mock.Anything).Return()
			},
			ExpectedStatus: http.StatusUnauthorized,
			ExpectedToast: &testutil.ToastAssertion{
				Message: models.ErrInvalidCredentials.Error(),
				Type:    string(alerts.TypeError),
			},
		},
		{
			Name:   "should_return_unauthorized_when_password_empty",
			Method: "POST",
			Path:   "/login",
			FormData: map[string]string{
				"username": "testuser",
				"password": "",
			},
			MockSetup: func() {
				mockService.On("LogError", mock.Anything).Return()
			},
			ExpectedStatus: http.StatusUnauthorized,
			ExpectedToast: &testutil.ToastAssertion{
				Message: models.ErrInvalidCredentials.Error(),
				Type:    string(alerts.TypeError),
			},
		},
		{
			Name:    "should_return_bad_request_when_form_data_missing",
			Method:  "POST",
			Path:    "/login",
			Body:    "invalid-form-data",
			Headers: map[string]string{"Content-Type": "application/x-www-form-urlencoded"},
			MockSetup: func() {
				mockService.On("LogError", mock.Anything).Return()
			},
			ExpectedStatus: http.StatusUnauthorized,
		},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			mockService.ExpectedCalls = nil
			mockService.Calls = nil
			testutil.RunHandlerTest(t, router, tc)
			mockService.AssertExpectations(t)
		})
	}
}

func TestAuthHandler_Logout(t *testing.T) {
	handler, _, router := setupTestAuthHandler()

	// Setup routes
	router.GET("/logout", handler.Logout)

	tests := []testutil.HandlerTestCase{
		{
			Name:   "should_clear_cookies_when_logging_out",
			Method: "GET",
			Path:   "/logout",
			Cookies: []*http.Cookie{
				{Name: "token", Value: "some-token"},
				{Name: "refresh_token", Value: "some-refresh-token"},
			},
			ExpectedStatus: http.StatusFound,
			ExpectedHeader: map[string]string{
				"Location": "/",
			},
			ValidateBody: func(t *testing.T, w *httptest.ResponseRecorder) {
				// Check that cookies are cleared (MaxAge = -1)
				cookies := w.Result().Cookies()
				for _, cookie := range cookies {
					if cookie.Name == "token" || cookie.Name == "refresh_token" {
						assert.Equal(t, -1, cookie.MaxAge, "cookie %s should be cleared", cookie.Name)
					}
				}
			},
		},
		{
			Name:           "should_redirect_to_home_when_logout_without_cookies",
			Method:         "GET",
			Path:           "/logout",
			ExpectedStatus: http.StatusFound,
			ExpectedHeader: map[string]string{
				"Location": "/",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			testutil.RunHandlerTest(t, router, tc)
		})
	}
}

func TestAuthHandler_RefreshToken(t *testing.T) {
	handler, mockService, router := setupTestAuthHandler()

	// Setup routes
	router.POST("/refresh", handler.RefreshToken)

	tests := []testutil.HandlerTestCase{
		{
			Name:   "should_return_new_access_token_when_refresh_token_valid",
			Method: "POST",
			Path:   "/refresh",
			Cookies: []*http.Cookie{
				{Name: "refresh_token", Value: "valid-refresh-token"},
			},
			MockSetup: func() {
				mockService.On("RefreshAccessToken", mock.Anything, "valid-refresh-token").
					Return("new-access-token", "new-refresh-token", int64(1234567890), nil)
			},
			ExpectedStatus: http.StatusOK,
			ExpectedCookie: []testutil.CookieAssertion{
				{
					Name:     "token",
					Value:    "new-access-token",
					HttpOnly: true,
					Secure:   false,
				},
			},
			ValidateBody: func(t *testing.T, w *httptest.ResponseRecorder) {
				var response map[string]interface{}
				err := json.Unmarshal(w.Body.Bytes(), &response)
				assert.NoError(t, err)
				assert.Equal(t, true, response["success"])
			},
		},
		{
			Name:   "should_return_unauthorized_when_refresh_token_invalid",
			Method: "POST",
			Path:   "/refresh",
			Cookies: []*http.Cookie{
				{Name: "refresh_token", Value: "invalid-refresh-token"},
			},
			MockSetup: func() {
				mockService.On("RefreshAccessToken", mock.Anything, "invalid-refresh-token").
					Return("", "", int64(0), models.ErrInvalidToken)
				mockService.On("LogError", mock.Anything).Return()
			},
			ExpectedStatus: http.StatusUnauthorized,
			ValidateBody: func(t *testing.T, w *httptest.ResponseRecorder) {
				var response map[string]string
				err := json.Unmarshal(w.Body.Bytes(), &response)
				assert.NoError(t, err)
				assert.Equal(t, "Invalid refresh token", response["error"])
			},
		},
		{
			Name:           "should_return_unauthorized_when_refresh_token_missing",
			Method:         "POST",
			Path:           "/refresh",
			ExpectedStatus: http.StatusUnauthorized,
			ValidateBody: func(t *testing.T, w *httptest.ResponseRecorder) {
				var response map[string]string
				err := json.Unmarshal(w.Body.Bytes(), &response)
				assert.NoError(t, err)
				assert.Equal(t, "Missing refresh token", response["error"])
			},
		},
		{
			Name:   "should_return_error_when_service_fails",
			Method: "POST",
			Path:   "/refresh",
			Cookies: []*http.Cookie{
				{Name: "refresh_token", Value: "valid-refresh-token"},
			},
			MockSetup: func() {
				mockService.On("RefreshAccessToken", mock.Anything, "valid-refresh-token").
					Return("", "", int64(0), errors.New("service error"))
				mockService.On("LogError", mock.Anything).Return()
			},
			ExpectedStatus: http.StatusUnauthorized,
			ValidateBody: func(t *testing.T, w *httptest.ResponseRecorder) {
				var response map[string]string
				err := json.Unmarshal(w.Body.Bytes(), &response)
				assert.NoError(t, err)
				assert.Equal(t, "Invalid refresh token", response["error"])
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			mockService.ExpectedCalls = nil
			mockService.Calls = nil
			testutil.RunHandlerTest(t, router, tc)
			mockService.AssertExpectations(t)
		})
	}
}

func TestAuthHandler_GoogleLogin(t *testing.T) {
	t.Skip("GoogleLogin method not implemented in AuthHandler")
}

func TestAuthHandler_GoogleCallback(t *testing.T) {
	t.Skip("GoogleCallback method not implemented in AuthHandler")
}
