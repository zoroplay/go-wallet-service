package models

import (
	"time"
)

type WithdrawalAccount struct {
	ID                int       `gorm:"primaryKey;autoIncrement" json:"id"`
	ClientId          int       `gorm:"column:client_id;not null" json:"client_id"`
	UserId            int       `gorm:"column:user_id;not null" json:"user_id"`
	BankId            *int      `gorm:"column:bank_id" json:"bank_id"`
	BankCode          string    `gorm:"column:bank_code;size:20" json:"bank_code"`
	AccountNumber     string    `gorm:"column:account_number;size:150;not null" json:"account_number"`
	AccountName       string    `gorm:"column:account_name;size:250" json:"account_name"`
	RecipientCode     string    `gorm:"column:recipient_code;size:150" json:"recipient_code"`
	AuthorizationCode string    `gorm:"column:authorization_code;size:150" json:"authorization_code"`
	Status            int       `gorm:"column:status;default:1" json:"status"`
	CreatedAt         time.Time `gorm:"column:created_at;autoCreateTime" json:"created_at"`
	UpdatedAt         time.Time `gorm:"column:updated_at;autoUpdateTime" json:"updated_at"`
}

func (WithdrawalAccount) TableName() string {
	return "withdrawal_accounts"
}
