package s3

import (
	"codex-files/configs"
	"codex-files/core/ports"
	"context"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type S3Provider struct {
	client       *minio.Client
	bucket       string
	externalHost string
}

func NewS3Provider(cfg *configs.Config) (ports.FileProvider, error) {
	if cfg.S3.Endpoint == "" {
		return nil, fmt.Errorf("S3 endpoint is required")
	}
	if cfg.S3.AccessKey == "" {
		return nil, fmt.Errorf("S3 access key is required")
	}
	if cfg.S3.SecretKey == "" {
		return nil, fmt.Errorf("S3 secret key is required")
	}
	if cfg.S3.Bucket == "" {
		return nil, fmt.Errorf("S3 bucket is required")
	}

	endpoint := cfg.S3.Endpoint
	useSSL := cfg.S3.UseSSL

	if parsedURL, err := url.Parse(cfg.S3.Endpoint); err == nil && parsedURL.Host != "" {
		endpoint = parsedURL.Host
		switch parsedURL.Scheme {
		case "https":
			useSSL = true
		case "http":
			useSSL = false
		}
	}

	client, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.S3.AccessKey, cfg.S3.SecretKey, ""),
		Secure: useSSL,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create minio client: %w", err)
	}

	return &S3Provider{
		client:       client,
		bucket:       cfg.S3.Bucket,
		externalHost: cfg.S3.ExternalHost,
	}, nil
}

func (p *S3Provider) GenerateUploadURL(ctx context.Context, s3Path string, contentType string, maxSize int64, ttl time.Duration) (string, error) {
	reqParams := make(url.Values)
	extraHeaders := make(http.Header)

	extraHeaders.Set("Content-Type", contentType)

	if maxSize > 0 {
		reqParams.Set("x-amz-content-length-range", fmt.Sprintf("0,%d", maxSize))
	}

	presignedURL, err := p.client.PresignHeader(ctx, http.MethodPut, p.bucket, s3Path, ttl, reqParams, extraHeaders)
	if err != nil {
		return "", fmt.Errorf("failed to generate upload URL: %w", err)
	}

	if p.externalHost != "" {
		presignedURL.Host = p.externalHost
		if presignedURL.Scheme == "http" {
			presignedURL.Scheme = "https"
		}
	}

	return presignedURL.String(), nil
}

func (p *S3Provider) GenerateDownloadURL(ctx context.Context, s3Path string, ttl time.Duration) (string, error) {
	presignedURL, err := p.client.PresignedGetObject(ctx, p.bucket, s3Path, ttl, nil)
	if err != nil {
		return "", fmt.Errorf("failed to generate download URL: %w", err)
	}

	if p.externalHost != "" {
		presignedURL.Host = p.externalHost
		if presignedURL.Scheme == "http" {
			presignedURL.Scheme = "https"
		}
	}

	return presignedURL.String(), nil
}
