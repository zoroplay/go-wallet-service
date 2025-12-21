package models

import (
	"time"
)

type Withdrawal struct {
	ID             int       `gorm:"primaryKey;autoIncrement" json:"id"`
	ClientId       int       `gorm:"column:client_id;not null;index:idx_withdrawal_client" json:"client_id"`
	UserId         int       `gorm:"column:user_id;not null;index:idx_withdrawal_user" json:"user_id"`
	Username       string    `gorm:"column:username;size:50;not null" json:"username"`
	Amount         float64   `gorm:"column:amount;type:decimal(20,2);not null" json:"amount"`
	WithdrawalCode string    `gorm:"column:withdrawal_code;size:40" json:"withdrawal_code"`
	AccountNumber  string    `gorm:"column:account_number;size:255" json:"account_number"`
	AccountName    string    `gorm:"column:account_name;size:150" json:"account_name"`
	BankName       string    `gorm:"column:bank_name;size:150" json:"bank_name"`
	BankCode       string    `gorm:"column:bank_code;size:150" json:"bank_code"`
	Comment        string    `gorm:"column:comment;type:text" json:"comment"`
	UpdatedBy      string    `gorm:"column:updated_by;size:150" json:"updated_by"`
	Status         int       `gorm:"column:status;default:0" json:"status"` // 0: pending, 1: approved, 2: rejected
	AffiliateId    int       `gorm:"column:affiliateId;default:0" json:"affiliateId"`
	CreatedAt      time.Time `gorm:"column:created_at;autoCreateTime" json:"created_at"`
	UpdatedAt      time.Time `gorm:"column:updated_at;autoUpdateTime" json:"updated_at"`
}

func (Withdrawal) TableName() string {
	return "withdrawals"
}
