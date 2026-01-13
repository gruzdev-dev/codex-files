package http

import (
	"codex-files/configs"
	"codex-files/core/domain"
	"codex-files/core/services"
	"codex-files/pkg/identity"
	"encoding/json"
	"log"
	"net/http"
	"net/url"
	"strings"

	"github.com/gorilla/mux"
	"github.com/minio/minio-go/v7/pkg/notification"
)

type Handler struct {
	cfg         *configs.Config
	fileService *services.FileService
}

func NewHandler(cfg *configs.Config, fileService *services.FileService) *Handler {
	return &Handler{
		cfg:         cfg,
		fileService: fileService,
	}
}

func (h *Handler) RegisterRoutes(api *mux.Router) {
	webhookAuthMiddleware := NewWebhookAuthMiddleware(h.cfg.S3.WebhookSecret)
	webhookHandler := webhookAuthMiddleware.Handler(http.HandlerFunc(h.HandleS3Webhook))
	api.Handle("/webhook/s3", webhookHandler).Methods("POST")

	authMiddleware := NewAuthMiddleware(h.cfg.Auth.JWTSecret)
	downloadHandler := authMiddleware.Handler(http.HandlerFunc(h.GetDownloadURL))
	api.Handle("/files/{file_id}/download", downloadHandler).Methods("GET")
}

func (h *Handler) GetDownloadURL(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	fileID := vars["file_id"]
	if fileID == "" {
		http.Error(w, "file_id is required", http.StatusBadRequest)
		return
	}

	user, ok := identity.FromCtx(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	result, err := h.fileService.GetDownloadURL(r.Context(), fileID, user.UserID, user.Scopes)
	if err != nil {
		if err == domain.ErrFileNotFound || err == domain.ErrFileIDRequired {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		if err == domain.ErrAccessDenied {
			http.Error(w, err.Error(), http.StatusForbidden)
			return
		}
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, result.DownloadURL, http.StatusFound)
}

func (h *Handler) HandleS3Webhook(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var info notification.Info
	if err := json.NewDecoder(r.Body).Decode(&info); err != nil {
		log.Printf("failed to decode webhook event: %v", err)
		w.WriteHeader(http.StatusOK)
		return
	}

	for _, record := range info.Records {
		if !strings.HasPrefix(record.EventName, "s3:ObjectCreated:") {
			continue
		}

		key := record.S3.Object.Key
		if key == "" {
			continue
		}

		decodedKey, err := url.QueryUnescape(key)
		if err != nil {
			decodedKey = key
		}

		parts := strings.Split(decodedKey, "/")
		if len(parts) < 2 {
			log.Printf("invalid s3 key format: %s", decodedKey)
			continue
		}
		fileID := parts[len(parts)-1]

		if err := h.fileService.ConfirmUpload(r.Context(), fileID); err != nil {
			log.Printf("failed to confirm upload for file %s: %v", fileID, err)
		}
	}

	w.WriteHeader(http.StatusOK)
}
