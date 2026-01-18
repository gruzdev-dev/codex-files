package grpc

import (
	"codex-files/core/services"
	"codex-files/proto"
	"context"
	"fmt"
)

type FilesHandler struct {
	proto.UnimplementedFilesServiceServer
	fileService *services.FileService
}

func NewFilesHandler(fileService *services.FileService) *FilesHandler {
	return &FilesHandler{
		fileService: fileService,
	}
}

func (h *FilesHandler) GeneratePresignedUrls(ctx context.Context, req *proto.GeneratePresignedUrlsRequest) (*proto.GeneratePresignedUrlsResponse, error) {
	if req.UserId == "" {
		return nil, fmt.Errorf("user_id is required")
	}
	if req.ContentType == "" {
		return nil, fmt.Errorf("content_type is required")
	}
	if req.Size <= 0 {
		return nil, fmt.Errorf("size must be positive")
	}

	result, err := h.fileService.GenerateUploadURL(ctx, req.UserId, req.ContentType, req.Size)
	if err != nil {
		return nil, fmt.Errorf("failed to generate upload URL: %w", err)
	}

	return &proto.GeneratePresignedUrlsResponse{
		FileId:      result.FileID,
		UploadUrl:   result.UploadURL,
		DownloadUrl: result.DownloadURL,
	}, nil
}

func (h *FilesHandler) DeleteFile(ctx context.Context, req *proto.DeleteFileRequest) (*proto.DeleteFileResponse, error) {
	if req.FileId == "" {
		return nil, fmt.Errorf("file_id is required")
	}

	err := h.fileService.DeleteFile(ctx, req.FileId)
	if err != nil {
		return nil, fmt.Errorf("failed to delete file: %w", err)
	}

	return &proto.DeleteFileResponse{}, nil
}
