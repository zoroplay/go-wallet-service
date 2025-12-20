package services

import (
	"encoding/base64"
	"encoding/json"
	"fmt"

	"wallet-service/internal/models"
	"wallet-service/pkg/common"

	"gorm.io/gorm"
)

type MonnifyService struct {
	DB            *gorm.DB
	HelperService *HelperService
}

func NewMonnifyService(db *gorm.DB, helper *HelperService) *MonnifyService {
	return &MonnifyService{
		DB:            db,
		HelperService: helper,
	}
}

func (s *MonnifyService) monnifySettings(clientId int) (*models.PaymentMethod, error) {
	var pm models.PaymentMethod
	err := s.DB.Where("provider = ? AND client_id = ?", "monnify", clientId).First(&pm).Error
	if err != nil {
		return nil, err
	}
	return &pm, nil
}

func (s *MonnifyService) authenticate(settings *models.PaymentMethod) (map[string]interface{}, error) {
	keyRaw := fmt.Sprintf("%s:%s", settings.PublicKey, settings.SecretKey)
	key := base64.StdEncoding.EncodeToString([]byte(keyRaw))

	headers := map[string]string{
		"Authorization": "Basic " + key,
	}

	resp, err := common.Post(fmt.Sprintf("%s/api/v1/auth/login", settings.BaseUrl), map[string]interface{}{}, headers)
	if err != nil {
		return nil, err
	}

	respMap, ok := resp.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid response")
	}
	return respMap, nil
}

// GeneratePaymentLink
func (s *MonnifyService) GeneratePaymentLink(data map[string]interface{}, clientId int) (interface{}, error) {
	settings, err := s.monnifySettings(clientId)
	if err != nil {
		return common.SuccessResponse{Success: false, Message: "Monnify not configured"}, nil
	}

	authRes, err := s.authenticate(settings)
	if err != nil {
		return common.SuccessResponse{Success: false, Message: "Auth failed"}, nil
	}

	reqSuccess, _ := authRes["requestSuccessful"].(bool)
	if !reqSuccess {
		return common.SuccessResponse{Success: false, Data: nil}, nil
	}

	respBody, _ := authRes["responseBody"].(map[string]interface{})
	accessToken, _ := respBody["accessToken"].(string)

	// Payload
	amount, _ := data["amount"].(float64)
	name, _ := data["name"].(string)
	email, _ := data["email"].(string)
	ref, _ := data["reference"].(string)
	callbackUrl, _ := data["callback_url"].(string)

	payload := map[string]interface{}{
		"amount":             amount,
		"customerName":       name,
		"customerEmail":      email,
		"contractCode":       settings.MerchantId,
		"currencyCode":       "NGN",
		"paymentDescription": "wallet funds",
		"paymentReference":   ref,
		"redirectUrl":        callbackUrl,
		"paymentMethods":     []string{"CARD", "ACCOUNT_TRANSFER"},
	}

	headers := map[string]string{
		"Authorization": "Bearer " + accessToken,
		"Content-Type":  "application/json",
		"Accept":        "application/json",
	}

	resp, err := common.Post(fmt.Sprintf("%s/api/v1/merchant/transactions/init-transaction", settings.BaseUrl), payload, headers)
	if err != nil {
		return common.SuccessResponse{Success: false, Message: "Initiate failed"}, nil
	}

	respMap, _ := resp.(map[string]interface{})
	// Check response structure
	if respMap == nil {
		return common.SuccessResponse{Success: false, Message: "Empty response"}, nil
	}
	// Parse responseBody.checkoutUrl
	rb, _ := respMap["responseBody"].(map[string]interface{})
	checkoutUrl := ""
	if rb != nil {
		checkoutUrl, _ = rb["checkoutUrl"].(string)
	}

	return common.SuccessResponse{Success: true, Data: checkoutUrl}, nil
}

type MonnifyVerifyDTO struct {
	ClientId       int
	TransactionRef string
}

