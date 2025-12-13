package services

import (
	"wallet-service/internal/models"
	"wallet-service/pkg/common"

	"gorm.io/gorm"
)

type WalletService struct {
	DB     *gorm.DB
	Helper *HelperService
}

func NewWalletService(db *gorm.DB, helper *HelperService) *WalletService {
	return &WalletService{DB: db, Helper: helper}
}

type CreateWalletDTO struct {
	UserId   int
	Username string
	ClientId int
	Amount   float64
	Bonus    float64
}

func (s *WalletService) CreateWallet(data CreateWalletDTO) (common.SuccessResponse, error) {
	wallet := models.Wallet{
		UserId:           data.UserId,
		Username:         data.Username,
		ClientId:         data.ClientId,
		Balance:          data.Amount, // Initial balance
		AvailableBalance: data.Amount,
		SportBonus:       data.Bonus,
		Currency:         "NGN", // Default
	}

	if err := s.DB.Create(&wallet).Error; err != nil {
		return common.SuccessResponse{}, err
	}

	// Transaction logic
	amt := data.Amount
	desc := "Initial Balance"
	subject := "Deposit"
	if data.Bonus > 0 {
		amt = data.Bonus
		desc = "Registration bonus"
		subject = "Bonus"
	}

	if data.Amount > 0 || data.Bonus > 0 {
		trxData := TransactionData{
			ClientId:        data.ClientId,
			TransactionNo:   common.GenerateTrxNo(),
			Amount:          amt,
			Description:     desc,
			Subject:         subject,
			Channel:         "Internal Transfer",
			FromUserId:      0,
			FromUsername:    "System",
			FromUserBalance: 0,
			ToUserId:        data.UserId,
			ToUsername:      data.Username,
			ToUserBalance:   0, // Should be updated balance? TS sends 0 here.
			Status:          1,
		}
		s.Helper.SaveTransaction(trxData)
	}

	return common.NewSuccessResponse(wallet, "Wallet created"), nil
}

type GetBalanceDTO struct {
	UserId   int
	ClientId int
}

func (s *WalletService) GetBalance(data GetBalanceDTO) (common.SuccessResponse, error) {
	var wallet models.Wallet
	if err := s.DB.Where("user_id = ? AND client_id = ?", data.UserId, data.ClientId).First(&wallet).Error; err != nil {
		return common.SuccessResponse{}, err
	}
	// Return fields similar to TS
	return common.NewSuccessResponse(wallet, "Wallet fetched"), nil
}

type CreditUserDTO struct {
	UserId        int
	ClientId      int
	Username      string
	Amount        float64
	Wallet        string
	Subject       string
	Description   string
	Channel       string
	Source        string
	TransactionNo string
}

func (s *WalletService) CreditUser(data CreditUserDTO) (common.SuccessResponse, error) {
	walletBalanceCol := "available_balance"
	walletType := "Main"

	switch data.Wallet {
	case "sport-bonus":
		walletBalanceCol = "sport_bonus_balance"
		walletType = "Sport Bonus"
	case "virtual":
		walletBalanceCol = "virtual_bonus_balance"
		walletType = "Virtual Bonus"
	case "casino":
		walletBalanceCol = "casino_bonus_balance"
		walletType = "Casino Bonus"
	case "trust":
		walletBalanceCol = "trust_balance"
		walletType = "Trust"
	}

	// Atomic update
	tx := s.DB.Model(&models.Wallet{}).
		Where("user_id = ? AND client_id = ?", data.UserId, data.ClientId).
		UpdateColumn(walletBalanceCol, gorm.Expr(walletBalanceCol+" + ?", data.Amount))

	if tx.Error != nil {
		return common.SuccessResponse{}, tx.Error
	}

	// Fetch updated wallet
	var wallet models.Wallet
	s.DB.Where("user_id = ?", data.UserId).First(&wallet)

	// Determine new balance for logging
	var newBalance float64
	switch data.Wallet {
	case "sport-bonus":
		newBalance = wallet.SportBonus
	case "virtual":
		newBalance = wallet.VirtualBonus
	case "casino":
		newBalance = wallet.CasinoBonus
	case "trust":
		newBalance = wallet.TrustBalance
	default:
		// Logic update for main wallet: In TS it updates 'available_balance',
		// but `Wallet` struct has `Balance` and `AvailableBalance`.
		// Go model maps `Balance` -> `balance` column, `AvailableBalance` -> `available_balance` column.
		// TS updates `available_balance`.
		// However, typical wallet logic might require updating BOTH if they are separate.
		// TS `app.service.ts` line 400 updates `[walletBalance]` which defaults to `available_balance`.
		// It does NOT update `balance` column explicitly in that query.
		// But later at line 474 `wallet.balance = balance`.

		// For now, I will follow TS strictly: Update mapped column.
		newBalance = wallet.AvailableBalance

		// If data.Wallet is default, we might also want to update `balance` column if business logic dictates so?
		// TS code only updates one column in `createQueryBuilder`.
	}

	// Save transaction
	trxData := TransactionData{
		ClientId:        data.ClientId,
		TransactionNo:   common.GenerateTrxNo(),
		Amount:          data.Amount,
		Description:     data.Description,
		Subject:         data.Subject,
		Channel:         data.Channel,
		Source:          data.Source,
		FromUserId:      0,
		FromUsername:    "System",
		FromUserBalance: 0,
		ToUserId:        data.UserId,
		ToUsername:      data.Username,
		ToUserBalance:   newBalance,
		Status:          1,
		WalletType:      walletType,
	}
	s.Helper.SaveTransaction(trxData)

	// Access Identity Service and Trackier - STUBBED
	// s.Helper.SendActivity(...)

	// Update DB object Balance field for response (TS does this manually at end)
	// wallet.Balance = newBalance // This is just for local object return

	// TS returns `wallet` object.
	// I'll return the wallet struct.

	return common.NewSuccessResponse(wallet, "Wallet credited"), nil
}

