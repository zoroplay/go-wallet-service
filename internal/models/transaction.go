package models

import (
	"time"
)

type Transaction struct {
	ID                     int       `gorm:"primaryKey;autoIncrement" json:"id"`
	ClientId               int       `gorm:"column:client_id;not null;index:idx_trx_user_client" json:"client_id"`
	UserId                 int       `gorm:"column:user_id;not null;index:idx_trx_user_client" json:"user_id"`
	Username               string    `gorm:"column:username;size:255;not null" json:"username"`
	TransactionNo          string    `gorm:"column:transaction_no;size:255;not null;index" json:"transaction_no"`
	Amount                 float64   `gorm:"column:amount;type:decimal(20,2);not null" json:"amount"`
	TrxType                string    `gorm:"column:tranasaction_type;size:50;not null" json:"tranasaction_type"` // Note: typo in TS entity preserved
	Subject                string    `gorm:"column:subject;size:255;not null" json:"subject"`
	Description            string    `gorm:"column:description;type:text" json:"description"`
	Source                 string    `gorm:"column:source;size:50" json:"source"`
	Channel                string    `gorm:"column:channel;size:50" json:"channel"`
	Balance                float64   `gorm:"column:balance;type:decimal(20,2);default:0.00" json:"balance"`
	Wallet                 string    `gorm:"column:wallet;size:50;default:main" json:"wallet"`
	Status                 int       `gorm:"column:status;default:0" json:"status"` // 0: pending, 1: success, 2: failed
	SettlementId           *string   `gorm:"column:settlementId;size:255" json:"settlementId"`
	AffiliateId            int       `gorm:"column:affiliateId;default:0" json:"affiliateId"`
	AffiliateTransactionNo *int      `gorm:"column:affiliate_transactionNo;default:0" json:"affiliate_transactionNo"`
	CreatedAt              time.Time `gorm:"column:created_at;autoCreateTime" json:"created_at"`
	UpdatedAt              time.Time `gorm:"column:updated_at;autoUpdateTime" json:"updated_at"`
}

func (Transaction) TableName() string {
	return "transactions"
}
