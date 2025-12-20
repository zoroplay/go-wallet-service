package services

import (
	"fmt"
	"log"

	"wallet-service/internal/models"
	"wallet-service/pkg/common"

	"gorm.io/gorm"
)

type WithdrawalService struct {
	DB       *gorm.DB
	Identity *IdentityClient
}

func NewWithdrawalService(db *gorm.DB, identity *IdentityClient) *WithdrawalService {
	return &WithdrawalService{DB: db, Identity: identity}
}

type WithdrawRequestDTO struct {
	UserId        int
	ClientId      int
	Amount        float64
	Type          string // "bank" or "shop"? TS logic infers "autoDisbursement"
	AccountNumber string
	AccountName   string
	BankCode      string
	BankName      string
}

// Stub for IdentityService.GetWithdrawalSettings
// WithdrawalSettings from IdentityService
type WithdrawalSettings struct {
	AutoDisbursement      int     `json:"autoDisbursement"`
	AutoDisbursementMin   float64 `json:"autoDisbursementMin"`
	AutoDisbursementMax   float64 `json:"autoDisbursementMax"`
	AutoDisbursementCount int     `json:"autoDisbursementCount"`
	MinimumWithdrawal     float64 `json:"minimumWithdrawal"`
	MaximumWithdrawal     float64 `json:"maximumWithdrawal"`
	AllowWithdrawalComm   int     `json:"allowWithdrawalComm"`
	WithdrawalComm        float64 `json:"withdrawalComm"`
}

func (s *WithdrawalService) getWithdrawalSettings(clientId, userId int) WithdrawalSettings {
	resp, err := s.Identity.GetWithdrawalSettings(clientId, userId)
	if err != nil {
		log.Printf("Failed to fetch withdrawal settings from identity service: %v", err)
		// Fallback to default settings
		return WithdrawalSettings{
			MinimumWithdrawal: 100.0,
			MaximumWithdrawal: 1000000.0,
		}
	}

	return WithdrawalSettings{
		AutoDisbursement:      int(resp.AutoDisbursement),
		AutoDisbursementMin:   float64(resp.AutoDisbursementMin),
		AutoDisbursementMax:   float64(resp.AutoDisbursementMax),
		AutoDisbursementCount: int(resp.AutoDisbursementCount),
		MinimumWithdrawal:     float64(resp.MinimumWithdrawal),
		MaximumWithdrawal:     float64(resp.MaximumWithdrawal),
		AllowWithdrawalComm:   int(resp.AllowWithdrawalComm),
		WithdrawalComm:        float64(resp.WithdrawalComm),
	}
}

// Stub for Queue.add
func (s *WithdrawalService) addToQueue(queueName string, data interface{}) {
	log.Printf("[QUEUE STUB] Adding to %s: %+v\n", queueName, data)
}

func (s *WithdrawalService) RequestWithdrawal(data WithdrawRequestDTO) (interface{}, error) {
	var wallet models.Wallet
	if err := s.DB.Where("user_id = ? AND client_id = ?", data.UserId, data.ClientId).First(&wallet).Error; err != nil {
		return common.NewErrorResponse("Wallet not found", nil, 404), nil
	}

	if wallet.AvailableBalance < data.Amount {
		return common.NewErrorResponse("You have insufficient funds to cover the withdrawal request.", nil, 400), nil
	}

	settings := s.getWithdrawalSettings(data.ClientId, data.UserId)
	if data.Amount < settings.MinimumWithdrawal {
		return common.NewErrorResponse(fmt.Sprintf("Minimum withdrawable amount is %f", settings.MinimumWithdrawal), nil, 400), nil
	}
	if data.Amount > settings.MaximumWithdrawal {
		return common.NewErrorResponse(fmt.Sprintf("Maximum withdrawable amount is %f", settings.MaximumWithdrawal), nil, 400), nil
	}

	jobData := map[string]interface{}{
		"userId":           data.UserId,
		"clientId":         data.ClientId,
		"amount":           data.Amount,
		"accountNumber":    data.AccountNumber,
		"accountName":      data.AccountName,
		"bankCode":         data.BankCode,
		"bankName":         data.BankName,
		"withdrawalCode":   common.GenerateTrxNo(),
		"balance":          wallet.AvailableBalance,
		"autoDisbursement": settings,
	}

	s.addToQueue("withdrawal-request", jobData)

	return common.NewSuccessResponse(map[string]interface{}{
		"balance": jobData["balance"],
		"code":    jobData["withdrawalCode"],
	}, "Successful"), nil
}

type FetchUsersWithdrawalDTO struct {
	UserId   int
	ClientId int
	Pending  bool
}

func (s *WithdrawalService) FetchUsersWithdrawal(data FetchUsersWithdrawalDTO) (common.SuccessResponse, error) {
	var withdrawals []models.Withdrawal
	query := s.DB.Where("user_id = ? AND client_id = ?", data.UserId, data.ClientId)

	if data.Pending {
		query = query.Where("status = ?", 0)
	}

	if err := query.Find(&withdrawals).Error; err != nil {
		return common.SuccessResponse{}, err
	}

	// Map response if needed, but returning struct is fine.
	return common.NewSuccessResponse(withdrawals, "Successful"), nil
}

type ListWithdrawalRequestsDTO struct {
	ClientId int
	From     string
	To       string
	Status   *int
	UserId   int
	Username string
	BankName string
	Page     int
	Limit    int
}

