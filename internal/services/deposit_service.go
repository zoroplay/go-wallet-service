package services

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
	"wallet-service/internal/models"
	"wallet-service/pkg/common"
	"wallet-service/proto/wallet"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"gorm.io/gorm"
)

type DepositService struct {
	DB     *gorm.DB
	Client *asynq.Client
}

func NewDepositService(db *gorm.DB, client *asynq.Client) *DepositService {
	return &DepositService{DB: db, Client: client}
}

// Task Types (copied from worker/tasks.go to avoid cycle)
const (
	TypeShopDeposit  = "shop-deposit"
	TypeCreditPlayer = "credit-player"
)

// DTOs for Task Payload (matching consumers DTOs)
type ShopDepositPayload struct {
	Amount          float64 `json:"amount"`
	ClientId        int     `json:"clientId"`
	FromUserId      int     `json:"fromUserId"`
	FromUsername    string  `json:"fromUsername"`
	ToUserId        int     `json:"toUserId"`
	ToUsername      string  `json:"toUsername"`
	Role            string  `json:"role"`
	TransactionCode string  `json:"transactionCode"`
}

type CreditPlayerPayload struct {
	AgentId             int     `json:"agentId"`
	ClientId            int     `json:"clientId"`
	UserId              int     `json:"userId"`
	Amount              float64 `json:"amount"`
	AgentBalance        float64 `json:"agentBalance"`
	UserBalance         float64 `json:"userBalance"`
	PlayerUsername      string  `json:"playerusername"`
	AgentUsername       string  `json:"agentUsername"`
	TransactionNo       string  `json:"transactionNo"`
	PlayerTransactionNo string  `json:"playerTransactionNo"`
	AgentSubject        string  `json:"agentSubject"`
	UserSubject         string  `json:"userSubject"`
}

func (s *DepositService) FetchBetRange(req *wallet.FetchBetRangeRequest) (interface{}, error) {
	var results []struct {
		UserId           int     `json:"userId"`
		AppId            int     `json:"appId"` // Assuming app_id is relevant if grouped by user
		Total            float64 `json:"total"`
		Count            int64   `json:"count"`
		AvailableBalance float64 `json:"availableBalance"`
	}

	query := s.DB.Table("transactions as t").
		Select("t.user_id, SUM(t.amount) as total, COUNT(*) as count, w.available_balance").
		Joins("JOIN wallets w ON w.user_id = t.user_id").
		Where("t.client_id = ? AND t.subject = ? AND t.user_id != 0 AND t.status = 1", req.ClientId, "Bet Deposit (Sport)").
		Where("t.amount >= ? AND t.amount <= ?", req.MinAmount, req.MaxAmount).
		Where("t.created_at BETWEEN ? AND ?", req.StartDate, req.EndDate).
		Group("t.user_id, w.available_balance")

	if err := query.Scan(&results).Error; err != nil {
		return common.NewErrorResponse(err.Error(), nil, http.StatusInternalServerError), nil
	}

	return map[string]interface{}{
		"success": true,
		"status":  http.StatusOK,
		"data":    results,
	}, nil
}

func (s *DepositService) FetchDepositRange(req *wallet.FetchDepositRangeRequest) (interface{}, error) {
	var results []struct {
		UserId           int     `json:"userId"`
		Total            float64 `json:"total"`
		AvailableBalance float64 `json:"availableBalance"`
	}

	query := s.DB.Table("transactions as t").
		Select("t.user_id, SUM(t.amount) as total, w.available_balance").
		Joins("JOIN wallets w ON w.user_id = t.user_id").
		Where("t.client_id = ? AND t.subject = ? AND t.user_id != 0 AND t.status = 1", req.ClientId, "Deposit").
		Where("t.amount >= ? AND t.amount <= ?", req.MinAmount, req.MaxAmount).
		Where("t.created_at BETWEEN ? AND ?", req.StartDate, req.EndDate).
		Group("t.user_id, w.available_balance")

	if err := query.Scan(&results).Error; err != nil {
		return common.NewErrorResponse(err.Error(), nil, http.StatusInternalServerError), nil
	}

	return map[string]interface{}{
		"success": true,
		"status":  http.StatusOK,
		"data":    results,
	}, nil
}

