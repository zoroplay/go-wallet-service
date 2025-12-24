package database

import (
	"fmt"
	"log"
	"os"
	"time"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

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

	// Configure a custom logger that ignores ErrRecordNotFound
	newLogger := logger.New(
		log.New(os.Stdout, "\r\n", log.LstdFlags), // io writer
		logger.Config{
			SlowThreshold:             time.Second,
			LogLevel:                  logger.Warn,
			IgnoreRecordNotFoundError: true,
			Colorful:                  false,
		},
	)

	// Configure GORM with optimized settings
	gormConfig := &gorm.Config{
		// Disable default transaction for single queries (faster)
		SkipDefaultTransaction: true,
		// Enable prepared statement cache (reuses prepared statements)
		PrepareStmt: true,
		// Set logger level based on environment
		Logger: newLogger,
	}

	DB, err = gorm.Open(mysql.Open(dsn), gormConfig)
	if err != nil {
		log.Fatal("Failed to connect to database: ", err)
	}

	// Configure connection pool
	sqlDB, err := DB.DB()
	if err != nil {
		log.Fatal("Failed to get DB instance: ", err)
	}

	// SetMaxIdleConns sets the maximum number of connections in the idle connection pool
	sqlDB.SetMaxIdleConns(10)

	// SetMaxOpenConns sets the maximum number of open connections to the database
	sqlDB.SetMaxOpenConns(100)

	// SetConnMaxLifetime sets the maximum amount of time a connection may be reused
	sqlDB.SetConnMaxLifetime(time.Hour)

	// SetConnMaxIdleTime sets the maximum amount of time a connection may be idle
	sqlDB.SetConnMaxIdleTime(10 * time.Minute)

	log.Println("Database connection established with connection pooling")
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
