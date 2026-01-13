package domain

import (
	"time"

	"github.com/google/uuid"
)

type FileStatus string

const (
	FileStatusPending  FileStatus = "pending"
	FileStatusUploaded FileStatus = "uploaded"
)

type File struct {
	ID          string
	OwnerID     string
	S3Path      string
	Size        int64
	ContentType string
	Status      FileStatus
	IsDeleted   bool
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func NewFile(ownerID, contentType string, size int64) *File {
	now := time.Now()
	return &File{
		ID:          uuid.New().String(),
		OwnerID:     ownerID,
		ContentType: contentType,
		Size:        size,
		Status:      FileStatusPending,
		IsDeleted:   false,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
}

func (f *File) MarkAsUploaded() {
	f.Status = FileStatusUploaded
	f.UpdatedAt = time.Now()
}

func (f *File) MarkAsDeleted() {
	f.IsDeleted = true
	f.UpdatedAt = time.Now()
}
