package services

import (
	"fmt"
	"net/http"
	"time"

	"wallet-service/internal/models"
	"wallet-service/pkg/common" // Using wallet proto for request/response structures if available, or map[string]interface{}
	"wallet-service/proto/identity"

	"gorm.io/gorm"
)

type DashboardService struct {
	DB             *gorm.DB
	IdentityClient *IdentityClient
}

func NewDashboardService(db *gorm.DB, identityClient *IdentityClient) *DashboardService {
	return &DashboardService{
		DB:             db,
		IdentityClient: identityClient,
	}
}

// Helper: GetDateRange
func (s *DashboardService) getDateRange(rangeZ string, date time.Time) (time.Time, time.Time) {
	start := date
	end := date

	switch rangeZ {
	case "day":
		start = time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, time.UTC)
		end = time.Date(end.Year(), end.Month(), end.Day(), 23, 59, 59, 999999999, time.UTC)
	case "week":
		// Monday as start of week
		weekday := start.Weekday()
		if weekday == time.Sunday {
			weekday = 7
		}
		diffToMonday := int(weekday) - 1
		start = start.AddDate(0, 0, -diffToMonday)
		start = time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, time.UTC)
		end = start.AddDate(0, 0, 6)
		end = time.Date(end.Year(), end.Month(), end.Day(), 23, 59, 59, 999999999, time.UTC)
	case "month":
		start = time.Date(start.Year(), start.Month(), 1, 0, 0, 0, 0, time.UTC)
		end = start.AddDate(0, 1, -1)
		end = time.Date(end.Year(), end.Month(), end.Day(), 23, 59, 59, 999999999, time.UTC)
	case "year":
		start = time.Date(start.Year(), 1, 1, 0, 0, 0, 0, time.UTC)
		end = time.Date(start.Year(), 12, 31, 23, 59, 59, 999999999, time.UTC)
	case "yesterday":
		start = start.AddDate(0, 0, -1)
		start = time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, time.UTC)
		end = end.AddDate(0, 0, -1)
		end = time.Date(end.Year(), end.Month(), end.Day(), 23, 59, 59, 999999999, time.UTC)
	default: // default to day
		start = time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, time.UTC)
		end = time.Date(end.Year(), end.Month(), end.Day(), 23, 59, 59, 999999999, time.UTC)
	}

	return start, end
}

// FinancialPerformance
func (s *DashboardService) FinancialPerformance(clientId int) (interface{}, error) {
	var depositSum float64
	var withdrawalSum float64
	var archivedDepositSum float64

	// Live Deposits
	s.DB.Model(&models.Transaction{}).
		Select("COALESCE(SUM(amount), 0)").
		Where("client_id = ? AND tranasaction_type = ? AND subject = ? AND status = ?", clientId, "credit", "Deposit", 1).
		Scan(&depositSum)

	// Withdrawals
	s.DB.Model(&models.Withdrawal{}).
		Select("COALESCE(SUM(amount), 0)").
		Where("client_id = ? AND status = ?", clientId, 1).
		Scan(&withdrawalSum)

	// Archived Deposits
	s.DB.Table("archived_transactions"). // Assuming table name
						Select("COALESCE(SUM(amount), 0)").
						Where("client_id = ? AND tranasaction_type = ? AND subject = ? AND status = ?", clientId, "credit", "Deposit", 1).
						Scan(&archivedDepositSum)

	totalDeposit := depositSum + archivedDepositSum
	totalWithdrawal := withdrawalSum

	return map[string]interface{}{
		"success":         true,
		"status":          http.StatusOK,
		"message":         "Wallet summary fetched successfully",
		"totalDeposit":    totalDeposit,
		"totalWithdrawal": totalWithdrawal,
	}, nil
}

