package services

import (
	"encoding/json"
	"fmt"

	"wallet-service/internal/models"
	"wallet-service/pkg/common"

	"gorm.io/gorm"
)

type Pitch90SMSService struct {
	DB            *gorm.DB
	HelperService *HelperService
}

func NewPitch90SMSService(db *gorm.DB, helper *HelperService) *Pitch90SMSService {
	return &Pitch90SMSService{
		DB:            db,
		HelperService: helper,
	}
}

func (s *Pitch90SMSService) getSettings(clientId int) (*models.PaymentMethod, error) {
	var pm models.PaymentMethod
	err := s.DB.Where("provider = ? AND client_id = ?", "stkpush", clientId).First(&pm).Error
	if err != nil {
		return nil, err
	}
	return &pm, nil
}

func (s *Pitch90SMSService) Deposit(param map[string]interface{}) (interface{}, error) {
	clientIdFloat, _ := param["clientId"].(float64)
	clientId := int(clientIdFloat)
	user, _ := param["user"].(map[string]interface{})
	username := user["username"].(string)
	amount := param["amount"]

	settings, err := s.getSettings(clientId)
	if err != nil {
		return nil, err
	}

	payload := map[string]interface{}{
		"amount":   fmt.Sprintf("%v", amount),
		"salt":     settings.SecretKey,
		"username": username,
		"msisdn":   "254" + username,
		"account":  "254" + username,
	}

	url := fmt.Sprintf("%s/wallet/stkpush", settings.BaseUrl)
	resp, err := common.Post(url, payload, nil)
	if err != nil {
		return map[string]interface{}{"success": false, "message": err.Error()}, nil // Simplified
	}

	respMap, _ := resp.(map[string]interface{})
	if status, ok := respMap["status"].(string); ok && status == "Fail" {
		return map[string]interface{}{"success": false, "message": respMap["error_desc"]}, nil
	}

	return map[string]interface{}{"success": true, "data": respMap, "message": respMap["message"]}, nil
}

func (s *Pitch90SMSService) StkDepositNotification(data map[string]interface{}) (interface{}, error) {
	clientIdFloat, _ := data["clientId"].(float64)
	clientId := int(clientIdFloat)
	msisdn, _ := data["msisdn"].(string)
	refId, _ := data["refId"].(string)
	amountFloat, _ := data["amount"].(float64)

	username := ""
	if len(msisdn) > 3 {
		username = msisdn[3:]
	}

	// Save callback log
	logData, _ := json.Marshal(data)
	log := models.CallbackLog{
		ClientId:      clientId,
		Request:       string(logData),
		TransactionId: refId,
		RequestType:   "Deposit Notification",
		PaymentMethod: "Stkpush",
		Status:        1,
	}
	s.DB.Create(&log)

	var wallet models.Wallet
	if err := s.DB.Where("username = ?", username).First(&wallet).Error; err != nil {
		// Log response
		s.updateCallbackResponse(int(log.ID), map[string]interface{}{"success": false, "data": map[string]string{"refId": refId, "message": "wallet/user not found"}})
		return map[string]interface{}{"success": false, "data": map[string]string{"refId": refId, "message": "wallet not found"}}, nil
	}

	var transaction models.Transaction
	if err := s.DB.Where("transaction_no = ? AND tranasaction_type = ? AND status = ?", refId, "credit", 0).First(&transaction).Error; err != nil {
		s.updateCallbackResponse(int(log.ID), map[string]interface{}{"success": false, "data": map[string]string{"refId": refId, "message": "transaction not found"}})
		return map[string]interface{}{"success": false, "data": map[string]string{"refId": refId, "message": "transaction not found"}}, nil
	}

	if transaction.Status == 1 {
		s.updateCallbackResponse(int(log.ID), map[string]interface{}{"success": false, "data": map[string]string{"refId": refId, "message": "transaction already processed"}})
		return map[string]interface{}{"success": false, "data": map[string]string{"refId": refId, "message": "transaction already processed"}}, nil
	}

	balance := wallet.AvailableBalance + amountFloat
	s.HelperService.UpdateWallet(balance, wallet.UserId)

	s.DB.Model(&models.Transaction{}).Where("transaction_no = ?", refId).Updates(map[string]interface{}{
		"status":  1,
		"balance": balance,
	})

	response := map[string]interface{}{"success": true, "data": map[string]string{"refId": refId}}
	s.updateCallbackResponse(int(log.ID), response)
	return response, nil
}

