package services

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"wallet-service/internal/models"
	"wallet-service/pkg/common"

	"gorm.io/gorm"
)

type CoralPayService struct {
	DB            *gorm.DB
	HelperService *HelperService
}

func NewCoralPayService(db *gorm.DB, helper *HelperService) *CoralPayService {
	return &CoralPayService{
		DB:            db,
		HelperService: helper,
	}
}

func (s *CoralPayService) coralPaySettings(clientId int) (*models.PaymentMethod, error) {
	var pm models.PaymentMethod
	err := s.DB.Where("provider = ? AND client_id = ?", "coralpay", clientId).First(&pm).Error
	if err != nil {
		return nil, err
	}
	return &pm, nil
}

func (s *CoralPayService) generateSignature(merchantId, traceId, timeStamp, key string) string {
	raw := fmt.Sprintf("%s%s%s%s", merchantId, traceId, timeStamp, key)
	hash := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(hash[:])
}

// InitiatePayment initiates a payment
func (s *CoralPayService) InitiatePayment(data map[string]interface{}, clientId int) (interface{}, error) {
	settings, err := s.coralPaySettings(clientId)
	if err != nil {
		return common.SuccessResponse{Success: false, Message: "Coralpay has not been configured for client"}, nil
	}

	// Auth
	authPayload := map[string]string{
		"username": settings.PublicKey,
		"password": settings.SecretKey,
	}
	authResp, err := common.Post(fmt.Sprintf("%s/Authentication", settings.BaseUrl), authPayload, nil)
	if err != nil {
		fmt.Printf("CoralPay Auth Error: %v\n", err)
		return common.SuccessResponse{Success: false, Message: "Authentication failed"}, nil
	}

	authMap, ok := authResp.(map[string]interface{})
	if !ok {
		return common.SuccessResponse{Success: false, Message: "Invalid auth response"}, nil
	}

	token, _ := authMap["token"].(string)
	key, _ := authMap["key"].(string)

	merchantId := settings.MerchantId
	timeStamp := fmt.Sprintf("%d", time.Now().Unix())

	traceId, _ := data["traceId"].(string)

	signature := s.generateSignature(merchantId, traceId, timeStamp, key)

	reqHeader := map[string]string{
		"merchantId": merchantId,
		"timeStamp":  timeStamp,
		"signature":  signature,
	}

	// Construct Payload
	payload := make(map[string]interface{})
	for k, v := range data {
		payload[k] = v
	}
	payload["requestHeader"] = reqHeader

	headers := map[string]string{
		"Authorization": "Bearer " + token,
		"Content-Type":  "application/json",
	}

	res, err := common.Post(fmt.Sprintf("%s/InvokePayment", settings.BaseUrl), payload, headers)
	if err != nil {
		fmt.Printf("CoralPay Payment Error: %v\n", err)
		return common.SuccessResponse{Success: false, Message: "Payment request failed"}, nil // Todo: return error details from res if possible
	}

	resMap, ok := res.(map[string]interface{})
	if !ok {
		return common.SuccessResponse{Success: false, Message: "Invalid payment response"}, nil
	}

	return common.SuccessResponse{Success: true, Data: resMap["payPageLink"]}, nil
}

type CoralPayWebhookDTO struct {
	ClientId     int
	CallbackData map[string]interface{}
}

