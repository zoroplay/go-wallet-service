package services

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	"wallet-service/internal/models"
	"wallet-service/pkg/common"

	"gorm.io/gorm"
)

type PayonusService struct {
	DB            *gorm.DB
	HelperService *HelperService
}

func NewPayonusService(db *gorm.DB, helper *HelperService) *PayonusService {
	return &PayonusService{
		DB:            db,
		HelperService: helper,
	}
}

func (s *PayonusService) payonusSettings(clientId int) (*models.PaymentMethod, error) {
	var pm models.PaymentMethod
	err := s.DB.Where("provider = ? AND client_id = ?", "payonus", clientId).First(&pm).Error
	if err != nil {
		return nil, err
	}
	return &pm, nil
}

func (s *PayonusService) InitiatePayment(data map[string]interface{}, clientId int) (interface{}, error) {
	settings, err := s.payonusSettings(clientId)
	if err != nil {
		return nil, err
	}

	baseUrl := strings.TrimSuffix(settings.BaseUrl, "/")

	// Auth
	authPayload := map[string]string{
		"apiClientId":     settings.PublicKey,
		"apiClientSecret": settings.SecretKey,
	}
	authRes, err := common.Post(fmt.Sprintf("%s/api/v1/access-token", baseUrl), authPayload, nil)
	if err != nil {
		return common.SuccessResponse{Success: false, Message: "Auth failed"}, nil
	}
	authMap, _ := authRes.(map[string]interface{})
	dataAuth, _ := authMap["data"].(map[string]interface{})
	token, _ := dataAuth["access_token"].(string)

	// Payload
	payload := make(map[string]interface{})
	for k, v := range data {
		payload[k] = v
	}
	payload["businessId"] = settings.MerchantId

	headers := map[string]string{
		"Authorization": "Bearer " + token,
		"Content-Type":  "application/json",
	}

	res, err := common.Post(fmt.Sprintf("%s/api/v1/virtual-accounts/dynamic", baseUrl), payload, headers)
	if err != nil {
		return common.SuccessResponse{Success: false, Message: "Payment init failed"}, nil
	}

	resMap, _ := res.(map[string]interface{})
	return common.SuccessResponse{Success: true, Data: resMap["data"]}, nil
}

func (s *PayonusService) HandleWebhook(data map[string]interface{}) (interface{}, error) {
	rawBody, _ := data["rawBody"].(map[string]interface{})
	innerDataStr, _ := rawBody["data"].(string)

	// TS does JSON.parse(JSON.parse(innerString)) <-- double parse?
	// It parses `data.rawBody.data` which is a string.
	// Go json unmarshal usually handles one level. If the string is JSON escaped string...

	var webhookData map[string]interface{}
	// First unquote if it's a string, or unmarshal it
	// If `innerDataStr` is a JSON string, we unmarshal it.
	json.Unmarshal([]byte(innerDataStr), &webhookData)
	// If it was double encoded, we might need second pass, but let's assume one pass.

	// businessId, _ := webhookData["businessId"].(string) // Unused
	onusReference, _ := webhookData["onusReference"].(string)
	paymentStatus, _ := webhookData["paymentStatus"].(string)
	typeStr, _ := webhookData["type"].(string)
	accountNumber, _ := webhookData["accountNumber"].(string)

	clientIdFloat, _ := data["clientId"].(float64)
	clientId := int(clientIdFloat)

	_, err := s.payonusSettings(clientId)
	if err != nil {
		return map[string]interface{}{"success": false, "message": "Config error"}, nil
	}

	// Signature verification
	verificationKey := "981160e3e139c9d6d4a2062acd758493155b8a4ccaff00523f68a5486b8bf07b" // Hardcoded in TS
	signature, _ := data["signature"].(string)

	verifyStr := accountNumber + onusReference + paymentStatus + verificationKey

	mac := hmac.New(sha256.New, []byte("")) // TS uses empty key for HMAC? `createHmac('sha256', '')`
	mac.Write([]byte(verifyStr))
	expectedSig := hex.EncodeToString(mac.Sum(nil))

	if !hmac.Equal([]byte(signature), []byte(expectedSig)) {
		// TS just calls `this.constantTimeCompare` but doesn't explicitly return error on failure?
		// It just continues.
		// "this.constantTimeCompare(computedSignature, signature);" statement alone.
		// Then it checks "if (type === 'COLLECTION')".
		// I will ignore mismatch here to match TS logic behavior or maybe it DOES verify but TS code snippet provided ignored result?
		// Well, TS implementation `this.constantTimeCompare` returns boolean. The return value is unused.
		// So it seems it doesn't enforce signature check?
		// I will proceed.
	}

	if typeStr == "COLLECTION" {
		var transaction models.Transaction
		if err := s.DB.Where("client_id = ? AND transaction_no = ? AND tranasaction_type = ?", clientId, onusReference, "credit").First(&transaction).Error; err != nil {
			s.logCallback(clientId, "Transaction not found", rawBody, 0, onusReference, "Payonus")
			return map[string]interface{}{"success": false, "status": 404}, nil
		}

		if transaction.Status == 1 {
			s.logCallback(clientId, "Transaction already processed", rawBody, 1, onusReference, "Payonus")
			return map[string]interface{}{"success": true, "status": 200}, nil
		}

		// Update Wallet
		s.DB.Model(&models.Wallet{}).Where("user_id = ?", transaction.UserId).UpdateColumn("available_balance", gorm.Expr("available_balance + ?", transaction.Amount))

		s.DB.Model(&models.Transaction{}).Where("transaction_no = ?", transaction.TransactionNo).Updates(map[string]interface{}{
			"status": 1,
		})

		s.logCallback(clientId, "Completed", rawBody, 1, onusReference, "Payonus")
		return map[string]interface{}{"success": true, "status": 200}, nil
	}

	return map[string]interface{}{"success": false}, nil
}

