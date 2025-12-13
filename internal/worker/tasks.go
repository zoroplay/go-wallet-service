package worker

import (
	"encoding/json"

	"wallet-service/internal/consumers"

	"github.com/hibiken/asynq"
)

// Task Types
const (
	TypeShopDeposit                   = "shop-deposit"
	TypeCredit                        = "credit"
	TypeWithdrawalRequest             = "withdrawal-request"
	TypeShopWithdrawal                = "shop-withdrawal"
	TypeCommissionDeposit             = "commission-deposit"
	TypeCommissionDebit               = "commission-debit"
	TypeCommissionWithdrawal          = "commission-withdrawal"
	TypeCommissionReverse             = "commission-reverse"
	TypeAffiliateCommissionWithdrawal = "affiliate-commission-withdrawal"
	TypeCreditPlayer                  = "credit-player"
	TypeDebitUser                     = "debit-user" // Mapped to generic 'credit' in TS default? No, WithdrawalConsumer default is debitUser
	TypeMobileMoneyPayout             = "mobile-money-request"
	TypeSmileAndPayPayout             = "smileandpay-request"
)

// Task Creators

func NewShopDepositTask(payload consumers.ShopDepositDTO) (*asynq.Task, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	return asynq.NewTask(TypeShopDeposit, data), nil
}

func NewCreditTask(payload consumers.CreditDTO) (*asynq.Task, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	return asynq.NewTask(TypeCredit, data), nil
}

func NewWithdrawalRequestTask(payload consumers.WithdrawalJobDTO) (*asynq.Task, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	return asynq.NewTask(TypeWithdrawalRequest, data), nil
}

func NewShopWithdrawalTask(payload consumers.ShopWithdrawalDTO) (*asynq.Task, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	return asynq.NewTask(TypeShopWithdrawal, data), nil
}

func NewCommissionDepositTask(payload consumers.CommissionDTO) (*asynq.Task, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	return asynq.NewTask(TypeCommissionDeposit, data), nil
}

func NewCommissionDebitTask(payload consumers.CommissionDTO) (*asynq.Task, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	return asynq.NewTask(TypeCommissionDebit, data), nil
}

func NewCommissionWithdrawalTask(payload consumers.CommissionDTO) (*asynq.Task, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	return asynq.NewTask(TypeCommissionWithdrawal, data), nil
}

func NewCommissionReverseTask(payload consumers.CommissionDTO) (*asynq.Task, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	return asynq.NewTask(TypeCommissionReverse, data), nil
}

func NewAffiliateCommissionWithdrawalTask(payload consumers.AffiliateCommissionDTO) (*asynq.Task, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	return asynq.NewTask(TypeAffiliateCommissionWithdrawal, data), nil
}

func NewCreditPlayerTask(payload consumers.CreditPlayerDTO) (*asynq.Task, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	return asynq.NewTask(TypeCreditPlayer, data), nil
}

func NewDebitUserTask(payload consumers.CreditDTO) (*asynq.Task, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	return asynq.NewTask(TypeDebitUser, data), nil
}

func NewMobileMoneyPayoutTask(payload consumers.WithdrawalJobDTO) (*asynq.Task, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	return asynq.NewTask(TypeMobileMoneyPayout, data), nil
}

func NewSmileAndPayPayoutTask(payload consumers.WithdrawalJobDTO) (*asynq.Task, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	return asynq.NewTask(TypeSmileAndPayPayout, data), nil
}
