package services

import (
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	"wallet-service/internal/models"
	"wallet-service/pkg/common"

	"gorm.io/gorm"
)

type ProvidusService struct {
	DB            *gorm.DB
	HelperService *HelperService
}

func NewProvidusService(db *gorm.DB, helper *HelperService) *ProvidusService {
	return &ProvidusService{
		DB:            db,
		HelperService: helper,
	}
}

func (s *ProvidusService) providusSettings(clientId int) (*models.PaymentMethod, error) {
	var pm models.PaymentMethod
	err := s.DB.Where("provider = ? AND client_id = ?", "providus", clientId).First(&pm).Error
	if err != nil {
		return nil, err
	}
	return &pm, nil
}

func (s *ProvidusService) InitiatePayment(data map[string]interface{}, clientId int) (interface{}, error) {
	settings, err := s.providusSettings(clientId)
	if err != nil {
		return common.SuccessResponse{Success: false, Message: "Payment method not found"}, nil
	}

	url := fmt.Sprintf("%s/PiPCreateDynamicAccountNumber", settings.BaseUrl)
	merchantId := settings.MerchantId
	clientSecret := settings.SecretKey

	signatureInput := fmt.Sprintf("%s:%s", merchantId, clientSecret)
	hash := sha512.New()
	hash.Write([]byte(signatureInput))
	xAuthSignature := hex.EncodeToString(hash.Sum(nil))

	headers := map[string]string{
		"Content-Type":     "application/json",
		"Client-Id":        merchantId,
		"X-Auth-Signature": xAuthSignature,
	}

	response, err := common.Post(url, data, headers)
	if err != nil {
		// Log error if needed
		return common.SuccessResponse{Success: false, Message: "Payment request failed"}, nil // Simplified error
	}

	resMap, _ := response.(map[string]interface{})
	return common.SuccessResponse{Success: true, Data: resMap}, nil
}

func (s *ProvidusService) HandleWebhook(data map[string]interface{}) (interface{}, error) {
	clientIdFloat, _ := data["clientId"].(float64)
	clientId := int(clientIdFloat)

	rawBody, _ := data["rawBody"].(map[string]interface{})

	// Data passed might be flat or nested as per TS "data"
	// TS accesses `data.sessionId`, `data.rawBody.webhookBody.sessionId`
	// Assuming `data` passed here has the payload keys merged or we look into rawBody if it is the full request body.
	// But commonly in my porting, `data` is the body + extra helpers.
	// TS: `data.settlementId`, `data.accountNumber`.

	settlementId, _ := data["settlementId"].(string)
	accountNumber, _ := data["accountNumber"].(string)
	sessionId, _ := data["sessionId"].(string)
	headersStr, _ := data["headers"].(string) // "receivedSignature" from headers? TS: `receivedSignature = (data.headers || '').trim()` - passed as string?

	settings, err := s.providusSettings(clientId)
	if err != nil {
		return map[string]interface{}{
			"requestSuccessful": true,
			"sessionId":         sessionId,
			"responseMessage":   "Payment method not found",
			"responseCode":      "03",
		}, nil
	}

	if settlementId == "" || accountNumber == "" {
		return map[string]interface{}{
			"requestSuccessful": true,
			"sessionId":         sessionId,
			"responseMessage":   "rejected transaction",
			"responseCode":      "02",
		}, nil
	}

	expectedSignature := fmt.Sprintf("%s:%s", settings.MerchantId, settings.SecretKey)
	hash := sha512.New()
	hash.Write([]byte(expectedSignature))
	expectedHash := hex.EncodeToString(hash.Sum(nil))

	if !strings.EqualFold(expectedHash, headersStr) {
		return map[string]interface{}{
			"requestSuccessful": true,
			"sessionId":         sessionId,
			"responseMessage":   "rejected transaction",
			"responseCode":      "02",
		}, nil
	}

	// Find transaction
	var transaction models.Transaction
	if err := s.DB.Where("client_id = ? AND transaction_no = ? AND tranasaction_type = ?", clientId, accountNumber, "credit").First(&transaction).Error; err != nil {
		s.logCallback(clientId, "Transaction not found", rawBody, 0, accountNumber, "Providus")
		return map[string]interface{}{
			"requestSuccessful": true,
			"sessionId":         sessionId,
			"responseMessage":   "rejected transaction",
			"responseCode":      "02",
		}, nil
	}

	if transaction.Status == 1 {
		return map[string]interface{}{
			"requestSuccessful": true,
			"sessionId":         sessionId,
			"responseMessage":   "duplicate transaction",
			"responseCode":      "01",
		}, nil
	}

	if transaction.Status == 2 {
		return map[string]interface{}{
			"requestSuccessful": true,
			"sessionId":         sessionId,
			"responseMessage":   "rejected transaction",
			"responseCode":      "02",
		}, nil
	}

	// Check settlementId uniqueness
	var existingSettlement models.Transaction
	if err := s.DB.Where("settlement_id = ?", settlementId).First(&existingSettlement).Error; err == nil {
		s.logCallback(clientId, "Duplicate transaction", rawBody, 0, accountNumber, "Providus")
		return map[string]interface{}{
			"requestSuccessful": true,
			"sessionId":         sessionId,
			"responseMessage":   "duplicate transaction",
			"responseCode":      "01",
		}, nil
	}

	// Update settlementId if missing
	if transaction.SettlementId == nil || *transaction.SettlementId == "" {
		s.DB.Model(&transaction).Update("settlement_id", settlementId)
	}

	// Update Wallet
	if err := s.DB.Model(&models.Wallet{}).Where("user_id = ?", transaction.UserId).UpdateColumn("available_balance", gorm.Expr("available_balance + ?", transaction.Amount)).Error; err != nil {
		s.logCallback(clientId, "Wallet not found", rawBody, 0, accountNumber, "Providus")
		return map[string]interface{}{
			"requestSuccessful": true,
			"sessionId":         sessionId,
			"responseMessage":   "rejected transaction",
			"responseCode":      "02",
		}, nil
	}

	var wallet models.Wallet
	s.DB.Where("user_id = ?", transaction.UserId).First(&wallet)

	s.DB.Model(&models.Transaction{}).Where("transaction_no = ?", transaction.TransactionNo).Updates(map[string]interface{}{
		"status":            1,
		"available_balance": wallet.AvailableBalance,
	})

	s.logCallback(clientId, "Completed", rawBody, 0, accountNumber, "Providus")
	return map[string]interface{}{
		"requestSuccessful": true,
		"sessionId":         sessionId,
		"responseMessage":   "success",
		"responseCode":      "00",
	}, nil
}

func (s *ProvidusService) logCallback(clientId int, requestStr string, response interface{}, status int, trxId, method string) {
	respBytes, _ := json.Marshal(response)
	log := models.CallbackLog{
		ClientId:      clientId,
		Request:       requestStr,
		Response:      string(respBytes),
		Status:        status,
		RequestType:   "Webhook",
		TransactionId: trxId,
		PaymentMethod: method,
	}
	s.DB.Create(&log)
}
