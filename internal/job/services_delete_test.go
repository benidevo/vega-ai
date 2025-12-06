package job

import (
	"context"
	"errors"
	"testing"

	jobinterfacesmocks "github.com/benidevo/vega/internal/job/interfaces/mocks"
	"github.com/benidevo/vega/internal/job/models"
	"github.com/stretchr/testify/assert"
)

func TestJobService_DeleteMatchResult(t *testing.T) {
	t.Run("successful deletion", func(t *testing.T) {
		mockRepo := jobinterfacesmocks.NewMockJobRepository(t)
		cfg := setupTestConfig()
		service := NewJobService(mockRepo, nil, nil, nil, cfg)
		ctx := context.Background()
		jobID := 1
		matchID := 100

		mockRepo.On("MatchResultBelongsToJob", ctx, testUserID, matchID, jobID).Return(true, nil)
		mockRepo.On("DeleteMatchResult", ctx, testUserID, matchID).Return(nil)

		err := service.DeleteMatchResult(ctx, testUserID, jobID, matchID)

		assert.NoError(t, err)
	})

	t.Run("invalid job ID", func(t *testing.T) {
		mockRepo := jobinterfacesmocks.NewMockJobRepository(t)
		cfg := setupTestConfig()
		service := NewJobService(mockRepo, nil, nil, nil, cfg)
		ctx := context.Background()

		err := service.DeleteMatchResult(ctx, testUserID, 0, 100)

		assert.Equal(t, models.ErrInvalidJobID, err)
		mockRepo.AssertNotCalled(t, "MatchResultBelongsToJob")
		mockRepo.AssertNotCalled(t, "DeleteMatchResult")
	})

	t.Run("invalid match ID", func(t *testing.T) {
		mockRepo := jobinterfacesmocks.NewMockJobRepository(t)
		cfg := setupTestConfig()
		service := NewJobService(mockRepo, nil, nil, nil, cfg)
		ctx := context.Background()

		err := service.DeleteMatchResult(ctx, testUserID, 1, 0)

		assert.Equal(t, models.ErrInvalidJobID, err)
		mockRepo.AssertNotCalled(t, "MatchResultBelongsToJob")
		mockRepo.AssertNotCalled(t, "DeleteMatchResult")
	})

	t.Run("match result does not belong to job", func(t *testing.T) {
		mockRepo := jobinterfacesmocks.NewMockJobRepository(t)
		cfg := setupTestConfig()
		service := NewJobService(mockRepo, nil, nil, nil, cfg)
		ctx := context.Background()
		jobID := 1
		matchID := 100

		mockRepo.On("MatchResultBelongsToJob", ctx, testUserID, matchID, jobID).Return(false, nil)

		err := service.DeleteMatchResult(ctx, testUserID, jobID, matchID)

		assert.Equal(t, models.ErrJobNotFound, err)
		mockRepo.AssertNotCalled(t, "DeleteMatchResult")
	})

	t.Run("error checking match ownership", func(t *testing.T) {
		mockRepo := jobinterfacesmocks.NewMockJobRepository(t)
		cfg := setupTestConfig()
		service := NewJobService(mockRepo, nil, nil, nil, cfg)
		ctx := context.Background()
		jobID := 1
		matchID := 100
		expectedErr := errors.New("database error")

		mockRepo.On("MatchResultBelongsToJob", ctx, testUserID, matchID, jobID).Return(false, expectedErr)

		err := service.DeleteMatchResult(ctx, testUserID, jobID, matchID)

		assert.Equal(t, expectedErr, err)
		mockRepo.AssertNotCalled(t, "DeleteMatchResult")
	})

	t.Run("error deleting match result", func(t *testing.T) {
		mockRepo := jobinterfacesmocks.NewMockJobRepository(t)
		cfg := setupTestConfig()
		service := NewJobService(mockRepo, nil, nil, nil, cfg)
		ctx := context.Background()
		jobID := 1
		matchID := 100
		expectedErr := errors.New("delete error")

		mockRepo.On("MatchResultBelongsToJob", ctx, testUserID, matchID, jobID).Return(true, nil)
		mockRepo.On("DeleteMatchResult", ctx, testUserID, matchID).Return(expectedErr)

		err := service.DeleteMatchResult(ctx, testUserID, jobID, matchID)

		assert.Equal(t, expectedErr, err)
	})
}