func (s *DepositService) FetchDepositCount(req *wallet.FetchDepositCountRequest) (interface{}, error) {
	var results []struct {
		UserId           int     `json:"userId"`
		Total            int64   `json:"total"` // Count(*)
		AvailableBalance float64 `json:"availableBalance"`
	}

	query := s.DB.Table("transactions as t").
		Select("t.user_id, COUNT(*) as total, w.available_balance").
		Joins("JOIN wallets w ON w.user_id = t.user_id").
		Where("t.client_id = ? AND t.subject = ? AND t.user_id != 0 AND t.status = 1", req.ClientId, "Deposit").
		Where("t.created_at BETWEEN ? AND ?", req.StartDate, req.EndDate).
		Group("t.user_id, w.available_balance").
		Having("COUNT(*) >= ?", req.Count)

	if err := query.Scan(&results).Error; err != nil {
		return common.NewErrorResponse(err.Error(), nil, http.StatusInternalServerError), nil
	}

	return map[string]interface{}{
		"success": true,
		"status":  http.StatusOK,
		"data":    results,
	}, nil
}

func (s *DepositService) FetchPlayerDeposit(req *wallet.FetchPlayerDepositRequest) (interface{}, error) {
	var count int64
	s.DB.Model(&models.Transaction{}).
		Where("subject = ? AND user_id = ? AND status = 1", "Deposit", req.UserId).
		Where("created_at BETWEEN ? AND ?", req.StartDate, req.EndDate).
		Count(&count)

	if count > 0 {
		var wallet models.Wallet
		if err := s.DB.Where("user_id = ?", req.UserId).First(&wallet).Error; err != nil {
			return common.NewErrorResponse("Wallet not found", nil, http.StatusNotFound), nil
		}

		data := map[string]interface{}{
			"userId":              wallet.UserId, // Corrected from UserID
			"balance":             wallet.Balance,
			"availableBalance":    wallet.AvailableBalance,
			"trustBalance":        wallet.TrustBalance,
			"sportBonusBalance":   wallet.SportBonus,   // Corrected from SportBonusBalance
			"virtualBonusBalance": wallet.VirtualBonus, // Corrected from VirtualBonusBalance
			"casinoBonusBalance":  wallet.CasinoBonus,  // Corrected from CasinoBonusBalance
		}
		return map[string]interface{}{
			"success": true,
			"status":  http.StatusOK,
			"data":    data,
		}, nil
	}

	return map[string]interface{}{
		"success": false,
		"status":  http.StatusNotFound,
		"data":    nil,
	}, nil
}

func (s *DepositService) ValidateDepositCode(req *wallet.ValidateTransactionRequest) (interface{}, error) {
	var transaction models.Transaction
	err := s.DB.Where("client_id = ? AND transaction_no = ? AND tranasaction_type = ?", req.ClientId, req.Code, "credit").First(&transaction).Error

	if err != nil {
		return common.NewErrorResponse("Transaction not found", nil, http.StatusNotFound), nil
	}

	switch transaction.Status {
	case 0:
		return map[string]interface{}{
			"success": true,
			"message": "Transaction found",
			"data":    transaction, // Helper function to map transaction might be needed if models.Transaction has non-JSON fields
			"status":  http.StatusOK,
		}, nil
	case 1:
		return common.NewErrorResponse("Code has already been used", nil, http.StatusBadRequest), nil
	case 2:
		return common.NewErrorResponse("Code has expired", nil, http.StatusBadRequest), nil
	}

	return common.NewErrorResponse("Transaction not found", nil, http.StatusNotFound), nil
}

type ShopDepositRequest struct {
	ID       uint   `json:"id"`
	UserId   int    `json:"userId"`
	Username string `json:"username"`
	ClientId int    `json:"clientId"`
	UserRole string `json:"userRole"`
}

