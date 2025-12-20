package services

import (
	"fmt"
	"log"
	"time"

	"wallet-service/internal/models"
	"wallet-service/pkg/common"

	"gorm.io/gorm"
)

type HelperService struct {
	DB *gorm.DB
}

func NewHelperService(db *gorm.DB) *HelperService {
	return &HelperService{DB: db}
}

type TransactionData struct {
	ClientId               int
	TransactionNo          string
	Amount                 float64
	Subject                string
	Description            string
	Source                 string
	Channel                string
	FromUserId             int
	FromUsername           string
	FromUserBalance        float64
	ToUserId               int
	ToUsername             string
	ToUserBalance          float64
	Status                 int
	AffiliateId            int
	AffiliateTransactionNo int
	WalletType             string
}

func (s *HelperService) SaveTransaction(data TransactionData) error {
	var transactions []models.Transaction

	// Debit leg
	t1 := models.Transaction{
		ClientId:               data.ClientId,
		UserId:                 data.FromUserId,
		Username:               data.FromUsername,
		TransactionNo:          data.TransactionNo,
		Amount:                 data.Amount,
		TrxType:                "debit",
		Subject:                data.Subject,
		Description:            data.Description,
		Source:                 data.Source,
		Channel:                data.Channel,
		AvailableBalance:       data.FromUserBalance,
		Status:                 data.Status,
		AffiliateId:            data.AffiliateId,
		AffiliateTransactionNo: &data.AffiliateTransactionNo,
		Wallet:                 data.WalletType,
	}
	transactions = append(transactions, t1)

	// Credit leg
	t2 := models.Transaction{
		ClientId:               data.ClientId,
		UserId:                 data.ToUserId,
		Username:               data.ToUsername,
		TransactionNo:          data.TransactionNo,
		Amount:                 data.Amount,
		TrxType:                "credit",
		Subject:                data.Subject,
		Description:            data.Description,
		Source:                 data.Source,
		Channel:                data.Channel,
		AvailableBalance:       data.ToUserBalance,
		Status:                 data.Status,
		AffiliateId:            data.AffiliateId,
		AffiliateTransactionNo: &data.AffiliateTransactionNo,
		Wallet:                 data.WalletType,
	}
	transactions = append(transactions, t2)

	return s.DB.Transaction(func(tx *gorm.DB) error {
		for _, t := range transactions {
			if err := tx.Create(&t).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// Trackier integration
const trackierUrl = "https://api.trackierigaming.io"

func (s *HelperService) SendActivity(data map[string]interface{}, keys map[string]string) {
	// keys should contain ApiKey and AuthCode/AccessToken logic
	// For now implementing the core mapping logic

	payload := map[string]interface{}{
		"bets":          0,
		"timestamp":     time.Now().Unix(),
		"fees":          0,
		"wins":          0,
		"bonuses":       0,
		"currency":      "ngn",
		"deposits":      0,
		"productId":     "1",
		"customerId":    data["username"],
		"withdrawls":    0,
		"adjustments":   0,
		"chargebacks":   0,
		"transactionId": data["transactionId"],
	}

	subject, _ := data["subject"].(string)
	amount, _ := data["amount"].(float64)

	switch subject {
	case "Deposit":
		payload["deposits"] = amount
		payload["productId"] = "1"
	case "Withdrawal Request":
		payload["withdrawls"] = amount
	case "Sport Win":
		payload["wins"] = amount
	case "Bet Deposit (Sport)":
		payload["bets"] = amount
	}

	apiKey := keys["ApiKey"]
	authCode := keys["AuthCode"]

	if apiKey != "" {
		// Verify auth token
		authRes, err := s.getAccessToken(authCode)
		if err != nil || authRes["success"] != true {
			log.Println("Unable to get trackier auth token")
			return
		}

		tokenData, _ := authRes["data"].(map[string]interface{})
		accessToken, _ := tokenData["accessToken"].(string)

		// Check customer
		customerRes, err := s.getTrackierCustomer(apiKey, accessToken, payload["customerId"].(string))
		if err == nil && customerRes["success"] == true && customerRes["data"] != nil {
			// Send activity
			url := fmt.Sprintf("%s/api/admin/v2/activities", trackierUrl)
			headers := map[string]string{
				"x-api-key":     apiKey,
				"authorization": fmt.Sprintf("BEARER %s", accessToken),
			}

			_, err := common.Post(url, payload, headers)
			if err != nil {
				log.Printf("Trackier error: %v", err)
			} else {
				log.Println("Trackier activity sent successfully")
			}
		}
	}
}

func (s *HelperService) getAccessToken(authCode string) (map[string]interface{}, error) {
	url := fmt.Sprintf("%s/api/public/v2/oauth/access-refresh-token", trackierUrl)
	payload := map[string]string{"auth_code": authCode}

	resp, err := common.Post(url, payload, nil)
	if err != nil {
		return nil, err
	}

	respMap, ok := resp.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid response format")
	}

	// Check inner data structure based on TS code: res.data.data
	// common.Post returns res.data? Wait, common.Post returns res.data
	// TS code: res.data.data.
	// My common.Post: returns res.data.
	// So if response is {data: {accessToken: ...}}, common.Post returns {accessToken: ...}

	return map[string]interface{}{"success": true, "data": respMap}, nil
}

func (s *HelperService) getTrackierCustomer(apiKey, token, customerId string) (map[string]interface{}, error) {
	url := fmt.Sprintf("%s/api/admin/v2/customers/%s", trackierUrl, customerId)
	headers := map[string]string{
		"x-api-key":     apiKey,
		"authorization": fmt.Sprintf("BEARER %s", token),
	}

	resp, err := common.Get(url, headers)
	if err != nil {
		return nil, err
	}

	respMap, ok := resp.(map[string]interface{})
	if !ok {
		// If error or empty
		return map[string]interface{}{"success": false, "data": nil}, nil
	}

	return map[string]interface{}{"success": true, "data": respMap}, nil
}

func (s *HelperService) UpdateWallet(balance float64, userId int) error {
	return s.DB.Model(&models.Wallet{}).Where("user_id = ?", userId).Update("available_balance", balance).Error
}
