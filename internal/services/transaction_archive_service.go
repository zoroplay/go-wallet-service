package services

import (
	"log"
	"time"

	"github.com/robfig/cron/v3"

	"wallet-service/internal/models"

	"gorm.io/gorm"
)

type TransactionArchiveService struct {
	DB *gorm.DB
}

func NewTransactionArchiveService(db *gorm.DB) *TransactionArchiveService {
	return &TransactionArchiveService{DB: db}
}

// ArchiveTransactions moves transactions older than 4 months to archive table.
// TS used 'subMonths(new Date(), 4)' then found transactions < date.
func (s *TransactionArchiveService) ArchiveTransactions() {
	log.Println("Starting transaction archive process...")

	threeMonthsAgo := time.Now().AddDate(0, -4, 0)

	// Find old transactions
	var oldTransactions []models.Transaction
	if err := s.DB.Where("created_at < ?", threeMonthsAgo).Find(&oldTransactions).Error; err != nil {
		log.Printf("Error finding old transactions: %v", err)
		return
	}

	if len(oldTransactions) == 0 {
		log.Println("No transactions to archive")
		return
	}

	log.Printf("Found %d transactions to archive", len(oldTransactions))

	// Convert to ArchivedTransaction
	var archivedData []models.ArchivedTransaction
	for _, tx := range oldTransactions {
		archived := models.ArchivedTransaction{
			// ID is auto increment in archive, preserve or let DB handle?
			// Usually archive just keeps data, ID might not need to match exactly or can match.
			// TS code does not map ID explicitly in map(), so new IDs generated or if type matches it might.
			// Let's assume new IDs or just data mapping.
			ClientId:      tx.ClientId,
			UserId:        tx.UserId,
			Username:      tx.Username,
			TransactionNo: tx.TransactionNo,
			Amount:        tx.Amount,
			TrxType:       tx.TrxType,
			Status:        tx.Status,
			Channel:       tx.Channel,
			Subject:       tx.Subject,
			Description:   tx.Description,
			Source:        tx.Source,
			Balance:       tx.Balance,
			Wallet:        tx.Wallet,
			SettlementId:  tx.SettlementId,
			CreatedAt:     tx.CreatedAt,
			UpdatedAt:     tx.UpdatedAt,
		}
		archivedData = append(archivedData, archived)
	}

	// Transaction for atomic move
	err := s.DB.Transaction(func(tx *gorm.DB) error {
		// Bulk insert to archive
		if err := tx.Create(&archivedData).Error; err != nil {
			return err
		}

		// Delete from original
		ids := make([]uint, len(oldTransactions))
		for i, t := range oldTransactions {
			ids[i] = uint(t.ID)
		}

		if err := tx.Delete(&models.Transaction{}, ids).Error; err != nil {
			return err
		}
		return nil
	})

	if err != nil {
		log.Printf("Error during transaction archiving: %v", err)
	} else {
		log.Printf("Archived and removed %d transactions.", len(oldTransactions))
	}
}

// StartScheduler initializes the cron job to run daily at midnight
func (s *TransactionArchiveService) StartScheduler() {
	c := cron.New()
	// Run daily at midnight: "0 0 * * *"
	_, err := c.AddFunc("0 0 * * *", func() {
		log.Println("Running scheduled transaction archive task...")
		s.ArchiveTransactions()
	})
	if err != nil {
		log.Printf("Error scheduling archive task: %v", err)
		return
	}
	c.Start()
	log.Println("Transaction Archive Scheduler started (Daily at 00:00)")
}
