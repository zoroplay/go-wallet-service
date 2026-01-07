package services

import (
	"encoding/json"
	"testing"

	"wallet-service/internal/models"
	"wallet-service/pkg/common"

	"github.com/stretchr/testify/assert"
)

func TestPaystackHandleWebhook_ChargeSuccess(t *testing.T) {
	if testDB == nil {
		t.Skip("Database not configured")
	}
	// No defer cleanup here, maybe add manual cleanup or rely on deferred cleanup in subtests if any
	// Adding manual cleanup at end of test is safer as defer cleanup() in TestMain might not run between tests.
	// But wallet_service_test cleanup deletes everything.

	// Create dedicated cleanup for this test
	defer func() {
		testDB.Exec("DELETE FROM transaction")
		testDB.Exec("DELETE FROM wallet")
		testDB.Exec("DELETE FROM payment_method")
	}()

	db := testDB
	helper := NewHelperService(db)
	svc := NewPaystackService(db, helper, nil, nil)

	// Setup Data
	userId := 999
	clientId := 1
	trxNo := "TEST_PAYSTACK_" + common.GenerateTrxNo()

	// Create User Wallet
	db.Create(&models.Wallet{
		UserId:           userId,
		ClientId:         clientId,
		Username:         "testuser_paystack",
		AvailableBalance: 100.0,
		Currency:         "NGN",
	})

	// Create Pending Transaction
	db.Create(&models.Transaction{
		ClientId:      clientId,
		UserId:        userId,
		Username:      "testuser_paystack",
		TransactionNo: trxNo,
		Amount:        50.0,
		TrxType:       "credit",
		Subject:       "Deposit",
		Status:        0, // Pending
	})

	// Create Payment Method Settings (Mock)
	db.Create(&models.PaymentMethod{
		ClientId:  clientId,
		Provider:  "paystack",
		SecretKey: "sk_test_123",
		BaseUrl:   "https://api.paystack.co",
	})

	// Webhook Payload
	webhookBody := map[string]interface{}{
		"event": "charge.success",
		"data": map[string]interface{}{
			"reference": trxNo,
			"amount":    5000, // Paystack sends in kobo usually, but logic assumes amount matches?
			// Wait, in `handleChargeSuccess`:
			// transaction = findOne(..., transaction_no: ref)
			// balance = wallet.Available + transaction.Amount
			// So it relies on the amount ALREADY in the transaction record, not the webhook body amount.
			// This matches the implementation in `paystack_service.go` which reads `transaction.Amount`.
		},
	}
	bodyBytes, _ := json.Marshal(webhookBody)

	// Call HandleWebhook
	// Note: We are skipping signature verification fail for test simplicity or we need to generate valid HMAC.
	// `HandleWebhook` logic: if signature invalid, just log? In my impl I wrote:
	// "if expectedHash != dto.PaystackKey { ... TS logs invalid signature but commented out check ... }"
	// So it won't fail if sig is wrong.

	dto := WebhookDTO{
		ClientId:    clientId,
		PaystackKey: "dummy_sig",
		Body:        bodyBytes,
		Event:       "charge.success",
		Reference:   trxNo,
		Data:        webhookBody,
	}

	_, err := svc.HandleWebhook(dto)
	assert.Nil(t, err)

	// Verify Wallet Updated
	var wallet models.Wallet
	db.Where("user_id = ?", userId).First(&wallet)
	assert.Equal(t, 150.0, wallet.AvailableBalance)

	// Verify Transaction Updated
	var trx models.Transaction
	db.Where("transaction_no = ?", trxNo).First(&trx)
	assert.Equal(t, 1, trx.Status)
	assert.Equal(t, 150.0, trx.Balance)

	// Cleanup
	db.Where("transaction_no = ?", trxNo).Delete(&models.Transaction{})
	db.Where("user_id = ?", userId).Delete(&models.Wallet{})
	db.Where("provider = ?", "paystack").Delete(&models.PaymentMethod{})
}