// Balances
func (s *DashboardService) Balances(clientId int, rangeZ string, from, to string) (interface{}, error) {
	var start, end time.Time
	var err error

	if from != "" && to != "" {
		start, err = time.Parse("2006-01-02", from) // Adjust format as needed
		if err != nil {
			return common.NewErrorResponse("Invalid from date format", nil, http.StatusBadRequest), nil
		}
		end, err = time.Parse("2006-01-02", to)
		if err != nil {
			return common.NewErrorResponse("Invalid to date format", nil, http.StatusBadRequest), nil
		}
		// Set times
		start = time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, time.UTC)
		end = time.Date(end.Year(), end.Month(), end.Day(), 23, 59, 59, 999999999, time.UTC)
	} else {
		start, end = s.getDateRange(rangeZ, time.Now())
	}

	// Fetch users from Identity Service
	resp, err := s.IdentityClient.GetClientUsers(&identity.ClientIdRequest{ClientId: int32(clientId)})
	if err != nil {
		return common.NewErrorResponse("Failed to fetch client users", nil, http.StatusInternalServerError), nil
	}

	var playerUserIds []int
	var retailUserIds []int
	retailRoles := map[string]bool{
		"Cashier": true, "Shop": true, "Agent": true, "Master Agent": true, "Super Agent": true, "POS": true,
	}

	for _, user := range resp.Data {
		roleVal := user.Fields["role"]
		role := ""
		if roleVal != nil {
			role = roleVal.GetStringValue()
		}
		idVal := user.Fields["id"]
		var id int
		if idVal != nil {
			id = int(idVal.GetNumberValue())
		}

		if id == 0 {
			continue
		}

		if role == "Player" || role == "" {
			playerUserIds = append(playerUserIds, id)
		} else if retailRoles[role] {
			retailUserIds = append(retailUserIds, id)
		}
	}

	// Player Balance
	var totalOnlinePlayerBalance float64
	if len(playerUserIds) > 0 {
		s.DB.Model(&models.Wallet{}).
			Select("COALESCE(SUM(available_balance), 0)").
			Where("user_id IN ?", playerUserIds).
			Scan(&totalOnlinePlayerBalance)
	}

	// Player Bonus
	var totalOnlinePlayerBonus float64
	if len(playerUserIds) > 0 {
		s.DB.Model(&models.Wallet{}).
			Select("COALESCE(SUM(sport_bonus_balance + virtual_bonus_balance + casino_bonus_balance), 0)").
			Where("user_id IN ?", playerUserIds).
			Scan(&totalOnlinePlayerBonus)
	}

	// Retail Balance
	var totalRetailBalance float64
	if len(retailUserIds) > 0 {
		s.DB.Model(&models.Wallet{}).
			Select("COALESCE(SUM(available_balance), 0)").
			Where("user_id IN ?", retailUserIds).
			Scan(&totalRetailBalance)
	}

	// Retail Trust Balance
	var totalRetailTrustBalance float64
	if len(retailUserIds) > 0 {
		s.DB.Model(&models.Wallet{}).
			Select("COALESCE(SUM(trust_balance), 0)").
			Where("user_id IN ?", retailUserIds).
			Scan(&totalRetailTrustBalance)
	}

	return map[string]interface{}{
		"success":                  true,
		"status":                   http.StatusOK, // 200
		"message":                  "Data fetched successfully",
		"totalOnlinePlayerBalance": totalOnlinePlayerBalance,
		"totalOnlinePlayerBonus":   totalOnlinePlayerBonus,
		"totalRetailBalance":       totalRetailBalance,
		"totalRetailTrustBalance":  totalRetailTrustBalance,
	}, nil
}

type ProductStat struct {
	Name           string
	StakeSubject   string
	WinningSubject string
	WalletField    string
}

