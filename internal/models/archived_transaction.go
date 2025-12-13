package models

import (
	"time"
)

type ArchivedTransaction struct {
	ID            uint    `gorm:"primaryKey"`
	ClientId      int     `gorm:"index"`
	UserId        int     `gorm:"index"`
	Username      string  `gorm:"type:varchar(100);index"`
	TransactionNo string  `gorm:"type:varchar(100);uniqueIndex"`
	Amount        float64 `gorm:"type:decimal(20,2)"`
	TrxType       string  `gorm:"index;column:tranasaction_type"` // DB column name from TS entity
	Status        int     `gorm:"default:0"`
	Channel       string
	Subject       string
	Description   string
	Source        string
	Balance       float64 `gorm:"type:decimal(20,2)"`
	Wallet        string  `gorm:"default:Main"`
	SettlementId  *string
	CreatedAt     time.Time `gorm:"autoCreateTime"`
	UpdatedAt     time.Time `gorm:"autoUpdateTime"`
}

func (ArchivedTransaction) TableName() string {
	return "archived_transactions"
}
