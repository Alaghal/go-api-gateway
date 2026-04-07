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
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/Alaghal/go-api-gateway/internal/config"
	"github.com/Alaghal/go-api-gateway/internal/handlers"
	appMetrics "github.com/Alaghal/go-api-gateway/internal/metrics"
	appMiddleware "github.com/Alaghal/go-api-gateway/internal/middleware"
	"github.com/Alaghal/go-api-gateway/internal/proxy"
)

type Server struct {
	httpServer *http.Server
	cfg        config.Config
}

func New(cfg config.Config) *Server {
	router := newRouter(cfg)

	httpServer := &http.Server{
		Addr:              fmt.Sprintf(":%d", cfg.Port),
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	return &Server{
		httpServer: httpServer,
		cfg:        cfg,
	}
}

func newRouter(cfg config.Config) http.Handler {
	router := chi.NewRouter()

	metricsRegistry := appMetrics.MustNew()
	rateLimiter := appMiddleware.NewRateLimiter(cfg.RateLimitRPS, cfg.RateLimitBurst)

	router.Use(chiMiddleware.Timeout(15 * time.Second))
	router.Use(appMiddleware.RequestID)
	router.Use(appMiddleware.Recovery)
	router.Use(appMiddleware.Logging)
	router.Use(appMiddleware.Metrics(metricsRegistry))
	router.Use(rateLimiter.Middleware)

	router.Get("/health", handlers.Health())
	router.Handle("/metrics", promhttp.Handler())

	router.Route("/api", func(r chi.Router) {
		r.Route("/v1", func(r chi.Router) {
			r.Get("/ping", handlers.Ping())
		})
	})

	authProxy, err := proxy.NewReverseProxy(cfg.AuthServiceURL, cfg.UpstreamTimeout)
	if err != nil {
		panic(fmt.Errorf("init auth reverse proxy: %w", err))
	}

	log.Printf("auth-service proxy configured target=%s", authProxy.Target())

	router.Handle("/api/v1/auth/*", authProxy)
	router.Handle("/api/v1/users/*", authProxy)

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
