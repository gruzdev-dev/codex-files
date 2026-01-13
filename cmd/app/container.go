package main

import (
	"context"

	grpcAdapter "codex-files/adapters/grpc"
	httpAdapter "codex-files/adapters/http"
	postgresAdapter "codex-files/adapters/storage/postgres"
	s3Adapter "codex-files/adapters/storage/s3"
	"codex-files/configs"
	"codex-files/core/ports"
	"codex-files/core/services"
	grpcServer "codex-files/servers/grpc"
	httpServer "codex-files/servers/http"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/dig"
)

func BuildContainer() (*dig.Container, error) {
	container := dig.New()

	if err := container.Provide(configs.NewConfig); err != nil {
		return nil, err
	}

	if err := container.Provide(newDBPool); err != nil {
		return nil, err
	}

	if err := container.Provide(postgresAdapter.NewFileRepo, dig.As(new(ports.FileRepository))); err != nil {
		return nil, err
	}

	if err := container.Provide(s3Adapter.NewS3Provider, dig.As(new(ports.FileProvider))); err != nil {
		return nil, err
	}

	if err := container.Provide(newFileService); err != nil {
		return nil, err
	}

	if err := container.Provide(grpcAdapter.NewFilesHandler); err != nil {
		return nil, err
	}

	if err := container.Provide(httpAdapter.NewHandler); err != nil {
		return nil, err
	}

	if err := container.Provide(httpServer.NewServer); err != nil {
		return nil, err
	}

	if err := container.Provide(grpcServer.NewServer); err != nil {
		return nil, err
	}

	return container, nil
}

func newDBPool(cfg *configs.Config) (*pgxpool.Pool, error) {
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, cfg.DatabaseURL())
	if err != nil {
		return nil, err
	}
	return pool, nil
}

func newFileService(
	repo ports.FileRepository,
	fileProvider ports.FileProvider,
	cfg *configs.Config,
) *services.FileService {
	return services.NewFileService(
		repo,
		fileProvider,
		cfg.Upload.MaxSize,
		cfg.Upload.TTL,
		cfg.Download.TTL,
	)
}
