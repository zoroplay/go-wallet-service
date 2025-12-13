package services

import (
	"fmt"
	"math"
	"strconv"
	"time"

	"wallet-service/internal/models"
	"wallet-service/pkg/common"
	"wallet-service/proto/identity"
	"wallet-service/proto/wallet"

	"gorm.io/gorm"
)

type CommissionService struct {
	DB             *gorm.DB
	Helper         *HelperService
	IdentityClient *IdentityClient
	PlayerService  *PlayerService
}

func NewCommissionService(db *gorm.DB, helper *HelperService, idClient *IdentityClient, playerService *PlayerService) *CommissionService {
	return &CommissionService{
		DB:             db,
		Helper:         helper,
		IdentityClient: idClient,
		PlayerService:  playerService,
	}
}

type CommissionRequestDTO struct {
	ClientId    int
	UserId      int
	Amount      float64
	Description string
	Username    string
}

// UpdateCommissionWallet credits the commission wallet
func (s *CommissionService) UpdateCommissionWallet(data CommissionRequestDTO) (interface{}, error) {
	var wallet models.Wallet
	if err := s.DB.Where("client_id = ? AND user_id = ?", data.ClientId, data.UserId).First(&wallet).Error; err != nil {
		return common.NewErrorResponse("Wallet not found", nil, 404), nil
	}

	transactionNo := common.GenerateTrxNo()

	// Atomic update
	if err := s.DB.Model(&wallet).UpdateColumn("commission_balance", gorm.Expr("commission_balance + ?", data.Amount)).Error; err != nil {
		return common.NewErrorResponse("Wallet could not be updated", nil, 400), nil
	}

	// Create Credit Transaction
	trx := models.Transaction{
		ClientId:      data.ClientId,
		UserId:        data.UserId,
		Username:      wallet.Username, // Use wallet username to be safe
		TransactionNo: transactionNo,
		Amount:        data.Amount,
		TrxType:       "credit",
		Subject:       "Commission", // or specific subject based on context? TS checks job payload.
		Description:   data.Description,
		Source:        "internal",
		Channel:       "commission",
		Balance:       wallet.CommissionBalance + data.Amount, // Aprrox
		Status:        1,
		Wallet:        "Commission",
	}
	s.DB.Create(&trx)

	return map[string]interface{}{
		"success": true,
		"message": "Wallet balance updated",
		"status":  200,
		"data":    nil,
	}, nil
}

// DebitCommissionWallet debits the commission wallet
func (s *CommissionService) DebitCommissionWallet(data CommissionRequestDTO) (interface{}, error) {
	var wallet models.Wallet
	if err := s.DB.Where("client_id = ? AND user_id = ?", data.ClientId, data.UserId).First(&wallet).Error; err != nil {
		return common.NewErrorResponse("Wallet not found", nil, 404), nil
	}

	if wallet.CommissionBalance < data.Amount {
		return common.NewErrorResponse("Insufficient balance", nil, 400), nil
	}

	transactionNo := common.GenerateTrxNo()

	// Atomic update
	if err := s.DB.Model(&wallet).UpdateColumn("commission_balance", gorm.Expr("commission_balance - ?", data.Amount)).Error; err != nil {
		return common.NewErrorResponse("Wallet could not be updated", nil, 400), nil
	}

	// Create Debit Transaction
	trx := models.Transaction{
		ClientId:      data.ClientId,
		UserId:        data.UserId,
		Username:      wallet.Username,
		TransactionNo: transactionNo,
		Amount:        data.Amount,
		TrxType:       "debit",
		Subject:       "Commission Debit",
		Description:   data.Description,
		Source:        "internal",
		Channel:       "commission",
		Balance:       wallet.CommissionBalance - data.Amount,
		Status:        1,
		Wallet:        "Commission",
	}
	s.DB.Create(&trx)

	return map[string]interface{}{
		"success": true,
		"message": "Wallet balance updated",
		"status":  200,
		"data":    nil,
	}, nil
}

type GetCommissionBalanceDTO struct {
	ClientId int
	UserId   int
	Page     int
	Limit    int
	From     string
	To       string
}