func (s *Pitch90SMSService) Withdraw(withdrawal *models.Withdrawal, clientId int) (interface{}, error) {
	settings, err := s.getSettings(clientId)
	if err != nil {
		return nil, err
	}

	payload := map[string]interface{}{
		"msisdn":   "254" + withdrawal.Username,
		"amount":   fmt.Sprintf("%v", withdrawal.Amount),
		"account":  withdrawal.Username,
		"salt":     settings.SecretKey,
		"username": withdrawal.Username,
	}

	url := fmt.Sprintf("%s/wallet/withdrawal", settings.BaseUrl)
	resp, err := common.Post(url, payload, nil)
	if err != nil {
		return map[string]interface{}{"success": false, "message": err.Error()}, nil
	}

	data, _ := resp.(map[string]interface{})
	if status, ok := data["status"].(string); ok && status == "Fail" {
		return map[string]interface{}{"success": false, "message": data["error_desc"]}, nil
	}

	refId, _ := data["ref_id"].(string)
	s.DB.Model(withdrawal).Update("withdrawal_code", refId)

	return map[string]interface{}{
		"success": true,
		"data":    data,
		"message": "Withdrawal processed.",
	}, nil
}

func (s *Pitch90SMSService) StkWithdrawalNotification(data map[string]interface{}) (interface{}, error) {
	clientIdFloat, _ := data["clientId"].(float64)
	clientId := int(clientIdFloat)
	refId, _ := data["refId"].(string)
	if refId == "" {
		refId, _ = data["ref_id"].(string)
	}

	logData, _ := json.Marshal(data)
	log := models.CallbackLog{
		ClientId:      clientId,
		Request:       string(logData),
		TransactionId: refId,
		RequestType:   "Withdrawal Notification",
		PaymentMethod: "Stkpush",
		Status:        1,
	}
	s.DB.Create(&log)

	err := s.DB.Model(&models.Withdrawal{}).Where("withdrawal_code = ?", refId).Update("status", 1).Error
	if err != nil {
		s.updateCallbackResponse(int(log.ID), map[string]interface{}{"success": false, "data": map[string]string{"refId": refId, "message": err.Error()}})
		return map[string]interface{}{"success": true, "data": map[string]string{"refId": refId, "message": err.Error()}}, nil // TS returns success: true even on error catch? "return response"
	}

	response := map[string]interface{}{"success": true, "data": map[string]string{"refId": refId}}
	s.updateCallbackResponse(int(log.ID), response)
	return response, nil
}

func (s *Pitch90SMSService) RegisterUrl(param map[string]interface{}) (interface{}, error) {
	action, _ := param["action"].(string)
	urlParam, _ := param["url"].(string)
	clientIdFloat, _ := param["clientId"].(float64)
	clientId := int(clientIdFloat)

	settings, err := s.getSettings(clientId)
	if err != nil {
		return map[string]interface{}{"success": false, "message": err.Error()}, nil
	}

	var endpoint string
	switch action {
	case "payment":
		endpoint = "/wallet/registerIpnUrl"
	case "withdrawal":
		endpoint = "/wallet/registerWithdrawalUrl"
	case "stkstatus":
		endpoint = "/wallet/registerStkStatusUrl"
	default:
		return map[string]interface{}{"success": false, "message": "WRONG ACTION!!!"}, nil
	}

	fullUrl := fmt.Sprintf("%s%s", settings.BaseUrl, endpoint)
	payload := map[string]interface{}{
		"url":  urlParam,
		"salt": settings.SecretKey,
	}

	resp, err := common.Post(fullUrl, payload, nil)
	if err != nil {
		return map[string]interface{}{"success": false, "message": err.Error()}, nil
	}

	data, _ := resp.(map[string]interface{})
	if status, ok := data["status"].(string); ok && status == "Fail" {
		return map[string]interface{}{"success": false, "message": data["error_desc"]}, nil
	}

	return map[string]interface{}{
		"success": true,
		"data":    data,
		"message": data["status"],
	}, nil
}

func (s *Pitch90SMSService) updateCallbackResponse(id int, response interface{}) {
	respBytes, _ := json.Marshal(response)
	s.DB.Model(&models.CallbackLog{}).Where("id = ?", id).Update("response", string(respBytes))
}