func (s *DashboardService) GetGamingSummary(clientId int, rangeZ string, from, to string) (interface{}, error) {
	start, end, err := s.parseDateRange(rangeZ, from, to)
	if err != nil {
		return common.NewErrorResponse(err.Error(), nil, http.StatusBadRequest), nil
	}

	products := []ProductStat{
		{"Sport", "Bet Deposit (Sport)", "Sport Win", "sport_bonus_balance"},
		{"Casino", "Bet Deposit (Casino)", "Bonus Bet (Casino)", "casino_bonus_balance"},
		{"Virtual Sport", "Bet Deposit (Virtual)", "Bonus Bet (Virtual)", "virtual_bonus_balance"},
	}

	var summary []map[string]interface{}

	for _, p := range products {
		var stake, winnings, bonusPlayed, bonusGiven float64
		var archStake, archWinnings, archBonusPlayed float64

		// Live Transactions
		s.DB.Model(&models.Transaction{}).Select("COALESCE(SUM(amount), 0)").Where("client_id = ? AND subject = ? AND created_at BETWEEN ? AND ?", clientId, p.StakeSubject, start, end).Scan(&stake)
		s.DB.Model(&models.Transaction{}).Select("COALESCE(SUM(amount), 0)").Where("client_id = ? AND subject = ? AND created_at BETWEEN ? AND ?", clientId, p.WinningSubject, start, end).Scan(&winnings)
		s.DB.Model(&models.Transaction{}).Select("COALESCE(SUM(amount), 0)").Where("client_id = ? AND subject = ? AND created_at BETWEEN ? AND ?", clientId, p.WinningSubject, start, end).Scan(&bonusPlayed)

		// Archived
		s.DB.Table("archived_transactions").Select("COALESCE(SUM(amount), 0)").Where("client_id = ? AND subject = ? AND created_at BETWEEN ? AND ?", clientId, p.StakeSubject, start, end).Scan(&archStake)
		s.DB.Table("archived_transactions").Select("COALESCE(SUM(amount), 0)").Where("client_id = ? AND subject = ? AND created_at BETWEEN ? AND ?", clientId, p.WinningSubject, start, end).Scan(&archWinnings)
		s.DB.Table("archived_transactions").Select("COALESCE(SUM(amount), 0)").Where("client_id = ? AND subject = ? AND created_at BETWEEN ? AND ?", clientId, p.WinningSubject, start, end).Scan(&archBonusPlayed)

		// Bonus Given (Wallet)
		s.DB.Model(&models.Wallet{}).Select(fmt.Sprintf("COALESCE(SUM(%s), 0)", p.WalletField)).Where("client_id = ? AND created_at BETWEEN ? AND ?", clientId, start, end).Scan(&bonusGiven)

		totalStake := stake + archStake
		totalWinnings := winnings + archWinnings
		totalBonusPlayed := bonusPlayed + archBonusPlayed

		ggr := totalStake - totalWinnings
		margin := "0%"
		if totalStake > 0 {
			margin = fmt.Sprintf("%.2f%%", (ggr/totalStake)*100)
		}

		stat := map[string]interface{}{
			"product":    p.Name,
			"turnover":   totalStake,
			"margin":     margin,
			"ggr":        ggr,
			"bonusGiven": bonusGiven,
			"bonusSpent": totalBonusPlayed,
			"ngr":        ggr,
		}
		summary = append(summary, stat)
	}

	return map[string]interface{}{
		"startDate": start.Format(time.RFC3339),
		"endDate":   end.Format(time.RFC3339),
		"data":      summary,
	}, nil
}

func (s *DashboardService) parseDateRange(rangeZ, from, to string) (time.Time, time.Time, error) {
	if from != "" && to != "" {
		start, err := time.Parse("2006-01-02", from)
		if err != nil {
			return time.Time{}, time.Time{}, fmt.Errorf("invalid from date")
		}
		end, err := time.Parse("2006-01-02", to)
		if err != nil {
			return time.Time{}, time.Time{}, fmt.Errorf("invalid to date")
		}
		start = time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, time.UTC)
		end = time.Date(end.Year(), end.Month(), end.Day(), 23, 59, 59, 999999999, time.UTC)
		return start, end, nil
	}
	start, end := s.getDateRange(rangeZ, time.Now())
	return start, end, nil
}

// GamingSummaryForOnline & GamingSummaryForRetail would follow similar patterns
// but additionally filtering by UserIDs fetched from IdentityService
// I will implement them as stubs or full implementations if space allows.
// Implementation for Online:

