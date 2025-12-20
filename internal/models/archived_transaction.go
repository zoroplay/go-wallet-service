package models

import (
	"time"
)

type ArchivedTransaction struct {
	ID            uint    `gorm:"primaryKey"`
	ClientId      int     `gorm:"index"`
	UserId        int     `gorm:"index"`
	Username      string  `gorm:"type:varchar(100);index"`
	TransactionNo string  `gorm:"type:varchar(100);index"`
	Amount        float64 `gorm:"type:decimal(20,2)"`
	TrxType       string  `gorm:"index;column:tranasaction_type"` // DB column name from TS entity
	Status        int     `gorm:"default:0"`
	Channel       string  `gorm:"type:varchar(50)"`
	Subject       string  `gorm:"type:varchar(100)"`
	Description   string  `gorm:"type:text"`
	Source        string  `gorm:"type:varchar(50)"`
	AvailableBalance float64 `gorm:"type:decimal(20,2);column:available_balance"`
	Wallet        string  `gorm:"default:Main"`
	SettlementId  *string
	CreatedAt     time.Time `gorm:"autoCreateTime"`
	UpdatedAt     time.Time `gorm:"autoUpdateTime"`
}

func (ArchivedTransaction) TableName() string {
	return "archived_transactions"
}
