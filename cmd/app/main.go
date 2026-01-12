package main

import (
	"context"
	"golang.org/x/sync/errgroup"
	"log"
	"os/signal"
	"syscall"

	grpcServer "codex-files/servers/grpc"
	httpServer "codex-files/servers/http"
)

func main() {
	container, err := BuildContainer()
	if err != nil {
		log.Fatalf("Fatal error building container: %v", err)
	}

	err = container.Invoke(func(
		httpSrv *httpServer.Server,
		grpcSrv *grpcServer.Server,
	) error {
		ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
		defer stop()

		g, ctx := errgroup.WithContext(ctx)

		g.Go(func() error {
			return httpSrv.Start()
		})

		g.Go(func() error {
			return grpcSrv.Start(ctx)
		})

		return g.Wait()
	})

	if err != nil {
		log.Fatalf("Application stopped with error: %v", err)
	}
}