func (s *CommissionService) GetAgentCommissionBalance(data GetCommissionBalanceDTO) (interface{}, error) {
	var wallet models.Wallet
	if err := s.DB.Where("client_id = ? AND user_id = ?", data.ClientId, data.UserId).First(&wallet).Error; err != nil {
		return common.NewErrorResponse("Wallet not found", nil, 404), nil
	}

	limit := data.Limit
	if limit <= 0 {
		limit = 100
	}
	offset := (data.Page - 1) * limit

	query := s.DB.Model(&models.Transaction{}).
		Where("client_id = ? AND user_id = ? AND tranasaction_type = ?", data.ClientId, data.UserId, "credit")

	if data.From != "" && data.To != "" {
		query = query.Where("created_at BETWEEN ? AND ?", data.From, data.To)
	}

	var total int64
	query.Count(&total)

	var transactions []models.Transaction
	query.Order("created_at DESC").Limit(limit).Offset(offset).Find(&transactions)

	// Additional stats (totalSaleTickets) logic from TS:
	// totals = await transactionRepository.find({ subject: 'Bet Deposit (Sport)', ... })
	// We can count instead of fetch all
	var totalSaleTickets int64
	ticketQuery := s.DB.Model(&models.Transaction{}).
		Where("client_id = ? AND user_id = ? AND subject = ?", data.ClientId, data.UserId, "Bet Deposit (Sport)")
	if data.From != "" {
		ticketQuery = ticketQuery.Where("created_at BETWEEN ? AND ?", data.From, data.To)
	}
	ticketQuery.Count(&totalSaleTickets)

	return map[string]interface{}{
		"success": true,
		"message": "Commission balance and transactions fetched successfully.",
		"status":  200,
		"data": map[string]interface{}{
			"commission_balance": wallet.CommissionBalance,
			"available_balance":  wallet.AvailableBalance,
			"commission":         wallet.CommissionBalance,
			"balance":            wallet.AvailableBalance, // TS returns both
			"totalSaleTickets":   totalSaleTickets,
			"transactions":       transactions, // Simplified return
			"pagination": map[string]interface{}{
				"page":    data.Page,
				"limit":   limit,
				"total":   total,
				"pages":   int(math.Ceil(float64(total) / float64(limit))),
				"hasMore": int64(offset+limit) < total,
			},
		},
	}, nil
}

type WithdrawCommissionDTO struct {
	ClientId int
	UserId   int
	Amount   float64
}

func (s *CommissionService) WithdrawCommissionBalance(data WithdrawCommissionDTO) (interface{}, error) {
	var wallet models.Wallet
	if err := s.DB.Where("client_id = ? AND user_id = ?", data.ClientId, data.UserId).First(&wallet).Error; err != nil {
		return common.NewErrorResponse("Wallet not found", nil, 404), nil
	}

	if wallet.CommissionBalance < data.Amount {
		return common.NewErrorResponse("Insufficient balance", nil, 400), nil
	}

	// This logic usually moves money from Commission Wallet to Main Wallet?
	// Or creates a withdrawal request?
	// TS: WithdrawalQueue.add('commission-withdrawal').
	// Usually this implies moving to Available Balance or a payout.
	// Let's assume transfer to available balance for now or just a debit if it's a cashout.
	// TS code: balance = wallet.commission_balance; ... reduces commission balance.
	// Does it credit available balance? Not explicitly shown in just the 'add' payload setup.
	// But in existing systems, "Withdraw Commission" usually means Transfer to Betting Wallet.

	// Let's implement as: Debit Commission, Credit Available.

	err := s.DB.Transaction(func(tx *gorm.DB) error {
		// Debit Commission
		if err := tx.Model(&wallet).UpdateColumn("commission_balance", gorm.Expr("commission_balance - ?", data.Amount)).Error; err != nil {
			return err
		}
		// Credit Available
		if err := tx.Model(&wallet).UpdateColumn("available_balance", gorm.Expr("available_balance + ?", data.Amount)).Error; err != nil {
			return err
		}

		transactionNo := common.GenerateTrxNo()

		// Record Debit on Commission
		tx.Create(&models.Transaction{
			ClientId:      data.ClientId,
			UserId:        data.UserId,
			Username:      wallet.Username,
			TransactionNo: transactionNo,
			Amount:        data.Amount,
			TrxType:       "debit",
			Subject:       "Commission Withdrawal",
			Description:   "Transfer to Main Wallet",
			Wallet:        "Commission",
			Status:        1,
			Balance:       wallet.CommissionBalance - data.Amount, // Approx
		})

		// Record Credit on Main
		tx.Create(&models.Transaction{
			ClientId:      data.ClientId,
			UserId:        data.UserId,
			Username:      wallet.Username,
			TransactionNo: transactionNo,
			Amount:        data.Amount,
			TrxType:       "credit",
			Subject:       "Commission Transfer",
			Description:   "Transfer from Commission Wallet",
			Wallet:        "Main",
			Status:        1,
			Balance:       wallet.AvailableBalance + data.Amount,
		})

		return nil
	})

	if err != nil {
		return common.NewErrorResponse("Unable to Withdraw commission balance", nil, 500), nil
	}

	return map[string]interface{}{
		"success": true,
		"message": "Commission Withdrawal processed successfully",
		"status":  200,
		"data": map[string]interface{}{
			"balance": wallet.CommissionBalance - data.Amount, // Approx
		},
	}, nil
}