type DebitUserDTO struct {
	UserId      int
	ClientId    int
	Username    string
	Amount      float64
	Wallet      string
	Subject     string
	Description string
	Channel     string
	Source      string
}

func (s *WalletService) DebitUser(data DebitUserDTO) (interface{}, error) {
	var wallet models.Wallet
	if err := s.DB.Where("user_id = ?", data.UserId).First(&wallet).Error; err != nil {
		return nil, err
	}

	walletBalanceCol := "available_balance"
	walletType := "Main"
	var currentBalance float64

	switch data.Wallet {
	case "sport-bonus":
		currentBalance = wallet.SportBonus
		walletBalanceCol = "sport_bonus_balance"
		walletType = "Sport Bonus"
	case "virtual":
		currentBalance = wallet.VirtualBonus
		walletBalanceCol = "virtual_bonus_balance"
		walletType = "Virtual Bonus"
	case "casino":
		currentBalance = wallet.CasinoBonus
		walletBalanceCol = "casino_bonus_balance"
		walletType = "Casino Bonus"
	case "trust":
		currentBalance = wallet.TrustBalance
		walletBalanceCol = "trust_balance"
		walletType = "Trust"
	default:
		currentBalance = wallet.AvailableBalance
		walletBalanceCol = "available_balance"
	}

	if currentBalance < data.Amount {
		return common.NewErrorResponse("Insufficent balance", nil, 400), nil
	}

	// Atomic update
	tx := s.DB.Model(&models.Wallet{}).
		Where("user_id = ? AND client_id = ?", data.UserId, data.ClientId).
		UpdateColumn(walletBalanceCol, gorm.Expr(walletBalanceCol+" - ?", data.Amount))

	if tx.Error != nil {
		return nil, tx.Error
	}

	// Fetch updated
	s.DB.Where("user_id = ?", data.UserId).First(&wallet)

	var newBalance float64
	switch data.Wallet {
	case "sport-bonus":
		newBalance = wallet.SportBonus
	case "virtual":
		newBalance = wallet.VirtualBonus
	case "casino":
		newBalance = wallet.CasinoBonus
	case "trust":
		newBalance = wallet.TrustBalance
	default:
		newBalance = wallet.AvailableBalance
	}

	// Save Transaction
	trxData := TransactionData{
		ClientId:        data.ClientId,
		TransactionNo:   common.GenerateTrxNo(),
		Amount:          data.Amount,
		Description:     data.Description,
		Subject:         data.Subject,
		Channel:         data.Channel,
		Source:          data.Source,
		FromUserId:      data.UserId,
		FromUsername:    data.Username,
		FromUserBalance: newBalance, // TS uses the new balance for FromUserBalance? Yes. `fromUserBalance: balance` (line 598)
		ToUserId:        0,
		ToUsername:      "System",
		ToUserBalance:   0,
		Status:          1,
		WalletType:      walletType,
	}
	s.Helper.SaveTransaction(trxData)

	// Trackier stub
	// s.Helper.SendActivity(...)

	return common.NewSuccessResponse(wallet, "Wallet debited"), nil
}

type ListDepositsDTO struct {
	ClientId      int
	StartDate     string
	EndDate       string
	PaymentMethod string
	Status        *int
	Username      string
	TransactionId string
	Bank          string
	Page          int
}

