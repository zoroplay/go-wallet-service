package services

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"wallet-service/internal/models"
	"wallet-service/pkg/common"
	"wallet-service/proto/identity"

	"gorm.io/gorm"
)

type SummaryService struct {
	DB             *gorm.DB
	IdentityClient *IdentityClient
}

func NewSummaryService(db *gorm.DB, identityClient *IdentityClient) *SummaryService {
	return &SummaryService{
		DB:             db,
		IdentityClient: identityClient,
	}
}

func (s *SummaryService) parseDateRange(rangeZ, from, to string) (time.Time, time.Time, error) {
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

func (s *SummaryService) getDateRange(rangeZ string, date time.Time) (time.Time, time.Time) {
	start := date
	end := date

	switch rangeZ {
	case "day":
		start = time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, time.UTC)
		end = time.Date(end.Year(), end.Month(), end.Day(), 23, 59, 59, 999999999, time.UTC)
	case "week":
		day := int(start.Weekday())
		diffToMonday := (day + 6) % 7
		start = start.AddDate(0, 0, -diffToMonday)
		start = time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, time.UTC)
		end = start.AddDate(0, 0, 6)
		end = time.Date(end.Year(), end.Month(), end.Day(), 23, 59, 59, 999999999, time.UTC)
	case "month":
		start = time.Date(start.Year(), start.Month(), 1, 0, 0, 0, 0, time.UTC)
		end = start.AddDate(0, 1, 0).Add(-time.Nanosecond)
	case "year":
		start = time.Date(start.Year(), time.January, 1, 0, 0, 0, 0, time.UTC)
		end = time.Date(start.Year(), time.December, 31, 23, 59, 59, 999999999, time.UTC)
	case "yesterday":
		start = start.AddDate(0, 0, -1)
		start = time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, time.UTC)
		end = end.AddDate(0, 0, -1)
		end = time.Date(end.Year(), end.Month(), end.Day(), 23, 59, 59, 999999999, time.UTC)
	default: // Default to day
		start = time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, time.UTC)
		end = time.Date(end.Year(), end.Month(), end.Day(), 23, 59, 59, 999999999, time.UTC)
	}
	return start, end
}

func (s *SummaryService) GetSummary(clientId int, rangeZ, from, to string) (interface{}, error) {
	start, end, err := s.parseDateRange(rangeZ, from, to)
	if err != nil {
		return common.NewErrorResponse(err.Error(), nil, http.StatusBadRequest), nil
	}

	var depositSum, withdrawalSum float64

	s.DB.Model(&models.Transaction{}).
		Select("COALESCE(SUM(amount), 0)").
		Where("client_id = ? AND tranasaction_type = ? AND subject = ? AND status = ? AND created_at BETWEEN ? AND ?",
			clientId, "credit", "Deposit", 1, start, end).
		Scan(&depositSum)

	s.DB.Model(&models.Transaction{}).
		Select("COALESCE(SUM(amount), 0)").
		Where("client_id = ? AND tranasaction_type = ? AND subject = ? AND status = ? AND created_at BETWEEN ? AND ?",
			clientId, "debit", "Withdrawal", 1, start, end).
		Scan(&withdrawalSum)

	return map[string]interface{}{
		"success":         true,
		"status":          200,
		"message":         "Wallet summary fetched successfully",
		"totalDeposit":    depositSum,
		"totalWithdrawal": withdrawalSum,
	}, nil
}

