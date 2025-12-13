package consumers

import (
	"fmt"
	"log"

	"wallet-service/internal/models"
	"wallet-service/internal/services" // Import services to use Helper and PaymentService
	"wallet-service/pkg/common"

	"gorm.io/gorm"
)

type PaymentProcessor struct {
	DB             *gorm.DB
	Helper         *services.HelperService
	PaymentService *services.PaymentService
}

func NewPaymentProcessor(db *gorm.DB, helper *services.HelperService, paymentService *services.PaymentService) *PaymentProcessor {
	return &PaymentProcessor{
		DB:             db,
		Helper:         helper,
		PaymentService: paymentService,
	}
}

// --- DTOs ---

type ShopDepositDTO struct {
	FromUserId      int
	ToUserId        int
	ClientId        int
	Amount          float64
	TransactionCode string
	FromUsername    string
	UserRole        string
}

type CreditDTO struct {
	ClientId         int
	UserId           int
	Username         string
	Amount           float64
	Balance          float64
	AvailableBalance float64 // Used in DebitCommission
	TransactionNo    string
	Description      string
	Subject          string
	Channel          string
	Source           string
	WalletType       string
	Wallet           string
}

type WithdrawalJobDTO struct {
	ClientId               int
	UserId                 int
	Username               string
	Amount                 float64
	Balance                float64
	TransactionNo          string
	WithdrawalCode         string
	AccountName            string
	AccountNumber          string
	BankCode               string
	BankName               string
	Type                   string
	Source                 string
	AutoDisbursement       AutoDisbursementDTO
	Operator               string
	AffiliateTransactionNo int
}

type AutoDisbursementDTO struct {
	AutoDisbursement      int
	AutoDisbursementCount int64
	AutoDisbursementMin   float64
	AutoDisbursementMax   float64
}

type ShopWithdrawalDTO struct {
	ID               int
	UserId           int
	ClientId         int
	Username         string
	Amount           float64
	Balance          float64
	WithdrawalCharge float64
	WithdrawalCode   string
}

type CommissionDTO struct {
	ClientId         int
	UserId           int
	Username         string
	Amount           float64
	Balance          float64
	AvailableBalance float64
	TransactionNo    string
	Description      string
	Subject          string
	Channel          string
	Source           string
}

type CreditPlayerDTO struct {
	ClientId            int
	UserId              int
	PlayerUsername      string
	Amount              float64
	AgentBalance        float64
	UserBalance         float64
	AgentId             int
	AgentUsername       string
	TransactionNo       string
	PlayerTransactionNo string
	AgentSubject        string
	UserSubject         string
}

type AffiliateCommissionDTO struct {
	ClientId         int
	UserId           int
	Username         string
	Amount           float64
	Balance          float64
	WithdrawalCode   string
	AffiliateId      int
	AccountName      string
	BankCode         string
	BankName         string
	AccountNumber    string
	Type             string
	Source           string
	AutoDisbursement AutoDisbursementDTO
}

// --- Methods ---

// Payout

func (p *PaymentProcessor) ProcessShopDeposit(data ShopDepositDTO) {
	log.Printf("Processing Shop Deposit: %v", data)
	var fromWallet models.Wallet
	if err := p.DB.Where("user_id = ? AND client_id = ?", data.FromUserId, data.ClientId).First(&fromWallet).Error; err != nil {
		return
	}
	var toWallet models.Wallet
	if err := p.DB.Where("user_id = ? AND client_id = ?", data.ToUserId, data.ClientId).First(&toWallet).Error; err != nil {
		return
	}

	senderBalance := fromWallet.AvailableBalance - data.Amount
	receiverBalance := toWallet.AvailableBalance + data.Amount

	p.DB.Transaction(func(tx *gorm.DB) error {
		tx.Model(&fromWallet).Update("available_balance", senderBalance)
		tx.Model(&toWallet).Update("available_balance", receiverBalance)
		tx.Model(&models.Transaction{}).Where("transaction_no = ?", data.TransactionCode).Update("status", 1)

		tx.Model(&models.Transaction{}).Where("transaction_no = ? AND tranasaction_type = ?", data.TransactionCode, "debit").
			Updates(map[string]interface{}{"user_id": data.FromUserId, "username": data.FromUsername, "balance": senderBalance})

		tx.Model(&models.Transaction{}).Where("transaction_no = ? AND tranasaction_type = ?", data.TransactionCode, "credit").
			Updates(map[string]interface{}{"balance": receiverBalance})
		return nil
	})
}