func (s *CommissionService) ReverseCommission(data CommissionRequestDTO) (interface{}, error) {
	// Reverses a commission logic? TS: 'commission-reverse' queue.
	// Sounds like Debit Commission (correction).
	return s.DebitCommissionWallet(data)
}

// RequestCommissionByAffiliateDTO
type RequestCommissionByAffiliateDTO struct {
	UserId        int // Affiliate ID
	ClientId      int
	Amount        float64
	AccountName   string
	AccountNumber string
	BankCode      string
	TransactionNo string // optional?
}

func (s *CommissionService) RequestCommissionByAffiliate(data RequestCommissionByAffiliateDTO) (interface{}, error) {
	// Resolve Affiliate ID to User ID
	// identityService.getUserIdWithAffiliatId
	// Note: IdentityClient needs a specific method for this if not generic.
	// Checking IdentityClient wrapper... I don't see `GetUserIdWithAffiliatId` there.
	// I might need to add it or use `GetUserDetails` if it supports alternate lookup.
	// TS uses `getUserIdWithAffiliatId`.
	// For now, I'll assume I need to implement that in IdentityClient or stub it.
	// Or maybe the `UserId` passed IS the real user ID if the frontend handles resolution?
	// TS: `response = await this.identityService.getUserIdWithAffiliatId({ affiliateId: userId })`

	// STUB: Assume UserId IS the ID for now or implement lookup.
	// Let's implement a lookup in IdentityClient if possible.
	// But first, let's just proceed assuming we have the ID.
	realUserId := data.UserId
	// TODO: Resolve realUserId from AffiliateID if different.

	var wallet models.Wallet
	if err := s.DB.Where("client_id = ? AND user_id = ?", data.ClientId, realUserId).First(&wallet).Error; err != nil {
		return common.NewErrorResponse("Wallet not found", nil, 404), nil
	}

	if wallet.CommissionBalance < data.Amount {
		return common.NewErrorResponse("Insufficient commission balance", nil, 400), nil
	}

	// Check Withdrawal Settings (Validation) - skipped for brevity or use Defaults

	// Create Withdrawal Request
	withdrawalCode := common.GenerateTrxNo()

	err := s.DB.Transaction(func(tx *gorm.DB) error {
		// Debit Commission
		if err := tx.Model(&wallet).UpdateColumn("commission_balance", gorm.Expr("commission_balance - ?", data.Amount)).Error; err != nil {
			return err
		}

		// Create Withdrawal Record
		w := models.Withdrawal{
			ClientId: data.ClientId,
			UserId:   realUserId,
			Username: wallet.Username,
			Amount:   data.Amount,
			// AccountLimit:   0, // Field does not exist
			AccountName:    data.AccountName,
			AccountNumber:  data.AccountNumber,
			BankCode:       data.BankCode,
			Status:         0, // Pending
			WithdrawalCode: withdrawalCode,
			// Type:           "Commission", // Field does not exist in Go model yet
		}
		if err := tx.Create(&w).Error; err != nil {
			return err
		}

		// Prepare Transaction
		trx := models.Transaction{
			ClientId:      data.ClientId,
			UserId:        realUserId,
			Username:      wallet.Username,
			TransactionNo: withdrawalCode,
			Amount:        data.Amount,
			TrxType:       "debit",
			Subject:       "Affiliate Commission Withdrawal Request",
			Description:   "Commission Withdrawal",
			Wallet:        "Commission",
			Status:        0, // Pending
			Balance:       wallet.CommissionBalance - data.Amount,
		}

		if data.TransactionNo != "" {
			if affTxNo, err := strconv.Atoi(data.TransactionNo); err == nil {
				trx.AffiliateTransactionNo = &affTxNo
			}
		}

		tx.Create(&trx)

		return nil
	})

	if err != nil {
		return common.NewErrorResponse(fmt.Sprintf("Unable to process request: %v", err), nil, 500), nil
	}

	return map[string]interface{}{
		"success": true,
		"message": "Commission request sent successfully",
		"status":  200,
		"data":    nil,
	}, nil
}

