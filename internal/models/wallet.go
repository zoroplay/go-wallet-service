package models

import (
	"time"
)

type Wallet struct {
	ID                     int       `gorm:"primaryKey;autoIncrement" json:"id"`
	UserId                 int       `gorm:"column:user_id;not null;index:idx_wallet_user_client" json:"user_id"`
	Username               string    `gorm:"column:username;size:255;not null" json:"username"`
	ClientId               int       `gorm:"column:client_id;not null;index:idx_wallet_user_client" json:"client_id"`
	AvailableBalance       float64   `gorm:"column:available_balance;type:decimal(20,2);default:0.00" json:"available_balance"`
	TrustBalance           float64   `gorm:"column:trust_balance;type:decimal(20,2);default:0.00" json:"trust_balance"`
	SportBonus             float64   `gorm:"column:sport_bonus_balance;type:decimal(20,2);default:0.00" json:"sport_bonus_balance"`
	VirtualBonus           float64   `gorm:"column:virtual_bonus_balance;type:decimal(20,2);default:0.00" json:"virtual_bonus_balance"`
	CasinoBonus            float64   `gorm:"column:casino_bonus_balance;type:decimal(20,2);default:0.00" json:"casino_bonus_balance"`
	CommissionBalance      float64   `gorm:"column:commission_balance;type:decimal(20,2);default:0.00" json:"commission_balance"`
	VirtualAccountNo       string    `gorm:"column:virtual_account_no;size:50" json:"virtual_account_no"`
	VirtualBranchId        string    `gorm:"column:virtual_branch_id;size:50" json:"virtual_branch_id"`
	VirtualAccountName     string    `gorm:"column:virtual_account_name;size:255" json:"virtual_account_name"`
	VirtualBalance         float64   `gorm:"column:virtual_balance;type:decimal(20,2);default:0.00" json:"virtual_balance"`
	VirtualAccountDefault  bool      `gorm:"column:virtual_account_default;default:false" json:"virtual_account_default"`
	VirtualNubanAccountNo  string    `gorm:"column:virtual_nuban_account_no;size:50" json:"virtual_nuban_account_no"`
	VirtualAcctClosureFlag bool      `gorm:"column:virtual_acct_closure_flag;default:false" json:"virtual_acct_closure_flag"`
	VirtualAcctDeleteFlag  bool      `gorm:"column:virtual_acct_delete_flag;default:false" json:"virtual_acct_delete_flag"`
	Status                 int       `gorm:"column:status;default:0" json:"status"`
	Currency               string    `gorm:"column:currency;size:10;not null" json:"currency"`
	CreatedAt              time.Time `gorm:"column:created_at;autoCreateTime" json:"created_at"`
	UpdatedAt              time.Time `gorm:"column:updated_at;autoUpdateTime" json:"updated_at"`
}

func (Wallet) TableName() string {
	return "wallets"
}
