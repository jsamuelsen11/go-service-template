package app

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/jsamuelsen/go-service-template/internal/domain"
	"github.com/jsamuelsen/go-service-template/internal/mocks"
	"github.com/jsamuelsen/go-service-template/internal/ports"
)

func TestNewService(t *testing.T) {
	tests := []struct {
		name   string
		repo   ports.ExampleRepository
		client ports.ExampleClient
		flags  ports.FeatureFlags
		cfg    *ServiceConfig
	}{
		{
			name:   "with all dependencies",
			repo:   mocks.NewMockExampleRepository(t),
			client: mocks.NewMockExampleClient(t),
			flags:  mocks.NewMockFeatureFlags(t),
			cfg:    &ServiceConfig{Logger: discardLogger()},
		},
		{
			name:   "with nil config uses default logger",
			repo:   mocks.NewMockExampleRepository(t),
			client: mocks.NewMockExampleClient(t),
			flags:  mocks.NewMockFeatureFlags(t),
			cfg:    nil,
		},
		{
			name:   "with nil flags",
			repo:   mocks.NewMockExampleRepository(t),
			client: mocks.NewMockExampleClient(t),
			flags:  nil,
			cfg:    &ServiceConfig{Logger: discardLogger()},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := NewService(tt.repo, tt.client, tt.flags, tt.cfg)
			require.NotNil(t, svc)
		})
	}
}

func TestService_ProcessExample(t *testing.T) {
	tests := []struct {
		name         string
		id           string
		setupMocks   func(*mocks.MockExampleRepository, *mocks.MockExampleClient, *mocks.MockFeatureFlags)
		expectedData *ports.ExampleData
		errCheck     func(error) bool
	}{
		{
			name: "success without feature flag",
			id:   "test-123",
			setupMocks: func(repo *mocks.MockExampleRepository, client *mocks.MockExampleClient, flags *mocks.MockFeatureFlags) {
				flags.EXPECT().IsEnabled(mock.Anything, "use-new-processing", false).Return(false)
				client.EXPECT().Fetch(mock.Anything, "test-123").Return(&ports.ExampleData{
					ID:    "test-123",
					Value: "fetched-value",
				}, nil)
				repo.EXPECT().Save(mock.Anything, mock.MatchedBy(func(e *ports.ExampleEntity) bool {
					return e.ID == "test-123"
				})).Return(nil)
			},
			expectedData: &ports.ExampleData{ID: "test-123", Value: "fetched-value"},
		},
		{
			name: "success with feature flag enabled",
			id:   "test-456",
			setupMocks: func(repo *mocks.MockExampleRepository, client *mocks.MockExampleClient, flags *mocks.MockFeatureFlags) {
				flags.EXPECT().IsEnabled(mock.Anything, "use-new-processing", false).Return(true)
				client.EXPECT().Fetch(mock.Anything, "test-456").Return(&ports.ExampleData{
					ID:    "test-456",
					Value: "new-value",
				}, nil)
				repo.EXPECT().Save(mock.Anything, mock.Anything).Return(nil)
			},
			expectedData: &ports.ExampleData{ID: "test-456", Value: "new-value"},
		},
		{
			name: "validation error - empty ID",
			id:   "",
			setupMocks: func(repo *mocks.MockExampleRepository, client *mocks.MockExampleClient, flags *mocks.MockFeatureFlags) {
				// No mocks called for validation failure
			},
			errCheck: domain.IsValidation,
		},
		{
			name: "client fetch error",
			id:   "test-789",
			setupMocks: func(repo *mocks.MockExampleRepository, client *mocks.MockExampleClient, flags *mocks.MockFeatureFlags) {
				flags.EXPECT().IsEnabled(mock.Anything, "use-new-processing", false).Return(false)
				client.EXPECT().Fetch(mock.Anything, "test-789").Return(nil, domain.NewUnavailableError("external-api", "timeout"))
			},
			errCheck: domain.IsUnavailable,
		},
		{
			name: "repository save error",
			id:   "test-conflict",
			setupMocks: func(repo *mocks.MockExampleRepository, client *mocks.MockExampleClient, flags *mocks.MockFeatureFlags) {
				flags.EXPECT().IsEnabled(mock.Anything, "use-new-processing", false).Return(false)
				client.EXPECT().Fetch(mock.Anything, "test-conflict").Return(&ports.ExampleData{
					ID:    "test-conflict",
					Value: "value",
				}, nil)
				repo.EXPECT().Save(mock.Anything, mock.Anything).Return(domain.NewConflictError("entity", "version mismatch"))
			},
			errCheck: domain.IsConflict,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := mocks.NewMockExampleRepository(t)
			mockClient := mocks.NewMockExampleClient(t)
			mockFlags := mocks.NewMockFeatureFlags(t)

			tt.setupMocks(mockRepo, mockClient, mockFlags)

			svc := NewService(mockRepo, mockClient, mockFlags, &ServiceConfig{
				Logger: discardLogger(),
			})

			data, err := svc.ProcessExample(context.Background(), tt.id)

			if tt.errCheck != nil {
				require.Error(t, err)
				assert.True(t, tt.errCheck(err), "unexpected error type: %v", err)
				assert.Nil(t, data)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expectedData, data)
			}
		})
	}
}

