package services

import (
	"fmt"

	"wallet-service/internal/models"
	"wallet-service/pkg/common"

	"gorm.io/gorm"
)

type WayaQuickService struct {
	DB            *gorm.DB
	HelperService *HelperService
}

func NewWayaQuickService(db *gorm.DB, helper *HelperService) *WayaQuickService {
	return &WayaQuickService{
		DB:            db,
		HelperService: helper,
	}
}

func (s *WayaQuickService) getSettings(clientId int) (*models.PaymentMethod, error) {
	var pm models.PaymentMethod
	err := s.DB.Where("provider = ? AND client_id = ?", "wayaquick", clientId).First(&pm).Error
	if err != nil {
		return nil, err
	}
	return &pm, nil
}

func (s *WayaQuickService) GeneratePaymentLink(data map[string]interface{}, clientId int) (interface{}, error) {
	settings, err := s.getSettings(clientId)
	if err != nil {
		return map[string]interface{}{"success": false, "message": "WayaQuick not configured"}, nil
	}

	// Assumption: SDK uses a base URL. We'll use settings.BaseUrl if available, or a default.
	// Common WayaPay URL: https://services.wayapay.com/payment-gateway/api/v1/request/transaction ?
	// Since we can't see the SDK, relying on BaseUrl from DB if set, else TODO.
	baseUrl := settings.BaseUrl
	if baseUrl == "" {
		baseUrl = "https://services.wayapay.com/payment-gateway/api/v1" // Fallback/Guess
	}

	// Initialize Payment
	// SDK: initializePayment(data)
	// Payload mostly likely matches data passed?
	// TS passed: amount, email, firstName, lastName, narration, phoneNumber
	// We construct payload

	// TODO: Verify exact endpoint for Initialize Payment
	url := fmt.Sprintf("%s/request/transaction", baseUrl)

	headers := map[string]string{
		"Content-Type":  "application/json",
		"Authorization": settings.PublicKey, // Or MerchantId? SDK takes (merchantId, publicKey, mode). Usually publicKey/MerchantId used in headers.
	}

	payload := map[string]interface{}{
		"amount":      data["amount"],
		"description": data["narration"],
		"customer": map[string]interface{}{
			"email":       data["email"],
			"firstName":   data["firstName"],
			"lastName":    data["lastName"],
			"phoneNumber": data["phoneNumber"],
		},
		"merchantId": settings.MerchantId,
	}

	// This is a BEST GUESS implementation of the SDK logic.
	resp, err := common.Post(url, payload, headers)
	if err != nil {
		return map[string]interface{}{"success": false, "message": "Unable to initiate deposit with wayaquick"}, nil
	}

	respMap, _ := resp.(map[string]interface{})
	if status, ok := respMap["status"].(bool); !ok || !status {
		return map[string]interface{}{"success": false, "message": respMap["message"]}, nil
	}

	return map[string]interface{}{"success": true, "data": respMap["data"], "message": respMap["message"]}, nil
}

func (s *WayaQuickService) VerifyTransaction(data map[string]interface{}) (interface{}, error) {
	clientIdFloat, _ := data["clientId"].(float64)
	clientId := int(clientIdFloat)
	transactionRef, _ := data["transactionRef"].(string)

	settings, err := s.getSettings(clientId)
	if err != nil {
		return map[string]interface{}{"success": false, "message": "Settings not found"}, nil
	}

	baseUrl := settings.BaseUrl
	if baseUrl == "" {
		baseUrl = "https://services.wayapay.com/payment-gateway/api/v1"
	}
	// Verify Payment
	// SDK: verifyPayment(ref)
	url := fmt.Sprintf("%s/transaction/verify/%s", baseUrl, transactionRef)
	headers := map[string]string{
		"Authorization": settings.PublicKey,
		"MerchantId":    settings.MerchantId,
	}

	resp, err := common.Get(url, headers) // Assuming GET
	if err != nil {
		return map[string]interface{}{"success": false, "message": "Error verifying transaction"}, nil
	}

	resMap, _ := resp.(map[string]interface{})
	status, _ := resMap["status"].(bool)
	dataMap, _ := resMap["data"].(map[string]interface{})
	remoteStatus, _ := dataMap["Status"].(string)

	if status && remoteStatus == "SUCCESSFUL" {
		var transaction models.Transaction
		if err := s.DB.Where("client_id = ? AND transaction_no = ? AND tranasaction_type = ?", clientId, transactionRef, "credit").First(&transaction).Error; err != nil {
			return map[string]interface{}{"success": false, "message": "Transaction not found", "status": 404}, nil
		}

		if transaction.Status == 1 {
			return map[string]interface{}{"success": true, "message": "Transaction was successful", "status": 200}, nil
		}

		// Update Wallet
		var wallet models.Wallet
		s.DB.Where("user_id = ?", transaction.UserId).First(&wallet)
		balance := wallet.AvailableBalance + transaction.Amount
		s.HelperService.UpdateWallet(balance, transaction.UserId)

		s.DB.Model(&transaction).Updates(map[string]interface{}{
			"status":  1,
			"balance": balance,
		})

		// Trackier logic omitted or moved to helper

		return map[string]interface{}{"success": true, "message": "Transaction was successful", "status": 200}, nil
	} else if status && remoteStatus != "SUCCESSFUL" {
		s.DB.Model(&models.Transaction{}).Where("transaction_no = ?", transactionRef).Update("status", 2)
		return map[string]interface{}{"success": false, "message": "Transaction was not successful", "status": 400}, nil
	}

	return map[string]interface{}{"success": false, "message": "Transaction not successful"}, nil
}
