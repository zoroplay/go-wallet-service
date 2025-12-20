package services

import (
	"log"
	"math"
	"os"
	"testing"

	"wallet-service/internal/models"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// NOTE: These tests require a running MySQL instance.
// For this environment, we will write them to be ready for integration testing.
// In a real CI, we would spin up a container or use sqlite (if models are compatible).

var testDB *gorm.DB

func setup() {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		log.Println("Skipping DB tests: DATABASE_URL not set")
		return
	}

	var err error
	testDB, err = gorm.Open(mysql.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		log.Printf("Failed to connect to database: %v", err)
		return
	}

	// Migrate schemas
	testDB.AutoMigrate(&models.Wallet{}, &models.Transaction{}, &models.PaymentMethod{})
}

func cleanup() {
	if testDB != nil {
		testDB.Exec("DELETE FROM transaction")
		testDB.Exec("DELETE FROM wallet")
	}
}

func TestCreateWallet(t *testing.T) {
	if testDB == nil {
		t.Skip("Database not configured")
	}
	defer cleanup()

	helper := NewHelperService(testDB)
	svc := NewWalletService(testDB, helper)

	req := CreateWalletDTO{
		UserId:   101,
		Username: "testuser",
		ClientId: 1,
		Amount:   100.00,
		Bonus:    50.00,
	}

	res, err := svc.CreateWallet(req)
	if err != nil {
		t.Fatalf("CreateWallet failed: %v", err)
	}

	if !res.Success {
		t.Errorf("Expected success, got false")
	}

	wallet := res.Data.(models.Wallet)
	if wallet.AvailableBalance != 100.00 {
		t.Errorf("Expected balance 100, got %f", wallet.AvailableBalance)
	}
	if wallet.SportBonus != 50.00 {
		t.Errorf("Expected sport bonus 50, got %f", wallet.SportBonus)
	}
}

func TestCreditUser(t *testing.T) {
	if testDB == nil {
		t.Skip("Database not configured")
	}
	defer cleanup()

	helper := NewHelperService(testDB)
	svc := NewWalletService(testDB, helper)

	// Setup wallet
	svc.CreateWallet(CreateWalletDTO{
		UserId:   102,
		Username: "credituser",
		ClientId: 1,
		Amount:   100.00,
	})

	creditReq := CreditUserDTO{
		UserId:        102,
		ClientId:      1,
		Username:      "credituser",
		Amount:        50.00,
		Wallet:        "main",
		Description:   "Test Credit",
		Subject:       "Deposit",
		TransactionNo: "TRX123",
	}

	res, err := svc.CreditUser(creditReq)
	if err != nil {
		t.Fatalf("CreditUser failed: %v", err)
	}

	wallet := res.Data.(models.Wallet)
	// 100 + 50 = 150
	// NOTE: CreateWallet logic in our code sets AvailableBalance = Amount.
	// CreditUser logic updates `available_balance` += amount.
	if math.Abs(wallet.AvailableBalance-150.00) > 0.01 {
		t.Errorf("Expected AvailableBalance 150, got %f", wallet.AvailableBalance)
	}
}

func TestDebitUser(t *testing.T) {
	if testDB == nil {
		t.Skip("Database not configured")
	}
	defer cleanup()

	helper := NewHelperService(testDB)
	svc := NewWalletService(testDB, helper)

	svc.CreateWallet(CreateWalletDTO{
		UserId:   103,
		Username: "debituser",
		ClientId: 1,
		Amount:   100.00,
	})

	debitReq := DebitUserDTO{
		UserId:   103,
		ClientId: 1,
		Username: "debituser",
		Amount:   30.00,
		Wallet:   "main",
	}

	_, err := svc.DebitUser(debitReq)
	if err != nil {
		t.Fatalf("DebitUser failed: %v", err)
	}

	// Verify
	var wallet models.Wallet
	testDB.Where("user_id = ?", 103).First(&wallet)

	if math.Abs(wallet.AvailableBalance-70.00) > 0.01 {
		t.Errorf("Expected AvailableBalance 70, got %f", wallet.AvailableBalance)
	}
}

