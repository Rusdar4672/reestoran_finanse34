package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/yourusername/restaurant-finance/internal/bootstrap"
	"github.com/yourusername/restaurant-finance/internal/config"
)

func main() {
	cfg := config.LoadConfig()
	app, err := bootstrap.New(cfg, cfg.AllowedOrigin)
	if err != nil {
		log.Fatal(err)
	}
	defer app.Close()
	server := &http.Server{
		Addr:              ":" + cfg.ServerPort,
		Handler:           app.Router,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      2 * time.Minute,
		IdleTimeout:       2 * time.Minute,
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
	}()
	log.Printf("Restaurant Finance API listening on %s", server.Addr)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}
