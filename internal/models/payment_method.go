package models

import (
	"time"
)

type PaymentMethod struct {
	ID              int       `gorm:"primaryKey;autoIncrement" json:"id"`
	ClientId        int       `gorm:"column:client_id;not null" json:"client_id"`
	DisplayName     string    `gorm:"column:display_name;size:200;not null" json:"display_name"`
	Provider        string    `gorm:"column:provider;size:150;not null" json:"provider"`
	BaseUrl         string    `gorm:"column:base_url;size:150" json:"base_url"`
	SecretKey       string    `gorm:"column:secret_key;type:longtext" json:"secret_key"`
	PublicKey       string    `gorm:"column:public_key;type:longtext" json:"public_key"`
	MerchantId      string    `gorm:"column:merchant_id;size:150" json:"merchant_id"`
	LogoPath        string    `gorm:"column:logo_path;size:150" json:"logo_path"`
	Status          int       `gorm:"column:status;default:0" json:"status"`
	ForDisbursement int       `gorm:"column:for_disbursement;default:0" json:"for_disbursement"`
	CreatedAt       time.Time `gorm:"column:created_at;autoCreateTime" json:"created_at"`
	UpdatedAt       time.Time `gorm:"column:updated_at;autoUpdateTime" json:"updated_at"`
}

func (PaymentMethod) TableName() string {
	return "payment_methods"
}