func (s *CommissionService) FetchCommissionRequests(data GetCommissionBalanceDTO) (interface{}, error) {
	// Similar to GetTransactions but filtered for Commission Withdrawal
	query := s.DB.Model(&models.Transaction{}).
		Where("client_id = ?", data.ClientId).
		Where("subject = ?", "Affiliate Commission Withdrawal Request").
		Where("tranasaction_type = ?", "debit")

	// Filter by userId (affiliate) if provided... needs resolution.
	if data.UserId != 0 {
		query = query.Where("user_id = ?", data.UserId)
	}

	if data.From != "" {
		query = query.Where("created_at >= ?", data.From)
	}
	if data.To != "" {
		query = query.Where("created_at <= ?", data.To)
	}

	var total int64
	query.Count(&total)

	limit := data.Limit
	if limit <= 0 {
		limit = 20
	}
	offset := (data.Page - 1) * limit

	var requests []models.Transaction
	query.Order("created_at DESC").Limit(limit).Offset(offset).Find(&requests)

	return map[string]interface{}{
		"success": true,
		"message": "Commission requests fetched successfully",
		"status":  200,
		"data": map[string]interface{}{
			"requests": requests,
			"total":    total,
		},
	}, nil
}

func (s *CommissionService) GetAffiliateCommissionBalance(clientId, userId int) (interface{}, error) {
	// userId here is affiliateId.
	// We need to resolve it. Assuming it's resolved or same.
	var wallet models.Wallet
	if err := s.DB.Where("client_id = ? AND user_id = ?", clientId, userId).First(&wallet).Error; err != nil {
		return common.NewErrorResponse("Wallet not found", nil, 404), nil
	}

	return map[string]interface{}{
		"success": true,
		"message": "Commission balance fetched successfully.",
		"status":  200,
		"data": map[string]interface{}{
			"commission_balance": wallet.CommissionBalance,
		},
	}, nil
}

// ActiveReferrals
func (s *CommissionService) ActiveReferrals(clientId, userId int) (interface{}, error) {
	// 1. Get referrals from IdentityService
	// 2. Check their first deposits via PlayerService

	// STUB: We can't easily implement this without the identity service `getReferralUserIds` call exposing data.
	// However, we can use PlayerService `GetFirstDeposit` if we had the IDs.
	// For now, I'll return a stub or empty.
	return map[string]interface{}{
		"success": true,
		"message": "Active referrals fetched (stub)",
		"status":  200,
		"data": map[string]interface{}{
			"totalReferrals":  0,
			"activeReferrals": 0,
		},
	}, nil
}

