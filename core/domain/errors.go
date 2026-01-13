package domain

import "errors"

var (
	ErrFileNotFound   = errors.New("file not found")
	ErrFileIDRequired = errors.New("file id is required")
	ErrAccessDenied   = errors.New("access denied")
	ErrInvalidInput   = errors.New("invalid input data")
	ErrInternal       = errors.New("internal server error")
)
