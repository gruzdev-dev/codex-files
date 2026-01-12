package main

import (
	httpAdapter "codex-files/adapters/http"
	"codex-files/configs"
	grpcServer "codex-files/servers/grpc"
	httpServer "codex-files/servers/http"

	"go.uber.org/dig"
)

func BuildContainer() (*dig.Container, error) {
	container := dig.New()

	if err := container.Provide(configs.NewConfig); err != nil {
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
