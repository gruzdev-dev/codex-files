package services

import (
	"context"
	"fmt"
	"log"
	"slices"
	"time"

	"github.com/gruzdev-dev/codex-files/core/domain"
	"github.com/gruzdev-dev/codex-files/core/ports"

	"github.com/gruzdev-dev/codex-files/pkg/identity"
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

func (s *FileService) GenerateUploadURL(ctx context.Context, ownerID, contentType string, size int64) (*domain.GenerateUploadURLResult, error) {
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

	downloadURL := fmt.Sprintf("/files/%s/download", created.ID)

	return &domain.GenerateUploadURLResult{
		FileID:      created.ID,
		UploadURL:   uploadURL,
		DownloadURL: downloadURL,
	}, nil
}

func (s *FileService) GetDownloadURL(ctx context.Context, fileID string) (*domain.GetDownloadURLResult, error) {
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

	user, ok := identity.FromCtx(ctx)
	if !ok {
		return nil, domain.ErrAccessDenied
	}

	if !s.hasAccess(file, user.UserID, user.Scopes) {
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

	return &domain.GetDownloadURLResult{
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

	requiredScope := fmt.Sprintf("files:file:%s:read", file.ID)
	return slices.Contains(scopes, requiredScope)
}

func (s *FileService) ConfirmUpload(ctx context.Context, fileID string) error {
	if fileID == "" {
		return fmt.Errorf("%w: file ID is required", domain.ErrFileIDRequired)
	}

	file, err := s.repo.GetByID(ctx, fileID)
	if err != nil {
		if err == domain.ErrFileNotFound {
			log.Printf("webhook: file not found: %s", fileID)
			return nil
		}
		return fmt.Errorf("failed to get file: %w", err)
	}

	if file.Status == domain.FileStatusUploaded {
		return nil
	}

	if file.Status == domain.FileStatusPending {
		file.MarkAsUploaded()
		_, err := s.repo.Update(ctx, file)
		if err != nil {
			return fmt.Errorf("failed to update file status: %w", err)
		}
	}

	return nil
}
