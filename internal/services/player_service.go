package services

import (
	"fmt"
	"log"
	"math"
	"time"

	"wallet-service/internal/models"
	"wallet-service/pkg/common"

	"gorm.io/gorm"
)

type PlayerService struct {
	DB             *gorm.DB
	IdentityClient *IdentityClient
}

func NewPlayerService(db *gorm.DB, idClient *IdentityClient) *PlayerService {
	return &PlayerService{DB: db, IdentityClient: idClient}
}

func (s *PlayerService) ListPendingWithdrawals(userId, clientId int) (interface{}, error) {
	var withdrawals []models.Withdrawal
	if err := s.DB.Where("user_id = ? AND client_id = ? AND status = ?", userId, clientId, 0).Find(&withdrawals).Error; err != nil {
		return common.NewErrorResponse("Unable to fetch withdrawal", nil, 500), nil
	}
	return map[string]interface{}{
		"success": true,
		"message": "Fetched withdrawal successfully",
		"status":  200,
		"data":    withdrawals,
	}, nil
}

func (s *PlayerService) CancelPendingWithdrawal(payload map[string]interface{}) (interface{}, error) {
	withdrawalIdFloat, _ := payload["withdrawalId"].(float64)
	withdrawalId := uint(withdrawalIdFloat)
	userIdFloat, _ := payload["userId"].(float64)
	userId := int(userIdFloat)
	clientIdFloat, _ := payload["clientId"].(float64)
	clientId := int(clientIdFloat)
	comment, _ := payload["comment"].(string)

	// Identity check
	userDetails, err := s.IdentityClient.GetUser(userId)
	if err != nil {
		return common.NewErrorResponse("Error fetching user details", nil, 500), nil
	}

	var withdrawal models.Withdrawal
	if err := s.DB.Where("id = ? AND client_id = ?", withdrawalId, clientId).First(&withdrawal).Error; err != nil {
		return common.NewErrorResponse("Withdrawal not found", nil, 404), nil
	}

	if withdrawal.Status == 4 || withdrawal.Status == 3 {
		return common.NewErrorResponse("Withdrawal already cancelled", nil, 400), nil
	}
	if withdrawal.Status == 1 {
		return common.NewErrorResponse("Withdrawal already processed", nil, 400), nil
	}

	userRole := userDetails.Data.GetRole()
	newStatus := 0
	if userRole == "Shop" {
		newStatus = 3
	} else if userRole == "Player" {
		newStatus = 4
	} else {
		return common.NewErrorResponse(fmt.Sprintf("Unauthorized role: %s", userRole), nil, 400), nil
	}

	withdrawal.Status = newStatus
	withdrawal.Comment = comment
	s.DB.Save(&withdrawal)

	// Refund Wallet
	var wallet models.Wallet
	if err := s.DB.Where("user_id = ? AND client_id = ?", userId, clientId).First(&wallet).Error; err != nil {
		return common.NewErrorResponse("Wallet not found", nil, 404), nil
	}

	s.DB.Model(&wallet).UpdateColumn("available_balance", gorm.Expr("available_balance + ?", withdrawal.Amount))

	// Update Transactions
	var txs []models.Transaction
	s.DB.Where("transaction_no = ?", withdrawal.WithdrawalCode).Find(&txs)

	if len(txs) > 0 {
		s.DB.Model(&models.Transaction{}).Where("transaction_no = ?", withdrawal.WithdrawalCode).Update("status", newStatus)
	}

	return map[string]interface{}{
		"success": true,
		"status":  200,
		"message": "Withdrawal cancelled successfully",
		"data":    withdrawal,
	}, nil
}

// ShopUserRequest DTO
type ShopUserRequest struct {
	UserId   int
	ClientId int
	Page     int
	Status   *int
	Username string
	From     string
	To       string
}