func (s *DepositService) ProcessShopDeposit(data ShopDepositRequest) (interface{}, error) {
	var transaction models.Transaction
	if err := s.DB.Where("id = ? AND status = 0", data.ID).First(&transaction).Error; err != nil {
		return common.NewErrorResponse("Deposit request already processed or not found", nil, http.StatusBadRequest), nil
	}

	if transaction.UserId == data.UserId {
		return common.NewErrorResponse("You cannot process your own request", nil, http.StatusBadRequest), nil
	}

	var wallet models.Wallet
	if err := s.DB.Where("user_id = ? AND client_id = ?", data.UserId, data.ClientId).First(&wallet).Error; err != nil {
		return common.NewErrorResponse("Wallet not found", nil, http.StatusBadRequest), nil
	}

	if wallet.AvailableBalance < transaction.Amount {
		return common.NewErrorResponse("You do not have enough funds to complete this request", nil, http.StatusBadRequest), nil
	}

	payload := ShopDepositPayload{
		Amount:          transaction.Amount,
		ClientId:        data.ClientId,
		FromUserId:      data.UserId,
		FromUsername:    data.Username,
		ToUserId:        transaction.UserId,
		ToUsername:      transaction.Username,
		Role:            data.UserRole,
		TransactionCode: transaction.TransactionNo,
	}

	taskData, err := json.Marshal(payload)
	if err != nil {
		return common.NewErrorResponse("Failed to marshal task data", nil, http.StatusInternalServerError), nil
	}

	task := asynq.NewTask(TypeShopDeposit, taskData)

	info, err := s.Client.Enqueue(task, asynq.TaskID(fmt.Sprintf("shop-deposit:%s", transaction.TransactionNo)))
	if err != nil {
		return common.NewErrorResponse("Failed to enqueue task", nil, http.StatusInternalServerError), nil
	}
	_ = info // suppress unused variable

	return map[string]interface{}{
		"success": true,
		"status":  http.StatusOK,
		"message": "Transaction has been processed",
		"data": map[string]interface{}{
			"balance": wallet.AvailableBalance - transaction.Amount, // Optimistic update for UI
		},
	}, nil
}

type CreditUserFromAgentRequest struct {
	AgentId  int     `json:"agentId"`
	ClientId int     `json:"clientId"`
	UserId   int     `json:"userId"`
	Amount   float64 `json:"amount"`
	Source   string  `json:"source"` // Not used in TS but good to have?
}

func (s *DepositService) CreditUserFromAgent(data CreditUserFromAgentRequest) (interface{}, error) {
	var agentWallet models.Wallet
	if err := s.DB.Where("user_id = ? AND client_id = ?", data.AgentId, data.ClientId).First(&agentWallet).Error; err != nil {
		return common.NewErrorResponse("Agent wallet not found", nil, http.StatusBadRequest), nil
	}

	if data.AgentId == data.UserId {
		return common.NewErrorResponse("You cannot transfer to yourself", nil, http.StatusBadRequest), nil
	}

	if agentWallet.AvailableBalance < data.Amount {
		return common.NewErrorResponse("You do not have enough funds to complete this request", nil, http.StatusBadRequest), nil
	}

	var userWallet models.Wallet
	if err := s.DB.Where("user_id = ? AND client_id = ?", data.UserId, data.ClientId).First(&userWallet).Error; err != nil {
		return common.NewErrorResponse("Player wallet not found", nil, http.StatusBadRequest), nil
	}

	trxNo := uuid.NewString() // common.GenerateUUID() or equivalent
	playerTrxNo := uuid.NewString()

	payload := CreditPlayerPayload{
		AgentId:             data.AgentId,
		ClientId:            data.ClientId,
		UserId:              data.UserId,
		Amount:              data.Amount,
		AgentBalance:        agentWallet.AvailableBalance,
		UserBalance:         userWallet.AvailableBalance,
		PlayerUsername:      userWallet.Username,
		AgentUsername:       agentWallet.Username,
		TransactionNo:       trxNo,
		PlayerTransactionNo: playerTrxNo,
		AgentSubject:        "Player Funding",
		UserSubject:         "Deposit",
	}

	taskData, err := json.Marshal(payload)
	if err != nil {
		return common.NewErrorResponse("Failed to marshal task data", nil, http.StatusInternalServerError), nil
	}

	task := asynq.NewTask(TypeCreditPlayer, taskData)

	taskId := fmt.Sprintf("%d:%d:%s", data.UserId, data.ClientId, trxNo)
	_, err = s.Client.Enqueue(task, asynq.TaskID(taskId), asynq.ProcessIn(5*time.Second))
	if err != nil {
		return common.NewErrorResponse("Failed to enqueue task", nil, http.StatusInternalServerError), nil
	}

	return map[string]interface{}{
		"success": true,
		"status":  http.StatusOK,
		"message": "Deposit processed successfully",
	}, nil
}
