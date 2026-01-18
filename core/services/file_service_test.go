package services

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/gruzdev-dev/codex-files/core/domain"
	"github.com/gruzdev-dev/codex-files/core/ports"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

const (
	testOwnerID     = "test-owner-123"
	testContentType = "application/pdf"
	testFileSize    = int64(1024 * 1024)       // 1MB
	testMaxSize     = int64(100 * 1024 * 1024) // 100MB
	testFileID      = "test-file-id"
	testS3Path      = "test-owner-123/test-file-id"
	testUploadURL   = "https://s3.example.com/bucket/path?signature=upload"
	testDownloadURL = "https://s3.example.com/bucket/path?signature=download"
	testUserID      = "test-user-123"
)

func TestFileService_GenerateUploadURL(t *testing.T) {
	tests := []struct {
		name           string
		ownerID        string
		contentType    string
		size           int64
		uploadMaxSize  int64
		setupMocks     func(*ports.MockFileRepository, *ports.MockFileProvider)
		expectedError  error
		validateResult func(*testing.T, *domain.GenerateUploadURLResult, error)
	}{
		{
			name:          "success path",
			ownerID:       testOwnerID,
			contentType:   testContentType,
			size:          testFileSize,
			uploadMaxSize: testMaxSize,
			setupMocks: func(repo *ports.MockFileRepository, provider *ports.MockFileProvider) {
				repo.EXPECT().
					Create(gomock.Any(), gomock.Any()).
					DoAndReturn(func(ctx context.Context, file *domain.File) (*domain.File, error) {
						require.NotEmpty(t, file.ID)
						require.Equal(t, testOwnerID, file.OwnerID)
						require.Equal(t, testContentType, file.ContentType)
						require.Equal(t, testFileSize, file.Size)
						require.Equal(t, domain.FileStatusPending, file.Status)
						require.False(t, file.IsDeleted)
						require.Contains(t, file.S3Path, testOwnerID)
						require.Contains(t, file.S3Path, file.ID)
						return file, nil
					})
				provider.EXPECT().
					GenerateUploadURL(
						gomock.Any(),
						gomock.Any(),
						testContentType,
						testMaxSize,
						gomock.Any(),
					).
					Return(testUploadURL, nil)
			},
			expectedError: nil,
			validateResult: func(t *testing.T, result *domain.GenerateUploadURLResult, err error) {
				require.NoError(t, err)
				require.NotNil(t, result)
				assert.NotEmpty(t, result.FileID)
				assert.Equal(t, testUploadURL, result.UploadURL)
			},
		},
		{
			name:          "empty owner ID",
			ownerID:       "",
			contentType:   testContentType,
			size:          testFileSize,
			uploadMaxSize: testMaxSize,
			setupMocks:    func(*ports.MockFileRepository, *ports.MockFileProvider) {},
			expectedError: domain.ErrInvalidInput,
			validateResult: func(t *testing.T, result *domain.GenerateUploadURLResult, err error) {
				assert.Error(t, err)
				assert.Nil(t, result)
				assert.ErrorIs(t, err, domain.ErrInvalidInput)
				assert.Contains(t, err.Error(), "owner ID is required")
			},
		},
		{
			name:          "empty content type",
			ownerID:       testOwnerID,
			contentType:   "",
			size:          testFileSize,
			uploadMaxSize: testMaxSize,
			setupMocks:    func(*ports.MockFileRepository, *ports.MockFileProvider) {},
			expectedError: domain.ErrInvalidInput,
			validateResult: func(t *testing.T, result *domain.GenerateUploadURLResult, err error) {
				assert.Error(t, err)
				assert.Nil(t, result)
				assert.ErrorIs(t, err, domain.ErrInvalidInput)
				assert.Contains(t, err.Error(), "content type is required")
			},
		},
		{
			name:          "zero size",
			ownerID:       testOwnerID,
			contentType:   testContentType,
			size:          0,
			uploadMaxSize: testMaxSize,
			setupMocks:    func(*ports.MockFileRepository, *ports.MockFileProvider) {},
			expectedError: domain.ErrInvalidInput,
			validateResult: func(t *testing.T, result *domain.GenerateUploadURLResult, err error) {
				assert.Error(t, err)
				assert.Nil(t, result)
				assert.ErrorIs(t, err, domain.ErrInvalidInput)
				assert.Contains(t, err.Error(), "file size must be positive")
			},
		},
		{
			name:          "negative size",
			ownerID:       testOwnerID,
			contentType:   testContentType,
			size:          -1,
			uploadMaxSize: testMaxSize,
			setupMocks:    func(*ports.MockFileRepository, *ports.MockFileProvider) {},
			expectedError: domain.ErrInvalidInput,
			validateResult: func(t *testing.T, result *domain.GenerateUploadURLResult, err error) {
				assert.Error(t, err)
				assert.Nil(t, result)
				assert.ErrorIs(t, err, domain.ErrInvalidInput)
				assert.Contains(t, err.Error(), "file size must be positive")
			},
		},
		{
			name:          "size exceeds max",
			ownerID:       testOwnerID,
			contentType:   testContentType,
			size:          testMaxSize + 1,
			uploadMaxSize: testMaxSize,
			setupMocks:    func(*ports.MockFileRepository, *ports.MockFileProvider) {},
			expectedError: domain.ErrInvalidInput,
			validateResult: func(t *testing.T, result *domain.GenerateUploadURLResult, err error) {
				assert.Error(t, err)
				assert.Nil(t, result)
				assert.ErrorIs(t, err, domain.ErrInvalidInput)
				assert.Contains(t, err.Error(), "file size exceeds maximum allowed size")
			},
		},
		{
			name:          "repository create error",
			ownerID:       testOwnerID,
			contentType:   testContentType,
			size:          testFileSize,
			uploadMaxSize: testMaxSize,
			setupMocks: func(repo *ports.MockFileRepository, provider *ports.MockFileProvider) {
				repo.EXPECT().
					Create(gomock.Any(), gomock.Any()).
					Return(nil, errors.New("database error"))
			},
			expectedError: domain.ErrInternal,
			validateResult: func(t *testing.T, result *domain.GenerateUploadURLResult, err error) {
				assert.Error(t, err)
				assert.Nil(t, result)
				assert.ErrorIs(t, err, domain.ErrInternal)
				assert.Contains(t, err.Error(), "failed to create file record")
			},
		},
		{
			name:          "file provider error",
			ownerID:       testOwnerID,
			contentType:   testContentType,
			size:          testFileSize,
			uploadMaxSize: testMaxSize,
			setupMocks: func(repo *ports.MockFileRepository, provider *ports.MockFileProvider) {
				repo.EXPECT().
					Create(gomock.Any(), gomock.Any()).
					Return(&domain.File{
						ID:          testFileID,
						OwnerID:     testOwnerID,
						S3Path:      testS3Path,
						ContentType: testContentType,
						Size:        testFileSize,
						Status:      domain.FileStatusPending,
					}, nil)
				provider.EXPECT().
					GenerateUploadURL(
						gomock.Any(),
						gomock.Any(),
						gomock.Any(),
						gomock.Any(),
						gomock.Any(),
					).
					Return("", errors.New("s3 error"))
			},
			expectedError: domain.ErrInternal,
			validateResult: func(t *testing.T, result *domain.GenerateUploadURLResult, err error) {
				assert.Error(t, err)
				assert.Nil(t, result)
				assert.ErrorIs(t, err, domain.ErrInternal)
				assert.Contains(t, err.Error(), "failed to generate upload URL")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			repo := ports.NewMockFileRepository(ctrl)
			provider := ports.NewMockFileProvider(ctrl)

			tt.setupMocks(repo, provider)

			service := NewFileService(
				repo,
				provider,
				tt.uploadMaxSize,
				5*time.Minute,
				15*time.Minute,
			)

			result, err := service.GenerateUploadURL(
				context.Background(),
				tt.ownerID,
				tt.contentType,
				tt.size,
			)

			if tt.expectedError != nil {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			if tt.validateResult != nil {
				tt.validateResult(t, result, err)
			}
		})
	}
}

func TestFileService_GetDownloadURL(t *testing.T) {
	tests := []struct {
		name           string
		fileID         string
		userID         string
		scopes         []string
		setupMocks     func(*ports.MockFileRepository, *ports.MockFileProvider)
		expectedError  error
		validateResult func(*testing.T, *domain.GetDownloadURLResult, error)
	}{
		{
			name:   "success path - owner access",
			fileID: testFileID,
			userID: testOwnerID,
			scopes: []string{},
			setupMocks: func(repo *ports.MockFileRepository, provider *ports.MockFileProvider) {
				file := &domain.File{
					ID:          testFileID,
					OwnerID:     testOwnerID,
					S3Path:      testS3Path,
					ContentType: testContentType,
					Size:        testFileSize,
					Status:      domain.FileStatusUploaded,
					IsDeleted:   false,
				}
				repo.EXPECT().
					GetByID(gomock.Any(), testFileID).
					Return(file, nil)
				provider.EXPECT().
					GenerateDownloadURL(
						gomock.Any(),
						testS3Path,
						gomock.Any(),
					).
					Return(testDownloadURL, nil)
			},
			expectedError: nil,
			validateResult: func(t *testing.T, result *domain.GetDownloadURLResult, err error) {
				require.NoError(t, err)
				require.NotNil(t, result)
				assert.Equal(t, testDownloadURL, result.DownloadURL)
			},
		},
		{
			name:   "success path - scope access",
			fileID: testFileID,
			userID: testUserID,
			scopes: []string{"file:" + testFileID + ":read"},
			setupMocks: func(repo *ports.MockFileRepository, provider *ports.MockFileProvider) {
				file := &domain.File{
					ID:          testFileID,
					OwnerID:     testOwnerID,
					S3Path:      testS3Path,
					ContentType: testContentType,
					Size:        testFileSize,
					Status:      domain.FileStatusUploaded,
					IsDeleted:   false,
				}
				repo.EXPECT().
					GetByID(gomock.Any(), testFileID).
					Return(file, nil)
				provider.EXPECT().
					GenerateDownloadURL(
						gomock.Any(),
						testS3Path,
						gomock.Any(),
					).
					Return(testDownloadURL, nil)
			},
			expectedError: nil,
			validateResult: func(t *testing.T, result *domain.GetDownloadURLResult, err error) {
				require.NoError(t, err)
				require.NotNil(t, result)
				assert.Equal(t, testDownloadURL, result.DownloadURL)
			},
		},
		{
			name:          "empty file ID",
			fileID:        "",
			userID:        testUserID,
			scopes:        []string{},
			setupMocks:    func(*ports.MockFileRepository, *ports.MockFileProvider) {},
			expectedError: domain.ErrFileIDRequired,
			validateResult: func(t *testing.T, result *domain.GetDownloadURLResult, err error) {
				assert.Error(t, err)
				assert.Nil(t, result)
				assert.ErrorIs(t, err, domain.ErrFileIDRequired)
			},
		},
		{
			name:   "file not found",
			fileID: testFileID,
			userID: testUserID,
			scopes: []string{},
			setupMocks: func(repo *ports.MockFileRepository, provider *ports.MockFileProvider) {
				repo.EXPECT().
					GetByID(gomock.Any(), testFileID).
					Return(nil, domain.ErrFileNotFound)
			},
			expectedError: domain.ErrFileNotFound,
			validateResult: func(t *testing.T, result *domain.GetDownloadURLResult, err error) {
				assert.Error(t, err)
				assert.Nil(t, result)
				assert.Equal(t, domain.ErrFileNotFound, err)
			},
		},
		{
			name:   "repository error",
			fileID: testFileID,
			userID: testUserID,
			scopes: []string{},
			setupMocks: func(repo *ports.MockFileRepository, provider *ports.MockFileProvider) {
				repo.EXPECT().
					GetByID(gomock.Any(), testFileID).
					Return(nil, errors.New("database error"))
			},
			expectedError: domain.ErrInternal,
			validateResult: func(t *testing.T, result *domain.GetDownloadURLResult, err error) {
				assert.Error(t, err)
				assert.Nil(t, result)
				assert.ErrorIs(t, err, domain.ErrInternal)
				assert.Contains(t, err.Error(), "failed to get file")
			},
		},
		{
			name:   "access denied - not owner and no scope",
			fileID: testFileID,
			userID: testUserID,
			scopes: []string{},
			setupMocks: func(repo *ports.MockFileRepository, provider *ports.MockFileProvider) {
				file := &domain.File{
					ID:          testFileID,
					OwnerID:     testOwnerID,
					S3Path:      testS3Path,
					ContentType: testContentType,
					Size:        testFileSize,
					Status:      domain.FileStatusUploaded,
					IsDeleted:   false,
				}
				repo.EXPECT().
					GetByID(gomock.Any(), testFileID).
					Return(file, nil)
			},
			expectedError: domain.ErrAccessDenied,
			validateResult: func(t *testing.T, result *domain.GetDownloadURLResult, err error) {
				assert.Error(t, err)
				assert.Nil(t, result)
				assert.Equal(t, domain.ErrAccessDenied, err)
			},
		},
		{
			name:   "access denied - wrong scope",
			fileID: testFileID,
			userID: testUserID,
			scopes: []string{"file:other-file-id:read"},
			setupMocks: func(repo *ports.MockFileRepository, provider *ports.MockFileProvider) {
				file := &domain.File{
					ID:          testFileID,
					OwnerID:     testOwnerID,
					S3Path:      testS3Path,
					ContentType: testContentType,
					Size:        testFileSize,
					Status:      domain.FileStatusUploaded,
					IsDeleted:   false,
				}
				repo.EXPECT().
					GetByID(gomock.Any(), testFileID).
					Return(file, nil)
			},
			expectedError: domain.ErrAccessDenied,
			validateResult: func(t *testing.T, result *domain.GetDownloadURLResult, err error) {
				assert.Error(t, err)
				assert.Nil(t, result)
				assert.Equal(t, domain.ErrAccessDenied, err)
			},
		},
		{
			name:   "file provider error",
			fileID: testFileID,
			userID: testOwnerID,
			scopes: []string{},
			setupMocks: func(repo *ports.MockFileRepository, provider *ports.MockFileProvider) {
				file := &domain.File{
					ID:          testFileID,
					OwnerID:     testOwnerID,
					S3Path:      testS3Path,
					ContentType: testContentType,
					Size:        testFileSize,
					Status:      domain.FileStatusUploaded,
					IsDeleted:   false,
				}
				repo.EXPECT().
					GetByID(gomock.Any(), testFileID).
					Return(file, nil)
				provider.EXPECT().
					GenerateDownloadURL(
						gomock.Any(),
						gomock.Any(),
						gomock.Any(),
					).
					Return("", errors.New("s3 error"))
			},
			expectedError: domain.ErrInternal,
			validateResult: func(t *testing.T, result *domain.GetDownloadURLResult, err error) {
				assert.Error(t, err)
				assert.Nil(t, result)
				assert.ErrorIs(t, err, domain.ErrInternal)
				assert.Contains(t, err.Error(), "failed to generate download URL")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			repo := ports.NewMockFileRepository(ctrl)
			provider := ports.NewMockFileProvider(ctrl)

			tt.setupMocks(repo, provider)

			service := NewFileService(
				repo,
				provider,
				testMaxSize,
				5*time.Minute,
				15*time.Minute,
			)

			result, err := service.GetDownloadURL(
				context.Background(),
				tt.fileID,
				tt.userID,
				tt.scopes,
			)

			if tt.expectedError != nil {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			if tt.validateResult != nil {
				tt.validateResult(t, result, err)
			}
		})
	}
}

func TestFileService_DeleteFile(t *testing.T) {
	tests := []struct {
		name           string
		fileID         string
		setupMocks     func(*ports.MockFileRepository, *ports.MockFileProvider)
		expectedError  error
		validateResult func(*testing.T, error)
	}{
		{
			name:   "success path",
			fileID: testFileID,
			setupMocks: func(repo *ports.MockFileRepository, provider *ports.MockFileProvider) {
				repo.EXPECT().
					SoftDelete(gomock.Any(), testFileID).
					Return(nil)
			},
			expectedError: nil,
			validateResult: func(t *testing.T, err error) {
				assert.NoError(t, err)
			},
		},
		{
			name:          "empty file ID",
			fileID:        "",
			setupMocks:    func(*ports.MockFileRepository, *ports.MockFileProvider) {},
			expectedError: domain.ErrFileIDRequired,
			validateResult: func(t *testing.T, err error) {
				assert.Error(t, err)
				assert.ErrorIs(t, err, domain.ErrFileIDRequired)
			},
		},
		{
			name:   "file not found",
			fileID: testFileID,
			setupMocks: func(repo *ports.MockFileRepository, provider *ports.MockFileProvider) {
				repo.EXPECT().
					SoftDelete(gomock.Any(), testFileID).
					Return(domain.ErrFileNotFound)
			},
			expectedError: domain.ErrFileNotFound,
			validateResult: func(t *testing.T, err error) {
				assert.Error(t, err)
				assert.Equal(t, domain.ErrFileNotFound, err)
			},
		},
		{
			name:   "repository error",
			fileID: testFileID,
			setupMocks: func(repo *ports.MockFileRepository, provider *ports.MockFileProvider) {
				repo.EXPECT().
					SoftDelete(gomock.Any(), testFileID).
					Return(errors.New("database error"))
			},
			expectedError: domain.ErrInternal,
			validateResult: func(t *testing.T, err error) {
				assert.Error(t, err)
				assert.ErrorIs(t, err, domain.ErrInternal)
				assert.Contains(t, err.Error(), "failed to delete file")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			repo := ports.NewMockFileRepository(ctrl)
			provider := ports.NewMockFileProvider(ctrl)

			tt.setupMocks(repo, provider)

			service := NewFileService(
				repo,
				provider,
				testMaxSize,
				5*time.Minute,
				15*time.Minute,
			)

			err := service.DeleteFile(context.Background(), tt.fileID)

			if tt.expectedError != nil {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			if tt.validateResult != nil {
				tt.validateResult(t, err)
			}
		})
	}
}

func TestFileService_ConfirmUpload(t *testing.T) {
	tests := []struct {
		name           string
		fileID         string
		setupMocks     func(*ports.MockFileRepository, *ports.MockFileProvider)
		expectedError  error
		validateResult func(*testing.T, error)
	}{
		{
			name:   "success path - pending to uploaded",
			fileID: testFileID,
			setupMocks: func(repo *ports.MockFileRepository, provider *ports.MockFileProvider) {
				file := &domain.File{
					ID:          testFileID,
					OwnerID:     testOwnerID,
					S3Path:      testS3Path,
					ContentType: testContentType,
					Size:        testFileSize,
					Status:      domain.FileStatusPending,
					IsDeleted:   false,
				}
				repo.EXPECT().
					GetByID(gomock.Any(), testFileID).
					Return(file, nil)
				repo.EXPECT().
					Update(gomock.Any(), gomock.Any()).
					DoAndReturn(func(ctx context.Context, file *domain.File) (*domain.File, error) {
						require.Equal(t, domain.FileStatusUploaded, file.Status)
						require.Equal(t, testFileID, file.ID)
						return file, nil
					})
			},
			expectedError: nil,
			validateResult: func(t *testing.T, err error) {
				assert.NoError(t, err)
			},
		},
		{
			name:   "idempotency - already uploaded",
			fileID: testFileID,
			setupMocks: func(repo *ports.MockFileRepository, provider *ports.MockFileProvider) {
				file := &domain.File{
					ID:          testFileID,
					OwnerID:     testOwnerID,
					S3Path:      testS3Path,
					ContentType: testContentType,
					Size:        testFileSize,
					Status:      domain.FileStatusUploaded,
					IsDeleted:   false,
				}
				repo.EXPECT().
					GetByID(gomock.Any(), testFileID).
					Return(file, nil)
			},
			expectedError: nil,
			validateResult: func(t *testing.T, err error) {
				assert.NoError(t, err)
			},
		},
		{
			name:          "empty file ID",
			fileID:        "",
			setupMocks:    func(*ports.MockFileRepository, *ports.MockFileProvider) {},
			expectedError: domain.ErrFileIDRequired,
			validateResult: func(t *testing.T, err error) {
				assert.Error(t, err)
				assert.ErrorIs(t, err, domain.ErrFileIDRequired)
			},
		},
		{
			name:   "file not found",
			fileID: testFileID,
			setupMocks: func(repo *ports.MockFileRepository, provider *ports.MockFileProvider) {
				repo.EXPECT().
					GetByID(gomock.Any(), testFileID).
					Return(nil, domain.ErrFileNotFound)
			},
			expectedError: nil,
			validateResult: func(t *testing.T, err error) {
				assert.NoError(t, err)
			},
		},
		{
			name:   "repository get error",
			fileID: testFileID,
			setupMocks: func(repo *ports.MockFileRepository, provider *ports.MockFileProvider) {
				repo.EXPECT().
					GetByID(gomock.Any(), testFileID).
					Return(nil, errors.New("database error"))
			},
			expectedError: errors.New("database error"),
			validateResult: func(t *testing.T, err error) {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "failed to get file")
			},
		},
		{
			name:   "repository update error",
			fileID: testFileID,
			setupMocks: func(repo *ports.MockFileRepository, provider *ports.MockFileProvider) {
				file := &domain.File{
					ID:          testFileID,
					OwnerID:     testOwnerID,
					S3Path:      testS3Path,
					ContentType: testContentType,
					Size:        testFileSize,
					Status:      domain.FileStatusPending,
					IsDeleted:   false,
				}
				repo.EXPECT().
					GetByID(gomock.Any(), testFileID).
					Return(file, nil)
				repo.EXPECT().
					Update(gomock.Any(), gomock.Any()).
					Return(nil, errors.New("update error"))
			},
			expectedError: errors.New("update error"),
			validateResult: func(t *testing.T, err error) {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "failed to update file status")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			repo := ports.NewMockFileRepository(ctrl)
			provider := ports.NewMockFileProvider(ctrl)

			tt.setupMocks(repo, provider)

			service := NewFileService(
				repo,
				provider,
				testMaxSize,
				5*time.Minute,
				15*time.Minute,
			)

			err := service.ConfirmUpload(context.Background(), tt.fileID)

			if tt.expectedError != nil {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			if tt.validateResult != nil {
				tt.validateResult(t, err)
			}
		})
	}
}