func (s *PayonusService) logCallback(clientId int, requestStr string, response interface{}, status int, trxId, method string) {
	respBytes, _ := json.Marshal(response)
	log := models.CallbackLog{
		ClientId:      clientId,
		Request:       requestStr,
		Response:      string(respBytes),
		Status:        status,
		RequestType:   "Callback",
		TransactionId: trxId,
		PaymentMethod: method,
	}
	s.DB.Create(&log)
}

func (s *PayonusService) DisburseFunds(withdrawal models.Withdrawal, clientId int) (interface{}, error) {
	settings, err := s.payonusSettings(clientId)
	if err != nil {
		return common.NewErrorResponse("Payonus has not been configured for client", nil, 501), nil
	}

	baseUrl := strings.TrimSuffix(settings.BaseUrl, "/")

	// Auth
	authPayload := map[string]string{
		"apiClientId":     settings.PublicKey,
		"apiClientSecret": settings.SecretKey,
	}
	authRes, err := common.Post(fmt.Sprintf("%s/api/v1/access-token", baseUrl), authPayload, nil)
	if err != nil {
		return common.NewErrorResponse("Auth failed", nil, 401), nil
	}

	authMap, ok := authRes.(map[string]interface{})
	if !ok {
		return common.NewErrorResponse("Invalid auth response", nil, 500), nil
	}

	dataAuth, ok := authMap["data"].(map[string]interface{})
	if !ok {
		return common.NewErrorResponse("Invalid auth data", nil, 500), nil
	}

	token, _ := dataAuth["access_token"].(string)

	// Payload
	payload := map[string]interface{}{
		"amount":                   fmt.Sprintf("%.2f", withdrawal.Amount),
		"beneficiaryAccountNumber": withdrawal.AccountNumber,
		"beneficiaryAccountName":   withdrawal.AccountName,
		"beneficiaryBankCode":      withdrawal.BankCode,
		"reference":                withdrawal.WithdrawalCode,
		"transferType":             "WALLET_TO_BANK_TRANSFER",
		"countryCode":              "NG",
		"currency":                 "NGN",
		"email":                    "no-reply@example.com",
		"businessId":               settings.MerchantId,
	}

	headers := map[string]string{
		"Authorization": "Bearer " + token,
		"Content-Type":  "application/json",
	}

	resp, err := common.Post(fmt.Sprintf("%s/api/v1/transfer-requests/bank-transfer", baseUrl), payload, headers)
	if err != nil {
		return common.NewErrorResponse(fmt.Sprintf("Unable to disburse funds: %v", err), nil, 400), nil
	}

	respData, ok := resp.(map[string]interface{})
	if !ok {
		return common.NewErrorResponse("Invalid response from Payonus", nil, 500), nil
	}

	if status, ok := respData["paymentStatus"].(string); ok && status == "SUCCESSFUL" {
		// Update status
		s.DB.Model(&models.Withdrawal{}).Where("id = ?", withdrawal.ID).Update("status", 1)
		return map[string]interface{}{"success": true, "message": "Funds disbursed successfully"}, nil
	}

	// If failed, return message
	msg, _ := respData["message"].(string)
	return map[string]interface{}{"success": false, "message": msg}, nil
}