func (s *WalletService) ListDeposits(data ListDepositsDTO) (common.PaginationResult, error) {
	limit := 100
	page := data.Page
	if page < 1 {
		page = 1
	}

	offset := (page - 1) * limit

	query := s.DB.Model(&models.Transaction{}).
		Where("client_id = ?", data.ClientId).
		Where("user_id != 0").
		Where("subject = ?", "Deposit").
		Where("created_at >= ?", data.StartDate).
		Where("created_at <= ?", data.EndDate)

	if data.PaymentMethod != "" {
		query = query.Where("channel = ?", data.PaymentMethod)
	}
	if data.Username != "" {
		query = query.Where("username = ?", data.Username)
	}
	if data.TransactionId != "" {
		query = query.Where("transaction_no = ?", data.TransactionId)
	}
	if data.Status != nil {
		query = query.Where("status = ?", *data.Status)
	}
	if data.Bank != "" {
		query = query.Where("channel = ?", data.Bank)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return common.PaginationResult{}, err
	}

	var transactions []models.Transaction
	if err := query.Order("created_at DESC").Limit(limit).Offset(offset).Find(&transactions).Error; err != nil {
		return common.PaginationResult{}, err
	}

	// In Go, we just return the structs. TS formatted dates strings, but we return JSON with ISO dates usually.

	// Pass to PaginateResponse
	// We need to pass []Transaction, but PaginateResponse takes interface{}.
	// However, TS `paginateResponse` func expects `[results, count]` as first arg.
	// My Go `PaginateResponse` expects `data interface{}, total int64`.
	return common.PaginateResponse(transactions, total, page, limit, "Deposits fetched"), nil
}

type PaymentMethodDTO struct {
	ID              int
	ClientId        int
	Title           string
	Provider        string
	SecretKey       string
	PublicKey       string
	MerchantId      string
	BaseUrl         string
	Status          int
	ForDisbursement int
}

func (s *WalletService) SavePaymentMethod(data PaymentMethodDTO) (common.SuccessResponse, error) {
	var pm models.PaymentMethod

	if data.ID != 0 {
		if err := s.DB.First(&pm, data.ID).Error; err != nil {
			// If not found, maybe new? But ID provided. TS logic: if data.id findOne.
			// If not found, it would error or return empty? TS doesn't check 'if found'.
			// I'll assume if ID is sent, it exists, or upsert.
			// But for strict port:
		}
	}

	pm.ClientId = data.ClientId
	pm.DisplayName = data.Title
	pm.Provider = data.Provider
	pm.BaseUrl = data.BaseUrl
	pm.SecretKey = data.SecretKey
	pm.PublicKey = data.PublicKey
	pm.MerchantId = data.MerchantId
	pm.Status = data.Status
	pm.ForDisbursement = data.ForDisbursement
	// pm.ID is auto handled if 0, or preserved if updating

	if err := s.DB.Save(&pm).Error; err != nil {
		return common.SuccessResponse{}, err // Or handle error
	}

	return common.NewSuccessResponse(pm, "Saved"), nil
}

func (s *WalletService) GetPaymentMethods(clientId int, status *int) (common.SuccessResponse, error) {
	var methods []models.PaymentMethod
	query := s.DB.Where("client_id = ?", clientId)
	if status != nil {
		query = query.Where("status = ?", *status)
	}

	if err := query.Find(&methods).Error; err != nil {
		return common.SuccessResponse{}, err
	}

	return common.NewSuccessResponse(methods, "Payment methods retrieved successfully"), nil
}

type ClientRequestDTO struct {
	ClientId int
}

func (s *WalletService) ClientUsersWalletBal(data ClientRequestDTO) (common.SuccessResponse, error) {
	var wallets []models.Wallet
	if err := s.DB.Where("client_id = ?", data.ClientId).Find(&wallets).Error; err != nil {
		return common.SuccessResponse{}, err
	}

	return common.NewSuccessResponse(wallets, "Wallets fetched"), nil
}

// UpdatePaymentMethod updates a payment method
func (s *WalletService) UpdatePaymentMethod(data PaymentMethodDTO) (common.SuccessResponse, error) {
	return s.SavePaymentMethod(data)
}

// DeletePaymentMethod deletes a payment method
func (s *WalletService) DeletePaymentMethod(id, clientId int) (common.SuccessResponse, error) {
	if err := s.DB.Where("id = ? AND client_id = ?", id, clientId).Delete(&models.PaymentMethod{}).Error; err != nil {
		return common.SuccessResponse{}, err
	}
	return common.NewSuccessResponse(nil, "Deleted successfully"), nil
}

