package services

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"

	"wallet-service/internal/models"
	"wallet-service/pkg/common"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type MomoService struct {
	DB            *gorm.DB
	HelperService *HelperService
}

func NewMomoService(db *gorm.DB, helper *HelperService) *MomoService {
	return &MomoService{
		DB:            db,
		HelperService: helper,
	}
}

func (s *MomoService) mtnMomoSettings(clientId int) (*models.PaymentMethod, error) {
	var pm models.PaymentMethod
	err := s.DB.Where("provider = ? AND client_id = ?", "mtnmomo", clientId).First(&pm).Error
	if err != nil {
		return nil, err
	}
	return &pm, nil
}

func (s *MomoService) InitiatePayment(data map[string]interface{}, clientId int) (interface{}, error) {
	referenceId := uuid.New().String()
	callbackUrl := fmt.Sprintf("https://api.prod.sportsbookengine.com/api/v2/webhook/%d/mtnmomo/callback", clientId)
	if clientId == 4 {
		callbackUrl = "https://api.staging.sportsbookengine.com/api/v2/webhook/4/mtnmomo/callback"
	}

	settings, err := s.mtnMomoSettings(clientId)
	if err != nil {
		return common.SuccessResponse{Success: false, Message: "Tigo has not been configured for client"}, nil
	}

	// Step 1: Create API User
	apiUserPayload := map[string]string{
		"providerCallbackHost": callbackUrl,
	}

	apiUserHeaders := map[string]string{
		"X-Reference-Id":            referenceId,
		"Content-Type":              "application/json",
		"Ocp-Apim-Subscription-Key": settings.SecretKey,
	}

	_, err = common.Post(fmt.Sprintf("%s/v1_0/apiuser", settings.BaseUrl), apiUserPayload, apiUserHeaders)
	// TS code doesn't check error explicitly but logs. We should check.
	if err != nil {
		fmt.Printf("Momo API User Error: %v\n", err)
		// Proceeding as TS script continues...? No, TS awaits and error catches.
		// If this fails, next steps fail.
		return common.SuccessResponse{Success: false, Message: "Payment request failed"}, nil
	}

	// Step 2: Generate API Key
	apiKeyHeaders := map[string]string{
		"Content-Type":              "application/json",
		"Ocp-Apim-Subscription-Key": settings.SecretKey,
	}
	apiKeyResp, err := common.Post(fmt.Sprintf("%s/v1_0/apiuser/%s/apikey", settings.BaseUrl, referenceId), map[string]interface{}{}, apiKeyHeaders)
	if err != nil {
		return common.SuccessResponse{Success: false, Message: "Payment request failed"}, nil
	}

	apiKeyMap, _ := apiKeyResp.(map[string]interface{})
	apiKey, _ := apiKeyMap["apiKey"].(string)

	// Step 3: Get Access Token
	tokenHeaders := map[string]string{
		"Ocp-Apim-Subscription-Key": settings.SecretKey,
		"Content-Type":              "application/json",
	}
	// Basic Auth for token
	// TS: auth: { username: referenceId, password: apiKey }
	// Common.Post might not support separate Auth struct. We can add Authorization header.
	// Basic Auth is Base64(username:password)
	authStr := base64.StdEncoding.EncodeToString([]byte(referenceId + ":" + apiKey))
	tokenHeaders["Authorization"] = "Basic " + authStr

	tokenResp, err := common.Post(fmt.Sprintf("%s/collection/token/", settings.BaseUrl), map[string]interface{}{
		"providerCallbackHost": callbackUrl,
	}, tokenHeaders)

	if err != nil {
		return common.SuccessResponse{Success: false, Message: "Payment request failed"}, nil
	}

	tokenMap, _ := tokenResp.(map[string]interface{})
	token, _ := tokenMap["access_token"].(string)

	// Step 4: Prepare Payload
	// data has payer { partyId }
	payerData, _ := data["payer"].(map[string]interface{})
	partyId, _ := payerData["partyId"].(string)
	partyId = strings.ReplaceAll(partyId, "+", "")

	externalId, _ := data["externalId"].(string)

	payload := map[string]interface{}{
		"payerMessage": "Online Deposit (Mtn-momo)",
		"payeeNote":    "Online Deposit (Mtn-momo)",
		"payer": map[string]string{
			"partyIdType": "MSISDN",
			"partyId":     partyId,
		},
		"amount":     data["amount"],   // Assume amount is in data
		"currency":   data["currency"], // Assume currency is in data
		"externalId": externalId,
	}

	paymentId := uuid.New().String()

	// Step 5: Send Payment Request
	paymentHeaders := map[string]string{
		"X-Reference-Id":            paymentId,
		"X-Target-Environment":      "sandbox", // Hardcoded in TS? "sandbox"
		"Content-Type":              "application/json",
		"Ocp-Apim-Subscription-Key": settings.SecretKey,
		"Accept":                    "application/json",
		"Authorization":             "Bearer " + token,
	}

	// Note: TS has `...data` spread, I assumed amount and currency are there.
	_, err = common.Post(fmt.Sprintf("%s/collection/v1_0/requesttopay", settings.BaseUrl), payload, paymentHeaders)
	if err != nil {
		return common.SuccessResponse{Success: false, Message: "Payment request failed", Data: err.Error()}, nil // Return error details?
	}

	// TS returns specific object structure
	// res might be nil if 202 Accepted? Common.Post parsing might differ.
	// TS returns { success: true, message: ..., paymentId, externalId, status }

	return map[string]interface{}{
		"success":    true,
		"message":    "Payment request successfully submitted. Awaiting processing.",
		"paymentId":  paymentId,
		"externalId": externalId,
		"status":     202, // Assuming success
	}, nil
}

