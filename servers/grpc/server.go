package grpc

import (
	"context"
	"fmt"
	"net"

	grpcAdapter "github.com/gruzdev-dev/codex-files/adapters/grpc"
	"github.com/gruzdev-dev/codex-files/configs"
	"github.com/gruzdev-dev/codex-files/proto"

	"google.golang.org/grpc"
)

type Server struct {
	cfg        *configs.Config
	grpcServer *grpc.Server
}

func NewServer(cfg *configs.Config, handler *grpcAdapter.FilesHandler) *Server {
	opts := []grpc.ServerOption{
		grpc.UnaryInterceptor(grpcAdapter.AuthInterceptor(cfg.Auth.InternalSecret)),
	}

	s := grpc.NewServer(opts...)
	proto.RegisterFilesServiceServer(s, handler)

	return &Server{
		cfg:        cfg,
		grpcServer: s,
	}
}

func (s *Server) Start(ctx context.Context) error {
	addr := fmt.Sprintf(":%s", s.cfg.GRPC.Port)
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", addr, err)
	}

	fmt.Printf("gRPC server is running on %s\n", addr)
	errCh := make(chan error, 1)
	go func() {
		if err := s.grpcServer.Serve(lis); err != nil {
			errCh <- err
		}
	}()

	select {
	case <-ctx.Done():
		s.Stop()
		return nil
	case err := <-errCh:
		return fmt.Errorf("gRPC server error: %w", err)
	}
}

func (s *Server) Stop() {
	fmt.Println("Stopping gRPC server...")
	s.grpcServer.GracefulStop()
}