func (s *DashboardService) GamingSummaryForOnline(clientId int, rangeZ, from, to string) (interface{}, error) {
	start, end, err := s.parseDateRange(rangeZ, from, to)
	if err != nil {
		return common.NewErrorResponse(err.Error(), nil, http.StatusBadRequest), nil
	}

	resp, err := s.IdentityClient.GetClientUsers(&identity.ClientIdRequest{ClientId: int32(clientId)})
	if err != nil {
		return common.NewErrorResponse("Failed to fetch client users", nil, http.StatusInternalServerError), nil
	}

	var playerUserIds []int
	for _, user := range resp.Data {
		roleVal := user.Fields["role"]
		role := ""
		if roleVal != nil {
			role = roleVal.GetStringValue()
		}
		idVal := user.Fields["id"]
		var id int
		if idVal != nil {
			id = int(idVal.GetNumberValue())
		}
		if id == 0 {
			continue
		}
		if role == "Player" || role == "" {
			playerUserIds = append(playerUserIds, id)
		}
	}

	products := []ProductStat{
		{"Sport", "Bet Deposit (Sport)", "Sport Win", "sport_bonus_balance"},
		{"Casino", "Bet Deposit (Casino)", "Bonus Bet (Casino)", "casino_bonus_balance"},
		{"Virtual Sport", "Bet Deposit (Virtual)", "Bonus Bet (Virtual)", "virtual_bonus_balance"},
	}

	if len(playerUserIds) == 0 {
		var summary []map[string]interface{}
		for _, p := range products {
			summary = append(summary, map[string]interface{}{
				"product":    p.Name,
				"turnover":   0.0,
				"margin":     "0%",
				"ggr":        0.0,
				"bonusGiven": 0.0,
				"bonusSpent": 0.0,
				"ngr":        0.0,
			})
		}
		return map[string]interface{}{
			"startDate": start.Format(time.RFC3339),
			"endDate":   end.Format(time.RFC3339),
			"data":      summary,
		}, nil
	}

	var summary []map[string]interface{}

	for _, p := range products {
		var stake, winnings, bonusPlayed, bonusGiven float64
		var archStake, archWinnings, archBonusPlayed float64

		// Filtering by UserIDs
		s.DB.Model(&models.Transaction{}).Select("COALESCE(SUM(amount), 0)").Where("client_id = ? AND subject = ? AND created_at BETWEEN ? AND ? AND user_id IN ?", clientId, p.StakeSubject, start, end, playerUserIds).Scan(&stake)

		// Winnings Pattern (Using LIKE as in TS)
		s.DB.Model(&models.Transaction{}).Select("COALESCE(SUM(amount), 0)").Where("client_id = ? AND subject LIKE ? AND created_at BETWEEN ? AND ? AND user_id IN ?", clientId, "%Win%", start, end, playerUserIds).Scan(&winnings)

		// Bonus Played (Subject match)
		s.DB.Model(&models.Transaction{}).Select("COALESCE(SUM(amount), 0)").Where("client_id = ? AND subject = ? AND created_at BETWEEN ? AND ? AND user_id IN ?", clientId, p.WinningSubject, start, end, playerUserIds).Scan(&bonusPlayed)

		// Archived...
		s.DB.Table("archived_transactions").Select("COALESCE(SUM(amount), 0)").Where("client_id = ? AND subject = ? AND created_at BETWEEN ? AND ? AND user_id IN ?", clientId, p.StakeSubject, start, end, playerUserIds).Scan(&archStake)
		s.DB.Table("archived_transactions").Select("COALESCE(SUM(amount), 0)").Where("client_id = ? AND subject LIKE ? AND created_at BETWEEN ? AND ? AND user_id IN ?", clientId, "%Win%", start, end, playerUserIds).Scan(&archWinnings)
		s.DB.Table("archived_transactions").Select("COALESCE(SUM(amount), 0)").Where("client_id = ? AND subject = ? AND created_at BETWEEN ? AND ? AND user_id IN ?", clientId, p.WinningSubject, start, end, playerUserIds).Scan(&archBonusPlayed)

		// Bonus Given
		s.DB.Model(&models.Wallet{}).Select(fmt.Sprintf("COALESCE(SUM(%s), 0)", p.WalletField)).Where("user_id IN ?", playerUserIds).Scan(&bonusGiven)

		totalStake := stake + archStake
		totalWinnings := winnings + archWinnings
		totalBonusPlayed := bonusPlayed + archBonusPlayed

		ggr := totalStake - totalWinnings
		margin := "0%"
		if totalStake > 0 {
			margin = fmt.Sprintf("%.2f%%", (ggr/totalStake)*100)
		}

		stat := map[string]interface{}{
			"product":    p.Name,
			"turnover":   totalStake,
			"margin":     margin,
			"ggr":        ggr,
			"bonusGiven": bonusGiven,
			"bonusSpent": totalBonusPlayed,
			"ngr":        ggr,
		}
		summary = append(summary, stat)
	}

	return map[string]interface{}{
		"startDate": start.Format(time.RFC3339),
		"endDate":   end.Format(time.RFC3339),
		"data":      summary,
	}, nil
}

