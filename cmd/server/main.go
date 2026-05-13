package main

import (
	"log"

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
	proxySvc := service.NewProxyService(db, cache, cfg.Server.UpstreamTimeoutSeconds)
	adminSvc := service.NewAdminService(db, authSvc, cache)

	engine := router.New(router.Dependencies{
		Config: cfg,
		DB:     db,
		Auth:   authSvc,
		Admin:  adminSvc,
		Proxy:  proxySvc,
	})

	log.Printf("channel adapter gateway listening on %s", cfg.Server.Addr)
	if err := engine.Run(cfg.Server.Addr); err != nil {
		log.Fatalf("server start failed: %v", err)
	}
}
