package auth

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/benidevo/vega/internal/auth/services"
	"github.com/benidevo/vega/internal/common/testutil"
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

func (m *mockAuthService) RefreshAccessToken(ctx context.Context, refreshToken string) (string, string, int64, error) {
	args := m.Called(ctx, refreshToken)
	return args.String(0), args.String(1), args.Get(2).(int64), args.Error(3)
}

func (m *mockAuthService) VerifyToken(token string) (*services.Claims, error) {
	args := m.Called(token)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*services.Claims), args.Error(1)
}

func setupTestAuthAPIHandler() (*AuthAPIHandler, *mockAuthService, *gin.Engine) {
	mockAuth := new(mockAuthService)

	handler := NewAuthAPIHandler(mockAuth)
	router := testutil.SetupTestRouter()

	return handler, mockAuth, router
}

func TestAuthAPIHandler_RefreshToken(t *testing.T) {
	handler, mockAuth, router := setupTestAuthAPIHandler()

	// Setup routes
	router.POST("/api/auth/refresh", handler.RefreshToken)

	tests := []testutil.HandlerTestCase{
		{
			Name:   "should_refresh_token_when_valid_refresh_token",
			Method: "POST",
			Path:   "/api/auth/refresh",
			Body: map[string]string{
				"refresh_token": "valid-refresh-token",
			},
			Headers: map[string]string{
				"Content-Type": "application/json",
			},
			MockSetup: func() {
				mockAuth.On("RefreshAccessToken", mock.Anything, "valid-refresh-token").
					Return("new-access-token", "new-refresh-token", int64(1234567890), nil)
			},
			ExpectedStatus: http.StatusOK,
			ValidateBody: func(t *testing.T, w *httptest.ResponseRecorder) {
				var response map[string]interface{}
				err := json.Unmarshal(w.Body.Bytes(), &response)
				assert.NoError(t, err)
				assert.Equal(t, "new-access-token", response["token"])
				assert.Equal(t, "new-refresh-token", response["refresh_token"])
				assert.Equal(t, float64(1234567890), response["expires_at"])
			},
		},
		{
			Name:   "should_return_unauthorized_when_invalid_refresh_token",
			Method: "POST",
			Path:   "/api/auth/refresh",
			Body: map[string]string{
				"refresh_token": "invalid-refresh-token",
			},
			Headers: map[string]string{
				"Content-Type": "application/json",
			},
			MockSetup: func() {
				mockAuth.On("RefreshAccessToken", mock.Anything, "invalid-refresh-token").
					Return("", "", int64(0), errors.New("invalid token"))
			},
			ExpectedStatus: http.StatusUnauthorized,
			ValidateBody: func(t *testing.T, w *httptest.ResponseRecorder) {
				var response map[string]string
				err := json.Unmarshal(w.Body.Bytes(), &response)
				assert.NoError(t, err)
				assert.Equal(t, "failed to refresh access token", response["error"])
			},
		},
		{
			Name:   "should_return_bad_request_when_missing_refresh_token",
			Method: "POST",
			Path:   "/api/auth/refresh",
			Body:   map[string]string{},
			Headers: map[string]string{
				"Content-Type": "application/json",
			},
			ExpectedStatus: http.StatusBadRequest,
			ValidateBody: func(t *testing.T, w *httptest.ResponseRecorder) {
				var response map[string]string
				err := json.Unmarshal(w.Body.Bytes(), &response)
				assert.NoError(t, err)
				assert.Equal(t, "invalid request body", response["error"])
			},
		},
		{
			Name:           "should_return_bad_request_when_invalid_json",
			Method:         "POST",
			Path:           "/api/auth/refresh",
			Body:           "invalid-json",
			Headers:        map[string]string{"Content-Type": "application/json"},
			ExpectedStatus: http.StatusBadRequest,
			ValidateBody: func(t *testing.T, w *httptest.ResponseRecorder) {
				var response map[string]string
				err := json.Unmarshal(w.Body.Bytes(), &response)
				assert.NoError(t, err)
				assert.Equal(t, "invalid request body", response["error"])
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			mockAuth.ExpectedCalls = nil
			mockAuth.Calls = nil
			testutil.RunHandlerTest(t, router, tc)
			mockAuth.AssertExpectations(t)
		})
	}
}

func TestAuthAPIHandler_Login(t *testing.T) {
	handler, mockAuth, router := setupTestAuthAPIHandler()

	// Setup routes
	router.POST("/api/auth/login", handler.Login)

	tests := []testutil.HandlerTestCase{
		{
			Name:   "should_login_successfully_when_valid_credentials",
			Method: "POST",
			Path:   "/api/auth/login",
			Body: map[string]string{
				"username": "testuser",
				"password": "password123",
			},
			Headers: map[string]string{
				"Content-Type": "application/json",
			},
			MockSetup: func() {
				mockAuth.On("Login", mock.Anything, "testuser", "password123").
					Return("access-token", "refresh-token", int64(1234567890), nil)
			},
			ExpectedStatus: http.StatusOK,
			ValidateBody: func(t *testing.T, w *httptest.ResponseRecorder) {
				var response map[string]interface{}
				err := json.Unmarshal(w.Body.Bytes(), &response)
				assert.NoError(t, err)
				assert.Equal(t, "access-token", response["token"])
				assert.Equal(t, "refresh-token", response["refresh_token"])
				assert.Equal(t, float64(1234567890), response["expires_at"])
			},
		},
		{
			Name:   "should_return_unauthorized_when_invalid_credentials",
			Method: "POST",
			Path:   "/api/auth/login",
			Body: map[string]string{
				"username": "testuser",
				"password": "wrongpassword",
			},
			Headers: map[string]string{
				"Content-Type": "application/json",
			},
			MockSetup: func() {
				mockAuth.On("Login", mock.Anything, "testuser", "wrongpassword").
					Return("", "", int64(0), errors.New("invalid credentials"))
			},
			ExpectedStatus: http.StatusUnauthorized,
			ValidateBody: func(t *testing.T, w *httptest.ResponseRecorder) {
				var response map[string]string
				err := json.Unmarshal(w.Body.Bytes(), &response)
				assert.NoError(t, err)
				assert.Equal(t, "invalid username or password", response["error"])
			},
		},
		{
			Name:   "should_return_bad_request_when_invalid_request",
			Method: "POST",
			Path:   "/api/auth/login",
			Body:   map[string]string{},
			Headers: map[string]string{
				"Content-Type": "application/json",
			},
			ExpectedStatus: http.StatusBadRequest,
			ValidateBody: func(t *testing.T, w *httptest.ResponseRecorder) {
				var response map[string]string
				err := json.Unmarshal(w.Body.Bytes(), &response)
				assert.NoError(t, err)
				assert.Equal(t, "invalid request body", response["error"])
			},
		},
		{
			Name:           "should_return_bad_request_when_invalid_json",
			Method:         "POST",
			Path:           "/api/auth/login",
			Body:           "invalid-json",
			Headers:        map[string]string{"Content-Type": "application/json"},
			ExpectedStatus: http.StatusBadRequest,
			ValidateBody: func(t *testing.T, w *httptest.ResponseRecorder) {
				var response map[string]string
				err := json.Unmarshal(w.Body.Bytes(), &response)
				assert.NoError(t, err)
				assert.Equal(t, "invalid request body", response["error"])
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			mockAuth.ExpectedCalls = nil
			mockAuth.Calls = nil
			testutil.RunHandlerTest(t, router, tc)
			mockAuth.AssertExpectations(t)
		})
	}
}
