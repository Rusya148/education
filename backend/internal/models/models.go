package models

import "time"

type CardRequest struct {
	ID        int       `json:"id"`
	Name      string    `json:"name" binding:"required"`
	Phone     string    `json:"phone" binding:"required"`
	Email     string    `json:"email" binding:"required"`
	CreatedAt time.Time `json:"created_at"`
}
