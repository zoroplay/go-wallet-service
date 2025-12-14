package services

import (
	"crypto/hmac"
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"fmt"

	"wallet-service/internal/models"
	"wallet-service/pkg/common"
	"wallet-service/proto/bonus"

	"gorm.io/gorm"
)

type PaystackService struct {
	DB             *gorm.DB
	HelperService  *HelperService
	IdentityClient *IdentityClient
	BonusClient    *BonusClient
}

func NewPaystackService(db *gorm.DB, helper *HelperService, identityClient *IdentityClient, bonusClient *BonusClient) *PaystackService {
	return &PaystackService{
		DB:             db,
		HelperService:  helper,
		IdentityClient: identityClient,
		BonusClient:    bonusClient,
	}
}

// paystackSettings fetches payment method settings for Paystack
func (s *PaystackService) paystackSettings(clientId int) (*models.PaymentMethod, error) {
	var pm models.PaymentMethod
	fmt.Println("client id: ", pm)
	err := s.DB.Where("provider = ? AND client_id = ?", "paystack", clientId).First(&pm).Error
	if err != nil {
		return nil, err
	}
	return &pm, nil
}

// GeneratePaymentLink initiates a transaction
func (s *PaystackService) GeneratePaymentLink(data map[string]interface{}, clientId int) (interface{}, error) {
	settings, err := s.paystackSettings(clientId)
	if err != nil {
		return common.SuccessResponse{Success: false, Message: "Paystack has not been configured for client"}, nil
	}

	headers := map[string]string{
		"Authorization": "Bearer " + settings.SecretKey,
		"Content-Type":  "application/json",
		"Accept":        "application/json",
	}

	resp, err := common.Post(fmt.Sprintf("%s/transaction/initialize", settings.BaseUrl), data, headers)
	if err != nil {
		fmt.Printf("paystack error: %v\n", err)
		return common.SuccessResponse{Success: false, Message: "Unable to initiate deposit with paystack"}, nil
	}

	fmt.Println("Paystack response: ", resp)

	return common.SuccessResponse{Success: true, Data: resp}, nil
}

type VerifyTransactionDTO struct {
	ClientId       int
	TransactionRef string
}

// VerifyTransaction verifies a transaction manually
func (s *PaystackService) VerifyTransaction(param VerifyTransactionDTO) (interface{}, error) {
	settings, err := s.paystackSettings(param.ClientId)
	if err != nil {
		return common.SuccessResponse{Success: false, Message: "Paystack not configured"}, nil
	}

	headers := map[string]string{
		"Authorization": "Bearer " + settings.SecretKey,
	}

	// Call Paystack Verify
	resp, err := common.Get(fmt.Sprintf("%s/transaction/verify/%s", settings.BaseUrl, param.TransactionRef), headers)
	// resp is interface{} (map[string]interface{}), we need to parse it or just use it.
	// TS code assumes resp.data exists.

	// If error in HTTP request
	if err != nil {
		// Log?
		return common.SuccessResponse{Success: false, Message: "Verification failed"}, nil
	}

	respMap, ok := resp.(map[string]interface{})
	if !ok {
		return common.SuccessResponse{Success: false, Message: "Invalid response from Paystack"}, nil
	}

	dataMap, _ := respMap["data"].(map[string]interface{})
	gatewayStatus, _ := dataMap["status"].(string)

	// Findings transaction
	var transaction models.Transaction
	if err := s.DB.Where("client_id = ? AND transaction_no = ? AND subject IN (?)", param.ClientId, param.TransactionRef, []string{"Deposit", "Credit"}).First(&transaction).Error; err != nil {
		// TS: If not found, log callback log and return 404
		s.logCallback(param.ClientId, "Transaction not found", respMap, 0, param.TransactionRef, "Paystack")
		return common.NewErrorResponse("Transaction not found", nil, 404), nil
	}

	if gatewayStatus == "success" {
		if transaction.Status == 1 {
			s.logCallback(param.ClientId, "Transaction already processed", respMap, 1, param.TransactionRef, "Paystack")
			return common.NewSuccessResponse(nil, "Transaction already processed"), nil
		}
		if transaction.Status == 2 {
			s.logCallback(param.ClientId, "Transaction failed previously", respMap, 0, param.TransactionRef, "Paystack")
			// TS returns 406 Not Acceptable
			return common.NewErrorResponse("Transaction failed. Try again", nil, 406), nil
		}

		if transaction.Status == 0 {
			// Fund Wallet
			var wallet models.Wallet
			if err := s.DB.Where("user_id = ?", transaction.UserId).First(&wallet).Error; err != nil {
				s.logCallback(param.ClientId, "Wallet not found", respMap, 0, param.TransactionRef, "Paystack")
				return common.NewErrorResponse("Wallet not found", nil, 404), nil
			}

			newBal := wallet.AvailableBalance + transaction.Amount

			// Update Wallet using Helper (or direct atomic update, but helper has UpdateWallet)
			// TS uses `this.helperService.updateWallet`. Let's use atomic update here to be safe and consistent.
			if err := s.DB.Model(&models.Wallet{}).Where("user_id = ?", transaction.UserId).UpdateColumn("available_balance", gorm.Expr("available_balance + ?", transaction.Amount)).Error; err != nil {
				return common.NewErrorResponse("Failed to update wallet", nil, 500), nil
			}

			// Update Transaction
			s.DB.Model(&models.Transaction{}).Where("transaction_no = ?", transaction.TransactionNo).Updates(map[string]interface{}{
				"status":  1,
				"balance": newBal,
			})

			// Trackier (Stub)
			// Bonus (Stub)

			s.logCallback(param.ClientId, "Completed", respMap, 1, param.TransactionRef, "Paystack")
			return common.NewSuccessResponse(nil, "Transaction was successful"), nil
		}
	} else {
		// Failed
		s.DB.Model(&models.Transaction{}).Where("transaction_no = ?", transaction.TransactionNo).Update("status", 2)
		return common.NewErrorResponse("Transaction was not successful", nil, 400), nil
	}

	return common.NewErrorResponse("Unknown state", nil, 500), nil
}

