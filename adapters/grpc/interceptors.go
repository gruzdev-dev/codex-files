package grpc

import (
	"context"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

func AuthInterceptor(secret string) grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req any,
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (any, error) {
		md, ok := metadata.FromIncomingContext(ctx)
		if !ok {
			return nil, status.Error(codes.Unauthenticated, "metadata is missing")
		}

		values := md.Get("x-internal-token")
		if len(values) == 0 {
			return nil, status.Error(codes.Unauthenticated, "authorization token is missing")
		}

		if values[0] != secret {
			return nil, status.Error(codes.Unauthenticated, "invalid internal token")
		}

		return handler(ctx, req)
	}
}