func (p *PaymentProcessor) ProcessCredit(data CreditDTO) {
	log.Printf("Processing Credit: %v", data)
	p.Helper.SaveTransaction(services.TransactionData{
		ClientId:      data.ClientId,
		TransactionNo: data.TransactionNo,
		Amount:        data.Amount,
		Description:   data.Description,
		Subject:       data.Subject,
		Channel:       data.Channel,
		Source:        data.Source,
		ToUserId:      data.UserId,
		ToUsername:    data.Username,
		ToUserBalance: data.Balance,
		Status:        1,
		WalletType:    data.WalletType,
	})

	updateField := data.Wallet
	if updateField == "" {
		updateField = "available_balance"
	}
	p.DB.Model(&models.Wallet{}).Where("user_id = ? AND client_id = ?", data.UserId, data.ClientId).
		Update(updateField, data.Balance)
}

func (p *PaymentProcessor) ProcessWithdrawal(data WithdrawalJobDTO) {
	log.Printf("Processing Withdrawal: %v", data)
	withdrawal := models.Withdrawal{
		AccountName:    data.AccountName,
		BankCode:       data.BankCode,
		BankName:       data.BankName,
		AccountNumber:  data.AccountNumber,
		UserId:         data.UserId,
		Username:       data.Username,
		ClientId:       data.ClientId,
		Amount:         data.Amount,
		WithdrawalCode: data.WithdrawalCode,
		Status:         0,
	}
	p.DB.Create(&withdrawal)

	balance := data.Balance - data.Amount
	p.DB.Model(&models.Wallet{}).Where("user_id = ? AND client_id = ?", data.UserId, data.ClientId).
		Update("available_balance", balance)

	p.Helper.SaveTransaction(services.TransactionData{
		ClientId:               data.ClientId,
		TransactionNo:          data.WithdrawalCode,
		Amount:                 data.Amount,
		Description:            "withdrawal request",
		Subject:                "Withdrawal",
		Channel:                data.Type,
		Source:                 data.Source,
		FromUserId:             data.UserId,
		FromUsername:           data.Username,
		FromUserBalance:        balance,
		Status:                 0,
		AffiliateTransactionNo: data.AffiliateTransactionNo,
	})

	if data.AutoDisbursement.AutoDisbursement == 1 && data.Type != "cash" {
		count, err := p.PaymentService.CheckNoOfWithdrawals(data.UserId)
		if err == nil {
			if count <= data.AutoDisbursement.AutoDisbursementCount &&
				data.Amount >= data.AutoDisbursement.AutoDisbursementMin &&
				data.Amount <= data.AutoDisbursement.AutoDisbursementMax {
				// Stub auto disbursement
				p.PaymentService.UpdateWithdrawalStatus(services.UpdateWithdrawalDTO{
					ClientId:     data.ClientId,
					Status:       "approve",
					WithdrawalId: int(withdrawal.ID),
					Comment:      "automated withdrawal",
					UpdatedBy:    "System",
				})
			}
		}
	}
}

func (p *PaymentProcessor) ProcessCreditCommission(data CommissionDTO) {
	log.Printf("Processing Credit Commission: %v", data)
	newBalance := data.Balance + data.Amount // data.Balance assumed old balance
	p.Helper.SaveTransaction(services.TransactionData{
		Amount:        data.Amount,
		Channel:       "Commission",
		ClientId:      data.ClientId,
		ToUserId:      data.UserId,
		ToUsername:    data.Username,
		ToUserBalance: newBalance,
		Status:        1,
		Source:        "system",
		Subject:       "Commission",
		Description:   data.Description,
		TransactionNo: data.TransactionNo,
	})
	p.DB.Model(&models.Wallet{}).Where("user_id = ? AND client_id = ?", data.UserId, data.ClientId).
		Update("commission_balance", newBalance)
}

