package main

import (
	httpAdapter "codex-files/adapters/http"
	storageAdapter "codex-files/adapters/storage"
	"codex-files/configs"
	"codex-files/core/ports"
	"codex-files/core/service"
	httpServer "codex-files/servers/http"

	"go.uber.org/dig"
)

func BuildContainer() (*dig.Container, error) {
	container := dig.New()

	if err := container.Provide(configs.NewConfig); err != nil {
		return nil, err
	}
	if err := container.Provide(storageAdapter.NewInMemoryRepo); err != nil {
		return nil, err
	}
	if err := container.Provide(service.NewUserService, dig.As(new(ports.UserService))); err != nil {
		return nil, err
	}
	if err := container.Provide(httpAdapter.NewHandler); err != nil {
		return nil, err
	}
	if err := container.Provide(httpServer.NewServer); err != nil {
		return nil, err
	}

	return container, nil
}
