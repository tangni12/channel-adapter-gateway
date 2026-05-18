package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"channel-adapter-gateway/internal/config"
	"channel-adapter-gateway/internal/database"
	"channel-adapter-gateway/internal/router"
	"channel-adapter-gateway/internal/service"
)

func main() {
	cfg, err := config.Load("")
	if err != nil {
		log.Fatalf("load config failed: %v", err)
	}

	db, err := database.Open(cfg.Database)
	if err != nil {
		log.Fatalf("connect database failed: %v", err)
	}
	if cfg.Database.AutoMigrate {
		if err := database.AutoMigrate(db); err != nil {
			log.Fatalf("auto migrate failed: %v", err)
		}
	}
	if err := database.Seed(db, cfg); err != nil {
		log.Fatalf("seed data failed: %v", err)
	}

	cache := service.NewMappingCache(db)
	if err := cache.Refresh(); err != nil {
		log.Fatalf("load mapping cache failed: %v", err)
	}

	authSvc := service.NewAuthService(cfg.Server.JWTSecret)
	requestLogger := service.NewRequestLogger(db, cfg.Logging)
	proxySvc := service.NewProxyService(db, cache, cfg.Server.UpstreamTimeoutSeconds, requestLogger)
	adminSvc := service.NewAdminService(db, authSvc, cache)

	engine := router.New(router.Dependencies{
		Config: cfg,
		DB:     db,
		Auth:   authSvc,
		Admin:  adminSvc,
		Proxy:  proxySvc,
	})

	log.Printf("channel adapter gateway listening on %s", cfg.Server.Addr)
	server := &http.Server{
		Addr:    cfg.Server.Addr,
		Handler: engine,
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- server.ListenAndServe()
	}()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	select {
	case <-ctx.Done():
		log.Printf("shutdown signal received")
	case err := <-errCh:
		if !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("server start failed: %v", err)
		}
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("http server shutdown failed: %v", err)
	}

	logCtx, logCancel := context.WithTimeout(context.Background(), time.Duration(cfg.Logging.SyncOnShutdownSeconds)*time.Second)
	defer logCancel()
	requestLogger.Shutdown(logCtx)

	select {
	case err := <-errCh:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("server start failed: %v", err)
		}
	default:
	}
}
