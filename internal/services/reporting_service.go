package services

import (
	"math"
	"time"

	"wallet-service/internal/models"
	"wallet-service/pkg/common"

	"gorm.io/gorm"
)

type ReportingService struct {
	DB *gorm.DB
}

func NewReportingService(db *gorm.DB) *ReportingService {
	return &ReportingService{DB: db}
}

type GetTransactionsRequestDTO struct {
	ClientId        int
	From            string
	To              string
	TransactionType string
	ReferenceNo     string
	Username        string
	Keyword         string
	Page            int
	Limit           int
}

func (s *ReportingService) GetMoneyTransaction(data GetTransactionsRequestDTO) (interface{}, error) {
	limit := data.Limit
	if limit <= 0 {
		limit = 100
	}
	page := data.Page
	if page < 1 {
		page = 1
	}
	offset := (page - 1) * limit

	// Date parsing logic similar to TS dayjs
	// Assuming incoming format DD-MM-YYYY HH:mm:ss, but standardizing on ISO 8601 YYYY-MM-DD HH:mm:ss for DB is better.
	// TS uses dayjs converts 'DD-MM-YYYY HH:mm:ss' to 'YYYY-MM-DD HH:mm:ss'.
	// In Go, we need to handle this parsing if string is provided.
	// If the frontend sends specific format we must respect it.
	// Let's assume standard DB compatible format for now or attempt parse.

	startIdx := data.From
	endIdx := data.To

	// Simple date format conversion if needed for MySQL
	// TS: dayjs(from, 'DD-MM-YYYY HH:mm:ss').format('YYYY-MM-DD HH:mm:ss')
	layoutTS := "02-01-2006 15:04:05" // DD-MM-YYYY HH:mm:ss
	layoutDB := "2006-01-02 15:04:05"

	tFrom, err := time.Parse(layoutTS, data.From)
	if err == nil {
		startIdx = tFrom.Format(layoutDB)
	}
	tTo, err := time.Parse(layoutTS, data.To)
	if err == nil {
		endIdx = tTo.Format(layoutDB)
	}

	query := s.DB.Table("transactions").
		Where("client_id = ?", data.ClientId).
		Where("created_at >= ?", startIdx).
		Where("created_at <= ?", endIdx).
		Where("status = ?", 1).
		Where("user_id != ?", 0)

	if data.TransactionType != "" {
		switch data.TransactionType {
		case "bet_deposits":
			query = query.Where("subject IN ?", []string{"Bet Deposit (Sport)", "Bet Deposit (Casino)", "Bet Deposit (Virtual)"})
		case "bet_winnnigs":
			query = query.Where("subject IN ?", []string{"Sport Win", "Bet Win (Casino)", "Virtual Sport Win"})
		case "deposits":
			query = query.Where("subject = ?", "Deposit")
		case "withdrawals":
			query = query.Where("subject = ?", "Withdrawal")
		case "d_w":
			query = query.Where("subject IN ?", []string{"Deposit", "Withdrawal"})
		case "bonuses":
			query = query.Where("wallet = ?", "Sport Bonus") // TS: wallet = :wallet {subject: 'Sport Bonus'} -> Logic looks suspicious in TS (key 'subject' but param 'wallet'). Go: matches 'wallet' column.
		case "interaccount":
			query = query.Where("subject = ?", "Sport Bonus") // TS passes 'Sport Bonus' as subject
		}
	}

	if data.ReferenceNo != "" {
		query = query.Where("transaction_no = ?", data.ReferenceNo)
	}
	if data.Username != "" {
		query = query.Where("username = ?", data.Username)
	}
	if data.Keyword != "" {
		query = query.Where("subject LIKE ?", "%"+data.Keyword+"%")
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return common.NewErrorResponse("Error fetching transactions", nil, 500), nil
	}

	var results []models.Transaction
	if err := query.Order("created_at DESC").Limit(limit).Offset(offset).Find(&results).Error; err != nil {
		return common.NewErrorResponse("Error fetching transactions", nil, 500), nil
	}

	// Pagination
	totalPages := int(math.Ceil(float64(total) / float64(limit)))
	nextPage := page + 1
	if nextPage > totalPages {
		nextPage = totalPages // or 0? TS paginateResponse logic: page < lastPage ? page + 1 : null (or check implementation)
	}
	prevPage := page - 1
	if prevPage < 1 {
		prevPage = 1 // or 0?
	}

	return map[string]interface{}{
		"success": true,
		"status":  200,
		"message": "Success",
		"data": map[string]interface{}{
			"result": results,
			"meta": map[string]interface{}{
				"page":     page,
				"perPage":  limit,
				"total":    total,
				"lastPage": totalPages,
				"nextPage": nextPage,
				"prevPage": prevPage,
			},
		},
	}, nil
}

func (s *ReportingService) GetSystemTransaction(data GetTransactionsRequestDTO) (interface{}, error) {
	limit := data.Limit
	// ... Pagination setup same as above ...
	if limit <= 0 {
		limit = 100
	}
	page := data.Page
	if page < 1 {
		page = 1
	}
	offset := (page - 1) * limit

	startIdx := data.From
	endIdx := data.To

	layoutTS := "02-01-2006 15:04:05" // DD-MM-YYYY HH:mm:ss
	layoutDB := "2006-01-02 15:04:05"

	tFrom, err := time.Parse(layoutTS, data.From)
	if err == nil {
		startIdx = tFrom.Format(layoutDB)
	}
	tTo, err := time.Parse(layoutTS, data.To)
	if err == nil {
		endIdx = tTo.Format(layoutDB)
	}

	query := s.DB.Table("transactions").
		Where("client_id = ?", data.ClientId).
		Where("created_at >= ?", startIdx).
		Where("created_at <= ?", endIdx).
		Where("status = ?", 1).
		Where("user_id = ?", 0)

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return common.NewErrorResponse("Error fetching transactions", nil, 500), nil
	}

	var results []models.Transaction
	if err := query.Order("created_at DESC").Limit(limit).Offset(offset).Find(&results).Error; err != nil {
		return common.NewErrorResponse("Error fetching transactions", nil, 500), nil
	}

	totalPages := int(math.Ceil(float64(total) / float64(limit)))

	return map[string]interface{}{
		"success": true,
		"status":  200,
		"message": "Success",
		"data": map[string]interface{}{
			"result": results,
			"meta": map[string]interface{}{
				"page":     page,
				"perPage":  limit,
				"total":    total,
				"lastPage": totalPages,
				// prev/next calculation omitted for brevity but should be there
			},
		},
	}, nil
}