func (s *WithdrawalService) ListWithdrawalRequest(data ListWithdrawalRequestsDTO) (common.PaginationResult, error) {
	limit := data.Limit
	if limit <= 0 {
		limit = 50
	}
	page := data.Page
	if page < 1 {
		page = 1
	}
	offset := (page - 1) * limit

	query := s.DB.Model(&models.Withdrawal{}).Where("client_id = ?", data.ClientId)

	if data.From != "" && data.To != "" {
		// Expecting dates in YYYY-MM-DD format ideally or handle parsing
		query = query.Where("created_at BETWEEN ? AND ?", data.From, data.To)
	}

	if data.UserId != 0 {
		query = query.Where("user_id = ?", data.UserId)
	}
	if data.Username != "" {
		query = query.Where("username LIKE ?", "%"+data.Username+"%")
	}
	if data.BankName != "" {
		query = query.Where("bank_name LIKE ?", "%"+data.BankName+"%")
	}
	// Status was commented out in TS, but maybe useful?
	if data.Status != nil {
		query = query.Where("status = ?", *data.Status)
	}

	var total int64
	query.Count(&total)

	var list []models.Withdrawal
	query.Order("created_at DESC").Limit(limit).Offset(offset).Find(&list)

	// Aggregation total amount (filtered)
	var totalAmount float64
	// Reuse conditions for sum
	// Note: Cloned query might be easier but GORM query chain modifies state if not careful.
	// Re-building query for sum or using session.
	sumQuery := s.DB.Model(&models.Withdrawal{}).Where("client_id = ?", data.ClientId)
	if data.From != "" && data.To != "" {
		sumQuery = sumQuery.Where("created_at BETWEEN ? AND ?", data.From, data.To)
	}
	// ... add other filters if they apply to aggregation (TS code adds date filter to main sum,
	// but keeps other filters? TS Sum query lines 307-317 only checks clientId and date range.
	// It does NOT apply userId/username filters to the total amount. Standard Go-Server behavior should probably match TS).

	sumQuery.Select("COALESCE(SUM(amount), 0)").Scan(&totalAmount)

	return common.PaginateResponse(map[string]interface{}{
		"data":        list,
		"totalAmount": totalAmount,
	}, total, page, limit, "Withdrawal requests fetched successfully"), nil
}

type GetUserAccountsDTO struct {
	UserId int
}

func (s *WithdrawalService) GetUserBankAccounts(data GetUserAccountsDTO) (common.SuccessResponse, error) {
	// var accounts []models.WithdrawalAccount // Unused removed
	// TS does a join with Bank to potentially get bank name if valid code?
	// TS: .leftJoinAndSelect(Bank, 'bank', 'bank.code = account.bank_code')
	// If we want bank name populated we might need a custom struct result.

	type AccountResult struct {
		models.WithdrawalAccount
		BankName string `json:"bankName"`
	}

	var results []AccountResult

	// GORM join
	err := s.DB.Table("withdrawal_accounts").
		Select("withdrawal_accounts.*, banks.name as bank_name").
		Joins("LEFT JOIN banks ON banks.code = withdrawal_accounts.bank_code").
		Where("withdrawal_accounts.user_id = ?", data.UserId).
		Scan(&results).Error

	if err != nil {
		return common.SuccessResponse{Data: []interface{}{}}, nil // Return empty on error per TS
	}

	return common.NewSuccessResponse(results, "Successful"), nil
}

type ValidateWithdrawalCodeDTO struct {
	Code     string
	ClientId int
}

func (s *WithdrawalService) ValidateWithdrawalCode(data ValidateWithdrawalCodeDTO) (interface{}, error) {
	var request models.Withdrawal
	if err := s.DB.Where("withdrawal_code = ? AND client_id = ?", data.Code, data.ClientId).First(&request).Error; err != nil {
		return common.NewErrorResponse("Invalid code provided", nil, 404), nil
	}

	if request.Status != 0 {
		return common.NewErrorResponse("Code has already been used", nil, 404), nil
	}

	respData := map[string]interface{}{
		"requestId":             request.ID,
		"amount":                request.Amount,
		"withdrawalCharge":      0,
		"withdrawalFinalAmount": 0,
		"username":              request.Username,
	}

	return common.NewSuccessResponse(respData, "Valid"), nil
}

type ProcessShopWithdrawalDTO struct {
	Id       int
	UserId   int
	ClientId int
}

func (s *WithdrawalService) ProcessShopWithdrawal(data ProcessShopWithdrawalDTO) (interface{}, error) {
	var request models.Withdrawal
	if err := s.DB.Where("id = ? AND status = 0", data.Id).First(&request).Error; err != nil {
		return common.NewErrorResponse("Withdrawal request already processed", nil, 400), nil
	}

	if request.UserId == data.UserId {
		return common.NewErrorResponse("You cannot process your own request", nil, 400), nil
	}

	var wallet models.Wallet
	if err := s.DB.Where("user_id = ? AND client_id = ?", data.UserId, data.ClientId).First(&wallet).Error; err != nil {
		return common.NewErrorResponse("Wallet not found", nil, 404), nil
	}

	payload := map[string]interface{}{
		"id":       data.Id,
		"userId":   data.UserId,
		"clientId": data.ClientId,
		"balance":  wallet.AvailableBalance,
		"amount":   request.Amount,
	}

	s.addToQueue("shop-withdrawal", payload)

	return common.NewSuccessResponse(nil, "Withdrawal request successful"), nil
}