func (s *MomoService) HandleWebhook(data map[string]interface{}) (interface{}, error) {
	status, _ := data["status"].(string)
	externalId, _ := data["externalId"].(string)
	clientIdFloat, _ := data["clientId"].(float64)
	clientId := int(clientIdFloat)
	rawBody, _ := data["rawBody"].(map[string]interface{})

	_, err := s.mtnMomoSettings(clientId)
	if err != nil {
		return common.SuccessResponse{Success: false, Message: "Mtn MOMO has not been configured for client"}, nil
	}

	var transaction models.Transaction
	if err := s.DB.Where("client_id = ? AND transaction_no = ? AND tranasaction_type = ?", clientId, externalId, "credit").First(&transaction).Error; err != nil {
		s.logCallback(clientId, "Transaction not found", rawBody, 0, externalId, "Mtnmomo")
		return common.NewErrorResponse("Transaction not found", nil, 404), nil
	}

	if status == "SUCCESSFUL" {
		if transaction.Status == 1 {
			s.logCallback(clientId, "Transaction already successful", rawBody, 1, externalId, "Mtnmomo")
			return map[string]interface{}{
				"success": true,
				"message": "Transaction already successful",
			}, nil
		}

		// Update Wallet
		if err := s.DB.Model(&models.Wallet{}).Where("user_id = ?", transaction.UserId).UpdateColumn("available_balance", gorm.Expr("available_balance + ?", transaction.Amount)).Error; err != nil {
			s.logCallback(clientId, "Wallet not found", rawBody, 0, externalId, "Mtnmomo")
			return common.NewErrorResponse("Wallet not found for this user", nil, 404), nil
		}

		var wallet models.Wallet
		s.DB.Where("user_id = ?", transaction.UserId).First(&wallet)

		s.DB.Model(&models.Transaction{}).Where("transaction_no = ?", transaction.TransactionNo).Updates(map[string]interface{}{
			"status":  1,
			"balance": wallet.AvailableBalance,
		})

		s.logCallback(clientId, "Completed", rawBody, 1, externalId, "Mtnmomo")

		return map[string]interface{}{
			"success": true,
			"message": "Transaction successfully verified and processed",
		}, nil
	}

	return map[string]interface{}{"success": false, "message": "error occurred"}, nil
}

func (s *MomoService) logCallback(clientId int, requestStr string, response interface{}, status int, trxId, method string) {
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
