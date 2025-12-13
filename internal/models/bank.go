package models

import (
	"time"
)

type Bank struct {
	ID        int       `gorm:"primaryKey;autoIncrement" json:"id"`
	BankId    int       `gorm:"column:bank_id;not null" json:"bank_id"`
	Name      string    `gorm:"column:name;size:150;not null" json:"name"`
	Slug      string    `gorm:"column:slug;size:150;not null" json:"slug"`
	Code      string    `gorm:"column:code;size:20;not null" json:"code"`
	LongCode  string    `gorm:"column:long_code;size:20" json:"long_code"`
	Country   string    `gorm:"column:country;size:50;default:Nigeria" json:"country"`
	Currency  string    `gorm:"column:currency;size:20;default:NGN" json:"currency"`
	Type      string    `gorm:"column:type;size:20;default:nuban" json:"type"`
	Status    int       `gorm:"column:status;default:0" json:"status"`
	CreatedAt time.Time `gorm:"column:created_at;autoCreateTime" json:"created_at"`
	UpdatedAt time.Time `gorm:"column:updated_at;autoUpdateTime" json:"updated_at"`
}

func (Bank) TableName() string {
	return "banks"
}
