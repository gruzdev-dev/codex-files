//go:build integration

package tests

import (
	"context"
	"io"
	"log"
	"os"
	"testing"

	grpcAdapter "codex-files/adapters/grpc"
	httpAdapter "codex-files/adapters/http"
	postgresAdapter "codex-files/adapters/storage/postgres"
	"codex-files/configs"
	"codex-files/core/ports"
	"codex-files/core/services"
	"codex-files/migrations"
	grpcServer "codex-files/servers/grpc"

	"net"
	"net/http/httptest"
	"time"

	"github.com/gorilla/mux"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
	"go.uber.org/dig"
	"go.uber.org/mock/gomock"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"

	"codex-files/api/proto"
)

type TestEnv struct {
	Container  *dig.Container
	DB         *pgxpool.Pool
	GRPCClient FilesServiceClient
	ServerURL  string
	S3Mock     *ports.MockFileProvider
	Cleanup    func()
}

type FilesServiceClient interface {
	GenerateUploadUrl(ctx context.Context, req *GenerateUploadUrlRequest, opts ...grpc.CallOption) (*GenerateUploadUrlResponse, error)
	DeleteFile(ctx context.Context, req *DeleteFileRequest, opts ...grpc.CallOption) (*DeleteFileResponse, error)
}

type GenerateUploadUrlRequest struct {
	UserId      string
	ContentType string
	Size        int64
}

type GenerateUploadUrlResponse struct {
	FileId    string
	UploadUrl string
}

type DeleteFileRequest struct {
	FileId string
}

type DeleteFileResponse struct{}

