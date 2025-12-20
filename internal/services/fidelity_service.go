package services

import (
	"crypto/md5"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"

	"wallet-service/internal/models"
	"wallet-service/pkg/common"

	"gorm.io/gorm"
)

type FidelityService struct {
	DB            *gorm.DB
	HelperService *HelperService
}

func NewFidelityService(db *gorm.DB, helper *HelperService) *FidelityService {
	return &FidelityService{
		DB:            db,
		HelperService: helper,
	}
}

func (s *FidelityService) fidelitySettings(clientId int) (*models.PaymentMethod, error) {
	var pm models.PaymentMethod
	err := s.DB.Where("provider = ? AND client_id = ?", "fidelity", clientId).First(&pm).Error
	if err != nil {
		return nil, err
	}
	return &pm, nil
}

func (s *FidelityService) InitiatePay(data map[string]interface{}, clientId int) (interface{}, error) {
	settings, err := s.fidelitySettings(clientId)
	if err != nil {
		return common.SuccessResponse{Success: false, Message: "Fidelity has not been configured for client"}, nil
	}
	// As per TS, simply returns Success: true
	_ = settings // unused in TS logic for initiate except check
	return common.SuccessResponse{Success: true, Data: data}, nil
}

type FidelityPayData struct {
	RequestRef  string
	Transaction map[string]interface{}
	// Other fields...
}

func (s *FidelityService) HandlePay(data map[string]interface{}, clientId int) (interface{}, error) {
	settings, err := s.fidelitySettings(clientId)
	if err != nil {
		return common.SuccessResponse{Success: false, Message: "Fidelity has not been configured for client"}, nil
	}

	requestRef, _ := data["request_ref"].(string)

	// MD5(request_ref;client_secret)
	sigRaw := fmt.Sprintf("%s;%s", requestRef, settings.SecretKey)
	hash := md5.Sum([]byte(sigRaw))
	signature := hex.EncodeToString(hash[:])

	headers := map[string]string{
		"Authorization": "Bearer " + settings.PublicKey,
		"Signature":     signature,
		"Content-Type":  "application/json",
	}

	// Payload
	payload := make(map[string]interface{})
	for k, v := range data {
		payload[k] = v
	}
	payload["request_ref"] = requestRef

	url := fmt.Sprintf("%s/v2/transact", settings.BaseUrl)
	resp, err := common.Post(url, payload, headers)
	if err != nil {
		fmt.Printf("Fidelity Pay Error: %v\n", err)
		return common.SuccessResponse{Success: false, Message: "Payment request failed"}, nil // Simplify error for now
	}

	respMap, ok := resp.(map[string]interface{})
	if !ok {
		return common.SuccessResponse{Success: false, Message: "Invalid response"}, nil
	}

	dataResp, _ := respMap["data"].(map[string]interface{})
	providerResp, _ := dataResp["provider_response"].(map[string]interface{})
	status, _ := respMap["status"].(string) // or bool? TS says `response.data.status`

	// Mapping response
	// The TS expects `data.transaction.amount` but `data` here is the argument.
	// data argument has transaction? TS: `data.transaction.amount`.
	// Let's assume input data has it.

	trx, _ := data["transaction"].(map[string]interface{})
	amount := trx["amount"]

	resObject := map[string]interface{}{
		"amount":         amount,
		"bank_name":      providerResp["bank_name"],
		"account_number": providerResp["account_number"],
		"account_name":   providerResp["account_name"],
		"currency_code":  providerResp["currency_code"],
		"reference":      providerResp["reference"],
		"status":         status,
	}

	return resObject, nil
}

type FidelityWebhookDTO struct {
	ClientId             int
	TransactionReference string
	RawBody              map[string]interface{}
	// For handleCallback
	TransactionRef string
}

// Unify Webhook and Callback handling since they are similar in TS
// TS: handleWebhook(data) where data has clientId, transactionReference, rawBody...
// TS: handleCallback(data) has transactionRef...

func (s *FidelityService) HandleWebhook(dto FidelityWebhookDTO) (interface{}, error) {
	ref := dto.TransactionReference
	if ref == "" {
		ref = dto.TransactionRef
	}

	var transaction models.Transaction
	if err := s.DB.Where("client_id = ? AND transaction_no = ? AND tranasaction_type = ?", dto.ClientId, ref, "credit").First(&transaction).Error; err != nil {
		s.logCallback(dto.ClientId, "Transaction not found", dto.RawBody, 0, ref, "Fidelity")
		return common.NewErrorResponse("Transaction not found", nil, 404), nil
	}

	if transaction.Status == 1 {
		return map[string]interface{}{"success": true, "message": "Verified"}, nil
	}
	if transaction.Status == 2 {
		return map[string]interface{}{"success": false, "message": "Transaction failed"}, nil
	}

	rawBody := dto.RawBody
	wbBody, _ := rawBody["webhookBody"].(map[string]interface{})

	typeStr, _ := wbBody["type"].(string)
	statusOk, _ := wbBody["statusOk"].(bool)
	statusInt, _ := wbBody["status"].(float64)

	successCondition := typeStr == "success" && statusOk == true && (statusInt == 201 || statusInt == 200)

	if successCondition {
		// Update Wallet
		if err := s.DB.Model(&models.Wallet{}).Where("user_id = ?", transaction.UserId).UpdateColumn("available_balance", gorm.Expr("available_balance + ?", transaction.Amount)).Error; err != nil {
			s.logCallback(dto.ClientId, "Wallet not found", dto.RawBody, 0, ref, "Fidelity")
			return common.NewErrorResponse("Wallet not found for this user", nil, 404), nil
		}

		var wallet models.Wallet
		s.DB.Where("user_id = ?", transaction.UserId).First(&wallet)

		s.DB.Model(&models.Transaction{}).Where("transaction_no = ?", transaction.TransactionNo).Updates(map[string]interface{}{
			"status":            1,
			"available_balance": wallet.AvailableBalance,
		})

		s.logCallback(dto.ClientId, "Completed", dto.RawBody, 1, ref, "Fidelity")
		return common.NewSuccessResponse(nil, "Transaction successfully verified and processed"), nil
	}

	return common.NewSuccessResponse(nil, "Transaction not successful"), nil
}

