package handlers

import (
	"net/http"
	"strconv"

	"wallet-service/internal/database"
	"wallet-service/internal/models"

	"github.com/gin-gonic/gin"
)

type CreateWalletRequest struct {
	UserId   int    `json:"user_id" binding:"required"`
	Username string `json:"username" binding:"required"`
	ClientId int    `json:"client_id" binding:"required"`
	Currency string `json:"currency" binding:"required"`
}

func CreateWallet(c *gin.Context) {
	var req CreateWalletRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	wallet := models.Wallet{
		UserId:   req.UserId,
		Username: req.Username,
		ClientId: req.ClientId,
		Currency: req.Currency,
		AvailableBalance: 0.00,
	}

	if result := database.DB.Create(&wallet); result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": result.Error.Error()})
		return
	}

	c.JSON(http.StatusCreated, wallet)
}

func GetBalance(c *gin.Context) {
	userIdStr := c.Query("user_id")
	userId, err := strconv.Atoi(userIdStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user_id"})
		return
	}

	var wallet models.Wallet
	if result := database.DB.Where("user_id = ?", userId).First(&wallet); result.Error != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Wallet not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"balance":  wallet.AvailableBalance,
		"currency": wallet.Currency,
	})
}