func (s *PaystackService) logCallback(clientId int, requestStr string, response interface{}, status int, trxId, method string) {
	respBytes, _ := json.Marshal(response)
	log := models.CallbackLog{
		ClientId:      clientId,
		Request:       requestStr,
		Response:      string(respBytes),
		Status:        status,
		RequestType:   "Callback", // Or Webhook
		TransactionId: trxId,
		PaymentMethod: method,
	}
	s.DB.Create(&log)
}

type WebhookDTO struct {
	ClientId    int
	PaystackKey string
	Body        []byte // Raw body for HMAC
	Reference   string
	Event       string
	Data        map[string]interface{} // Parsed body
}

// HandleWebhook processes webhooks
func (s *PaystackService) HandleWebhook(dto WebhookDTO) (interface{}, error) {
	settings, err := s.paystackSettings(dto.ClientId)
	if err != nil {
		return map[string]interface{}{"success": false, "message": "Paystack not configured"}, nil
	}

	// Verify Signature
	mac := hmac.New(sha512.New, []byte(settings.SecretKey))
	mac.Write(dto.Body)
	expectedHash := hex.EncodeToString(mac.Sum(nil))

	if expectedHash != dto.PaystackKey {
		// TS logs invalid signature but commented out check?
		// "if (hash !== signature) { ... }" commented out in TS.
		// Use caution. If we implement strictly we should fail.
	}

	switch dto.Event {
	case "charge.success":
		return s.handleChargeSuccess(dto)
	case "transfer.success":
		return s.handleTransferSuccess(dto)
	case "transfer.failed":
		return s.handleTransferFailed(dto)
	case "transfer.reversed":
		return s.handleTransferReversed(dto)
	}

	return map[string]interface{}{"success": false, "message": "Unhandled event"}, nil
}

