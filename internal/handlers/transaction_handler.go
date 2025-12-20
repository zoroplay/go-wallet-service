package handlers

import (
	"net/http"
	"strconv"

	"wallet-service/internal/database"
	"wallet-service/internal/models"

	"github.com/gin-gonic/gin"
)

type CreditUserRequest struct {
	UserId        int     `json:"user_id" binding:"required"`
	Amount        float64 `json:"amount" binding:"required"`
	Source        string  `json:"source"`
	Description   string  `json:"description"`
	TransactionNo string  `json:"transaction_no" binding:"required"`
}

func CreditUser(c *gin.Context) {
	var req CreditUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	tx := database.DB.Begin()

	var wallet models.Wallet
	if err := tx.Where("user_id = ?", req.UserId).First(&wallet).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusNotFound, gin.H{"error": "Wallet not found"})
		return
	}

	wallet.AvailableBalance += req.Amount
	if err := tx.Save(&wallet).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update wallet"})
		return
	}

	transaction := models.Transaction{
		ClientId:      wallet.ClientId,
		UserId:        wallet.UserId,
		Username:      wallet.Username,
		TransactionNo: req.TransactionNo,
		Amount:        req.Amount,
		TrxType:       "credit",
		Subject:       "deposit",
		Description:   req.Description,
		Source:        req.Source,
		AvailableBalance: wallet.AvailableBalance,
		Status:        1,
	}

	if err := tx.Create(&transaction).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create transaction"})
		return
	}

	tx.Commit()
	c.JSON(http.StatusOK, gin.H{"message": "User credited successfully", "balance": wallet.AvailableBalance})
}

func DebitUser(c *gin.Context) {
	var req CreditUserRequest // Reusing struct for simplicity
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	tx := database.DB.Begin()

	var wallet models.Wallet
	if err := tx.Where("user_id = ?", req.UserId).First(&wallet).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusNotFound, gin.H{"error": "Wallet not found"})
		return
	}

	if wallet.AvailableBalance < req.Amount {
		tx.Rollback()
		c.JSON(http.StatusBadRequest, gin.H{"error": "Insufficient balance"})
		return
	}

	wallet.AvailableBalance -= req.Amount
	if err := tx.Save(&wallet).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update wallet"})
		return
	}

	transaction := models.Transaction{
		ClientId:      wallet.ClientId,
		UserId:        wallet.UserId,
		Username:      wallet.Username,
		TransactionNo: req.TransactionNo,
		Amount:        req.Amount,
		TrxType:       "debit",
		Subject:       "withdrawal",
		Description:   req.Description,
		Source:        req.Source,
		AvailableBalance: wallet.AvailableBalance,
		Status:        1,
	}

	if err := tx.Create(&transaction).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create transaction"})
		return
	}

	tx.Commit()
	c.JSON(http.StatusOK, gin.H{"message": "User debited successfully", "balance": wallet.AvailableBalance})
}

func GetTransactions(c *gin.Context) {
	userIdStr := c.Query("user_id")
	userId, err := strconv.Atoi(userIdStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user_id"})
		return
	}

	var transactions []models.Transaction
	if result := database.DB.Where("user_id = ?", userId).Order("created_at desc").Limit(20).Find(&transactions); result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch transactions"})
		return
	}

	c.JSON(http.StatusOK, transactions)
}
