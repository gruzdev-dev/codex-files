package ports

import (
	"context"
	"time"

	"github.com/gruzdev-dev/codex-files/core/domain"
)

//go:generate mockgen -source=file.go -destination=file_mocks.go -package=ports FileRepository,FileProvider

type FileRepository interface {
	Create(ctx context.Context, file *domain.File) (*domain.File, error)
	GetByID(ctx context.Context, id string) (*domain.File, error)
	Update(ctx context.Context, file *domain.File) (*domain.File, error)
	SoftDelete(ctx context.Context, id string) error
}

type FileProvider interface {
	GenerateUploadURL(ctx context.Context, s3Path string, contentType string, maxSize int64, ttl time.Duration) (string, error)
	GenerateDownloadURL(ctx context.Context, s3Path string, ttl time.Duration) (string, error)
}