func (s *SummaryService) GetShopUserWalletSummary(clientId int, rangeZ, from, to string) (interface{}, error) {
	start, end, err := s.parseDateRange(rangeZ, from, to)
	if err != nil {
		return common.NewErrorResponse(err.Error(), nil, http.StatusBadRequest), nil
	}

	resp, err := s.IdentityClient.GetAgents(&identity.ClientIdRequest{ClientId: int32(clientId)})
	if err != nil {
		return common.NewErrorResponse("Failed to fetch agents", nil, http.StatusInternalServerError), nil
	}

	// Proto response handling
	// Assuming resp.Data is a generic object and we need to parse it.
	// Based on TS code: users.data.data is the array of users.
	// The proto definition for CommonResponseObj has Data *structpb.Struct.
	// This parsing is tricky without knowing the exact structure returned by python/node service.
	// Simplification: We will try to map the structpb to a map and extract "data" field.

	usersData := resp.Data.AsMap()
	dataList, ok := usersData["data"].([]interface{})
	if !ok || len(dataList) == 0 {
		return map[string]interface{}{
			"success": false,
			"status":  http.StatusOK, // Or NotFound? TS returns Not Found if empty
			"message": "No agent users found for the client",
			"data":    []interface{}{},
		}, nil
	}

	var agentUsersSummary []map[string]interface{}

	for _, u := range dataList {
		userMap, ok := u.(map[string]interface{})
		if !ok {
			continue
		}

		idFloat, ok := userMap["id"].(float64)
		if !ok {
			continue
		}
		userId := int(idFloat)

		var depositSum, withdrawalSum float64

		s.DB.Model(&models.Transaction{}).
			Select("COALESCE(SUM(amount), 0)").
			Where("client_id = ? AND tranasaction_type = ? AND subject = ? AND status = ? AND created_at BETWEEN ? AND ? AND user_id = ?",
				clientId, "credit", "Deposit", 1, start, end, userId).
			Scan(&depositSum)

		s.DB.Model(&models.Transaction{}).
			Select("COALESCE(SUM(amount), 0)").
			Where("client_id = ? AND tranasaction_type = ? AND subject = ? AND status = ? AND created_at BETWEEN ? AND ? AND user_id = ?",
				clientId, "debit", "Withdrawal", 1, start, end, userId).
			Scan(&withdrawalSum)

		agentUsersSummary = append(agentUsersSummary, map[string]interface{}{
			"userId":                strconv.Itoa(userId),
			"totalDepositAmount":    depositSum,
			"totalWithdrawalAmount": withdrawalSum,
		})
	}

	return map[string]interface{}{
		"success": true,
		"status":  http.StatusOK,
		"message": "Wallet summary fetched successfully",
		"data":    agentUsersSummary,
	}, nil
}

func (s *SummaryService) GetNetCashFlow(clientId int, rangeZ, from, to string) (interface{}, error) {
	start, end, err := s.parseDateRange(rangeZ, from, to)
	if err != nil {
		return common.NewErrorResponse(err.Error(), nil, http.StatusBadRequest), nil
	}

	resp, err := s.IdentityClient.GetAgents(&identity.ClientIdRequest{ClientId: int32(clientId)})
	if err != nil {
		return common.NewErrorResponse("Failed to fetch agents", nil, http.StatusInternalServerError), nil
	}

	usersData := resp.Data.AsMap()
	dataList, ok := usersData["data"].([]interface{})
	if !ok {
		// handle empty or error
		return map[string]interface{}{
			"success": false,
			"status":  http.StatusOK,
			"message": "No shop users found",
			"data":    []interface{}{},
		}, nil
	}

	var summary []map[string]interface{}

	for _, u := range dataList {
		userMap, ok := u.(map[string]interface{})
		if !ok {
			continue
		}

		// Filter for Shop users (rolename == 'Shop' && role_id == 11)
		roleName, _ := userMap["rolename"].(string)
		roleId, _ := userMap["role_id"].(float64)

		if roleName != "Shop" || int(roleId) != 11 {
			continue
		}

		idFloat, _ := userMap["id"].(float64)
		userId := int(idFloat)
		username, _ := userMap["username"].(string)

		type Result struct {
			Count int
			Total float64
		}
		var deposit, withdrawal Result

		s.DB.Model(&models.Transaction{}).
			Select("COUNT(*) as count, COALESCE(SUM(amount), 0) as total").
			Where("client_id = ? AND user_id = ? AND tranasaction_type = ? AND subject = ? AND status = ? AND created_at BETWEEN ? AND ?",
				clientId, userId, "credit", "Deposit", 1, start, end).
			Scan(&deposit)

		s.DB.Model(&models.Transaction{}).
			Select("COUNT(*) as count, COALESCE(SUM(amount), 0) as total").
			Where("client_id = ? AND user_id = ? AND tranasaction_type = ? AND subject = ? AND status = ? AND created_at BETWEEN ? AND ?",
				clientId, userId, "debit", "Withdrawal", 1, start, end).
			Scan(&withdrawal)

		summary = append(summary, map[string]interface{}{
			"userId":              userId,
			"username":            username,
			"numberOfDeposits":    deposit.Count,
			"totalDeposits":       deposit.Total,
			"numberOfWithdrawals": withdrawal.Count,
			"totalWithdrawals":    withdrawal.Total,
		})
	}

	return map[string]interface{}{
		"success": true,
		"status":  http.StatusOK,
		"message": "Shop user net cash flow summary fetched successfully",
		"data":    summary,
	}, nil
}
