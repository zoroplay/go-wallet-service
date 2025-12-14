package database

import (
	"fmt"
	"log"
	"os"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"

	"wallet-service/internal/models"
)

var DB *gorm.DB

func Connect() {
	var err error
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		os.Getenv("DB_USER"),
		os.Getenv("DB_PASSWORD"),
		os.Getenv("DB_HOST"),
		os.Getenv("DB_PORT"),
		os.Getenv("DB_NAME"),
	)

	DB, err = gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatal("Failed to connect to database: ", err)
	}

	log.Println("Database connection established")
}

func Migrate() {
	err := DB.AutoMigrate(
		&models.Wallet{},
		&models.PaymentMethod{},
		&models.Transaction{},
		&models.ArchivedTransaction{},
		&models.Bank{},
		&models.CallbackLog{},
		&models.Withdrawal{},
		&models.WithdrawalAccount{},
	)
	if err != nil {
		log.Fatal("Failed to migrate database: ", err)
	}
	log.Println("Database migration completed")
}