// AwardBonusWinning
func (s *WalletService) AwardBonusWinning(data CreditUserDTO) (common.SuccessResponse, error) {
	// Atomic update
	tx := s.DB.Model(&models.Wallet{}).
		Where("user_id = ? AND client_id = ?", data.UserId, data.ClientId).
		Updates(map[string]interface{}{
			"available_balance":   gorm.Expr("available_balance + ?", data.Amount),
			"sport_bonus_balance": 0,
		})

	if tx.Error != nil {
		return common.SuccessResponse{}, tx.Error
	}

	// Fetch updated wallet
	var wallet models.Wallet
	s.DB.Where("user_id = ?", data.UserId).First(&wallet)

	// Transaction
	trxData := TransactionData{
		ClientId:        data.ClientId,
		TransactionNo:   common.GenerateTrxNo(),
		Amount:          data.Amount,
		Description:     data.Description,
		Subject:         data.Subject,
		Channel:         data.Channel,
		Source:          data.Source,
		FromUserId:      0,
		FromUsername:    "System",
		FromUserBalance: 0,
		ToUserId:        data.UserId,
		ToUsername:      data.Username,
		ToUserBalance:   wallet.AvailableBalance,
		Status:          1,
		WalletType:      "Main",
	}
	s.Helper.SaveTransaction(trxData)

	return common.NewSuccessResponse(wallet, "Bonus awarded"), nil
}

// DebitAgentBalance
func (s *WalletService) DebitAgentBalance(data DebitUserDTO) (common.SuccessResponse, error) {
	// Logic: balance -= amount.
	tx := s.DB.Model(&models.Wallet{}).
		Where("user_id = ? AND client_id = ?", data.UserId, data.ClientId).
		UpdateColumn("balance", gorm.Expr("balance - ?", data.Amount))

	if tx.Error != nil {
		return common.SuccessResponse{}, tx.Error
	}

	var wallet models.Wallet
	s.DB.Where("user_id = ?", data.UserId).First(&wallet)

	return common.NewSuccessResponse(wallet, "Agent balance debited"), nil
}

func (s *WalletService) ListBanks() (common.SuccessResponse, error) {
	var banks []models.Bank
	if err := s.DB.Find(&banks).Error; err != nil {
		return common.SuccessResponse{}, err
	}
	return common.NewSuccessResponse(banks, "Banks retrieved"), nil
}

type UserTransactionDTO struct {
	ClientId  int
	UserId    int
	StartDate string
	EndDate   string
	Page      int
	Limit     int
}

func (s *WalletService) GetUserTransactions(data UserTransactionDTO) (common.PaginationResult, error) {
	limit := data.Limit
	if limit <= 0 {
		limit = 50
	}
	page := data.Page
	if page < 1 {
		page = 1
	}
	offset := (page - 1) * limit

	query := s.DB.Model(&models.Transaction{}).
		Where("client_id = ? AND user_id = ?", data.ClientId, data.UserId)

	if data.StartDate != "" {
		query = query.Where("DATE(created_at) >= ?", data.StartDate)
	}
	if data.EndDate != "" {
		query = query.Where("DATE(created_at) <= ?", data.EndDate)
	}

	var total int64
	query.Count(&total)

	var transactions []models.Transaction
	query.Order("created_at DESC").Limit(limit).Offset(offset).Find(&transactions)

	return common.PaginateResponse(transactions, total, page, limit, "Successful"), nil
}

type WalletSummaryDTO struct {
	ClientId int
	UserId   int
}