func SetupTestEnv(t *testing.T) *TestEnv {
	ctx := context.Background()

	quietLogger := log.New(io.Discard, "", 0)

	pgContainer, err := postgres.Run(ctx,
		"postgres:16-alpine",
		postgres.WithDatabase("codex_files_test"),
		postgres.WithUsername("testuser"),
		postgres.WithPassword("testpass"),
		postgres.WithSQLDriver("pgx"),
		testcontainers.WithLogger(quietLogger),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(30*time.Second)),
	)
	require.NoError(t, err)

	connectionString, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)

	dbPool, err := pgxpool.New(ctx, connectionString)
	require.NoError(t, err)

	if err := dbPool.Ping(ctx); err != nil {
		t.Fatalf("failed to ping db: %v", err)
	}

	initSchema(ctx, dbPool)

	os.Setenv("HTTP_PORT", "8080")
	os.Setenv("GRPC_PORT", "8081")
	os.Setenv("JWT_SECRET", "test-secret")
	os.Setenv("INTERNAL_SERVICE_SECRET", "test-internal-secret")
	os.Setenv("S3_ENDPOINT", "localhost:9000")
	os.Setenv("S3_ACCESS_KEY", "minioadmin")
	os.Setenv("S3_SECRET_KEY", "minioadmin")
	os.Setenv("S3_BUCKET", "test-bucket")
	os.Setenv("S3_EXTERNAL_HOST", "s3.test.local")
	os.Setenv("S3_USE_SSL", "false")
	os.Setenv("S3_WEBHOOK_SECRET", "test-webhook-secret")
	os.Setenv("UPLOAD_MAX_SIZE", "104857600")
	os.Setenv("UPLOAD_TTL", "5m")
	os.Setenv("DOWNLOAD_TTL", "15m")

	cfg, err := configs.NewConfig()
	require.NoError(t, err)

	cfg.DB.Host = "localhost"
	cfg.DB.Port = "5432"
	cfg.DB.User = "testuser"
	cfg.DB.Password = "testpass"
	cfg.DB.Database = "codex_files_test"

	cfg.Upload.MaxSize = 100 * 1024 * 1024
	cfg.Upload.TTL = 5 * time.Minute
	cfg.Download.TTL = 15 * time.Minute

	container := dig.New()

	if err := container.Provide(func() *configs.Config { return cfg }); err != nil {
		t.Fatalf("failed to provide config: %v", err)
	}

	if err := container.Provide(func() *pgxpool.Pool { return dbPool }); err != nil {
		t.Fatalf("failed to provide db pool: %v", err)
	}

	if err := container.Provide(postgresAdapter.NewFileRepo, dig.As(new(ports.FileRepository))); err != nil {
		t.Fatalf("failed to provide file repo: %v", err)
	}

	ctrl := gomock.NewController(t)
	s3Mock := ports.NewMockFileProvider(ctrl)
	if err := container.Provide(func() ports.FileProvider { return s3Mock }); err != nil {
		t.Fatalf("failed to provide s3 mock: %v", err)
	}

	if err := container.Provide(newFileService); err != nil {
		t.Fatalf("failed to provide file service: %v", err)
	}

	if err := container.Provide(grpcAdapter.NewFilesHandler); err != nil {
		t.Fatalf("failed to provide grpc handler: %v", err)
	}

	if err := container.Provide(httpAdapter.NewHandler); err != nil {
		t.Fatalf("failed to provide http handler: %v", err)
	}

	if err := container.Provide(grpcServer.NewServer); err != nil {
		t.Fatalf("failed to provide grpc server: %v", err)
	}

	var grpcHandler *grpcAdapter.FilesHandler
	var httpHandler *httpAdapter.Handler

	err = container.Invoke(func(
		h *grpcAdapter.FilesHandler,
		httpH *httpAdapter.Handler,
	) {
		grpcHandler = h
		httpHandler = httpH
	})
	require.NoError(t, err)

	const bufSize = 1024 * 1024
	lis := bufconn.Listen(bufSize)

	s := grpc.NewServer()
	proto.RegisterFilesServiceServer(s, grpcHandler)

	go func() {
		if err := s.Serve(lis); err != nil {
			return
		}
	}()

	conn, err := grpc.NewClient("passthrough://bufnet",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
			return lis.Dial()
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)

	grpcClient := &filesServiceClient{
		client: proto.NewFilesServiceClient(conn),
	}

	router := mux.NewRouter()

	api := router.PathPrefix("/api/v1").Subrouter()
	httpHandler.RegisterRoutes(api)

	ts := httptest.NewServer(router)

	cleanup := func() {
		ctrl.Finish()
		s.Stop()
		_ = conn.Close()
		ts.Close()
		dbPool.Close()
		_ = pgContainer.Terminate(context.Background())
	}

	return &TestEnv{
		Container:  container,
		DB:         dbPool,
		GRPCClient: grpcClient,
		ServerURL:  ts.URL,
		S3Mock:     s3Mock,
		Cleanup:    cleanup,
	}
}

func initSchema(ctx context.Context, pool *pgxpool.Pool) {
	migrationSQL, err := migrations.FS.ReadFile("001_init.up.sql")
	if err != nil {
		log.Fatalf("failed to read migration file: %s", err)
	}

	_, err = pool.Exec(ctx, string(migrationSQL))
	if err != nil {
		log.Fatalf("failed to apply migration: %s", err)
	}
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

type filesServiceClient struct {
	client proto.FilesServiceClient
}

func (c *filesServiceClient) GenerateUploadUrl(ctx context.Context, req *GenerateUploadUrlRequest, opts ...grpc.CallOption) (*GenerateUploadUrlResponse, error) {
	resp, err := c.client.GenerateUploadUrl(ctx, &proto.GenerateUploadUrlRequest{
		UserId:      req.UserId,
		ContentType: req.ContentType,
		Size:        req.Size,
	}, opts...)
	if err != nil {
		return nil, err
	}
	return &GenerateUploadUrlResponse{
		FileId:    resp.FileId,
		UploadUrl: resp.UploadUrl,
	}, nil
}

func (c *filesServiceClient) DeleteFile(ctx context.Context, req *DeleteFileRequest, opts ...grpc.CallOption) (*DeleteFileResponse, error) {
	_, err := c.client.DeleteFile(ctx, &proto.DeleteFileRequest{
		FileId: req.FileId,
	}, opts...)
	if err != nil {
		return nil, err
	}
	return &DeleteFileResponse{}, nil
}
