package main

import (
	"log"
	"os"

	"wallet-service/internal/database"
	grpcServer "wallet-service/internal/grpc"
	"wallet-service/internal/handlers"
	"wallet-service/internal/services"

	"github.com/gin-gonic/gin"
	"github.com/hibiken/asynq"
	"github.com/joho/godotenv"
)

func main() {
	// Load environment variables
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found in current directory, trying parent")
		if err := godotenv.Load("../.env"); err != nil {
			log.Println("No .env file found, using system environment variables")
		}
	}

	if mode := os.Getenv("GIN_MODE"); mode != "" {
		gin.SetMode(mode)
	}

	// Initialize Database
	database.Connect()
	database.Migrate()
	db := database.DB

	// Init Services
	helperService := services.NewHelperService(db)

	// Identity Client
	identityClient, err := services.NewIdentityClient()
	if err != nil {
		log.Fatalf("Failed to create identity client: %v", err)
	}

	// Bonus Client
	bonusClient, err := services.NewBonusClient()
	if err != nil {
		log.Fatalf("Failed to create bonus client: %v", err)
	}
	defer bonusClient.Close()

	// Providers
	paystackService := services.NewPaystackService(db, helperService, identityClient, bonusClient)
	flutterwaveService := services.NewFlutterwaveService(db, helperService, identityClient, bonusClient)
	monnifyService := services.NewMonnifyService(db, helperService)
	wayaQuickService := services.NewWayaQuickService(db, helperService)
	wayaBankService := services.NewWayaBankService(db)
	pitch90Service := services.NewPitch90SMSService(db, helperService)
	korapayService := services.NewKorapayService(db, helperService)
	tigoService := services.NewTigoService(db, helperService)
	providusService := services.NewProvidusService(db, helperService)
	smileAndPayService := services.NewSmileAndPayService(db, helperService)
	fidelityService := services.NewFidelityService(db, helperService)
	payonusService := services.NewPayonusService(db, helperService)
	pawapayService := services.NewPawapayService(db, helperService)

	// Business Logic Services
	// Initialize new services
	momoService := services.NewMomoService(db, helperService)
	opayService := services.NewOpayService(db, helperService)
	coralPayService := services.NewCoralPayService(db, helperService)
	globusService := services.NewGlobusService(db, helperService)
	palmPayService := services.NewPalmPayService(db, helperService)

	paymentService := services.NewPaymentService(
		db,
		helperService,
		identityClient,
		paystackService,
		flutterwaveService,
		monnifyService,
		wayaQuickService,
		wayaBankService,
		pitch90Service,
		korapayService,
		tigoService,
		providusService,
		smileAndPayService,
		fidelityService,
		payonusService,
		momoService,
		opayService,
		coralPayService,
		globusService,
		palmPayService,
		pawapayService,
	)

	walletService := services.NewWalletService(db, helperService)
	withdrawalService := services.NewWithdrawalService(db)

	// Redis/Asynq Client
	redisAddr := os.Getenv("REDIS_URL")
	if redisAddr == "" {
		redisAddr = "localhost:6379"
	}
	asynqClient := asynq.NewClient(asynq.RedisClientOpt{Addr: redisAddr})
	defer asynqClient.Close()

	depositService := services.NewDepositService(db, asynqClient)
	reportingService := services.NewReportingService(db)
	playerService := services.NewPlayerService(db, identityClient)
	commissionService := services.NewCommissionService(db, helperService, identityClient, playerService)
	dashboardService := services.NewDashboardService(db, identityClient)
	summaryService := services.NewSummaryService(db, identityClient)

	// Suppress unused variable error until injection
	_ = summaryService

	// Initialize Gin
	r := gin.Default()

	// Ping endpoint
	r.GET("/ping", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"message": "Welcome To Sbe Wallet service",
		})
	})

	// Wallet Routes - Deprecated? Or keep for REST fallback
	r.POST("/wallets", handlers.CreateWallet)
	r.GET("/wallets/balance", handlers.GetBalance)
	r.POST("/wallets/credit", handlers.CreditUser)
	r.POST("/wallets/debit", handlers.DebitUser)
	r.GET("/transactions", handlers.GetTransactions)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	grpcPort := os.Getenv("GRPC_PORT")
	if grpcPort == "" {
		grpcPort = "50051"
	}

	// Start gRPC server
	go grpcServer.StartGRPCServer(
		grpcPort,
		walletService,
		paymentService,
		withdrawalService,
		depositService,
		reportingService,
		commissionService,
		playerService,
		paystackService,
		flutterwaveService,
		monnifyService,
		wayaBankService,
		wayaQuickService,
		korapayService,
		pawapayService,
		tigoService,
		providusService,
		fidelityService,
		smileAndPayService,
		payonusService,
		dashboardService,
	)

	// Start Cron Schedulers
	paymentService.StartScheduler()

	transactionArchiveService := services.NewTransactionArchiveService(db)
	transactionArchiveService.StartScheduler()

	log.Printf("HTTP Server starting on port %s", port)
	if err := r.Run(":" + port); err != nil {
		log.Fatal("Failed to start server: ", err)
	}
}