func (s *FidelityService) HandleCallback(dto FidelityWebhookDTO) (interface{}, error) {
	ref := dto.TransactionRef
	var transaction models.Transaction
	if err := s.DB.Where("client_id = ? AND transaction_no = ? AND tranasaction_type = ?", dto.ClientId, ref, "credit").First(&transaction).Error; err != nil {
		s.logCallback(dto.ClientId, "Transaction not found", dto, 0, ref, "Fidelity")
		return common.NewErrorResponse("Transaction not found", nil, 404), nil
	}

	if transaction.Status == 1 {
		return map[string]interface{}{"success": true, "message": "Verified"}, nil
	}
	if transaction.Status == 2 {
		return map[string]interface{}{"success": false, "message": "Transaction failed"}, nil
	}

	if err := s.DB.Model(&models.Wallet{}).Where("user_id = ?", transaction.UserId).UpdateColumn("available_balance", gorm.Expr("available_balance + ?", transaction.Amount)).Error; err != nil {
		s.logCallback(dto.ClientId, "Wallet not found", dto, 0, ref, "Fidelity")
		return common.NewErrorResponse("Wallet not found for this user", nil, 404), nil
	}

	var wallet models.Wallet
	s.DB.Where("user_id = ?", transaction.UserId).First(&wallet)

	s.DB.Model(&models.Transaction{}).Where("transaction_no = ?", transaction.TransactionNo).Updates(map[string]interface{}{
		"status":            1,
		"available_balance": wallet.AvailableBalance,
	})

	s.logCallback(dto.ClientId, "Completed", dto, 1, ref, "Fidelity")
	return common.NewSuccessResponse(nil, "Transaction successfully verified and processed"), nil
}

func (s *FidelityService) HandleVerifyPay(dto FidelityWebhookDTO) (interface{}, error) {
	settings, err := s.fidelitySettings(dto.ClientId)
	if err != nil {
		return common.SuccessResponse{Success: false, Message: "Fidelity has not been configured for client"}, nil
	}

	encodedKey := base64.StdEncoding.EncodeToString([]byte(settings.PublicKey))
	headers := map[string]string{
		"Authorization": "Bearer " + encodedKey,
		"Content-Type":  "application/json",
	}

	url := fmt.Sprintf("%s/transfer_notification_controller/transaction-query?transaction_reference=%s", settings.BaseUrl, dto.TransactionRef)
	resp, err := common.Get(url, headers)
	if err != nil {
		// TS logs error
		return common.SuccessResponse{Success: false, Message: "Error verifying"}, nil
	}

	respMap, ok := resp.(map[string]interface{})
	if !ok {
		return common.SuccessResponse{Success: false, Message: "Invalid response"}, nil
	}

	success, _ := respMap["success"].(bool)
	dataMap, _ := respMap["data"].(map[string]interface{})
	merchantRef, _ := dataMap["merchant_transaction_reference"].(string)

	if success && merchantRef == dto.TransactionRef {
		return s.HandleCallback(dto) // Reuse Logic as TS duplicates it
	}

	return common.NewErrorResponse("Verification failed", nil, 400), nil
}

func (s *FidelityService) FidelityWebhook(dto map[string]interface{}) (interface{}, error) {
	// Logic from fidelityWebhook
	// Checks success == true and transactionStatus == "Completed"
	// Then credit.

	success, _ := dto["success"].(bool)
	txStatus, _ := dto["transactionStatus"].(string)
	clientIdFloat, _ := dto["clientId"].(float64)
	clientId := int(clientIdFloat)
	ref, _ := dto["transactionReference"].(string)

	if success && txStatus == "Completed" {
		// Reuse crediting logic
		return s.HandleCallback(FidelityWebhookDTO{ClientId: clientId, TransactionRef: ref})
	}

	return common.NewErrorResponse("Transaction not completed", nil, 400), nil
}

func (s *FidelityService) logCallback(clientId int, requestStr string, response interface{}, status int, trxId, method string) {
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
