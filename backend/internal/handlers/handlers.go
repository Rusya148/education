package handlers

import (
	"encoding/json"
	"log"
	"net/http"

	"minion-bank-backend/internal/db"
	"minion-bank-backend/internal/kafka"
	"minion-bank-backend/internal/models"

	"github.com/gin-gonic/gin"
)

func Healthz(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func Ready(c *gin.Context) {
	if err := db.DB.Ping(); err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"status": "error", "error": "database unreachable"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ready"})
}

func Apply(c *gin.Context) {
	var req models.CardRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 1. Save to DB
	query := "INSERT INTO card_requests (name, phone, email) VALUES ($1, $2, $3) RETURNING id"
	err := db.DB.QueryRow(query, req.Name, req.Phone, req.Email).Scan(&req.ID)
	if err != nil {
		log.Printf("DB error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save request"})
		return
	}

	// 2. Publish to Kafka
	payload, _ := json.Marshal(req)
	if err := kafka.PublishMessage(payload); err != nil {
		log.Printf("Kafka error: %v", err)
		// We still return success but log the error (common pattern or choice)
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "Карта в пути! Ожидайте бананов 🍌",
		"id":      req.ID,
	})
}
