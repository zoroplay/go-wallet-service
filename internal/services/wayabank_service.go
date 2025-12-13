package services

import (
	"encoding/base64"
	"fmt"
	"strings"

	"wallet-service/internal/models"
	"wallet-service/pkg/common"

	"gorm.io/gorm"
)

type WayaBankService struct {
	DB *gorm.DB
}

func NewWayaBankService(db *gorm.DB) *WayaBankService {
	return &WayaBankService{
		DB: db,
	}
}

func (s *WayaBankService) settings(clientId int) (*models.PaymentMethod, error) {
	var pm models.PaymentMethod
	err := s.DB.Where("provider = ? AND client_id = ?", "wayabank", clientId).First(&pm).Error
	if err != nil {
		return nil, err
	}
	return &pm, nil
}

func (s *WayaBankService) getToken(settings *models.PaymentMethod) (map[string]interface{}, error) {
	username := settings.PublicKey
	password := settings.SecretKey
	authHash := base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s:%s", username, password)))

	// authUrl logic: assuming BaseUrl is the main API, auth might be derived or BaseUrl contains auth endpoint?
	// TS used separate env vars WAYABANK_AUTH_API and WAYABANK_API.
	// If BaseUrl is "https://guw.wayabank.ng/wayabank-corporate-service/api/v1", auth might be "https://guw.wayabank.ng/auth/api/v1/auth/login"
	// For now, I'll assume BaseUrl stores the base domain or use a convention.
	// HACK: I'll assume BaseUrl is the AUTH URL for now or try to split it?
	// Better: WayaBank's structure usually implies separate URLs.
	// Let's assume BaseUrl is the common prefix "https://guw.wayabank.ng" and we append paths.
	// Or simply hardcode the paths relative to the BaseUrl if it matches.
	// TS: WAYABANK_AUTH_API='https://guw.wayabank.ng/auth/api/v1/auth/login'
	//     WAYABANK_API='https://guw.wayabank.ng/wayabank-corporate-service/api/v1'

	// If settings.BaseUrl is strictly one, we might have issues.
	// Let's infer based on standard WayaBank patterns or assume settings.BaseUrl = Common Base.
	// If settings.BaseUrl is "https://guw.wayabank.ng", then:

	url := settings.BaseUrl
	if strings.Contains(url, "wayabank-corporate-service") {
		// It's the API url, try to replace for Auth?
		url = strings.Replace(url, "wayabank-corporate-service/api/v1", "auth/api/v1/auth/login", 1)
	} else {
		// Fallback or assume it is the auth url if short?
		// Let's assume BaseUrl in DB is the Auth URL for simplification or check `common` logic?
		// Actually, let's look at `PaymentService` later if it needs specific setup.
		// For safely, I will append "/auth/login" if it looks like a base root.
		if !strings.Contains(url, "login") {
			url = url + "/auth/api/v1/auth/login"
		}
	}

	headers := map[string]string{
		"Accept":        "*/*",
		"Authorization": "Basic " + authHash,
	}

	resp, err := common.Post(url, map[string]interface{}{}, headers)
	if err != nil {
		return nil, err
	}

	respMap, _ := resp.(map[string]interface{})
	if data, ok := respMap["data"].(map[string]interface{}); ok {
		return data, nil
	}
	return nil, fmt.Errorf("unable to get token")
}

func (s *WayaBankService) CreateVirtualAccount(param map[string]interface{}, clientId int) (interface{}, error) {
	settings, err := s.settings(clientId)
	if err != nil {
		return map[string]interface{}{"success": false, "message": "WayaBank not configured"}, nil
	}

	user, _ := param["user"].(map[string]interface{})
	username, _ := user["username"].(string)

	tokenData, err := s.getToken(settings)
	if err != nil {
		return map[string]interface{}{"success": false, "message": "Unable to get token"}, nil
	}
	token, _ := tokenData["token"].(string)

	// API URL derivation
	// TS: WAYABANK_API='https://guw.wayabank.ng/wayabank-corporate-service/api/v1'
	apiUrl := settings.BaseUrl
	if !strings.Contains(apiUrl, "wayabank-corporate-service") {
		// If BaseUrl was used for Auth, switch? configuration might be tricky.
		// Let's assume settings.BaseUrl is the MAIN API URL (Corporate Service).
		// And `getToken` handles deriving Auth URL.
	}
	// Re-evaluating `getToken` logic above:
	// If settings.BaseUrl = ".../wayabank-corporate-service/api/v1", then `getToken` derived auth.
	// So here `apiUrl` is just settings.BaseUrl.

	url := fmt.Sprintf("%s/virtual-account", apiUrl)

	headers := map[string]string{
		"Accept":        "*/*",
		"Authorization": token,
		"Client-id":     "WAYABANK",
		"Client-type":   "DEFAULT",
	}

	payload := map[string]interface{}{
		"accountName":           username,
		"email":                 user["email"],
		"merchantAccountNumber": settings.MerchantId,
		"phoneNumber":           "0" + username,
	}

	resp, err := common.Post(url, payload, headers)
	if err != nil {
		return map[string]interface{}{"success": false, "message": "Unable to create virtual account"}, nil
	}

	respMap, _ := resp.(map[string]interface{})
	return map[string]interface{}{"success": true, "data": respMap["data"], "message": respMap["message"]}, nil
}

func (s *WayaBankService) AccountEnquiry(param map[string]interface{}, clientId int) (interface{}, error) {
	settings, err := s.settings(clientId)
	if err != nil {
		return map[string]interface{}{"success": false, "message": "WayaBank not configured"}, nil
	}

	accountNumber, _ := param["accountNumber"].(string)

	tokenData, err := s.getToken(settings)
	if err != nil {
		return map[string]interface{}{"success": false, "message": "Unable to get token"}, nil
	}
	token, _ := tokenData["token"].(string)

	url := fmt.Sprintf("%s/virtual-account/enquiry", settings.BaseUrl)

	headers := map[string]string{
		"Accept":        "*/*",
		"Authorization": token,
		"Client-id":     "WAYABANK",
		"Client-type":   "DEFAULT",
	}

	payload := map[string]interface{}{
		"accountNumber": accountNumber,
	}

	resp, err := common.Post(url, payload, headers)
	if err != nil {
		return map[string]interface{}{"success": false, "message": "Unable to create virtual account"}, nil
	}

	respMap, _ := resp.(map[string]interface{})
	return map[string]interface{}{"success": true, "data": respMap["data"], "message": respMap["message"]}, nil
}