func (s *PlayerService) ListShopUserWithdrawals(req ShopUserRequest) (interface{}, error) {
	limit := 30
	offset := (req.Page - 1) * limit

	query := s.DB.Model(&models.Withdrawal{}).Where("client_id = ? AND user_id = ?", req.ClientId, req.UserId)

	if req.Status != nil {
		query = query.Where("status = ?", *req.Status)
	}
	if req.Username != "" {
		query = query.Where("username LIKE ?", "%"+req.Username+"%")
	}

	if req.From != "" && req.To != "" {
		query = query.Where("created_at BETWEEN ? AND ?", req.From, req.To)
	}

	var total int64
	query.Count(&total)

	var result []models.Withdrawal
	query.Order("created_at DESC").Limit(limit).Offset(offset).Find(&result)

	var totalAmount float64
	for _, w := range result {
		totalAmount += w.Amount
	}

	return map[string]interface{}{
		"success": true,
		"status":  200,
		"message": "Fetched withdrawals successfully",
		"data": []map[string]interface{}{{
			"withdrawals": result,
			"summary": map[string]interface{}{
				"total":       total,
				"totalAmount": totalAmount,
			},
			"meta": map[string]interface{}{
				"total":      total,
				"page":       req.Page,
				"perPage":    limit,
				"totalPages": int(math.Ceil(float64(total) / float64(limit))),
			},
		}},
	}, nil
}

func (s *PlayerService) ListShopUserDeposit(req ShopUserRequest) (interface{}, error) {
	limit := 30
	offset := (req.Page - 1) * limit

	query := s.DB.Model(&models.Transaction{}).
		Where("client_id = ? AND user_id = ? AND tranasaction_type = ? AND subject = ?", req.ClientId, req.UserId, "credit", "Deposit")

	if req.Status != nil {
		query = query.Where("status = ?", *req.Status)
	}
	if req.Username != "" {
		query = query.Where("username LIKE ?", "%"+req.Username+"%")
	}
	if req.From != "" && req.To != "" {
		query = query.Where("created_at BETWEEN ? AND ?", req.From, req.To)
	}

	var total int64
	query.Count(&total)

	var result []models.Transaction
	query.Order("created_at DESC").Limit(limit).Offset(offset).Find(&result)

	var totalAmount float64
	for _, t := range result {
		totalAmount += t.Amount
	}

	return map[string]interface{}{
		"success": true,
		"status":  200,
		"message": "Fetched deposits successfully",
		"data": []map[string]interface{}{{
			"deposit": result,
			"summary": map[string]interface{}{
				"total":       total,
				"totalAmount": totalAmount,
			},
			"meta": map[string]interface{}{
				"total":      total,
				"page":       req.Page,
				"perPage":    limit,
				"totalPages": int(math.Ceil(float64(total) / float64(limit))),
			},
		}},
	}, nil
}

func (s *PlayerService) ShopUserProfit(req ShopUserRequest) (interface{}, error) {
	// Deposit Sum
	type SumResult struct {
		Sum float64
	}
	var depSum SumResult
	s.DB.Model(&models.Transaction{}).
		Select("SUM(amount) as sum").
		Where("client_id = ? AND user_id = ? AND tranasaction_type = ? AND subject = ? AND status = 1", req.ClientId, req.UserId, "credit", "Deposit").
		Where("created_at BETWEEN ? AND ?", req.From, req.To).
		Scan(&depSum)

	// Withdrawal Sum
	var withSum SumResult
	s.DB.Model(&models.Withdrawal{}).
		Select("SUM(amount) as sum").
		Where("client_id = ? AND user_id = ? AND status = 1", req.ClientId, req.UserId).
		Where("created_at BETWEEN ? AND ?", req.From, req.To).
		Scan(&withSum)

	profit := depSum.Sum - withSum.Sum

	return map[string]interface{}{
		"success": true,
		"status":  200,
		"message": "Profit calculated successfully",
		"data": []map[string]interface{}{{
			"userId":          req.UserId,
			"clientId":        req.ClientId,
			"totalDeposit":    depSum.Sum,
			"totalWithdrawal": withSum.Sum,
			"profit":          profit,
		}},
	}, nil
}

