package main

import (
	"log"
	"net/http"
	"time"

	"github.com/anadubesko/go-do-parser/admin/internal/api"
	"github.com/anadubesko/go-do-parser/admin/internal/auth"
	"github.com/anadubesko/go-do-parser/admin/internal/config"
	"github.com/anadubesko/go-do-parser/admin/internal/db"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}

	database, err := db.Open(cfg.DatabaseURL)
	if err != nil {
		log.Fatal(err)
	}
	defer database.Close()

	authService := auth.New(cfg.AdminPassword, cfg.JWTSecret)

	server := api.New(database, authService)
	httpServer := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           server.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
	}

	log.Printf("admin listening on :%s", cfg.Port)
	if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}