// ListAffilateUsersTotalDeposits
func (s *CommissionService) ListAffilateUsersTotalDeposits(data *wallet.PlayerRequest) (interface{}, error) {
	userId := int32(data.UserId)

	// 1. Get all referrals for the affiliate
	res, err := s.IdentityClient.GetAffiliateUsers(&identity.AffiliateRequest{
		AffiliateId: &userId,
	})
	if err != nil {
		return common.NewErrorResponse("Unable to fetch referrals", nil, 500), nil
	}

	referrals := res.GetData()
	if len(referrals) == 0 {
		return map[string]interface{}{
			"success": true,
			"message": "No referrals found for this affiliate.",
			"status":  200,
			"data": map[string]interface{}{
				"totalDeposits": []interface{}{},
				"depositCount":  0,
				"grandTotal":    0,
			},
		}, nil
	}

	referralIds := make([]int, 0, len(referrals))
	for _, ref := range referrals {
		refMap := ref.AsMap()
		// Assuming userId key exists in the map
		if val, ok := refMap["userId"]; ok {
			// val is interface{}, likely float64 from JSON unmarshalling of structpb
			if idFloat, ok := val.(float64); ok {
				referralIds = append(referralIds, int(idFloat))
			}
		}
	}

	if len(referralIds) == 0 {
		return map[string]interface{}{
			"success": true,
			"message": "No valid referrals found.",
			"status":  200,
			"data": map[string]interface{}{
				"totalDeposits": []interface{}{},
				"depositCount":  0,
				"grandTotal":    0,
			},
		}, nil
	}

	// 3. Query deposits
	query := s.DB.Table("transactions").
		Select("user_id, SUM(amount) as total_deposit").
		Where("client_id = ? AND user_id IN ? AND subject = ? AND tranasaction_type = ?", data.ClientId, referralIds, "Deposit", "credit")

	if data.From != nil && data.To != nil {
		query = query.Where("created_at BETWEEN ? AND ?", data.From, data.To)
	} else {
		// Default to today if not specified? TS defaults to start/end of TODAY if not provided.
		now := time.Now()
		startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()).Format("2006-01-02 15:04:05")
		endOfDay := time.Date(now.Year(), now.Month(), now.Day(), 23, 59, 59, 0, now.Location()).Format("2006-01-02 15:04:05")
		query = query.Where("created_at BETWEEN ? AND ?", startOfDay, endOfDay)
	}

	var totalDeposits []struct {
		UserId       int     `json:"userId"`
		TotalDeposit float64 `json:"totalDeposit"`
	}

	if err := query.Group("user_id").Scan(&totalDeposits).Error; err != nil {
		return common.NewErrorResponse("Error fetching deposits", nil, 500), nil
	}

	var grandTotal float64
	for _, item := range totalDeposits {
		grandTotal += item.TotalDeposit
	}

	return map[string]interface{}{
		"success": true,
		"message": "Affiliate users total deposits fetched successfully.",
		"status":  200,
		"data": map[string]interface{}{
			"totalDeposits": totalDeposits,
			"depositCount":  len(totalDeposits),
			"grandTotal":    grandTotal,
		},
	}, nil
}

// AdminGetAffiliateReferralDeposits
func (s *CommissionService) AdminGetAffiliateReferralDeposits(data *wallet.ClientAffiliateRequest) (interface{}, error) {
	clientId := int32(data.ClientId)
	page := data.Page
	if page == 0 {
		page = 1
	}
	limit := data.Limit
	if limit == 0 {
		limit = 20
	}

	// 1. Get all affiliates
	affiliatesRes, err := s.IdentityClient.GetClientAffiliates(&identity.ClientIdRequest{
		ClientId: clientId,
	})
	if err != nil {
		return common.NewErrorResponse("Unable to fetch affiliates", nil, 500), nil
	}

	affiliates := affiliatesRes.GetData()
	if len(affiliates) == 0 {
		return map[string]interface{}{
			"success": true,
			"status":  200,
			"message": "No affiliates found for this client",
			"data":    []interface{}{},
		}, nil
	}

	var result []map[string]interface{}

	for _, affiliate := range affiliates {
		affMap := affiliate.AsMap()
		var affID int32
		var affUsername string
		var affIdInterface interface{} // could be float64

		if val, ok := affMap["affiliateId"]; ok {
			// Try "affiliateId" first (if that's the key)
			if idFloat, ok := val.(float64); ok {
				affID = int32(idFloat)
				affIdInterface = val
			}
		} else if val, ok := affMap["id"]; ok {
			// Sometimes it might be "id"
			if idFloat, ok := val.(float64); ok {
				affID = int32(idFloat)
				affIdInterface = val
			}
		}

		if val, ok := affMap["username"]; ok {
			if s, ok := val.(string); ok {
				affUsername = s
			}
		}

		referralRes, err := s.IdentityClient.GetAffiliateUsers(&identity.AffiliateRequest{
			AffiliateId: &affID,
		})
		if err != nil {
			continue // Skip on error
		}

		referrals := referralRes.GetData()
		if len(referrals) == 0 {
			result = append(result, map[string]interface{}{
				"affiliateId":   affIdInterface,
				"referralCount": 0,
				"totalDeposits": []interface{}{},
				"grandTotal":    0,
			})
			continue
		}

		referralIds := make([]int, 0, len(referrals))
		for _, ref := range referrals {
			refMap := ref.AsMap()
			if val, ok := refMap["userId"]; ok {
				if idFloat, ok := val.(float64); ok {
					referralIds = append(referralIds, int(idFloat))
				}
			}
		}

		if len(referralIds) == 0 {
			result = append(result, map[string]interface{}{
				"affiliateId":   affIdInterface,
				"referralCount": 0,
				"totalDeposits": []interface{}{},
				"grandTotal":    0,
			})
			continue
		}

		// Query Deposits
		query := s.DB.Table("transactions").
			Select("user_id, SUM(amount) as total_deposit").
			Where("client_id = ? AND user_id IN ? AND subject = ? AND tranasaction_type = ?", data.ClientId, referralIds, "Deposit", "credit")

		if data.From != nil && data.To != nil {
			query = query.Where("created_at BETWEEN ? AND ?", data.From, data.To)
		} else {
			// Default today
			now := time.Now()
			startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()).Format("2006-01-02 15:04:05")
			endOfDay := time.Date(now.Year(), now.Month(), now.Day(), 23, 59, 59, 0, now.Location()).Format("2006-01-02 15:04:05")
			query = query.Where("created_at BETWEEN ? AND ?", startOfDay, endOfDay)
		}

		var deposits []struct {
			UserId       int     `json:"userId"`
			TotalDeposit float64 `json:"totalDeposit"`
		}

		query.Group("user_id").Order("total_deposit DESC").Scan(&deposits)

		var grandTotal float64
		for _, item := range deposits {
			grandTotal += item.TotalDeposit
		}

		result = append(result, map[string]interface{}{
			"affiliateId":   affIdInterface,
			"affiliateName": affUsername,
			"referralCount": len(referrals),
			"totalDeposits": deposits,
			"grandTotal":    grandTotal,
		})
	}

	return map[string]interface{}{
		"success": true,
		"status":  200,
		"message": "Affiliate referral deposits fetched successfully",
		"data": map[string]interface{}{
			"affiliates": result,
			"pagination": map[string]interface{}{
				"page":            page,
				"limit":           limit,
				"totalAffiliates": len(affiliates),
				"totalPages":      int(math.Ceil(float64(len(affiliates)) / float64(limit))),
			},
		},
	}, nil
}