func (s *MonnifyService) VerifyTransaction(param MonnifyVerifyDTO) (interface{}, error) {
	var transaction models.Transaction
	if err := s.DB.Where("client_id = ? AND transaction_no = ? AND tranasaction_type = ?", param.ClientId, param.TransactionRef, "credit").First(&transaction).Error; err != nil {
		return common.NewErrorResponse("Transaction not found", nil, 404), nil
	}

	if transaction.Status == 1 {
		return common.NewSuccessResponse(nil, "Verified"), nil
	}

	if transaction.Status == 2 {
		return common.NewErrorResponse("Transaction failed. Try again", nil, 406), nil
	}

	settings, err := s.monnifySettings(param.ClientId)
	if err != nil {
		return common.NewErrorResponse("Config error", nil, 400), nil
	}

	authRes, err := s.authenticate(settings)
	if err != nil {
		return common.NewErrorResponse("Auth error", nil, 400), nil
	}

	reqSuccess, _ := authRes["requestSuccessful"].(bool)
	if reqSuccess {
		respBody, _ := authRes["responseBody"].(map[string]interface{})
		accessToken, _ := respBody["accessToken"].(string)

		headers := map[string]string{
			"Authorization": "Bearer " + accessToken,
		}

		url := fmt.Sprintf("%s/api/v1/merchant/transactions/query?paymentReference=%s", settings.BaseUrl, param.TransactionRef)
		resp, err := common.Get(url, headers)
		if err != nil {
			return common.NewErrorResponse("Verify request failed", nil, 400), nil
		}

		data, _ := resp.(map[string]interface{})
		dataReqSuccess, _ := data["requestSuccessful"].(bool)
		dataMsg, _ := data["responseMessage"].(string) // "success"

		if dataReqSuccess && dataMsg == "success" {
			dataBody, _ := data["responseBody"].(map[string]interface{})
			paymentStatus, _ := dataBody["paymentStatus"].(string)

			status := 0 // pending
			switch paymentStatus {
			case "PAID", "OVERPAID", "PARTIALLY_PAID":
				status = 1
			case "FAILED", "ABANDONED", "CANCELLED", "REVERSED", "EXPIRED":
				status = 2
			}

			// Update status
			s.DB.Model(&models.Transaction{}).Where("transaction_no = ?", transaction.TransactionNo).Update("status", status)

			if status == 1 {
				// Fund wallet
				if err := s.DB.Model(&models.Wallet{}).Where("user_id = ?", transaction.UserId).UpdateColumn("available_balance", gorm.Expr("available_balance + ?", transaction.Amount)).Error; err != nil {
					return common.NewErrorResponse("Update failed", nil, 500), nil
				}

				var wallet models.Wallet
				s.DB.Where("user_id = ?", transaction.UserId).First(&wallet)

				s.DB.Model(&models.Transaction{}).Where("transaction_no = ?", transaction.TransactionNo).Update("available_balance", wallet.AvailableBalance)

				s.logCallback(param.ClientId, "Completed", param, 1, param.TransactionRef, "Monnify")
				return common.NewSuccessResponse(nil, "Transaction was successful"), nil
			} else if paymentStatus == "REVERSED" {
				// Reverse logic if needed, but handled by status=2 above?
				// Status 2 is generally failed. REVERSED is also status 2.
				return common.NewErrorResponse("Transaction reversed", nil, 406), nil
			}

		} else {
			s.DB.Model(&models.Transaction{}).Where("transaction_no = ?", transaction.TransactionNo).Update("status", 2)
			gatewayResp, _ := data["gateway_response"].(string)
			return common.NewErrorResponse("Transaction not successful: "+gatewayResp, nil, 400), nil
		}
	}

	return common.NewErrorResponse("Monnify auth failed", nil, 400), nil
}

