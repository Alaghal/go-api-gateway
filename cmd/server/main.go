package main

import (
	"context"
	"log"
	"os/signal"
	"syscall"

	"github.com/Alaghal/go-api-gateway/internal/config"
	"github.com/Alaghal/go-api-gateway/internal/server"
)

func main() {
	cfg := config.MustLoad()

	srv := server.New(cfg)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := srv.Run(ctx); err != nil {
		log.Fatalf("server stopped with error: %v", err)
	}
}