func (s *PlayerService) GetFirstDeposit(referralIds []int64) (interface{}, error) {
	if len(referralIds) == 0 {
		return common.NewErrorResponse("No referral IDs provided", nil, 400), nil
	}

	// Subquery equivalent in GORM
	// select t2.created_at from transactions t2 where t2.user_id = tx.user_id ... order by created_at limit 1 ??
	// TS used a subquery for MIN(created_at).

	// Using raw SQL might be cleaners or GORM specific:
	// "tx.created_at = (SELECT MIN(t2.created_at) FROM transactions t2 WHERE t2.user_id = tx.user_id ...)"

	subQuery := s.DB.Table("transactions as t2").
		Select("MIN(t2.created_at)").
		Where("t2.user_id = transactions.user_id").
		Where("t2.status = 1").
		Where("t2.tranasaction_type = ?", "credit").
		Where("t2.subject = ?", "Deposit")

	var results []struct {
		UserId           int       `json:"userId"`
		Amount           float64   `json:"amount"`
		FirstDepositDate time.Time `json:"firstDepositDate"`
	}

	err := s.DB.Table("transactions").
		Select("user_id, amount, created_at as first_deposit_date").
		Where("created_at = (?)", subQuery).
		Where("status = 1").
		Where("tranasaction_type = ?", "credit").
		Where("user_id IN ?", referralIds).
		Scan(&results).Error

	if err != nil {
		log.Println("GetFirstDeposit error:", err)
		return common.NewErrorResponse("Error fetching first deposits", nil, 500), nil
	}

	return map[string]interface{}{
		"status":  200,
		"success": true,
		"message": "Fetched first deposits successfully",
		"data":    results, // Verify if common response wrapper handles struct slice properly, else []interface{}
	}, nil
}

type ReferralStatsRequest struct {
	ReferralIds []int64
	StartDate   string
	EndDate     string
}

func (s *PlayerService) GetReferralTransactions(req ReferralStatsRequest) (interface{}, error) {
	if len(req.ReferralIds) == 0 {
		return common.NewErrorResponse("No referral IDs provided", nil, 400), nil
	}

	depositSubjects := []string{"Bet Deposit (Sport)", "Bet Deposit (Casino)", "Bet Deposit (Virtual)"}
	winSubjects := []string{"Sport Win", "Casino Win", "Virtual Win"}
	allSubjects := append(depositSubjects, winSubjects...)

	type TxResult struct {
		UserId      int
		Subject     string
		TotalAmount float64
	}

	var results []TxResult
	s.DB.Table("transactions").
		Select("user_id, subject, SUM(amount) as total_amount").
		Where("user_id IN ?", req.ReferralIds).
		Where("created_at BETWEEN ? AND ?", req.StartDate, req.EndDate).
		Where("subject IN ?", allSubjects).
		Group("user_id, subject").
		Scan(&results)

	if len(results) == 0 {
		return common.NewErrorResponse("No gaming activity found", nil, 404), nil
	}

	// Process in memory
	statsMap := make(map[int]map[string]float64) // userId -> "deposits"/"wins" -> val

	for _, res := range results {
		if _, ok := statsMap[res.UserId]; !ok {
			statsMap[res.UserId] = map[string]float64{"deposits": 0, "wins": 0}
		}

		// check subjects
		isDep := false
		for _, ds := range depositSubjects {
			if ds == res.Subject {
				isDep = true
				break
			}
		}
		if isDep {
			statsMap[res.UserId]["deposits"] += res.TotalAmount
		} else {
			statsMap[res.UserId]["wins"] += res.TotalAmount
		}
	}

	var output []map[string]interface{}
	for _, rid := range req.ReferralIds {
		// Need to iterate original list to preserve order or just map?
		// TS iterates referralIds.
		uid := int(rid)
		st, ok := statsMap[uid]
		if ok {
			output = append(output, map[string]interface{}{
				"userId":           uid,
				"totalDeposits":    st["deposits"],
				"totalWins":        st["wins"],
				"netGamingRevenue": st["deposits"] - st["wins"],
			})
		} else {
			output = append(output, map[string]interface{}{
				"userId":           uid,
				"totalDeposits":    0,
				"totalWins":        0,
				"netGamingRevenue": 0,
			})
		}
	}

	return map[string]interface{}{
		"status":  200,
		"success": true,
		"message": "Referral NGR calculated successfully",
		"data":    output,
	}, nil
}

func (s *PlayerService) GetOneReferralTransactions(referralId int, start, end string) (interface{}, error) {
	return s.GetReferralTransactions(ReferralStatsRequest{
		ReferralIds: []int64{int64(referralId)},
		StartDate:   start,
		EndDate:     end,
	})
}
