package http

import (
	"codex-files/adapters/http"
	"codex-files/configs"
	middleware "codex-files/middleware/http"
	"context"
	"fmt"
	"log"
	nethttp "net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gorilla/mux"
)

type Server struct {
	cfg     *configs.Config
	handler *http.Handler
}

func NewServer(cfg *configs.Config, handler *http.Handler) *Server {
	return &Server{
		cfg:     cfg,
		handler: handler,
	}
}

func (s *Server) Start() error {
	router := mux.NewRouter()
	router.Use(middleware.Logging())
	
	// Health check endpoints for Kubernetes
	router.HandleFunc("/healthz", func(w nethttp.ResponseWriter, r *nethttp.Request) {
		w.WriteHeader(nethttp.StatusOK)
		w.Write([]byte("OK"))
	}).Methods("GET")
	
	router.HandleFunc("/readyz", func(w nethttp.ResponseWriter, r *nethttp.Request) {
		w.WriteHeader(nethttp.StatusOK)
		w.Write([]byte("OK"))
	}).Methods("GET")
	
	s.handler.RegisterRoutes(router)

	srv := &nethttp.Server{
		Addr:    ":" + s.cfg.Server.Port,
		Handler: router,
	}

	serverErrors := make(chan error, 1)
	go func() {
		log.Printf("Starting server on port %s", s.cfg.Server.Port)
		serverErrors <- srv.ListenAndServe()
	}()

	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM)

	select {
	case err := <-serverErrors:
		return fmt.Errorf("server error: %w", err)
	case sig := <-shutdown:
		log.Printf("shutting down server... signal: %v", sig)
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		if err := srv.Shutdown(ctx); err != nil {
			return fmt.Errorf("server forced to shutdown: %w", err)
		}
	}

	log.Println("server exited")
	return nil
}
