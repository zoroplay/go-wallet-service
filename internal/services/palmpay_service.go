package services

import (
	"crypto"
	"crypto/md5"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"sort"
	"strings"
	"time"

	"wallet-service/internal/models"
	"wallet-service/pkg/common"

	"gorm.io/gorm"
)

type PalmPayService struct {
	DB            *gorm.DB
	HelperService *HelperService
	// IdentityService omitted, we access User via DB or other means if needed for Payout?
	// The `payoutToUser` needs `user` to get phone number.
	// We might need to inject IdentityService or mock it.
	// For now, I'll access checks via DB or assume phone is in Withdrawal? No, withdrawal has user_id.
	// I will omit IdentityService usage or find workaround if possible.
	// Withdrawal entity usually has necessary details or we can fetch User from DB.
	// Assuming User model exists and can be queried.
}

func NewPalmPayService(db *gorm.DB, helper *HelperService) *PalmPayService {
	return &PalmPayService{
		DB:            db,
		HelperService: helper,
	}
}

func (s *PalmPayService) palmPaySettings(clientId int) (*models.PaymentMethod, error) {
	var pm models.PaymentMethod
	err := s.DB.Where("provider = ? AND client_id = ?", "palmpay", clientId).First(&pm).Error
	if err != nil {
		return nil, err
	}
	return &pm, nil
}

func (s *PalmPayService) generateSignatureRSA(data string, privateKeyPem string) (string, error) {
	block, _ := pem.Decode([]byte(privateKeyPem))
	if block == nil {
		return "", fmt.Errorf("failed to decode PEM block")
	}

	privKey, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		// Try PKCS1
		privKey, err = x509.ParsePKCS1PrivateKey(block.Bytes)
		if err != nil {
			return "", err
		}
	}

	rsaKey, ok := privKey.(*rsa.PrivateKey)
	if !ok {
		return "", fmt.Errorf("not an RSA private key")
	}

	rng := rand.Reader
	hashed := crypto.SHA1.New()
	hashed.Write([]byte(data))
	digest := hashed.Sum(nil)

	signature, err := rsa.SignPKCS1v15(rng, rsaKey, crypto.SHA1, digest)
	if err != nil {
		return "", err
	}

	return base64.StdEncoding.EncodeToString(signature), nil
}

// InitiatePayment
func (s *PalmPayService) InitiatePayment(data map[string]interface{}, clientId int) (interface{}, error) {
	settings, err := s.palmPaySettings(clientId)
	if err != nil {
		return common.SuccessResponse{Success: false, Message: "PalmPay not configured"}, nil
	}

	requestTime := time.Now().UnixMilli()
	nonceBytes := make([]byte, 8) // 16 params in hex string?
	rand.Read(nonceBytes)
	nonceStr := hex.EncodeToString(nonceBytes)

	appId := settings.MerchantId

	// Construct payload
	payload := make(map[string]interface{})
	for k, v := range data {
		payload[k] = v
	}
	payload["requestTime"] = requestTime
	payload["nonceStr"] = nonceStr
	payload["appId"] = appId
	payload["version"] = "1.0"

	// Sign
	keys := make([]string, 0, len(payload))
	for k := range payload {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var sigParts []string
	for _, k := range keys {
		sigParts = append(sigParts, fmt.Sprintf("%s=%v", k, payload[k]))
	}
	signatureString := strings.Join(sigParts, "&")

	hash := md5.Sum([]byte(signatureString))
	parseStr := strings.ToUpper(hex.EncodeToString(hash[:]))

	privKeyPem := strings.ReplaceAll(settings.SecretKey, "\\n", "\n")

	signature, err := s.generateSignatureRSA(parseStr, privKeyPem)
	if err != nil {
		fmt.Printf("PalmPay Sign Error: %v\n", err)
		return common.SuccessResponse{Success: false, Message: "Signing failed"}, nil
	}

	headers := map[string]string{
		"Content-Type":  "application/json",
		"Authorization": "Bearer " + appId,
		"Signature":     signature,
		"CountryCode":   "NG",
	}

	res, err := common.Post(fmt.Sprintf("%s/api/v2/payment/merchant/createorder", settings.BaseUrl), payload, headers)
	if err != nil {
		return common.SuccessResponse{Success: false, Message: "Unable to initiate deposit"}, nil
	}

	resMap, _ := res.(map[string]interface{})
	dataMap, _ := resMap["data"].(map[string]interface{})
	checkoutUrl, _ := dataMap["checkoutUrl"].(string)

	return common.SuccessResponse{Success: true, Data: checkoutUrl}, nil
}

func (s *PalmPayService) HandleWebhook(data map[string]interface{}) (interface{}, error) {
	rawBody, _ := data["rawBody"].(map[string]interface{})
	payload, _ := rawBody["payload"].(map[string]interface{})
	status, _ := payload["status"].(string)

	clientIdFloat, _ := data["clientId"].(float64)
	clientId := int(clientIdFloat)

	if status == "SUCCESS" {
		ref, _ := payload["reference"].(string)

		var transaction models.Transaction
		if err := s.DB.Where("client_id = ? AND transaction_no = ? AND tranasaction_type = ?", clientId, ref, "credit").First(&transaction).Error; err != nil {
			s.logCallback(clientId, "Transaction not found", rawBody, 0, ref, "Opay") // Original code said Opay?
			return common.NewErrorResponse("Transaction not found", nil, 404), nil
		}

		if transaction.Status == 1 {
			s.logCallback(clientId, "Transaction already processed", rawBody, 1, ref, "Opay")
			return map[string]interface{}{
				"success": true,
				"message": "Transaction already successful",
				"status":  200,
				"data":    map[string]interface{}{},
			}, nil
		}

		// Update Wallet
		if err := s.DB.Model(&models.Wallet{}).Where("user_id = ?", transaction.UserId).UpdateColumn("available_balance", gorm.Expr("available_balance + ?", transaction.Amount)).Error; err != nil {
			s.logCallback(clientId, "Wallet not found", rawBody, 0, ref, "Opay")
			return common.NewErrorResponse("Wallet not found", nil, 404), nil
		}

		var wallet models.Wallet
		s.DB.Where("user_id = ?", transaction.UserId).First(&wallet)

		s.DB.Model(&models.Transaction{}).Where("transaction_no = ?", transaction.TransactionNo).Updates(map[string]interface{}{
			"status":  1,
			"balance": wallet.AvailableBalance,
		})

		s.logCallback(clientId, "Completed", rawBody, 1, ref, "Opay")

		return map[string]interface{}{
			"status":  200,
			"success": true,
			"message": "Transaction successfully verified and processed",
			"data":    map[string]interface{}{},
		}, nil
	}

	// Error case from TS
	ref, _ := payload["reference"].(string)
	s.logCallback(clientId, "Failed", rawBody, 0, ref, "Opay")
	return map[string]interface{}{
		"success": false,
		"message": "Error occurred during processing",
		"status":  500,
		"data":    map[string]interface{}{},
	}, nil
}

func (s *PalmPayService) logCallback(clientId int, requestStr string, response interface{}, status int, trxId, method string) {
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
