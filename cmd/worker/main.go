package main

import (
	"log"
	"os"

	"github.com/hibiken/asynq"
	"github.com/joho/godotenv"

	"wallet-service/internal/consumers"
	"wallet-service/internal/database"
	"wallet-service/internal/services"
	"wallet-service/internal/worker"
)

func main() {
	// Load env
	if err := godotenv.Load("../../.env"); err != nil {
		log.Println("No .env file found in ../../.env, trying .env")
		if err := godotenv.Load(".env"); err != nil {
			log.Println("No .env file found, using system env")
		}
	}

	// Connect DB
	database.Connect()
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
	momoService := services.NewMomoService(db, helperService)
	opayService := services.NewOpayService(db, helperService)
	coralPayService := services.NewCoralPayService(db, helperService)
	globusService := services.NewGlobusService(db, helperService)
	palmPayService := services.NewPalmPayService(db, helperService)

	// Payment Service
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

	// Processor
	processor := consumers.NewPaymentProcessor(db, helperService, paymentService)

	// Redis
	redisAddr := os.Getenv("REDIS_URL")
	if redisAddr == "" {
		redisAddr = "localhost:6379"
	}
	redisOpt := asynq.RedisClientOpt{Addr: redisAddr}

	log.Println("Starting Asynq Worker...")
	worker.StartWorker(redisOpt, processor)
}