func (p *PaymentProcessor) ProcessDebitCommission(data CommissionDTO) {
	log.Printf("Processing Debit Commission: %v", data)
	p.Helper.SaveTransaction(services.TransactionData{
		Amount:        data.Amount,
		Channel:       "Commission",
		ClientId:      data.ClientId,
		ToUserId:      data.UserId,
		ToUsername:    data.Username,
		ToUserBalance: data.AvailableBalance,
		Status:        1,
		Source:        "system",
		Subject:       "Commission",
		Description:   data.Description,
		TransactionNo: data.TransactionNo,
	})
	newBalance := data.Balance - data.Amount
	p.DB.Model(&models.Wallet{}).Where("user_id = ? AND client_id = ?", data.UserId, data.ClientId).
		Update("commission_balance", newBalance)
}

func (p *PaymentProcessor) ProcessCreditPlayer(data CreditPlayerDTO) {
	log.Printf("Processing Credit Player: %v", data)
	deductAmount := data.AgentBalance - data.Amount
	increaseAmount := data.UserBalance + data.Amount

	p.Helper.SaveTransaction(services.TransactionData{
		Amount:          data.Amount,
		Channel:         "Agent Transfer",
		ClientId:        data.ClientId,
		ToUserId:        data.UserId,
		ToUsername:      data.PlayerUsername,
		ToUserBalance:   increaseAmount,
		FromUserId:      data.AgentId,
		FromUsername:    data.AgentUsername,
		FromUserBalance: deductAmount,
		Status:          1,
		Source:          "agent",
		Subject:         data.AgentSubject,
		Description:     fmt.Sprintf("Agent %s funded %s", data.AgentUsername, data.PlayerUsername),
		TransactionNo:   data.TransactionNo,
	})

	p.Helper.SaveTransaction(services.TransactionData{
		Amount:          data.Amount,
		Channel:         "Player Deposit",
		ClientId:        data.ClientId,
		ToUserId:        data.UserId,
		ToUsername:      data.PlayerUsername,
		ToUserBalance:   increaseAmount,
		FromUserId:      data.AgentId,
		FromUsername:    data.AgentUsername,
		FromUserBalance: deductAmount,
		Status:          1,
		Source:          "system",
		Subject:         data.UserSubject,
		Description:     fmt.Sprintf("Funded by Agent %s", data.AgentUsername),
		TransactionNo:   data.PlayerTransactionNo,
	})

	p.DB.Model(&models.Wallet{}).Where("user_id = ?", data.AgentId).Update("available_balance", deductAmount)
	p.DB.Model(&models.Wallet{}).Where("user_id = ?", data.UserId).Update("available_balance", increaseAmount)
}

func (p *PaymentProcessor) DebitUser(data CreditDTO) {
	log.Printf("Processing Debit User: %v", data)
	updateField := data.Wallet
	if updateField == "" {
		updateField = "available_balance"
	}
	p.DB.Model(&models.Wallet{}).Where("user_id = ? AND client_id = ?", data.UserId, data.ClientId).
		Update(updateField, data.Balance)

	p.Helper.SaveTransaction(services.TransactionData{
		ClientId:        data.ClientId,
		TransactionNo:   common.GenerateTrxNo(),
		Amount:          data.Amount,
		Description:     data.Description,
		Subject:         data.Subject,
		Channel:         data.Channel,
		Source:          data.Source,
		FromUserId:      data.UserId,
		FromUsername:    data.Username,
		FromUserBalance: data.Balance,
		Status:          1,
		WalletType:      data.WalletType,
	})
}

func (p *PaymentProcessor) ProcessMobileMoneyPayout(data WithdrawalJobDTO) {
	p.processPayoutCommon(data)
}

func (p *PaymentProcessor) ProcessSmileAndPayPayout(data WithdrawalJobDTO) {
	p.processPayoutCommon(data)
}

