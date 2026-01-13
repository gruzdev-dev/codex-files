package services

import (
	"codex-files/core/domain"
	"codex-files/core/ports"
	"context"
	"fmt"
	"time"
)

type FileService struct {
	repo          ports.FileRepository
	fileProvider  ports.FileProvider
	uploadMaxSize int64
	uploadTTL     time.Duration
	downloadTTL   time.Duration
}

func NewFileService(
	repo ports.FileRepository,
	fileProvider ports.FileProvider,
	uploadMaxSize int64,
	uploadTTL time.Duration,
	downloadTTL time.Duration,
) *FileService {
	return &FileService{
		repo:          repo,
		fileProvider:  fileProvider,
		uploadMaxSize: uploadMaxSize,
		uploadTTL:     uploadTTL,
		downloadTTL:   downloadTTL,
	}
}

type GenerateUploadURLResult struct {
	FileID    string
	UploadURL string
}

func (s *FileService) GenerateUploadURL(ctx context.Context, ownerID, contentType string, size int64) (*GenerateUploadURLResult, error) {
	if ownerID == "" {
		return nil, fmt.Errorf("%w: owner ID is required", domain.ErrInvalidInput)
	}
	if contentType == "" {
		return nil, fmt.Errorf("%w: content type is required", domain.ErrInvalidInput)
	}
	if size <= 0 {
		return nil, fmt.Errorf("%w: file size must be positive", domain.ErrInvalidInput)
	}
	if size > s.uploadMaxSize {
		return nil, fmt.Errorf("%w: file size exceeds maximum allowed size", domain.ErrInvalidInput)
	}

	file := domain.NewFile(ownerID, contentType, size)
	file.S3Path = fmt.Sprintf("%s/%s", ownerID, file.ID)

	created, err := s.repo.Create(ctx, file)
	if err != nil {
		return nil, fmt.Errorf("%w: failed to create file record: %v", domain.ErrInternal, err)
	}

	uploadURL, err := s.fileProvider.GenerateUploadURL(
		ctx,
		created.S3Path,
		created.ContentType,
		s.uploadMaxSize,
		s.uploadTTL,
	)
	if err != nil {
		return nil, fmt.Errorf("%w: failed to generate upload URL: %v", domain.ErrInternal, err)
	}

	return &GenerateUploadURLResult{
		FileID:    created.ID,
		UploadURL: uploadURL,
	}, nil
}

type GetDownloadURLResult struct {
	DownloadURL string
}

func (s *FileService) GetDownloadURL(ctx context.Context, fileID, userID string, scopes []string) (*GetDownloadURLResult, error) {
	if fileID == "" {
		return nil, fmt.Errorf("%w: file ID is required", domain.ErrFileIDRequired)
	}

	file, err := s.repo.GetByID(ctx, fileID)
	if err != nil {
		if err == domain.ErrFileNotFound {
			return nil, err
		}
		return nil, fmt.Errorf("%w: failed to get file: %v", domain.ErrInternal, err)
	}

	if !s.hasAccess(file, userID, scopes) {
		return nil, domain.ErrAccessDenied
	}

	downloadURL, err := s.fileProvider.GenerateDownloadURL(
		ctx,
		file.S3Path,
		s.downloadTTL,
	)
	if err != nil {
		return nil, fmt.Errorf("%w: failed to generate download URL: %v", domain.ErrInternal, err)
	}

	return &GetDownloadURLResult{
		DownloadURL: downloadURL,
	}, nil
}

func (s *FileService) DeleteFile(ctx context.Context, fileID string) error {
	if fileID == "" {
		return fmt.Errorf("%w: file ID is required", domain.ErrFileIDRequired)
	}

	err := s.repo.SoftDelete(ctx, fileID)
	if err != nil {
		if err == domain.ErrFileNotFound {
			return err
		}
		return fmt.Errorf("%w: failed to delete file: %v", domain.ErrInternal, err)
	}

	return nil
}

func (s *FileService) hasAccess(file *domain.File, userID string, scopes []string) bool {
	if file.OwnerID == userID {
		return true
	}

	requiredScope := fmt.Sprintf("file:%s:read", file.ID)
	for _, scope := range scopes {
		if scope == requiredScope {
			return true
		}
	}

	return false
}
