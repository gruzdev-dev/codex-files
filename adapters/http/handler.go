package http

import (
	"codex-files/core/domain"
	"codex-files/core/services"
	"codex-files/pkg/identity"
	"fmt"
	"net/http"

	"github.com/gorilla/mux"
)

type Handler struct {
	fileService *services.FileService
}

func NewHandler(fileService *services.FileService) *Handler {
	return &Handler{
		fileService: fileService,
	}
}

func (h *Handler) RegisterRoutes(router *mux.Router) {
	router.HandleFunc("/", h.HealthCheck).Methods("GET")
	router.HandleFunc("/api/v1/files/{file_id}/download", h.GetDownloadURL).Methods("GET")
}

func (h *Handler) HealthCheck(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = fmt.Fprintln(w, "OK")
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
