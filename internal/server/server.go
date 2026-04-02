package server

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	chiMiddleware "github.com/go-chi/chi/v5/middleware"

	"github.com/Alaghal/go-api-gateway/internal/config"
	"github.com/Alaghal/go-api-gateway/internal/handlers"
	appMiddleware "github.com/Alaghal/go-api-gateway/internal/middleware"
)

type Server struct {
	httpServer *http.Server
	cfg        config.Config
}

func New(cfg config.Config) *Server {
	router := newRouter()

	httpServer := &http.Server{
		Addr:              fmt.Sprintf(":%d", cfg.Port),
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	return &Server{
		httpServer: httpServer,
		cfg:        cfg,
	}
}

func newRouter() http.Handler {
	router := chi.NewRouter()

	router.Use(chiMiddleware.Timeout(10 * time.Second))
	router.Use(appMiddleware.RequestID)
	router.Use(appMiddleware.Logging)
	router.Use(appMiddleware.Recovery)

	router.Get("/health", handlers.Health())

	router.Route("/api", func(r chi.Router) {
		r.Route("/v1", func(r chi.Router) {
			r.Get("/ping", handlers.Ping())
		})
	})

	return router
}

func (s *Server) Run(ctx context.Context) error {
	errCh := make(chan error, 1)

	go func() {
		log.Printf("starting go-api-gateway on port %d (env=%s)", s.cfg.Port, s.cfg.AppEnv)
		if err := s.httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	select {
	case <-ctx.Done():
		log.Println("shutdown signal received")

		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if err := s.httpServer.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("graceful shutdown failed: %w", err)
		}

		log.Println("server stopped gracefully")
		return nil

	case err := <-errCh:
		return fmt.Errorf("http server failed: %w", err)
	}
}