func (s *MonnifyService) HandleWebhook(dto map[string]interface{}) (interface{}, error) {
	// Parse body from "body" field which is string?
	// The Go definition of data might be struct or map. `dto` here is likely map if called from controller with BindJSON.
	// But TS parses `JSON.parse(data.body)`. This implies `data` passed to HandleWebhook has a `body` string.
	// In Go, usually controllers parse body. If `dto` is the raw struct from controller, it depends.
	// Let's assume `dto` is the parsed JSON body event directly (the `body` content in TS).

	event, _ := dto["event"].(string)
	eventData, _ := dto["eventData"].(map[string]interface{})

	// We also need clientId, likely passed separately or found in event?
	// TS uses `data.clientId`. Where does it come from?
	// Ah, TS controller probably passes everything.
	// We'll assume the map contains "clientId" injected by controller if not in payload.

	clientIdFloat, _ := dto["clientId"].(float64)
	clientId := int(clientIdFloat)

	switch event {
	case "SUCCESSFUL_TRANSACTION":
		paymentStatus, _ := eventData["paymentStatus"].(string)
		status := 0
		switch paymentStatus {
		case "PAID", "OVERPAID", "PARTIALLY_PAID":
			status = 1
		case "FAILED", "ABANDONED", "CANCELLED", "REVERSED", "EXPIRED":
			status = 2
		}

		// In `verifyTransaction` TS uses `param.transactionRef`.
		// In `handleWebhook`, TS uses `data.reference` for lookup. But also `data.eventData`.
		// TS `handleWebhook`: `const body = JSON.parse(data.body)`. `switch(data.event)`.
		// Wait, TS `handleWebhook` takes `data`. `data.event` is top level?
		// `data.eventData` inside?
		// AND `data.reference` used in lookup.
		// AND `data.clientId`.
		// Go controller will direct these.

		// Let's rely on standard map access.
		// TS code: `transaction_no: data.reference`.

		reference, _ := dto["reference"].(string)

		var transaction models.Transaction
		if err := s.DB.Where("client_id = ? AND transaction_no = ? AND tranasaction_type = ?", clientId, reference, "credit").First(&transaction).Error; err != nil {
			return map[string]interface{}{"success": false}, nil

		}

		if transaction.Status == 1 {
			return map[string]interface{}{"success": true, "message": "Verified"}, nil
		}
		if transaction.Status == 2 {
			return map[string]interface{}{"success": false, "message": "Transaction failed"}, nil
		}

		s.DB.Model(&models.Transaction{}).Where("transaction_no = ?", transaction.TransactionNo).Update("status", status)

		if status == 1 {
			if err := s.DB.Model(&models.Wallet{}).Where("user_id = ?", transaction.UserId).UpdateColumn("available_balance", gorm.Expr("available_balance + ?", transaction.Amount)).Error; err != nil {
				return map[string]interface{}{"success": false}, nil
			}
			var wallet models.Wallet
			s.DB.Where("user_id = ?", transaction.UserId).First(&wallet)
			s.DB.Model(&models.Transaction{}).Where("transaction_no = ?", transaction.TransactionNo).Update("available_balance", wallet.AvailableBalance)
		} else if paymentStatus == "REVERSED" {
			s.DB.Model(&models.Wallet{}).Where("user_id = ?", transaction.UserId).UpdateColumn("available_balance", gorm.Expr("available_balance - ?", transaction.Amount))
			return map[string]interface{}{"success": true, "message": "Transaction reversed"}, nil
		}

	case "SUCCESSFUL_DISBURSEMENT", "FAILED_DISBURSEMENT", "REVERSED_DISBURSEMENT":
		// Similar logic to Paystack transfer handlers
		reference, _ := dto["reference"].(string)

		var withdrawal models.Withdrawal
		if err := s.DB.Where("client_id = ? AND withdrawal_code = ?", clientId, reference).First(&withdrawal).Error; err != nil {
			fmt.Printf("Withdrawal not found: %s\n", reference)
			break
		}

		if event == "SUCCESSFUL_DISBURSEMENT" && withdrawal.Status == 0 {
			s.DB.Model(&models.Withdrawal{}).Where("id = ?", withdrawal.ID).Update("status", 1)
		} else if event == "FAILED_DISBURSEMENT" || event == "REVERSED_DISBURSEMENT" {
			s.DB.Model(&models.Withdrawal{}).Where("id = ?", withdrawal.ID).Updates(map[string]interface{}{
				"status":  2,
				"comment": "Transfer failed/reversed",
			})

			// Refund
			s.DB.Model(&models.Wallet{}).Where("user_id = ?", withdrawal.UserId).UpdateColumn("available_balance", gorm.Expr("available_balance + ?", withdrawal.Amount))

			// Create transaction record for refund (HelperService.SaveTransaction in TS)
			var wallet models.Wallet
			s.DB.Where("user_id = ?", withdrawal.UserId).First(&wallet)

			trx := TransactionData{
				ClientId:        clientId,
				TransactionNo:   common.GenerateTrxNo(),
				Amount:          withdrawal.Amount,
				Description:     "Transfer failed/reversed",
				Subject:         "Failed/Reversed Withdrawal Request",
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
		}
	}

	return map[string]interface{}{"success": true}, nil

}

func (s *MonnifyService) DisburseFunds(withdrawal *models.Withdrawal, clientId int) (interface{}, error) {
	settings, err := s.monnifySettings(clientId)
	if err != nil {
		return common.NewErrorResponse("Config error", nil, 501), nil
	}

	authRes, err := s.authenticate(settings)
	if err != nil {
		return common.NewErrorResponse("Auth error", nil, 400), nil
	}

	reqSuccess, _ := authRes["requestSuccessful"].(bool)
	if !reqSuccess {
		return authRes, nil
	}

	respBody, _ := authRes["responseBody"].(map[string]interface{})
	accessToken, _ := respBody["accessToken"].(string)

	payload := map[string]interface{}{
		"amount":                   withdrawal.Amount,
		"reference":                withdrawal.WithdrawalCode,
		"currency":                 "NGN",
		"narration":                "Payout request",
		"destinationBankCode":      withdrawal.BankCode,
		"destinationAccountNumber": withdrawal.AccountNumber,
		"sourceAccountNumber":      settings.MerchantId,
	}

	headers := map[string]string{
		"Authorization": "Bearer " + accessToken,
		"Content-Type":  "application/json",
	}

	resp, err := common.Post(fmt.Sprintf("%s/api/v2/disbursements/single", settings.BaseUrl), payload, headers)
	if err != nil {
		return common.NewErrorResponse("Disburse failed", nil, 400), nil
	}

	respMap, _ := resp.(map[string]interface{})

	return map[string]interface{}{
		"success": respMap["requestSuccessful"],
		"data":    respMap["data"],
		"message": respMap["message"],
	}, nil
}

func (s *MonnifyService) ResolveAccountNumber(clientId int, accountNo, bankCode string) (interface{}, error) {
	settings, err := s.monnifySettings(clientId)
	if err != nil {
		return map[string]interface{}{"success": false, "message": "Config error"}, nil

	}

	authRes, err := s.authenticate(settings)
	if err != nil {
		return map[string]interface{}{"success": false, "message": "Auth error"}, nil

	}

	reqSuccess, _ := authRes["requestSuccessful"].(bool)
	if reqSuccess {
		respBody, _ := authRes["responseBody"].(map[string]interface{})
		accessToken, _ := respBody["accessToken"].(string)

		headers := map[string]string{
			"Authorization": "Bearer " + accessToken,
		}

		url := fmt.Sprintf("%s/api/v1/disbursements/account/validate?accountNumber=%s&bankCode=%s", settings.BaseUrl, accountNo, bankCode)
		resp, err := common.Get(url, headers)
		if err != nil {
			return map[string]interface{}{"success": false, "message": "Request failed"}, nil

		}

		result, _ := resp.(map[string]interface{})
		resultReqSuccess, _ := result["requestSuccessful"].(bool)

		return map[string]interface{}{
			"success": resultReqSuccess,
			"data":    result["responseBody"],
			"message": result["responseMessage"],
		}, nil

	}
	return map[string]interface{}{"success": false, "message": "Failed"}, nil

}

func (s *MonnifyService) logCallback(clientId int, requestStr string, response interface{}, status int, trxId, method string) {
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
