package settings

import (
	"context"
	"errors"
	"testing"
	"time"

	authModels "github.com/benidevo/vega/internal/auth/models"
	authRepoMocks "github.com/benidevo/vega/internal/auth/repository/mocks"
	"github.com/benidevo/vega/internal/config"
	settingsInterfacesMocks "github.com/benidevo/vega/internal/settings/interfaces/mocks"
	settingsMocks "github.com/benidevo/vega/internal/settings/mocks"
	settingsModels "github.com/benidevo/vega/internal/settings/models"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func init() {
	zerolog.SetGlobalLevel(zerolog.Disabled)
}

func createTestConfig() *config.Settings {
	return &config.Settings{}
}

func createTestProfile(userID int) *settingsModels.Profile {
	return &settingsModels.Profile{
		ID:        1,
		UserID:    userID,
		FirstName: "John",
		LastName:  "Doe",
		Email:     "john@example.com",
		Title:     "Software Engineer",
		Industry:  settingsModels.IndustryTechnology,
		Skills:    []string{"Go", "Python", "JavaScript"},
	}
}

func TestSettingsService_GetProfileSettings(t *testing.T) {
	tests := []struct {
		name          string
		userID        int
		setupMocks    func(*settingsInterfacesMocks.MockSettingsRepository)
		expectedError bool
		validateFunc  func(*testing.T, *settingsModels.Profile)
	}{
		{
			name:   "should_return_existing_profile_when_found",
			userID: 1,
			setupMocks: func(mockRepo *settingsInterfacesMocks.MockSettingsRepository) {
				profile := createTestProfile(1)
				mockRepo.On("GetProfileWithRelated", mock.Anything, 1).Return(profile, nil)
			},
			expectedError: false,
			validateFunc: func(t *testing.T, result *settingsModels.Profile) {
				assert.Equal(t, 1, result.ID)
				assert.Equal(t, 1, result.UserID)
				assert.Equal(t, "John", result.FirstName)
				assert.Equal(t, "Doe", result.LastName)
				assert.Equal(t, "john@example.com", result.Email)
			},
		},
		{
			name:   "should_return_empty_profile_when_not_found",
			userID: 2,
			setupMocks: func(mockRepo *settingsInterfacesMocks.MockSettingsRepository) {
				mockRepo.On("GetProfileWithRelated", mock.Anything, 2).Return(nil, nil)
			},
			expectedError: false,
			validateFunc: func(t *testing.T, result *settingsModels.Profile) {
				assert.Equal(t, 2, result.UserID)
				assert.Empty(t, result.FirstName)
				assert.Empty(t, result.Skills)
			},
		},
		{
			name:   "should_return_error_when_repository_fails",
			userID: 3,
			setupMocks: func(mockRepo *settingsInterfacesMocks.MockSettingsRepository) {
				mockRepo.On("GetProfileWithRelated", mock.Anything, 3).Return(nil, errors.New("database error"))
			},
			expectedError: true,
			validateFunc:  nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Arrange
			mockSettingsRepo := settingsInterfacesMocks.NewMockSettingsRepository(t)
			mockUserRepo := authRepoMocks.NewMockUserRepository(t)
			mockAuthService := settingsMocks.NewMockAuthServiceInterface(t)

			tc.setupMocks(mockSettingsRepo)

			service := NewSettingsService(mockSettingsRepo, createTestConfig(), mockUserRepo, mockAuthService)

			// Act
			result, err := service.GetProfileSettings(context.Background(), tc.userID)

			// Assert
			if tc.expectedError {
				require.Error(t, err)
				assert.Nil(t, result)
			} else {
				require.NoError(t, err)
				require.NotNil(t, result)
				if tc.validateFunc != nil {
					tc.validateFunc(t, result)
				}
			}
		})
	}
}

func TestSettingsService_GetProfileWithRelated(t *testing.T) {
	tests := []struct {
		name          string
		userID        int
		setupMocks    func(*settingsInterfacesMocks.MockSettingsRepository)
		expectedError bool
		validateFunc  func(*testing.T, *settingsModels.Profile)
	}{
		{
			name:   "should_return_profile_with_related_entities",
			userID: 1,
			setupMocks: func(mockRepo *settingsInterfacesMocks.MockSettingsRepository) {
				profile := createTestProfile(1)
				profile.WorkExperience = []settingsModels.WorkExperience{
					{ID: 1, ProfileID: 1, Company: "Acme Corp", Title: "Engineer"},
				}
				profile.Education = []settingsModels.Education{
					{ID: 1, ProfileID: 1, Institution: "MIT", Degree: "BS"},
				}
				mockRepo.On("GetProfileWithRelated", mock.Anything, 1).Return(profile, nil)
			},
			expectedError: false,
			validateFunc: func(t *testing.T, result *settingsModels.Profile) {
				assert.Len(t, result.WorkExperience, 1)
				assert.Len(t, result.Education, 1)
				assert.Equal(t, "Acme Corp", result.WorkExperience[0].Company)
			},
		},
		{
			name:   "should_return_empty_profile_when_not_found",
			userID: 99,
			setupMocks: func(mockRepo *settingsInterfacesMocks.MockSettingsRepository) {
				mockRepo.On("GetProfileWithRelated", mock.Anything, 99).Return(nil, nil)
			},
			expectedError: false,
			validateFunc: func(t *testing.T, result *settingsModels.Profile) {
				assert.Equal(t, 99, result.UserID)
				assert.Empty(t, result.WorkExperience)
				assert.Empty(t, result.Education)
				assert.Empty(t, result.Certifications)
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mockSettingsRepo := settingsInterfacesMocks.NewMockSettingsRepository(t)
			mockUserRepo := authRepoMocks.NewMockUserRepository(t)
			mockAuthService := settingsMocks.NewMockAuthServiceInterface(t)

			tc.setupMocks(mockSettingsRepo)

			service := NewSettingsService(mockSettingsRepo, createTestConfig(), mockUserRepo, mockAuthService)

			result, err := service.GetProfileWithRelated(context.Background(), tc.userID)

			if tc.expectedError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.NotNil(t, result)
				if tc.validateFunc != nil {
					tc.validateFunc(t, result)
				}
			}
		})
	}
}