func (p *PaymentProcessor) processPayoutCommon(data WithdrawalJobDTO) {
	log.Printf("Processing Payout: %v", data)
	balance := data.Balance - data.Amount

	p.DB.Model(&models.Wallet{}).Where("user_id = ? AND client_id = ?", data.UserId, data.ClientId).
		Update("available_balance", balance)

	withdrawal := models.Withdrawal{
		AccountName:    data.Username,
		BankCode:       data.BankCode,
		BankName:       data.Operator,
		AccountNumber:  data.Username,
		UserId:         data.UserId,
		Username:       data.Username,
		ClientId:       data.ClientId,
		Amount:         data.Amount,
		WithdrawalCode: data.WithdrawalCode,
		Status:         0,
	}
	p.DB.Create(&withdrawal)

	p.Helper.SaveTransaction(services.TransactionData{
		ClientId:        data.ClientId,
		TransactionNo:   withdrawal.WithdrawalCode,
		Amount:          data.Amount,
		Description:     "withdrawal request",
		Subject:         "Withdrawal",
		Channel:         data.Type,
		Source:          data.Source,
		FromUserId:      data.UserId,
		FromUsername:    data.Username,
		FromUserBalance: balance,
		Status:          0,
	})

	if data.AutoDisbursement.AutoDisbursement == 1 && data.Type != "cash" {
		count, _ := p.PaymentService.CheckNoOfWithdrawals(data.UserId)
		if count <= data.AutoDisbursement.AutoDisbursementCount &&
			data.Amount >= data.AutoDisbursement.AutoDisbursementMin &&
			data.Amount <= data.AutoDisbursement.AutoDisbursementMax {

			p.PaymentService.UpdateWithdrawalStatus(services.UpdateWithdrawalDTO{
				ClientId:     data.ClientId,
				Status:       "approve",
				WithdrawalId: int(withdrawal.ID),
				Comment:      "automated withdrawal",
				UpdatedBy:    "System",
			})
		}
	}
}

func (p *PaymentProcessor) ProcessShopWithdrawal(data ShopWithdrawalDTO) {
	log.Printf("Processing Shop Withdrawal: %v", data)
	p.DB.Model(&models.Withdrawal{}).Where("id = ?", data.ID).Updates(map[string]interface{}{
		"status":     1,
		"updated_by": data.Username,
	})

	var shopWallet models.Wallet
	if err := p.DB.Where("user_id = ? AND client_id = ?", data.UserId, data.ClientId).First(&shopWallet).Error; err != nil {
		return
	}

	updatedBalance := data.Balance + data.Amount
	p.DB.Model(&shopWallet).Update("available_balance", updatedBalance)

	var withdrawal models.Withdrawal
	p.DB.Where("id = ?", data.ID).First(&withdrawal)

	p.DB.Model(&models.Transaction{}).Where("transaction_no = ? AND tranasaction_type = ?", withdrawal.WithdrawalCode, "credit").
		Updates(map[string]interface{}{
			"description": "Withdrawal Payout",
			"subject":     "Withdrawal",
			"user_id":     data.UserId,
			"username":    data.Username,
			"balance":     updatedBalance,
			"status":      1,
		})
	p.DB.Model(&models.Transaction{}).Where("transaction_no = ?", withdrawal.WithdrawalCode).Update("status", 1)

	if data.WithdrawalCharge > 0 {
		updatedBalance += data.WithdrawalCharge
		p.DB.Model(&shopWallet).Update("available_balance", updatedBalance)
		p.Helper.SaveTransaction(services.TransactionData{
			ClientId:      data.ClientId,
			TransactionNo: common.GenerateTrxNo(),
			Amount:        data.Amount,
			Description:   "Commission on withdrawal payout",
			Subject:       "Withdrawal Comm.",
			Channel:       "sbengine",
			Source:        "shop",
			ToUserId:      data.UserId,
			ToUsername:    data.Username,
			ToUserBalance: updatedBalance,
			Status:        1,
		})
	}
}