func TestClientUsersWalletBal(t *testing.T) {
	if testDB == nil {
		t.Skip("Database not configured")
	}
	defer cleanup()

	helper := NewHelperService(testDB)
	svc := NewWalletService(testDB, helper)

	// Create 2 users for client 1
	svc.CreateWallet(CreateWalletDTO{
		UserId:   201,
		Username: "user1",
		ClientId: 1,
		Amount:   10.00,
	})
	svc.CreateWallet(CreateWalletDTO{
		UserId:   202,
		Username: "user2",
		ClientId: 1,
		Amount:   20.00,
	})
	// Create user for client 2
	svc.CreateWallet(CreateWalletDTO{
		UserId:   203,
		Username: "user3",
		ClientId: 2,
		Amount:   30.00,
	})

	res, err := svc.ClientUsersWalletBal(ClientRequestDTO{ClientId: 1})
	if err != nil {
		t.Fatalf("ClientUsersWalletBal failed: %v", err)
	}

	wallets := res.Data.([]models.Wallet)
	if len(wallets) != 2 {
		t.Errorf("Expected 2 wallets, got %d", len(wallets))
	}
}

func TestAwardBonusWinning(t *testing.T) {
	if testDB == nil {
		t.Skip("Database not configured")
	}
	defer cleanup()

	helper := NewHelperService(testDB)
	svc := NewWalletService(testDB, helper)

	svc.CreateWallet(CreateWalletDTO{
		UserId:   301,
		Username: "bonususer",
		ClientId: 1,
		Amount:   100.00,
		Bonus:    50.00,
	})

	// Award Bonus Winning (Credits Main, Clears Bonus) (?)
	// Wait, TS logic: available_balance += amount, sport_bonus_balance = 0.
	// So if amount passed is e.g. 50 (winnings), it adds 50 to main, and clears sport bonus.

	req := CreditUserDTO{
		UserId:   301,
		ClientId: 1,
		Amount:   200.00, // Winnings
		Subject:  "Sport Win",
	}

	res, err := svc.AwardBonusWinning(req)
	if err != nil {
		t.Fatalf("AwardBonusWinning failed: %v", err)
	}

	wallet := res.Data.(models.Wallet)
	// 100 (initial) + 200 (winnings) = 300
	if wallet.AvailableBalance != 300.00 {
		t.Errorf("Expected AvailableBalance 300, got %f", wallet.AvailableBalance)
	}
	// Bonus cleared
	if wallet.SportBonus != 0 {
		t.Errorf("Expected SportBonus 0, got %f", wallet.SportBonus)
	}
}

func TestGetNetworkBalance(t *testing.T) {
	if testDB == nil {
		t.Skip("Database not configured")
	}
	defer cleanup()

	helper := NewHelperService(testDB)
	svc := NewWalletService(testDB, helper)

	// Create Agent
	svc.CreateWallet(CreateWalletDTO{
		UserId:   900,
		Username: "agent",
		ClientId: 1,
		Amount:   1000.00, // Agent balance
	})

	// Create Users
	svc.CreateWallet(CreateWalletDTO{
		UserId:   901,
		Username: "userA",
		ClientId: 1,
		Amount:   50.00,
	})
	svc.CreateWallet(CreateWalletDTO{
		UserId:   902,
		Username: "userB",
		ClientId: 1,
		Amount:   150.00,
	})

	req := GetNetworkBalanceDTO{
		AgentId: 900,
		UserIds: []string{"901", "902"},
	}

	res, err := svc.GetNetworkBalance(req)
	if err != nil {
		t.Fatalf("GetNetworkBalance failed: %v", err)
	}

	// Network Balance = Sum(Users) + Agent
	// (50 + 150) + 1000 = 1200

	netBal, ok := res["networkBalance"].(float64)
	if !ok || netBal != 1200.00 {
		t.Errorf("Expected NetworkBalance 1200, got %v", res["networkBalance"])
	}
}

func TestMain(m *testing.M) {
	setup()
	code := m.Run()
	// cleanup() // Optional
	os.Exit(code)
}
