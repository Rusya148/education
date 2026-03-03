package main

import (
	"log"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"

	"minion-bank-backend/internal/config"
	"minion-bank-backend/internal/db"
	"minion-bank-backend/internal/handlers"
	"minion-bank-backend/internal/kafka"
)

func main() {
	cfg := config.LoadConfig()

	// Initialize Infrastructure
	db.InitDB(cfg)
	kafka.InitKafka(cfg)

	// Router Setup
	r := gin.Default()
	r.Use(cors.Default())

	// Routes
	r.GET("/healthz", handlers.Healthz)
	r.GET("/ready", handlers.Ready)
	r.POST("/apply", handlers.Apply)

	log.Printf("Minion Bank Backend starting on port %s", cfg.Port)
	if err := r.Run(":" + cfg.Port); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}