func (s *WalletService) GetWalletSummary(data WalletSummaryDTO) (interface{}, error) {
	var wallet models.Wallet
	if err := s.DB.Where("user_id = ? AND client_id = ?", data.UserId, data.ClientId).First(&wallet).Error; err != nil {
		return nil, nil // Return null as error? TS returns null.
	}

	// Sum deposits
	var totalDeposits float64
	s.DB.Model(&models.Transaction{}).
		Where("user_id = ? AND subject = ? AND status = ?", data.UserId, "Deposit", 1).
		Select("COALESCE(SUM(amount), 0)").Scan(&totalDeposits)

	// Sum withdrawals
	var totalWithdrawals float64
	s.DB.Model(&models.Withdrawal{}).
		Where("user_id = ? AND status = ?", data.UserId, 1). // 1 = approved
		Select("COALESCE(SUM(amount), 0)").Scan(&totalWithdrawals)

	// Pending withdrawals
	var pendingWithdrawals float64
	s.DB.Model(&models.Withdrawal{}).
		Where("user_id = ? AND status = ?", data.UserId, 0).
		Select("COALESCE(SUM(amount), 0)").Scan(&pendingWithdrawals)

	// Last Deposit
	var lastDeposit models.Transaction
	s.DB.Where("user_id = ? AND subject = ? AND status = ?", data.UserId, "Deposit", 1).
		Order("created_at DESC").First(&lastDeposit)

	// Last Withdrawal
	var lastWithdrawal models.Withdrawal
	s.DB.Where("user_id = ? AND status = ?", data.UserId, 1).
		Order("created_at DESC").First(&lastWithdrawal)

	// First Activity
	var firstActivity models.Transaction
	s.DB.Where("user_id = ? AND status = ?", data.UserId, 1).
		Order("created_at ASC").First(&firstActivity)

	// Last Activity
	var lastActivity models.Transaction
	s.DB.Where("user_id = ? AND status = ?", data.UserId, 1).
		Order("created_at DESC").First(&lastActivity)

	// Avg withdrawals
	var avgWithdrawals float64
	s.DB.Model(&models.Transaction{}).
		Where("user_id = ? AND status = ?", data.UserId, 1).
		Select("COALESCE(AVG(amount), 0)").Scan(&avgWithdrawals)

	// Count deposits
	var noOfDeposits int64
	s.DB.Model(&models.Transaction{}).
		Where("user_id = ? AND subject = ? AND status = ?", data.UserId, "Deposit", 1).
		Count(&noOfDeposits)

	// Count withdrawals
	var noOfWithdrawals int64
	s.DB.Model(&models.Withdrawal{}).
		Where("user_id = ? AND status = ?", data.UserId, 1).
		Count(&noOfWithdrawals)

	return map[string]interface{}{
		"noOfDeposits":         noOfDeposits,
		"noOfWithdrawals":      noOfWithdrawals,
		"totalDeposits":        totalDeposits,
		"totalWithdrawals":     totalWithdrawals,
		"pendingWithdrawals":   pendingWithdrawals,
		"avgWithdrawals":       avgWithdrawals,
		"sportBalance":         wallet.AvailableBalance,
		"sportBonusBalance":    wallet.SportBonus,
		"lastDepositDate":      lastDeposit.CreatedAt,
		"lastDepositAmount":    lastDeposit.Amount,
		"lastWithdrawalDate":   lastWithdrawal.CreatedAt,
		"lastWithdrawalAmount": lastWithdrawal.Amount,
		"firstActivityDate":    firstActivity.CreatedAt,
		"lastActivityDate":     lastActivity.CreatedAt,
	}, nil
}

type GetNetworkBalanceDTO struct {
	AgentId int
	UserIds []string // Comma separated in TS, but we can take slice
}

func (s *WalletService) GetNetworkBalance(data GetNetworkBalanceDTO) (map[string]interface{}, error) {
	var agentWallet models.Wallet
	if err := s.DB.Where("user_id = ?", data.AgentId).First(&agentWallet).Error; err != nil {
		// TS returns default structure on error
		return map[string]interface{}{
			"success":             true,
			"networkBalance":      0,
			"networkTrustBalance": 0,
			"trustBalance":        0,
			"availableBalance":    0,
			"balance":             0,
		}, nil
	}

	type Result struct {
		NetBal   float64
		TrustBal float64
	}
	var res Result

	// Sum for users
	if len(data.UserIds) > 0 {
		s.DB.Model(&models.Wallet{}).
			Where("user_id IN ?", data.UserIds).
			Select("SUM(available_balance) as net_bal, SUM(trust_balance) as trust_bal").
			Scan(&res)
	}

	return map[string]interface{}{
		"success":             true,
		"message":             "Success",
		"networkBalance":      res.NetBal + agentWallet.AvailableBalance,
		"networkTrustBalance": res.TrustBal + agentWallet.TrustBalance,
		"trustBalance":        agentWallet.TrustBalance,
		"availableBalance":    agentWallet.AvailableBalance,
		"balance":             agentWallet.Balance,
		"commissionBalance":   agentWallet.CommissionBalance,
	}, nil
}

func (s *WalletService) DeletePlayerData(userId int) (common.SuccessResponse, error) {
	// Transaction for safety
	err := s.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("user_id = ?", userId).Delete(&models.Transaction{}).Error; err != nil {
			return err
		}
		if err := tx.Where("user_id = ?", userId).Delete(&models.Wallet{}).Error; err != nil {
			return err
		}
		if err := tx.Where("user_id = ?", userId).Delete(&models.Withdrawal{}).Error; err != nil {
			return err
		}
		return nil
	})

	if err != nil {
		return common.SuccessResponse{}, err
	}

	return common.NewSuccessResponse(nil, "Successful"), nil
}