func TestSettingsService_UpdateProfile(t *testing.T) {
	tests := []struct {
		name          string
		profile       *settingsModels.Profile
		setupMocks    func(*settingsInterfacesMocks.MockSettingsRepository)
		expectedError bool
		errorContains string
	}{
		{
			name: "should_update_profile_when_valid",
			profile: &settingsModels.Profile{
				ID:        1,
				UserID:    1,
				FirstName: "John",
				LastName:  "Doe",
				Email:     "john@example.com",
				Title:     "Engineer",
				Industry:  settingsModels.IndustryTechnology,
				Skills:    []string{"Go"},
			},
			setupMocks: func(mockRepo *settingsInterfacesMocks.MockSettingsRepository) {
				mockRepo.On("UpdateProfile", mock.Anything, mock.AnythingOfType("*models.Profile")).Return(nil)
			},
			expectedError: false,
		},
		{
			name: "should_return_error_when_repository_fails",
			profile: &settingsModels.Profile{
				ID:        1,
				UserID:    1,
				FirstName: "John",
				LastName:  "Doe",
				Email:     "john@example.com",
				Title:     "Engineer",
				Industry:  settingsModels.IndustryTechnology,
				Skills:    []string{"Go"},
			},
			setupMocks: func(mockRepo *settingsInterfacesMocks.MockSettingsRepository) {
				mockRepo.On("UpdateProfile", mock.Anything, mock.AnythingOfType("*models.Profile")).
					Return(errors.New("database error"))
			},
			expectedError: true,
			errorContains: "failed to update",
		},
		{
			name: "should_return_error_when_validation_fails",
			profile: &settingsModels.Profile{
				ID:     1,
				UserID: 1,
				// Missing required fields triggers validation error
				Email: "invalid-email",
			},
			setupMocks:    func(mockRepo *settingsInterfacesMocks.MockSettingsRepository) {},
			expectedError: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mockSettingsRepo := settingsInterfacesMocks.NewMockSettingsRepository(t)
			mockUserRepo := authRepoMocks.NewMockUserRepository(t)
			mockAuthService := settingsMocks.NewMockAuthServiceInterface(t)

			tc.setupMocks(mockSettingsRepo)

			service := NewSettingsService(mockSettingsRepo, createTestConfig(), mockUserRepo, mockAuthService)

			err := service.UpdateProfile(context.Background(), tc.profile)

			if tc.expectedError {
				require.Error(t, err)
				if tc.errorContains != "" {
					assert.Contains(t, err.Error(), tc.errorContains)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestSettingsService_GetSecuritySettings(t *testing.T) {
	tests := []struct {
		name          string
		username      string
		setupMocks    func(*authRepoMocks.MockUserRepository)
		expectedError bool
		validateFunc  func(*testing.T, *settingsModels.SecuritySettings)
	}{
		{
			name:     "should_return_security_settings_when_user_found",
			username: "johndoe",
			setupMocks: func(mockRepo *authRepoMocks.MockUserRepository) {
				lastLogin := time.Now().Add(-24 * time.Hour)
				createdAt := time.Now().Add(-30 * 24 * time.Hour)
				user := &authModels.User{
					ID:        1,
					Username:  "johndoe",
					LastLogin: lastLogin,
					CreatedAt: createdAt,
				}
				mockRepo.On("FindByUsername", mock.Anything, "johndoe").Return(user, nil)
			},
			expectedError: false,
			validateFunc: func(t *testing.T, result *settingsModels.SecuritySettings) {
				assert.NotNil(t, result.Activity)
				assert.False(t, result.Activity.LastLogin.IsZero())
				assert.False(t, result.Activity.CreatedAt.IsZero())
			},
		},
		{
			name:     "should_return_error_when_user_not_found",
			username: "unknown",
			setupMocks: func(mockRepo *authRepoMocks.MockUserRepository) {
				mockRepo.On("FindByUsername", mock.Anything, "unknown").Return(nil, errors.New("user not found"))
			},
			expectedError: true,
			validateFunc:  nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mockSettingsRepo := settingsInterfacesMocks.NewMockSettingsRepository(t)
			mockUserRepo := authRepoMocks.NewMockUserRepository(t)
			mockAuthService := settingsMocks.NewMockAuthServiceInterface(t)

			tc.setupMocks(mockUserRepo)

			service := NewSettingsService(mockSettingsRepo, createTestConfig(), mockUserRepo, mockAuthService)

			result, err := service.GetSecuritySettings(context.Background(), tc.username)

			if tc.expectedError {
				require.Error(t, err)
				assert.Nil(t, result)
			} else {
				require.NoError(t, err)
				require.NotNil(t, result)
				if tc.validateFunc != nil {
					tc.validateFunc(t, result)
				}
			}
		})
	}
}

func TestSettingsService_DeleteAllWorkExperience(t *testing.T) {
	tests := []struct {
		name          string
		profileID     int
		setupMocks    func(*settingsInterfacesMocks.MockSettingsRepository)
		expectedError bool
	}{
		{
			name:      "should_delete_all_work_experience_when_successful",
			profileID: 1,
			setupMocks: func(mockRepo *settingsInterfacesMocks.MockSettingsRepository) {
				mockRepo.On("DeleteAllWorkExperience", mock.Anything, 1).Return(nil)
			},
			expectedError: false,
		},
		{
			name:      "should_return_error_when_repository_fails",
			profileID: 1,
			setupMocks: func(mockRepo *settingsInterfacesMocks.MockSettingsRepository) {
				mockRepo.On("DeleteAllWorkExperience", mock.Anything, 1).Return(errors.New("database error"))
			},
			expectedError: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mockSettingsRepo := settingsInterfacesMocks.NewMockSettingsRepository(t)
			mockUserRepo := authRepoMocks.NewMockUserRepository(t)
			mockAuthService := settingsMocks.NewMockAuthServiceInterface(t)

			tc.setupMocks(mockSettingsRepo)

			service := NewSettingsService(mockSettingsRepo, createTestConfig(), mockUserRepo, mockAuthService)

			err := service.DeleteAllWorkExperience(context.Background(), tc.profileID)

			if tc.expectedError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestSettingsService_DeleteAllEducation(t *testing.T) {
	tests := []struct {
		name          string
		profileID     int
		setupMocks    func(*settingsInterfacesMocks.MockSettingsRepository)
		expectedError bool
	}{
		{
			name:      "should_delete_all_education_when_successful",
			profileID: 1,
			setupMocks: func(mockRepo *settingsInterfacesMocks.MockSettingsRepository) {
				mockRepo.On("DeleteAllEducation", mock.Anything, 1).Return(nil)
			},
			expectedError: false,
		},
		{
			name:      "should_return_error_when_repository_fails",
			profileID: 1,
			setupMocks: func(mockRepo *settingsInterfacesMocks.MockSettingsRepository) {
				mockRepo.On("DeleteAllEducation", mock.Anything, 1).Return(errors.New("database error"))
			},
			expectedError: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mockSettingsRepo := settingsInterfacesMocks.NewMockSettingsRepository(t)
			mockUserRepo := authRepoMocks.NewMockUserRepository(t)
			mockAuthService := settingsMocks.NewMockAuthServiceInterface(t)

			tc.setupMocks(mockSettingsRepo)

			service := NewSettingsService(mockSettingsRepo, createTestConfig(), mockUserRepo, mockAuthService)

			err := service.DeleteAllEducation(context.Background(), tc.profileID)

			if tc.expectedError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestSettingsService_DeleteAllCertifications(t *testing.T) {
	tests := []struct {
		name          string
		profileID     int
		setupMocks    func(*settingsInterfacesMocks.MockSettingsRepository)
		expectedError bool
	}{
		{
			name:      "should_delete_all_certifications_when_successful",
			profileID: 1,
			setupMocks: func(mockRepo *settingsInterfacesMocks.MockSettingsRepository) {
				mockRepo.On("DeleteAllCertifications", mock.Anything, 1).Return(nil)
			},
			expectedError: false,
		},
		{
			name:      "should_return_error_when_repository_fails",
			profileID: 1,
			setupMocks: func(mockRepo *settingsInterfacesMocks.MockSettingsRepository) {
				mockRepo.On("DeleteAllCertifications", mock.Anything, 1).Return(errors.New("database error"))
			},
			expectedError: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mockSettingsRepo := settingsInterfacesMocks.NewMockSettingsRepository(t)
			mockUserRepo := authRepoMocks.NewMockUserRepository(t)
			mockAuthService := settingsMocks.NewMockAuthServiceInterface(t)

			tc.setupMocks(mockSettingsRepo)

			service := NewSettingsService(mockSettingsRepo, createTestConfig(), mockUserRepo, mockAuthService)

			err := service.DeleteAllCertifications(context.Background(), tc.profileID)

			if tc.expectedError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