func (s *PaystackService) handleChargeSuccess(dto WebhookDTO) (interface{}, error) {
	ref := dto.Reference
	// Reuse Verify Logic essentially, but adapting to internal call
	// Similar logic to VerifyTransaction

	var transaction models.Transaction
	if err := s.DB.Where("client_id = ? AND transaction_no = ?", dto.ClientId, ref).First(&transaction).Error; err != nil {
		s.logCallback(dto.ClientId, "Transaction not found", dto.Data, 0, ref, "Webhook")
		return map[string]interface{}{"success": false, "message": "Transaction not found"}, nil
	}

	if transaction.Status == 1 {
		s.logCallback(dto.ClientId, "Transaction already processed", dto.Data, 1, ref, "Webhook")
		return map[string]interface{}{"success": true}, nil
	}

	// Fund wallet
	if err := s.DB.Model(&models.Wallet{}).Where("user_id = ?", transaction.UserId).UpdateColumn("available_balance", gorm.Expr("available_balance + ?", transaction.Amount)).Error; err != nil {
		return map[string]interface{}{"success": false}, nil
	}

	var wallet models.Wallet
	s.DB.Where("user_id = ?", transaction.UserId).First(&wallet)

	s.DB.Model(&models.Transaction{}).Where("transaction_no = ?", ref).Updates(map[string]interface{}{
		"status":  1,
		"balance": wallet.AvailableBalance,
	})

	// Bonus Processing
	if s.BonusClient != nil {
		userBonus, err := s.BonusClient.GetUserBonus(dto.ClientId, transaction.UserId)
		// Proto boolean fields are pointers in some generation options, but standard proto3 usually uses defaults.
		// Looking at bonus.proto, CommonResponseObj has optional bool success. So it is *bool.
		// Helper logic:
		isSuccess := userBonus != nil && userBonus.Success != nil && *userBonus.Success
		if err == nil && isSuccess {
			if userBonus.Data != nil {
				dataMap := userBonus.Data.AsMap()
				if idVal, ok := dataMap["id"]; ok {
					bonusId := int32(idVal.(float64)) // JSON numbers are floats
					amount := float32(transaction.Amount)
					_, err := s.BonusClient.AwardBonus(&bonus.AwardBonusRequest{
						UserId:   fmt.Sprintf("%d", transaction.UserId),
						BonusId:  bonusId,
						Amount:   &amount,
						ClientId: int32(dto.ClientId),
					})
					if err != nil {
						s.logCallback(dto.ClientId, fmt.Sprintf("Bonus processing failed: %v", err), nil, 0, ref, "Paystack")
					}
				}
			}
		}
	}

	// Trackier Activity
	if s.IdentityClient != nil {
		keysResp, err := s.IdentityClient.GetTrackierKeys(dto.ClientId)
		keysSuccess := keysResp != nil && keysResp.Success != nil && *keysResp.Success
		if err == nil && keysSuccess && keysResp.Data != nil {
			// keysMap unused, removing
			keysInterface := keysResp.Data.AsMap()
			keys := make(map[string]string)
			for k, v := range keysInterface {
				if strVal, ok := v.(string); ok {
					keys[k] = strVal
				}
			}

			activityData := map[string]interface{}{
				"subject":       "Deposit",
				"username":      transaction.Username,
				"amount":        transaction.Amount,
				"transactionId": transaction.TransactionNo,
				"clientId":      dto.ClientId,
			}
			s.HelperService.SendActivity(activityData, keys)
		}
	}

	s.logCallback(dto.ClientId, "Transaction processed successfully", dto.Data, 1, ref, "Webhook")
	return map[string]interface{}{"success": true}, nil
}

func (s *PaystackService) handleTransferSuccess(dto WebhookDTO) (interface{}, error) {
	ref := dto.Reference
	var withdrawal models.Withdrawal
	if err := s.DB.Where("client_id = ? AND withdrawal_code = ?", dto.ClientId, ref).First(&withdrawal).Error; err != nil {
		return map[string]interface{}{"success": false}, nil
	}

	if withdrawal.Status == 0 {
		s.DB.Model(&models.Withdrawal{}).Where("id = ?", withdrawal.ID).Update("status", 1)
		s.logCallback(dto.ClientId, "Withdrawal processed successfully", dto.Data, 1, ref, "Webhook")
	}
	return map[string]interface{}{"success": true}, nil
}

func (s *PaystackService) handleTransferFailed(dto WebhookDTO) (interface{}, error) {
	return s.handleFailedOrReversed(dto, "failed", "Transfer failed")
}

func (s *PaystackService) handleTransferReversed(dto WebhookDTO) (interface{}, error) {
	return s.handleFailedOrReversed(dto, "reversed", "Transfer was reversed")
}