func TestService_ProcessExample_NilFlags(t *testing.T) {
	mockRepo := mocks.NewMockExampleRepository(t)
	mockClient := mocks.NewMockExampleClient(t)

	mockClient.EXPECT().Fetch(mock.Anything, "test-123").Return(&ports.ExampleData{
		ID:    "test-123",
		Value: "value",
	}, nil)
	mockRepo.EXPECT().Save(mock.Anything, mock.Anything).Return(nil)

	svc := NewService(mockRepo, mockClient, nil, &ServiceConfig{
		Logger: discardLogger(),
	})

	data, err := svc.ProcessExample(context.Background(), "test-123")

	require.NoError(t, err)
	assert.Equal(t, "test-123", data.ID)
}

func TestService_GetExample(t *testing.T) {
	tests := []struct {
		name           string
		id             string
		setupMock      func(*mocks.MockExampleRepository)
		expectedEntity *ports.ExampleEntity
		errCheck       func(error) bool
	}{
		{
			name: "success",
			id:   "entity-123",
			setupMock: func(repo *mocks.MockExampleRepository) {
				repo.EXPECT().GetByID(mock.Anything, "entity-123").Return(&ports.ExampleEntity{
					ID:      "entity-123",
					Version: 1,
				}, nil)
			},
			expectedEntity: &ports.ExampleEntity{ID: "entity-123", Version: 1},
		},
		{
			name: "validation error - empty ID",
			id:   "",
			setupMock: func(repo *mocks.MockExampleRepository) {
				// No mock call expected
			},
			errCheck: domain.IsValidation,
		},
		{
			name: "not found",
			id:   "nonexistent",
			setupMock: func(repo *mocks.MockExampleRepository) {
				repo.EXPECT().GetByID(mock.Anything, "nonexistent").Return(nil, domain.NewNotFoundError("entity", "nonexistent"))
			},
			errCheck: domain.IsNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := mocks.NewMockExampleRepository(t)
			mockClient := mocks.NewMockExampleClient(t)
			mockFlags := mocks.NewMockFeatureFlags(t)

			tt.setupMock(mockRepo)

			svc := NewService(mockRepo, mockClient, mockFlags, &ServiceConfig{
				Logger: discardLogger(),
			})

			entity, err := svc.GetExample(context.Background(), tt.id)

			if tt.errCheck != nil {
				require.Error(t, err)
				assert.True(t, tt.errCheck(err))
				assert.Nil(t, entity)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expectedEntity, entity)
			}
		})
	}
}

func TestService_DeleteExample(t *testing.T) {
	tests := []struct {
		name      string
		id        string
		setupMock func(*mocks.MockExampleRepository)
		errCheck  func(error) bool
	}{
		{
			name: "success",
			id:   "entity-123",
			setupMock: func(repo *mocks.MockExampleRepository) {
				repo.EXPECT().Delete(mock.Anything, "entity-123").Return(nil)
			},
			errCheck: nil,
		},
		{
			name: "validation error - empty ID",
			id:   "",
			setupMock: func(repo *mocks.MockExampleRepository) {
				// No mock call expected
			},
			errCheck: domain.IsValidation,
		},
		{
			name: "not found",
			id:   "nonexistent",
			setupMock: func(repo *mocks.MockExampleRepository) {
				repo.EXPECT().Delete(mock.Anything, "nonexistent").Return(domain.NewNotFoundError("entity", "nonexistent"))
			},
			errCheck: domain.IsNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := mocks.NewMockExampleRepository(t)
			mockClient := mocks.NewMockExampleClient(t)
			mockFlags := mocks.NewMockFeatureFlags(t)

			tt.setupMock(mockRepo)

			svc := NewService(mockRepo, mockClient, mockFlags, &ServiceConfig{
				Logger: discardLogger(),
			})

			err := svc.DeleteExample(context.Background(), tt.id)

			if tt.errCheck != nil {
				require.Error(t, err)
				assert.True(t, tt.errCheck(err))
			} else {
				require.NoError(t, err)
			}
		})
	}
}
