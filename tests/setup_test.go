//go:build integration

package tests

import (
	"context"
	"io"
	"log"
	"net"
	"net/http/httptest"
	"testing"
	"time"

	grpcAdapter "github.com/gruzdev-dev/codex-files/adapters/grpc"
	httpAdapter "github.com/gruzdev-dev/codex-files/adapters/http"
	postgresAdapter "github.com/gruzdev-dev/codex-files/adapters/storage/postgres"
	"github.com/gruzdev-dev/codex-files/configs"
	"github.com/gruzdev-dev/codex-files/core/ports"
	"github.com/gruzdev-dev/codex-files/core/services"
	"github.com/gruzdev-dev/codex-files/migrations"
	"github.com/gruzdev-dev/codex-files/proto"
	grpcServer "github.com/gruzdev-dev/codex-files/servers/grpc"

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
)

type TestEnv struct {
	Container  *dig.Container
	DB         *pgxpool.Pool
	GRPCClient proto.FilesServiceClient
	ServerURL  string
	S3Mock     *ports.MockFileProvider
	Cleanup    func()
}

func SetupTestEnv(t *testing.T) *TestEnv {
	ctx := context.Background()

	// create logger
	logger := log.New(io.Discard, "", 0)

	// create db pool and postgres container
	dbPool, pgContainer := newPool(t, ctx, logger)

	// create config
	config := newConfig(t, ctx, pgContainer)

	// create gomock controller
	ctrl := gomock.NewController(t)

	// create s3 mock (do not create Minio instance)
	s3Mock := ports.NewMockFileProvider(ctrl)

	// create container
	container := newContainer(t, config, dbPool, s3Mock)

	// create pointer to grpc and http handlers
	var grpcHandler *grpcAdapter.FilesHandler
	var httpHandler *httpAdapter.Handler

	err := container.Invoke(func(
		grpcH *grpcAdapter.FilesHandler,
		httpH *httpAdapter.Handler,
	) {
		grpcHandler = grpcH
		httpHandler = httpH
	})
	require.NoError(t, err)

	// create grpc server
	const bufSize = 1024 * 1024
	lis := bufconn.Listen(bufSize)
	s := grpc.NewServer()
	proto.RegisterFilesServiceServer(s, grpcHandler)

	go func() {
		if err := s.Serve(lis); err != nil {
			log.Printf("Server exited with error: %v", err)
		}
	}()

	// create grpc client
	conn, err := grpc.NewClient("passthrough://bufnet",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
			return lis.Dial()
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)
	grpcClient := proto.NewFilesServiceClient(conn)

	// http test server
	router := mux.NewRouter()
	httpHandler.RegisterRoutes(router.PathPrefix("/api/v1").Subrouter())
	ts := httptest.NewServer(router)

	return &TestEnv{
		Container:  container,
		DB:         dbPool,
		GRPCClient: grpcClient,
		ServerURL:  ts.URL,
		S3Mock:     s3Mock,
		Cleanup: func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			s.GracefulStop()
			_ = conn.Close()
			ts.Close()
			dbPool.Close()
			_ = pgContainer.Terminate(ctx)
			ctrl.Finish()
		},
	}
}

func newContainer(t *testing.T, config *configs.Config, pool *pgxpool.Pool, s3Mock *ports.MockFileProvider) *dig.Container {
	container := dig.New()

	if err := container.Provide(func() *configs.Config { return config }); err != nil {
		t.Fatalf("failed to provide config: %v", err)
	}

	if err := container.Provide(func() *pgxpool.Pool { return pool }); err != nil {
		t.Fatalf("failed to provide db pool: %v", err)
	}

	if err := container.Provide(postgresAdapter.NewFileRepo, dig.As(new(ports.FileRepository))); err != nil {
		t.Fatalf("failed to provide file repo: %v", err)
	}

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

	return container
}

func newPool(t *testing.T, ctx context.Context, logger *log.Logger) (*pgxpool.Pool, *postgres.PostgresContainer) {
	pgContainer, err := postgres.Run(ctx,
		"postgres:16-alpine",
		postgres.WithDatabase("codex_files_test"),
		postgres.WithUsername("testuser"),
		postgres.WithPassword("testpass"),
		postgres.WithSQLDriver("pgx"),
		testcontainers.WithLogger(logger),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(30*time.Second)),
	)

	if err != nil {
		t.Fatalf("failed to run postgres container: %v", err)
	}

	connectionString, err := pgContainer.ConnectionString(ctx, "sslmode=disable")

	if err != nil {
		t.Fatalf("failed to get connection string: %v", err)
	}

	pool, err := pgxpool.New(ctx, connectionString)
	if err != nil {
		t.Fatalf("failed to create pool: %v", err)
	}

	if err := pool.Ping(ctx); err != nil {
		t.Fatalf("failed to ping db: %v", err)
	}

	migrationSQL, err := migrations.FS.ReadFile("001_init.up.sql")
	if err != nil {
		t.Fatalf("failed to read migration file: %s", err)
	}

	_, err = pool.Exec(ctx, string(migrationSQL))
	if err != nil {
		t.Fatalf("failed to apply migration: %s", err)
	}

	return pool, pgContainer
}

func newConfig(t *testing.T, ctx context.Context, pgContainer *postgres.PostgresContainer) *configs.Config {
	host, err := pgContainer.Host(ctx)
	if err != nil {
		t.Fatalf("failed to get host: %v", err)
	}

	port, err := pgContainer.MappedPort(ctx, "5432")
	if err != nil {
		t.Fatalf("failed to get mapped port: %v", err)
	}

	cfg, err := configs.NewConfig()

	if err != nil {
		t.Fatalf("failed to create config: %v", err)
	}

	cfg.HTTP.Port = "8080"
	cfg.GRPC.Port = "8081"
	cfg.Auth.JWTSecret = "test-secret"
	cfg.Auth.InternalSecret = "test-internal-secret"
	cfg.DB.Host = host
	cfg.DB.Port = port.Port()
	cfg.DB.User = "testuser"
	cfg.DB.Password = "testpass"
	cfg.DB.Database = "codex_files_test"
	cfg.S3.Endpoint = "localhost:9000"
	cfg.S3.AccessKey = "minioadmin"
	cfg.S3.SecretKey = "minioadmin"
	cfg.S3.Bucket = "test-bucket"
	cfg.S3.ExternalHost = "s3.test.local"
	cfg.S3.UseSSL = false
	cfg.S3.WebhookSecret = "test-webhook-secret"
	cfg.Upload.MaxSize = 100 * 1024 * 1024
	cfg.Upload.TTL = 5 * time.Minute
	cfg.Download.TTL = 15 * time.Minute

	return cfg
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