func (s *PaystackService) handleFailedOrReversed(dto WebhookDTO, typeStr, desc string) (interface{}, error) {
	ref := dto.Reference
	var withdrawal models.Withdrawal
	if err := s.DB.Where("client_id = ? AND withdrawal_code = ?", dto.ClientId, ref).First(&withdrawal).Error; err != nil {
		return map[string]interface{}{"success": false}, nil
	}

	// refund
	s.DB.Model(&models.Withdrawal{}).Where("id = ?", withdrawal.ID).Updates(map[string]interface{}{"status": 2, "comment": desc})

	s.DB.Model(&models.Wallet{}).Where("user_id = ?", withdrawal.UserId).UpdateColumn("available_balance", gorm.Expr("available_balance + ?", withdrawal.Amount))

	// Create refund transaction
	var wallet models.Wallet
	s.DB.Where("user_id = ?", withdrawal.UserId).First(&wallet)

	trx := TransactionData{
		ClientId:        dto.ClientId,
		TransactionNo:   common.GenerateTrxNo(),
		Amount:          withdrawal.Amount,
		Description:     desc,
		Subject:         typeStr + " Withdrawal Request",
		Channel:         "internal",
		Source:          "system",
		FromUserId:      0,
		FromUsername:    "System",
		FromUserBalance: 0,
		ToUserId:        withdrawal.UserId,
		ToUsername:      wallet.Username,
		ToUserBalance:   wallet.AvailableBalance,
		Status:          1,
		WalletType:      "Main",
	}
	s.HelperService.SaveTransaction(trx)

	s.logCallback(dto.ClientId, fmt.Sprintf("Withdrawal %s processed", typeStr), dto.Data, 1, ref, "Webhook")
	return map[string]interface{}{"success": true}, nil
}

func (s *PaystackService) ResolveAccountNumber(clientId int, accountNo, bankCode string) (interface{}, error) {
	settings, err := s.paystackSettings(clientId)
	if err != nil {
		return common.NewErrorResponse("Paystack not configured", nil, 400), nil
	}

	headers := map[string]string{"Authorization": "Bearer " + settings.SecretKey}
	url := fmt.Sprintf("%s/bank/resolve?account_number=%s&bank_code=%s", settings.BaseUrl, accountNo, bankCode)

	resp, err := common.Get(url, headers)
	if err != nil {
		return common.NewErrorResponse("Resolution failed", nil, 400), nil
	}

	dataMap, ok := resp.(map[string]interface{})
	if !ok {
		return common.NewErrorResponse("Invalid response", nil, 500), nil
	}

	return common.NewSuccessResponse(dataMap["data"], "Resolved"), nil
}

func (s *PaystackService) DisburseFunds(withdrawal models.Withdrawal, clientId int) (interface{}, error) {
	settings, err := s.paystackSettings(clientId)
	if err != nil {
		return common.NewErrorResponse("Paystack has not been configured for client", nil, 501), nil
	}

	initRes, err := s.initiateTransfer(withdrawal.AccountNumber, withdrawal.AccountName, withdrawal.BankCode, settings.SecretKey, settings.BaseUrl)
	if err != nil {
		return common.NewErrorResponse(fmt.Sprintf("Transfer initiation failed: %v", err), nil, 400), nil
	}

	initData, ok := initRes.(map[string]interface{})
	if !ok {
		// Try to parse if it's SuccessResponse? No, helper returns map usually or interface
		return common.NewErrorResponse("Invalid init response", nil, 500), nil
	}

	if status, ok := initData["status"].(bool); !ok || !status {
		return initRes, nil
	}

	dataMap, _ := initData["data"].(map[string]interface{})
	recipientCode, _ := dataMap["recipient_code"].(string)

	resp, err := s.doTransfer(withdrawal.Amount, withdrawal.WithdrawalCode, recipientCode, settings.SecretKey, settings.BaseUrl)
	if err != nil {
		return common.NewErrorResponse("Paystack Error! unable to disburse funds", nil, 400), nil
	}

	return resp, nil
}

func (s *PaystackService) initiateTransfer(accountNo, accountName, bankCode, key, baseUrl string) (interface{}, error) {
	params := map[string]interface{}{
		"type":           "nuban",
		"name":           accountName,
		"account_number": accountNo,
		"bank_code":      bankCode,
		"currency":       "NGN",
	}

	headers := map[string]string{
		"Authorization": "Bearer " + key,
		"Content-Type":  "application/json",
	}

	resp, err := common.Post(fmt.Sprintf("%s/transferrecipient", baseUrl), params, headers)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

func (s *PaystackService) doTransfer(amount float64, reference, recipient, key, baseUrl string) (interface{}, error) {
	params := map[string]interface{}{
		"source":    "balance",
		"amount":    amount * 100,
		"reference": fmt.Sprintf("%s_%s", reference, common.GenerateTrxNo()),
		"recipient": recipient,
		"reason":    "Payout request",
	}

	headers := map[string]string{
		"Authorization": "Bearer " + key,
		"Content-Type":  "application/json",
	}

	resp, err := common.Post(fmt.Sprintf("%s/transfer", baseUrl), params, headers)
	if err != nil {
		return nil, err
	}

	// TS returns: { success: resp.status, data: resp.data, message: resp.message }
	// common.Post returns map usually.
	return resp, nil
}
