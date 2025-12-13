package services

import "gorm.io/gorm"

type MgurushService struct {
	DB *gorm.DB
}

func NewMgurushService(db *gorm.DB) *MgurushService {
	return &MgurushService{
		DB: db,
	}
}
