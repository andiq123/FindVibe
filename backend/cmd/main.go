package main

import (
	"os"

	"github.com/andiq123/FindVibeFiber/internal/config"
	"github.com/andiq123/FindVibeFiber/internal/database"
	"github.com/andiq123/FindVibeFiber/internal/di"
	"github.com/andiq123/FindVibeFiber/internal/server"
	"github.com/andiq123/FindVibeFiber/internal/utils"
	"github.com/joho/godotenv"
)

func init() {
	if os.Getenv("IS_LOCAL") == "true" {
		_ = godotenv.Load()
	}
}

func main() {
	cfg := config.LoadConfig()
	srv := server.NewServer(cfg.Server)

	// Listen first so Render /health stops 504'ing during DB connect.
	go func() {
		db := database.InitDb(cfg.Database)
		srv.Mount(di.InitializeHandlers(db, cfg))
	}()

	utils.GetLogger().Info("Booting HTTP before database")
	srv.Start()
}
