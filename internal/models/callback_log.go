package models

import (
	"time"
)

type CallbackLog struct {
	ID            uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	ClientId      int       `gorm:"column:client_id;not null" json:"client_id"`
	Request       string    `gorm:"column:request;type:longtext" json:"request"`
	Response      string    `gorm:"column:response;type:longtext" json:"response"`
	Status        int       `gorm:"column:status;default:0" json:"status"`
	RequestType   string    `gorm:"column:request_type;size:255" json:"request_type"` // Maps to `type` in TS, renamed to avoid keyword conflict if preferred, or `Type`
	TransactionId string    `gorm:"column:transaction_id;size:255" json:"transaction_id"`
	PaymentMethod string    `gorm:"column:payment_method;size:255" json:"payment_method"`
	CreatedAt     time.Time `gorm:"column:created_at;autoCreateTime" json:"created_at"`
	UpdatedAt     time.Time `gorm:"column:updated_at;autoUpdateTime" json:"updated_at"`
}

func (CallbackLog) TableName() string {
	return "callback_logs"
}
