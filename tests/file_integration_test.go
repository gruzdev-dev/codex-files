//go:build integration

package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"codex-files/core/domain"

	"strings"

	"github.com/golang-jwt/jwt/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"google.golang.org/grpc/metadata"
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

	const testUserID = "test-user-123"
	const testContentType = "application/pdf"
	const testFileSize = int64(1024 * 1024) // 1MB

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
				return "http://s3.test.local/test-bucket/" + s3Path + "?X-Amz-Algorithm=...", nil
			}).
			Times(1)

		resp, err := env.GRPCClient.GenerateUploadUrl(ctx, &GenerateUploadUrlRequest{
			UserId:      testUserID,
			ContentType: testContentType,
			Size:        testFileSize,
		})
		require.NoError(t, err)
		require.NotEmpty(t, resp.FileId)
		require.NotEmpty(t, resp.UploadUrl)

		fileID = resp.FileId
		uploadURL = resp.UploadUrl

		assert.Contains(t, uploadURL, "s3.test.local")
		assert.Contains(t, uploadURL, testUserID)
		assert.Contains(t, uploadURL, fileID)

		t.Logf("Generated file ID: %s", fileID)
		t.Logf("Generated upload URL: %s", uploadURL)
	})

	t.Run("Step 2: Verify file record in database", func(t *testing.T) {
		require.NotEmpty(t, fileID, "File ID should be set from previous step")

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

		t.Logf("File record verified: %+v", file)
	})

	t.Run("Step 3: Simulate file upload to S3", func(t *testing.T) {
		require.NotEmpty(t, uploadURL, "Upload URL should be set from previous step")

		req, err := http.NewRequest("PUT", uploadURL, nil)
		require.NoError(t, err)
		req.Header.Set("Content-Type", testContentType)
		req.Header.Set("Content-Length", "1048576")

		resp, err := client.Do(req)
		if err != nil {
			t.Logf("Expected error when uploading to mocked S3: %v", err)
		} else {
			resp.Body.Close()
		}

		ctx := context.Background()
		file, err := getFileFromDB(ctx, env.DB, fileID)
		require.NoError(t, err)

		assert.Equal(t, domain.FileStatusPending, file.Status)
		t.Logf("File upload simulated (S3 is mocked, so actual upload would fail)")
	})

	t.Run("Step 4: Get Download URL via HTTP", func(t *testing.T) {
		require.NotEmpty(t, fileID, "File ID should be set from previous step")

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
				return "http://s3.test.local/test-bucket/" + s3Path + "?X-Amz-Algorithm=...", nil
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

		t.Logf("Download URL: %s", downloadURL)
	})

	t.Run("Step 5: Simulate S3 webhook notification", func(t *testing.T) {
		require.NotEmpty(t, fileID, "File ID should be set from previous step")

		s3Path := testUserID + "/" + fileID
		webhookPayload := map[string]interface{}{
			"Records": []map[string]interface{}{
				{
					"eventVersion": "2.0",
					"eventSource":  "minio:s3",
					"awsRegion":    "",
					"eventTime":    time.Now().Format(time.RFC3339),
					"eventName":    "s3:ObjectCreated:Put",
					"userIdentity": map[string]interface{}{
						"principalId": "minio",
					},
					"requestParameters": map[string]string{
						"sourceIPAddress": "127.0.0.1",
					},
					"responseElements": map[string]string{
						"x-amz-request-id": "test-request-id",
					},
					"s3": map[string]interface{}{
						"s3SchemaVersion": "1.0",
						"configurationId": "test-config",
						"bucket": map[string]interface{}{
							"name":          "test-bucket",
							"ownerIdentity": map[string]interface{}{"principalId": "minio"},
							"arn":           "arn:aws:s3:::test-bucket",
						},
						"object": map[string]interface{}{
							"key":       s3Path,
							"size":      testFileSize,
							"eTag":      "test-etag",
							"sequencer": "test-sequencer",
						},
					},
					"source": map[string]interface{}{
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

		assert.Equal(t, http.StatusOK, resp.StatusCode, "Webhook should return 200 OK")

		ctx := context.Background()
		file, err := getFileFromDB(ctx, env.DB, fileID)
		require.NoError(t, err)
		require.NotNil(t, file)

		assert.Equal(t, domain.FileStatusUploaded, file.Status, "File status should be updated to uploaded")
		assert.Equal(t, fileID, file.ID)

		t.Logf("Webhook notification processed successfully, file status updated to: %s", file.Status)
	})

	t.Run("Step 6: Verify download URL access (simulated)", func(t *testing.T) {
		require.NotEmpty(t, downloadURL, "Download URL should be set from previous step")

		req, err := http.NewRequest("GET", downloadURL, nil)
		require.NoError(t, err)

		resp, err := client.Do(req)
		if err != nil {
			t.Logf("Expected error when downloading from mocked S3: %v", err)
		} else {
			resp.Body.Close()
		}

		t.Logf("Download access simulated (S3 is mocked, so actual download would fail)")
	})

	t.Run("Step 7: Delete file via gRPC", func(t *testing.T) {
		require.NotEmpty(t, fileID, "File ID should be set from previous step")

		md := metadata.Pairs("x-internal-token", "test-internal-secret")
		ctx := metadata.NewOutgoingContext(context.Background(), md)

		_, err := env.GRPCClient.DeleteFile(ctx, &DeleteFileRequest{
			FileId: fileID,
		})
		require.NoError(t, err)

		t.Logf("File deleted successfully")
	})

	t.Run("Step 7: Verify file is soft-deleted", func(t *testing.T) {
		require.NotEmpty(t, fileID, "File ID should be set from previous step")

		ctx := context.Background()
		file, err := getFileFromDB(ctx, env.DB, fileID)
		require.NoError(t, err)
		require.NotNil(t, file)

		assert.True(t, file.IsDeleted, "File should be marked as deleted")
		assert.Equal(t, fileID, file.ID)
		assert.Equal(t, testUserID, file.OwnerID)

		t.Logf("File soft-delete verified: IsDeleted=%v", file.IsDeleted)
	})

	t.Run("Step 9: Verify file is not accessible after deletion", func(t *testing.T) {
		require.NotEmpty(t, fileID, "File ID should be set from previous step")

		token, err := createTestJWTToken("test-secret", testUserID, []string{})
		require.NoError(t, err)

		req, err := http.NewRequest("GET", env.ServerURL+"/api/v1/files/"+fileID+"/download", nil)
		require.NoError(t, err)
		req.Header.Set("Authorization", "Bearer "+token)

		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusNotFound, resp.StatusCode, "Deleted file should not be accessible")
		t.Logf("File access denied after deletion (404) as expected")
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