func (p *PaymentProcessor) ProcessCommission(data CommissionDTO) {
	log.Printf("Processing Commission Withdrawal: %v", data)
	balance := data.Balance - data.Amount
	p.DB.Model(&models.Wallet{}).Where("user_id = ? AND client_id = ?", data.UserId, data.ClientId).
		Update("commission_balance", balance)

	withdrawal := models.Withdrawal{
		AccountName:    data.Username,
		BankCode:       "000",
		BankName:       "System",
		AccountNumber:  data.Username,
		UserId:         data.UserId,
		Username:       data.Username,
		ClientId:       data.ClientId,
		Amount:         data.Amount,
		WithdrawalCode: data.TransactionNo,
		Status:         1,
	}
	p.DB.Create(&withdrawal)

	p.Helper.SaveTransaction(services.TransactionData{
		ClientId:        data.ClientId,
		TransactionNo:   withdrawal.WithdrawalCode,
		Amount:          data.Amount,
		Description:     "Commission Withdrawal",
		Subject:         "Withdrawal",
		Channel:         "Internal",
		Source:          "admin",
		FromUserId:      data.UserId,
		FromUsername:    data.Username,
		FromUserBalance: balance,
		Status:          1,
	})
}

func (p *PaymentProcessor) ProcessReverseCommission(data CommissionDTO) {
	log.Printf("Processing Reverse Commission: %v", data)
	balance := data.Balance - data.Amount
	p.DB.Model(&models.Wallet{}).Where("user_id = ? AND client_id = ?", data.UserId, data.ClientId).
		Update("commission_balance", balance)

	withdrawal := models.Withdrawal{
		AccountName:    data.Username,
		AccountNumber:  data.Username,
		UserId:         data.UserId,
		Username:       data.Username,
		ClientId:       data.ClientId,
		Amount:         data.Amount,
		WithdrawalCode: data.TransactionNo,
		Status:         1,
	}
	p.DB.Create(&withdrawal)

	p.Helper.SaveTransaction(services.TransactionData{
		ClientId:        data.ClientId,
		TransactionNo:   data.TransactionNo,
		Amount:          data.Amount,
		Description:     "Commission Reversal",
		Subject:         "Withdrawal",
		Channel:         "Internal",
		Source:          "admin",
		FromUserId:      data.UserId,
		FromUsername:    data.Username,
		FromUserBalance: balance,
		Status:          1,
	})
}

func (p *PaymentProcessor) ProcessAffiliateCommission(data AffiliateCommissionDTO) {
	log.Printf("Processing Affiliate Commission: %v", data)
	withdrawal := models.Withdrawal{
		AccountName:    data.AccountName,
		BankCode:       data.BankCode,
		BankName:       data.BankName,
		AccountNumber:  data.AccountNumber,
		UserId:         data.UserId,
		Username:       data.Username,
		ClientId:       data.ClientId,
		Amount:         data.Amount,
		WithdrawalCode: data.WithdrawalCode,
		AffiliateId:    data.AffiliateId,
	}
	p.DB.Create(&withdrawal)

	balance := data.Balance - data.Amount
	p.DB.Model(&models.Wallet{}).Where("user_id = ? AND client_id = ?", data.UserId, data.ClientId).
		Update("commission_balance", balance)

	p.Helper.SaveTransaction(services.TransactionData{
		ClientId:        data.ClientId,
		TransactionNo:   data.WithdrawalCode,
		Amount:          data.Amount,
		Description:     fmt.Sprintf("Affiliate commission withdrawal request of amount %f by user %d", data.Amount, data.UserId),
		Subject:         "Affiliate Commission Withdrawal Request",
		Channel:         data.Type,
		Source:          data.Source,
		FromUserId:      data.UserId,
		FromUsername:    data.Username,
		FromUserBalance: balance,
		Status:          0,
		AffiliateId:     data.AffiliateId,
	})

	if data.AutoDisbursement.AutoDisbursement == 1 && data.Type != "cash" {
		count, _ := p.PaymentService.CheckNoOfWithdrawals(data.UserId)
		if count <= data.AutoDisbursement.AutoDisbursementCount &&
			data.Amount >= data.AutoDisbursement.AutoDisbursementMin &&
			data.Amount <= data.AutoDisbursement.AutoDisbursementMax {

			p.PaymentService.ApproveAndRejectCommissionRequest(services.CommissionApprovalDTO{
				ClientId:      data.ClientId,
				Status:        1,
				TransactionNo: data.WithdrawalCode,
				UserId:        data.UserId,
			})
		}
	}
}