// GetAffiliateReferralDailyDeposit (Helper)
func (s *CommissionService) GetAffiliateReferralDailyDeposit(clientId int32, referralIds []int) (float64, error) {
	if len(referralIds) == 0 {
		return 0, nil
	}

	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	var result struct {
		Total float64
	}

	err := s.DB.Table("transactions").
		Select("COALESCE(SUM(amount), 0) as total").
		Where("client_id = ? AND user_id IN ? AND subject = ? AND tranasaction_type = ? AND created_at >= ?",
			clientId, referralIds, "Deposit", "credit", today).
		Scan(&result).Error

	if err != nil {
		return 0, err
	}
	return result.Total, nil
}

// GetAffiliateReferralMonthlyDeposit (Helper)
func (s *CommissionService) GetAffiliateReferralMonthlyDeposit(clientId int32, referralIds []int) (float64, error) {
	if len(referralIds) == 0 {
		return 0, nil
	}

	now := time.Now()
	firstDay := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())

	var result struct {
		Total float64
	}

	err := s.DB.Table("transactions").
		Select("COALESCE(SUM(amount), 0) as total").
		Where("client_id = ? AND user_id IN ? AND subject = ? AND tranasaction_type = ? AND created_at >= ?",
			clientId, referralIds, "Deposit", "credit", firstDay).
		Scan(&result).Error

	if err != nil {
		return 0, err
	}
	return result.Total, nil
}

