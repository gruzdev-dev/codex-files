package http

import (
	"codex-files/core/ports"
	"fmt"
	"net/http"

	"github.com/gorilla/mux"
)

type Handler struct {
	service ports.UserService
}

func NewHandler(service ports.UserService) *Handler {
	return &Handler{
		service: service,
	}
}

func (h *Handler) RegisterRoutes(router *mux.Router) {
	router.HandleFunc("/", h.HealthCheck).Methods("GET")
}

func (h *Handler) HealthCheck(w http.ResponseWriter, r *http.Request) {
	status := h.service.HealthCheck()
	w.WriteHeader(http.StatusOK)
	fmt.Fprintln(w, status)
}
