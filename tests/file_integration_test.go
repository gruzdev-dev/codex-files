//go:build integration

package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/gruzdev-dev/codex-files/core/domain"
	"github.com/gruzdev-dev/codex-files/proto"

	"github.com/golang-jwt/jwt/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"google.golang.org/grpc/metadata"
)

const (
	testUserID      = "test-user-123"
	testContentType = "application/pdf"
	testFileSize    = int64(1024 * 1024) // 1MB
	s3Basic         = "http://s3.test.local/test-bucket/"
)

func createTestJWTToken(secret string, userID string, scopes []string) (string, error) {
	claims := jwt.MapClaims{
		"sub":   userID,
		"scope": strings.Join(scopes, " "),
		"exp":   time.Now().Add(time.Hour).Unix(),
		"iat":   time.Now().Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}

func TestFileIntegration(t *testing.T) {
	env := SetupTestEnv(t)
	defer env.Cleanup()

	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	var fileID string
	var uploadURL string
	var downloadURL string

	t.Run("Step 1: Generate Upload URL via gRPC", func(t *testing.T) {
		md := metadata.Pairs("x-internal-token", "test-internal-secret")
		ctx := metadata.NewOutgoingContext(context.Background(), md)

		env.S3Mock.EXPECT().
			GenerateUploadURL(
				gomock.Any(),
				gomock.Any(),
				testContentType,
				int64(104857600),
				gomock.Any(),
			).
			DoAndReturn(func(ctx context.Context, s3Path, contentType string, maxSize int64, ttl time.Duration) (string, error) {
				assert.Contains(t, s3Path, testUserID)
				return s3Basic + s3Path, nil
			}).
			Times(1)

		resp, err := env.GRPCClient.GeneratePresignedUrls(ctx, &proto.GeneratePresignedUrlsRequest{
			UserId:      testUserID,
			ContentType: testContentType,
			Size:        testFileSize,
		})

		require.NoError(t, err)
		require.NotEmpty(t, resp.FileId)
		require.NotEmpty(t, resp.UploadUrl)
		require.NotEmpty(t, resp.DownloadUrl)

		fileID = resp.FileId
		uploadURL = resp.UploadUrl
		downloadURL = resp.DownloadUrl

		assert.Contains(t, uploadURL, "s3.test.local")
		assert.Contains(t, uploadURL, testUserID)
		assert.Contains(t, uploadURL, fileID)
	})

	t.Run("Step 2: Verify file record in database", func(t *testing.T) {
		require.NotEmpty(t, fileID)

		ctx := context.Background()
		file, err := getFileFromDB(ctx, env.DB, fileID)
		require.NoError(t, err)
		require.NotNil(t, file)

		assert.Equal(t, fileID, file.ID)
		assert.Equal(t, testUserID, file.OwnerID)
		assert.Equal(t, testContentType, file.ContentType)
		assert.Equal(t, testFileSize, file.Size)
		assert.Equal(t, domain.FileStatusPending, file.Status)
		assert.False(t, file.IsDeleted)
		assert.Contains(t, file.S3Path, testUserID)
		assert.Contains(t, file.S3Path, fileID)
	})

	t.Run("Step 3: Simulate S3 webhook notification (after upload)", func(t *testing.T) {
		s3Path := testUserID + "/" + fileID
		webhookPayload := map[string]any{
			"Records": []map[string]any{
				{
					"eventVersion": "2.0",
					"eventSource":  "minio:s3",
					"awsRegion":    "",
					"eventTime":    time.Now().Format(time.RFC3339),
					"eventName":    "s3:ObjectCreated:Put",
					"userIdentity": map[string]any{
						"principalId": "minio",
					},
					"requestParameters": map[string]string{
						"sourceIPAddress": "127.0.0.1",
					},
					"responseElements": map[string]string{
						"x-amz-request-id": "test-request-id",
					},
					"s3": map[string]any{
						"s3SchemaVersion": "1.0",
						"configurationId": "test-config",
						"bucket": map[string]any{
							"name":          "test-bucket",
							"ownerIdentity": map[string]any{"principalId": "minio"},
							"arn":           "arn:aws:s3:::test-bucket",
						},
						"object": map[string]any{
							"key":       s3Path,
							"size":      testFileSize,
							"eTag":      "test-etag",
							"sequencer": "test-sequencer",
						},
					},
					"source": map[string]any{
						"host":      "127.0.0.1",
						"port":      "9000",
						"userAgent": "MinIO",
					},
				},
			},
		}

		payloadBytes, err := json.Marshal(webhookPayload)
		require.NoError(t, err)

		req, err := http.NewRequest("POST", env.ServerURL+"/api/v1/webhook/s3", bytes.NewReader(payloadBytes))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Codex-Webhook-Secret", "test-webhook-secret")

		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		ctx := context.Background()
		file, err := getFileFromDB(ctx, env.DB, fileID)
		require.NoError(t, err)
		require.NotNil(t, file)

		assert.Equal(t, domain.FileStatusUploaded, file.Status)
		assert.Equal(t, fileID, file.ID)
	})

	t.Run("Step 4: Get Download URL via HTTP", func(t *testing.T) {
		token, err := createTestJWTToken("test-secret", testUserID, []string{})
		require.NoError(t, err)

		env.S3Mock.EXPECT().
			GenerateDownloadURL(
				gomock.Any(),
				gomock.Any(),
				gomock.Any(),
			).
			DoAndReturn(func(ctx context.Context, s3Path string, ttl time.Duration) (string, error) {
				assert.Contains(t, s3Path, testUserID)
				assert.Contains(t, s3Path, fileID)
				return s3Basic + s3Path, nil
			}).
			Times(1)

		req, err := http.NewRequest("GET", env.ServerURL+"/api/v1/files/"+fileID+"/download", nil)
		require.NoError(t, err)
		req.Header.Set("Authorization", "Bearer "+token)

		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusFound, resp.StatusCode, "Expected 302 redirect")
		downloadURL = resp.Header.Get("Location")
		require.NotEmpty(t, downloadURL)
		assert.Contains(t, downloadURL, "s3.test.local")
		assert.Contains(t, downloadURL, testUserID)
		assert.Contains(t, downloadURL, fileID)
	})

	t.Run("Step 5: Delete file via gRPC", func(t *testing.T) {
		md := metadata.Pairs("x-internal-token", "test-internal-secret")
		ctx := metadata.NewOutgoingContext(context.Background(), md)

		_, err := env.GRPCClient.DeleteFile(ctx, &proto.DeleteFileRequest{
			FileId: fileID,
		})
		require.NoError(t, err)
	})

	t.Run("Step 6: Verify file is soft-deleted", func(t *testing.T) {
		ctx := context.Background()
		file, err := getFileFromDB(ctx, env.DB, fileID)
		require.NoError(t, err)
		require.NotNil(t, file)

		assert.True(t, file.IsDeleted)
		assert.Equal(t, fileID, file.ID)
		assert.Equal(t, testUserID, file.OwnerID)
	})

	t.Run("Step 7: Verify file is not accessible after deletion", func(t *testing.T) {
		token, err := createTestJWTToken("test-secret", testUserID, []string{})
		require.NoError(t, err)

		req, err := http.NewRequest("GET", env.ServerURL+"/api/v1/files/"+fileID+"/download", nil)
		require.NoError(t, err)
		req.Header.Set("Authorization", "Bearer "+token)

		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	})
}

func getFileFromDB(ctx context.Context, pool *pgxpool.Pool, fileID string) (*domain.File, error) {
	query := `SELECT id, owner_id, s3_path, size, content_type, status, is_deleted, created_at, updated_at 
	          FROM files 
	          WHERE id = $1`

	var file domain.File
	var statusStr string
	err := pool.QueryRow(ctx, query, fileID).Scan(
		&file.ID,
		&file.OwnerID,
		&file.S3Path,
		&file.Size,
		&file.ContentType,
		&statusStr,
		&file.IsDeleted,
		&file.CreatedAt,
		&file.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	file.Status = domain.FileStatus(statusStr)
	return &file, nil
}