func (s *CoralPayService) HandleWebhook(param CoralPayWebhookDTO) (interface{}, error) {
	settings, err := s.coralPaySettings(param.ClientId)
	if err != nil {
		return common.NewErrorResponse("CoralPay not configured", nil, 400), nil
	}

	payload, _ := param.CallbackData["payload"].(map[string]interface{})
	ref, _ := payload["transactionReference"].(string)

	var transaction models.Transaction
	if err := s.DB.Where("client_id = ? AND transaction_no = ? AND tranasaction_type = ?", param.ClientId, ref, "credit").First(&transaction).Error; err != nil {
		s.logCallback(param.ClientId, "Transaction not found", param.CallbackData, 0, ref, "CoralPay")
		return common.NewErrorResponse("Transaction not found", nil, 404), nil
	}

	if transaction.Status == 1 {
		return common.NewSuccessResponse(nil, "Verified"), nil
	}
	if transaction.Status == 2 {
		return common.NewErrorResponse("Transaction failed", nil, 406), nil
	}

	if s.verifySignature(param.CallbackData, settings.SecretKey) {
		status, _ := payload["status"].(string)
		if status == "SUCCESSFUL" {
			if err := s.DB.Model(&models.Wallet{}).Where("user_id = ?", transaction.UserId).UpdateColumn("available_balance", gorm.Expr("available_balance + ?", transaction.Amount)).Error; err != nil {
				return common.NewErrorResponse("Wallet not found", nil, 404), nil
			}

			var wallet models.Wallet
			s.DB.Where("user_id = ?", transaction.UserId).First(&wallet)

			s.DB.Model(&models.Transaction{}).Where("transaction_no = ?", transaction.TransactionNo).Updates(map[string]interface{}{
				"status":            1,
				"available_balance": wallet.AvailableBalance,
			})

			s.logCallback(param.ClientId, "Completed", param.CallbackData, 1, ref, "CoralPay")
			return common.NewSuccessResponse(nil, "Transaction successfully verified and processed"), nil
		}
	}

	return common.NewErrorResponse("Transaction failed", nil, 400), nil
}

func (s *CoralPayService) validateCallbackPayload(data map[string]interface{}) string {
	requiredFields := []string{"MerchantId", "TraceId", "TransactionId", "Amount", "ResponseCode", "Signature", "TimeStamp"}
	for _, field := range requiredFields {
		if _, ok := data[field]; !ok {
			return fmt.Sprintf("Missing required field: %s", field)
		}
	}
	return ""
}

func (s *CoralPayService) verifySignature(data map[string]interface{}, secretKey string) bool {
	mId, _ := data["MerchantId"].(string)
	tId, _ := data["TraceId"].(string)
	ts, _ := data["TimeStamp"].(string)

	signatureBase := fmt.Sprintf("%s%s%s", mId, tId, ts)

	mac := hmac.New(sha256.New, []byte(secretKey))
	mac.Write([]byte(signatureBase))
	expectedSignature := hex.EncodeToString(mac.Sum(nil))

	recSig, _ := data["Signature"].(string)

	return recSig == expectedSignature
}

func (s *CoralPayService) VerifyTransaction(param VerifyTransactionDTO) (interface{}, error) {
	var transaction models.Transaction
	if err := s.DB.Where("client_id = ? AND transaction_no = ? AND subject = ?", param.ClientId, param.TransactionRef, "Deposit").First(&transaction).Error; err != nil {
		return common.NewErrorResponse("Transaction not found", nil, 404), nil
	}

	if transaction.Status == 1 {
		return common.NewSuccessResponse(nil, "Verified"), nil
	}
	if transaction.Status == 2 {
		return common.NewErrorResponse("Transaction failed. Try again", nil, 406), nil
	}

	// For CoralPay, VerifyTransaction in TS just credits if found and not successful.
	// It doesn't call an external API.
	if err := s.DB.Model(&models.Wallet{}).Where("user_id = ?", transaction.UserId).UpdateColumn("available_balance", gorm.Expr("available_balance + ?", transaction.Amount)).Error; err != nil {
		return common.NewErrorResponse("Wallet not found for this user", nil, 404), nil
	}

	var wallet models.Wallet
	s.DB.Where("user_id = ?", transaction.UserId).First(&wallet)

	s.DB.Model(&models.Transaction{}).Where("transaction_no = ?", transaction.TransactionNo).Updates(map[string]interface{}{
		"status":            1,
		"available_balance": wallet.AvailableBalance,
	})

	return common.NewSuccessResponse(nil, "Transaction successfully verified and processed"), nil
}

func (s *CoralPayService) logCallback(clientId int, requestStr string, response interface{}, status int, trxId, method string) {
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
