package services

import (
	"testing"
	"time"

	"wallet-service/internal/models"
	"wallet-service/pkg/common"
)

func TestRequestWithdrawal(t *testing.T) {
	if testDB == nil {
		t.Skip("Database not configured")
	}
	defer cleanup()

	svc := NewWithdrawalService(testDB, nil)

	// Create wallet
	testDB.Create(&models.Wallet{
		UserId:           401,
		ClientId:         1,
		Username:         "withdrawer",
		AvailableBalance: 500.0,
	})

	// 1. Success case
	req := WithdrawRequestDTO{
		UserId:   401,
		ClientId: 1,
		Amount:   200.0,
	}
	resInterface, err := svc.RequestWithdrawal(req)
	if err != nil {
		t.Fatalf("RequestWithdrawal failed: %v", err)
	}

	// Check if success response
	if res, ok := resInterface.(common.SuccessResponse); ok {
		if !res.Success {
			t.Errorf("Expected success")
		}
	} else {
		t.Errorf("Expected SuccessResponse, got %T", resInterface)
	}

	// 2. Insufficient funds
	req.Amount = 600.0
	resInterface, err = svc.RequestWithdrawal(req)
	// Expecting ErrorResponse struct returned as value, err might be nil
	if err != nil {
		// If err is returned (unexpected for validation logic we wrote), fail
		t.Fatalf("Unexpected error: %v", err)
	}
	if errRes, ok := resInterface.(common.ErrorResponse); ok {
		if errRes.Success {
			t.Errorf("Expected failure")
		}
	} else {
		t.Errorf("Expected ErrorResponse for insufficient funds, got %T", resInterface)
	}

	// 3. Min limit
	req.Amount = 50.0
	resInterface, _ = svc.RequestWithdrawal(req)
	if _, ok := resInterface.(common.ErrorResponse); !ok {
		t.Errorf("Expected ErrorResponse for min limit")
	}
}

func TestListWithdrawalRequest(t *testing.T) {
	if testDB == nil {
		t.Skip("Database not configured")
	}
	defer cleanup()
	svc := NewWithdrawalService(testDB, nil)

	// Seed withdrawals
	w1 := models.Withdrawal{
		UserId: 501, ClientId: 1, Amount: 100, Username: "u1", CreatedAt: time.Now(),
	}
	w2 := models.Withdrawal{
		UserId: 502, ClientId: 1, Amount: 200, Username: "u2", CreatedAt: time.Now(),
	}
	testDB.Create(&w1)
	testDB.Create(&w2)

	req := ListWithdrawalRequestsDTO{
		ClientId: 1,
		Limit:    10,
	}

	res, err := svc.ListWithdrawalRequest(req)
	if err != nil {
		t.Fatalf("ListWithdrawalRequest failed: %v", err)
	}

	// Data should be map with data and totalAmount
	dataMap, ok := res.Data.(map[string]interface{})
	if !ok {
		t.Fatalf("Response data has unexpected type")
	}

	list := dataMap["data"].([]models.Withdrawal)
	if len(list) != 2 {
		t.Errorf("Expected 2 items, got %d", len(list))
	}

	// Check total amount
	totalAmount, ok := dataMap["totalAmount"].(float64)
	if !ok || totalAmount != 300.0 {
		t.Errorf("Expected total 300, got %v", dataMap["totalAmount"])
	}
}

func TestValidateWithdrawalCode(t *testing.T) {
	if testDB == nil {
		t.Skip("Database not configured")
	}
	defer cleanup()
	svc := NewWithdrawalService(testDB)

	testDB.Create(&models.Withdrawal{
		UserId: 601, ClientId: 1, Amount: 100, Username: "codeusers", WithdrawalCode: "VALID123", Status: 0,
	})

	// Valid
	resInterface, err := svc.ValidateWithdrawalCode(ValidateWithdrawalCodeDTO{Code: "VALID123", ClientId: 1})
	if err != nil {
		t.Fatalf("Validate failed: %v", err)
	}
	if res, ok := resInterface.(common.SuccessResponse); ok {
		if !res.Success {
			t.Errorf("Expected success")
		}
	} else {
		t.Errorf("Expected SuccessResponse")
	}

	// Invalid
	resInterface, _ = svc.ValidateWithdrawalCode(ValidateWithdrawalCodeDTO{Code: "INVALID", ClientId: 1})
	if _, ok := resInterface.(common.ErrorResponse); !ok {
		t.Errorf("Expected ErrorResponse for invalid code")
	}
}
