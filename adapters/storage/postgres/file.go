package postgres

import (
	"codex-files/core/domain"
	"codex-files/core/ports"
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type FileRepo struct {
	pool *pgxpool.Pool
}

func NewFileRepo(pool *pgxpool.Pool) ports.FileRepository {
	return &FileRepo{
		pool: pool,
	}
}

func (r *FileRepo) Create(ctx context.Context, file *domain.File) (*domain.File, error) {
	query := `INSERT INTO files (id, owner_id, s3_path, size, content_type, status, is_deleted, created_at, updated_at) 
	          VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9) 
	          RETURNING id, owner_id, s3_path, size, content_type, status, is_deleted, created_at, updated_at`

	var created domain.File
	err := r.pool.QueryRow(ctx, query,
		file.ID,
		file.OwnerID,
		file.S3Path,
		file.Size,
		file.ContentType,
		string(file.Status),
		file.IsDeleted,
		file.CreatedAt,
		file.UpdatedAt,
	).Scan(
		&created.ID,
		&created.OwnerID,
		&created.S3Path,
		&created.Size,
		&created.ContentType,
		&created.Status,
		&created.IsDeleted,
		&created.CreatedAt,
		&created.UpdatedAt,
	)

	if err != nil {
		return nil, err
	}

	return &created, nil
}

func (r *FileRepo) GetByID(ctx context.Context, id string) (*domain.File, error) {
	query := `SELECT id, owner_id, s3_path, size, content_type, status, is_deleted, created_at, updated_at 
	          FROM files 
	          WHERE id = $1 AND is_deleted = false`

	var file domain.File
	var statusStr string
	err := r.pool.QueryRow(ctx, query, id).Scan(
		&file.ID,
		&file.OwnerID,
		&file.S3Path,
		&file.Size,
		&file.ContentType,
		&statusStr,
		&file.IsDeleted,
		&file.CreatedAt,
		&file.UpdatedAt,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, domain.ErrFileNotFound
		}
		return nil, err
	}

	file.Status = domain.FileStatus(statusStr)
	return &file, nil
}

func (r *FileRepo) Update(ctx context.Context, file *domain.File) (*domain.File, error) {
	file.UpdatedAt = time.Now()
	query := `UPDATE files 
	          SET s3_path = $2, size = $3, content_type = $4, status = $5, updated_at = $6 
	          WHERE id = $1 AND is_deleted = false
	          RETURNING id, owner_id, s3_path, size, content_type, status, is_deleted, created_at, updated_at`

	var updated domain.File
	var statusStr string
	err := r.pool.QueryRow(ctx, query,
		file.ID,
		file.S3Path,
		file.Size,
		file.ContentType,
		string(file.Status),
		file.UpdatedAt,
	).Scan(
		&updated.ID,
		&updated.OwnerID,
		&updated.S3Path,
		&updated.Size,
		&updated.ContentType,
		&statusStr,
		&updated.IsDeleted,
		&updated.CreatedAt,
		&updated.UpdatedAt,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, domain.ErrFileNotFound
		}
		return nil, err
	}

	updated.Status = domain.FileStatus(statusStr)
	return &updated, nil
}

func (r *FileRepo) SoftDelete(ctx context.Context, id string) error {
	query := `UPDATE files 
	          SET is_deleted = true, updated_at = $2 
	          WHERE id = $1 AND is_deleted = false`

	result, err := r.pool.Exec(ctx, query, id, time.Now())
	if err != nil {
		return err
	}

	if result.RowsAffected() == 0 {
		return domain.ErrFileNotFound
	}

	return nil
}