func (s *DashboardService) GetSportSummary(clientId int, rangeZ, from, to string) (interface{}, error) {
	start, end, err := s.parseDateRange(rangeZ, from, to)
	if err != nil {
		return common.NewErrorResponse(err.Error(), nil, http.StatusBadRequest), nil
	}

	sportProduct := ProductStat{
		Name:           "Sport",
		StakeSubject:   "Bet Deposit (Sport)",
		WinningSubject: "Sport Win",
		WalletField:    "sport_bonus_balance",
	}

	var stake, winnings, bonusPlayed, bonusGiven float64
	var archStake, archWinnings, archBonusPlayed float64

	// Live
	s.DB.Model(&models.Transaction{}).Select("COALESCE(SUM(amount), 0)").Where("client_id = ? AND subject = ? AND created_at BETWEEN ? AND ?", clientId, sportProduct.StakeSubject, start, end).Scan(&stake)
	s.DB.Model(&models.Transaction{}).Select("COALESCE(SUM(amount), 0)").Where("client_id = ? AND subject = ? AND created_at BETWEEN ? AND ?", clientId, sportProduct.WinningSubject, start, end).Scan(&winnings)
	s.DB.Model(&models.Transaction{}).Select("COALESCE(SUM(amount), 0)").Where("client_id = ? AND subject = ? AND created_at BETWEEN ? AND ?", clientId, sportProduct.WinningSubject, start, end).Scan(&bonusPlayed)

	// Archived
	s.DB.Table("archived_transactions").Select("COALESCE(SUM(amount), 0)").Where("client_id = ? AND subject = ? AND created_at BETWEEN ? AND ?", clientId, sportProduct.StakeSubject, start, end).Scan(&archStake)
	s.DB.Table("archived_transactions").Select("COALESCE(SUM(amount), 0)").Where("client_id = ? AND subject = ? AND created_at BETWEEN ? AND ?", clientId, sportProduct.WinningSubject, start, end).Scan(&archWinnings)
	s.DB.Table("archived_transactions").Select("COALESCE(SUM(amount), 0)").Where("client_id = ? AND subject = ? AND created_at BETWEEN ? AND ?", clientId, sportProduct.WinningSubject, start, end).Scan(&archBonusPlayed)

	// Bonus Given
	s.DB.Model(&models.Wallet{}).Select(fmt.Sprintf("COALESCE(SUM(%s), 0)", sportProduct.WalletField)).Where("client_id = ? AND created_at BETWEEN ? AND ?", clientId, start, end).Scan(&bonusGiven)

	totalStake := stake + archStake
	totalWinnings := winnings + archWinnings
	totalBonusPlayed := bonusPlayed + archBonusPlayed

	ggr := totalStake - totalWinnings
	margin := "0%"
	if totalStake > 0 {
		margin = fmt.Sprintf("%.2f%%", (ggr/totalStake)*100)
	}

	return map[string]interface{}{
		"startDate": start.Format("Mon Jan 02 2006 15:04:05 GMT-0700 (Coordinated Universal Time)"),
		"endDate":   end.Format("Mon Jan 02 2006 15:04:05 GMT-0700 (Coordinated Universal Time)"),
		"data": []interface{}{
			map[string]interface{}{
				"product":    sportProduct.Name,
				"turnover":   totalStake,
				"margin":     margin,
				"ggr":        ggr,
				"bonusGiven": bonusGiven,
				"bonusSpent": totalBonusPlayed,
				"ngr":        ggr,
			},
		},
	}, nil
}

func (s *DashboardService) GetMonthlyGamingTurnover(clientId int, year string) (interface{}, error) {
	products := []struct {
		Key     string
		Subject string
	}{
		{"Games", "Bet Deposit (Games)"},
		{"Casino", "Bet Deposit (Casino)"},
		{"Sport", "Bet Deposit (Sport)"},
		{"Virtual", "Bet Deposit (Virtual)"},
	}

	monthNames := []string{
		"January", "February", "March", "April", "May", "June",
		"July", "August", "September", "October", "November", "December",
	}

	monthlyMap := make(map[string][]float64)

	for _, product := range products {
		monthlyTotals := make([]float64, 12)

		type Result struct {
			Month int
			Total float64
		}

		var results []Result
		s.DB.Model(&models.Transaction{}).
			Select("MONTH(created_at) as month, SUM(amount) as total").
			Where("client_id = ? AND subject = ? AND YEAR(created_at) = ?", clientId, product.Subject, year).
			Group("MONTH(created_at)").
			Scan(&results)

		for _, r := range results {
			if r.Month >= 1 && r.Month <= 12 {
				monthlyTotals[r.Month-1] += r.Total
			}
		}

		// Archived
		var archResults []Result
		s.DB.Table("archived_transactions").
			Select("MONTH(created_at) as month, SUM(amount) as total").
			Where("client_id = ? AND subject = ? AND YEAR(created_at) = ?", clientId, product.Subject, year).
			Group("MONTH(created_at)").
			Scan(&archResults)

		for _, r := range archResults {
			if r.Month >= 1 && r.Month <= 12 {
				monthlyTotals[r.Month-1] += r.Total
			}
		}

		monthlyMap[product.Key] = monthlyTotals
	}

	var data []interface{}
	for _, product := range products {
		monthlyTurnover := monthlyMap[product.Key]
		var monthlyData []interface{}
		for i, turnover := range monthlyTurnover {
			monthlyData = append(monthlyData, map[string]interface{}{
				"month":    monthNames[i],
				"turnover": turnover,
			})
		}
		data = append(data, map[string]interface{}{
			"product":     product.Key,
			"monthlyData": monthlyData,
		})
	}

	return map[string]interface{}{
		"year": year,
		"data": data,
	}, nil
}