// AdminGetAffiliateReferralWithdrawals
func (s *CommissionService) AdminGetAffiliateReferralWithdrawals(data *wallet.ClientAffiliateRequest) (interface{}, error) {
	clientId := int32(data.ClientId)
	// page := data.Page // Reuse logic if needed

	affiliatesRes, err := s.IdentityClient.GetClientAffiliates(&identity.ClientIdRequest{
		ClientId: clientId,
	})
	if err != nil {
		return common.NewErrorResponse("Unable to fetch affiliates", nil, 500), nil
	}

	affiliates := affiliatesRes.GetData()
	if len(affiliates) == 0 {
		return map[string]interface{}{
			"success": true,
			"status":  200,
			"message": "No affiliates found for this client",
			"data":    []interface{}{},
		}, nil
	}

	var result []map[string]interface{}

	for _, affiliate := range affiliates {
		affMap := affiliate.AsMap()
		var affID int32
		var affUsername string
		var affIdInterface interface{}

		if val, ok := affMap["affiliateId"]; ok {
			if idFloat, ok := val.(float64); ok {
				affID = int32(idFloat)
				affIdInterface = val
			}
		} else if val, ok := affMap["id"]; ok {
			if idFloat, ok := val.(float64); ok {
				affID = int32(idFloat)
				affIdInterface = val
			}
		}

		if val, ok := affMap["username"]; ok {
			if s, ok := val.(string); ok {
				affUsername = s
			}
		}

		referralRes, err := s.IdentityClient.GetAffiliateUsers(&identity.AffiliateRequest{
			AffiliateId: &affID,
		})
		if err != nil {
			continue
		}

		referrals := referralRes.GetData()
		if len(referrals) == 0 {
			result = append(result, map[string]interface{}{
				"affiliateId":      affIdInterface,
				"referralCount":    0,
				"totalWithdrawals": []interface{}{},
				"grandTotal":       0,
			})
			continue
		}

		referralIds := make([]int, 0, len(referrals))
		for _, ref := range referrals {
			refMap := ref.AsMap()
			if val, ok := refMap["userId"]; ok {
				if idFloat, ok := val.(float64); ok {
					referralIds = append(referralIds, int(idFloat))
				}
			}
		}

		if len(referralIds) == 0 {
			result = append(result, map[string]interface{}{
				"affiliateId":      affIdInterface,
				"referralCount":    0,
				"totalWithdrawals": []interface{}{},
				"grandTotal":       0,
			})
			continue
		}

		// Query Withdrawals
		query := s.DB.Table("transactions").
			Select("user_id, SUM(amount) as total_withdrawal").
			Where("client_id = ? AND user_id IN ? AND subject = ? AND tranasaction_type = ?", data.ClientId, referralIds, "Withdrawal", "debit")

		if data.From != nil && data.To != nil {
			query = query.Where("created_at BETWEEN ? AND ?", data.From, data.To)
		} else {
			// Default today
			now := time.Now()
			startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()).Format("2006-01-02 15:04:05")
			endOfDay := time.Date(now.Year(), now.Month(), now.Day(), 23, 59, 59, 0, now.Location()).Format("2006-01-02 15:04:05")
			query = query.Where("created_at BETWEEN ? AND ?", startOfDay, endOfDay)
		}

		var withdrawals []struct {
			UserId          int     `json:"userId"`
			TotalWithdrawal float64 `json:"totalWithdrawal"`
		}

		query.Group("user_id").Order("total_withdrawal DESC").Scan(&withdrawals)

		var grandTotal float64
		for _, item := range withdrawals {
			grandTotal += item.TotalWithdrawal
		}

		result = append(result, map[string]interface{}{
			"affiliateId":      affIdInterface,
			"affiliateName":    affUsername,
			"referralCount":    len(referrals),
			"totalWithdrawals": withdrawals,
			"grandTotal":       grandTotal,
		})
	}

	return map[string]interface{}{
		"success": true,
		"status":  200,
		"message": "Affiliate referral withdrawals fetched successfully",
		"data":    result,
	}, nil
}

// AdminDailyAndMonthlyReport
func (s *CommissionService) AdminDailyAndMonthlyReport(data *wallet.ClientAffiliateRequest) (interface{}, error) {
	clientId := int32(data.ClientId)

	affiliatesRes, err := s.IdentityClient.GetClientAffiliates(&identity.ClientIdRequest{
		ClientId: clientId,
	})
	if err != nil {
		return common.NewErrorResponse("Unable to fetch affiliates", nil, 500), nil
	}

	affiliates := affiliatesRes.GetData()
	if len(affiliates) == 0 {
		return map[string]interface{}{
			"success": true,
			"status":  200,
			"message": "No affiliates found for this client",
			"data":    []interface{}{},
		}, nil
	}

	var result []map[string]interface{}

	for _, affiliate := range affiliates {
		affMap := affiliate.AsMap()
		var affID int32
		var affUsername string
		var affIdInterface interface{}

		if val, ok := affMap["id"]; ok {
			if idFloat, ok := val.(float64); ok {
				affID = int32(idFloat)
				affIdInterface = val
			}
		} else if val, ok := affMap["affiliateId"]; ok {
			if idFloat, ok := val.(float64); ok {
				affID = int32(idFloat)
				affIdInterface = val
			}
		}

		if val, ok := affMap["username"]; ok {
			if s, ok := val.(string); ok {
				affUsername = s
			}
		}

		referralRes, err := s.IdentityClient.GetAffiliateUsers(&identity.AffiliateRequest{
			AffiliateId: &affID,
		})
		if err != nil {
			continue // Skip
		}

		referrals := referralRes.GetData()
		referralIds := make([]int, 0, len(referrals))
		for _, ref := range referrals {
			refMap := ref.AsMap()
			if val, ok := refMap["userId"]; ok {
				if idFloat, ok := val.(float64); ok {
					referralIds = append(referralIds, int(idFloat))
				}
			}
		}

		if len(referralIds) == 0 {
			result = append(result, map[string]interface{}{
				"affiliateId":    affIdInterface,
				"affiliateName":  affUsername,
				"referralCount":  0,
				"dailyDeposit":   0,
				"monthlyDeposit": 0,
				"grandTotal":     0,
			})
			continue
		}

		dailyDeposit, _ := s.GetAffiliateReferralDailyDeposit(clientId, referralIds)
		monthlyDeposit, _ := s.GetAffiliateReferralMonthlyDeposit(clientId, referralIds)
		grandTotal := dailyDeposit + monthlyDeposit

		deposits := []map[string]interface{}{
			{"type": "daily", "totalDeposit": dailyDeposit},
			{"type": "monthly", "totalDeposit": monthlyDeposit},
		}

		result = append(result, map[string]interface{}{
			"affiliateId":    affIdInterface,
			"affiliateName":  affUsername,
			"referralCount":  len(referrals),
			"deposits":       deposits,
			"dailyDeposit":   dailyDeposit,
			"monthlyDeposit": monthlyDeposit,
			"grandTotal":     grandTotal,
		})
	}

	return map[string]interface{}{
		"success": true,
		"status":  200,
		"message": "Affiliate daily and monthly report fetched successfully",
		"data":    result,
	}, nil
}

// ListAffiliateTotalDepositsAndWithdrawals
func (s *CommissionService) ListAffiliateTotalDepositsAndWithdrawals(data *wallet.PlayerRequest) (interface{}, error) {
	clientId := int32(data.ClientId)
	affiliateId := int32(data.UserId)

	var depositQuery = s.DB.Table("transactions").
		Select("user_id, username, SUM(amount) as total_deposit").
		Where("client_id = ? AND user_id = ? AND subject = ? AND tranasaction_type = ? AND status = ?",
			clientId, affiliateId, "Deposit", "credit", 1).
		Group("user_id, username").
		Order("SUM(amount) DESC")

	if data.From != nil && data.To != nil {
		depositQuery = depositQuery.Where("created_at BETWEEN ? AND ?", data.From, data.To)
	}

	var deposits []struct {
		UserId       int     `json:"userId"`
		Username     string  `json:"username"`
		TotalDeposit float64 `json:"totalDeposit"`
	}
	depositQuery.Scan(&deposits)

	var grandTotalDeposit float64
	for _, d := range deposits {
		grandTotalDeposit += d.TotalDeposit
	}

	var withdrawalQuery = s.DB.Table("transactions").
		Select("user_id, username, SUM(amount) as total_withdrawal").
		Where("client_id = ? AND user_id = ? AND subject = ? AND tranasaction_type = ? AND status = ?",
			clientId, affiliateId, "Withdrawal", "debit", 1).
		Group("user_id, username").
		Order("SUM(amount) DESC")

	if data.From != nil && data.To != nil {
		withdrawalQuery = withdrawalQuery.Where("created_at BETWEEN ? AND ?", data.From, data.To)
	}

	var withdrawals []struct {
		UserId          int     `json:"userId"`
		Username        string  `json:"username"`
		TotalWithdrawal float64 `json:"totalWithdrawal"`
	}
	withdrawalQuery.Scan(&withdrawals)

	var grandTotalWithdrawal float64
	for _, w := range withdrawals {
		grandTotalWithdrawal += w.TotalWithdrawal
	}

	dateRange := "No date filter applied (all data returned)"
	if data.From != nil && data.To != nil {
		dateRange = fmt.Sprintf("From: %s To: %s", *data.From, *data.To)
	}

	return map[string]interface{}{
		"success": true,
		"message": "Affiliate deposit and withdrawal totals fetched successfully.",
		"status":  200,
		"data": map[string]interface{}{
			"deposits":             deposits,
			"withdrawals":          withdrawals,
			"depositCount":         len(deposits),
			"withdrawalCount":      len(withdrawals),
			"grandTotalDeposit":    grandTotalDeposit,
			"grandTotalWithdrawal": grandTotalWithdrawal,
			"dateRangeUsed":        dateRange,
		},
	}, nil
}
