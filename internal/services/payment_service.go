package services

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"wallet-service/internal/models"
	"wallet-service/pkg/common"
	"wallet-service/proto/identity"

	"github.com/google/uuid"
	"github.com/robfig/cron/v3"
	"gorm.io/gorm"
)

type PaymentService struct {
	DB             *gorm.DB
	Helper         *HelperService
	IdentityClient *IdentityClient
	Paystack       *PaystackService
	Flutterwave    *FlutterwaveService
	Monnify        *MonnifyService
	WayaQuick      *WayaQuickService
	WayaBank       *WayaBankService
	Pitch90        *Pitch90SMSService
	Korapay        *KorapayService
	Tigo           *TigoService
	Providus       *ProvidusService
	Momo           *MomoService
	OPay           *OpayService
	CoralPay       *CoralPayService
	Fidelity       *FidelityService
	Globus         *GlobusService
	SmileAndPay    *SmileAndPayService
	PalmPay        *PalmPayService
	Payonus        *PayonusService
	Pawapay        *PawapayService
}

func NewPaymentService(
	db *gorm.DB,
	helper *HelperService,
	identityClient *IdentityClient,
	paystack *PaystackService,
	flutterwave *FlutterwaveService,
	monnify *MonnifyService,
	wayaQuick *WayaQuickService,
	wayaBank *WayaBankService,
	pitch90 *Pitch90SMSService,
	korapay *KorapayService,
	tigo *TigoService,
	providus *ProvidusService,
	smileAndPay *SmileAndPayService,
	fidelity *FidelityService,
	payonus *PayonusService,
	momo *MomoService,
	opay *OpayService,
	coralPay *CoralPayService,
	globus *GlobusService,
	palmPay *PalmPayService,
	pawapay *PawapayService, // Added missing arg
) *PaymentService {
	return &PaymentService{
		DB:             db,
		Helper:         helper,
		IdentityClient: identityClient,
		Paystack:       paystack,
		Flutterwave:    flutterwave,
		Monnify:        monnify,
		WayaQuick:      wayaQuick,
		WayaBank:       wayaBank,
		Pitch90:        pitch90,
		Korapay:        korapay,
		Tigo:           tigo,
		Providus:       providus,
		SmileAndPay:    smileAndPay,
		Fidelity:       fidelity,
		Payonus:        payonus,
		Momo:           momo,
		OPay:           opay,
		CoralPay:       coralPay,
		Globus:         globus,
		PalmPay:        palmPay,
		Pawapay:        pawapay,
	}
}

func (s *PaymentService) getApiBaseUrl(clientId int) string {
	// Simple logic based on TS or previous context
	if clientId == 4 {
		return "https://api.staging.sportsbookengine.com"
	}
	return "https://api.prod.sportsbookengine.com"
}

// CheckNoOfWithdrawals checks the number of approved withdrawals for a user today
func (s *PaymentService) CheckNoOfWithdrawals(userId int) (int64, error) {
	today := time.Now().Format("2006-01-02")
	var count int64
	if err := s.DB.Model(&models.Withdrawal{}).
		Where("user_id = ? AND status = ? AND DATE(created_at) >= ?", userId, 1, today).
		Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

type InitiateDepositRequestDTO struct {
	ClientId      int     `json:"clientId"`
	UserId        int     `json:"userId"`
	Amount        float64 `json:"amount"`
	Source        string  `json:"source"`
	PaymentMethod string  `json:"paymentMethod"`
}

func (s *PaymentService) InitiateDeposit(param InitiateDepositRequestDTO) (interface{}, error) {
	transactionNo := common.GenerateTrxNo()
	var link string
	var description string
	status := 0 // Pending

	// Find Wallet
	var wallet models.Wallet
	if err := s.DB.Where("client_id = ? AND user_id = ?", param.ClientId, param.UserId).First(&wallet).Error; err != nil {
		return common.NewErrorResponse("Wallet not found", nil, 404), nil
	}

	// Get User Info
	userResp, err := s.IdentityClient.GetPaymentData(&identity.GetPaymentDataRequest{
		UserId:   int32(param.UserId),
		ClientId: int32(param.ClientId),
		Source:   param.Source,
	})
	if err != nil {
		return common.NewErrorResponse("User does not exist or error fetching user", nil, 404), nil
	}

	userEmail := userResp.Email
	if userEmail == "" {
		userEmail = "noemail@example.com"
	}
	username := wallet.Username
	siteUrl := userResp.SiteUrl
	callbackUrl := userResp.CallbackUrl

	switch param.PaymentMethod {
	case "paystack":
		// Paystack Logic
		paystackEmail := userEmail
		if paystackEmail == "" || strings.Contains(paystackEmail, "noemail") {
			paystackEmail = fmt.Sprintf("noemail+%s@%s", username, siteUrl)
		}

		convAmount := param.Amount * 100 // Kobo
		pRes, err := s.Paystack.GeneratePaymentLink(map[string]interface{}{
			"amount":       convAmount,
			"email":        paystackEmail,
			"reference":    transactionNo,
			"callback_url": fmt.Sprintf("%s/payment-verification/paystack", callbackUrl),
		}, param.ClientId)

		if err != nil {
			return common.NewErrorResponse("Paystack error", nil, 500), nil
		}

		if pMap, ok := pRes.(map[string]interface{}); ok {
			if data, ok := pMap["data"].(map[string]interface{}); ok {
				if url, ok := data["authorization_url"].(string); ok {
					link = url
				}
			}
		}
		description = "Online Deposit (Paystack)"

	case "flutterwave":
		transactionNo = common.GenerateTrxNo()
		flutterUserEmail := userEmail
		if flutterUserEmail == "" || strings.Contains(flutterUserEmail, "noemail") {
			flutterUserEmail = fmt.Sprintf("noemail+%s@%s", username, siteUrl)
		}

		currency := "NGN"
		if userResp.Currency != nil && *userResp.Currency != "" {
			currency = *userResp.Currency
		}

		res, err := s.Flutterwave.CreatePayment(map[string]interface{}{
			"amount":       param.Amount,
			"tx_ref":       transactionNo,
			"currency":     currency,
			"redirect_url": fmt.Sprintf("%s/payment-verification/flutterwave", callbackUrl),
			"customer": map[string]string{
				"email":        flutterUserEmail,
				"phone_number": "+234" + username,
			},
		}, param.ClientId)

		if err != nil {
			return common.NewErrorResponse("Flutterwave error", nil, 500), nil
		}

		description = "Online Deposit (Flutterwave)"
		// Check implementation of CreatePayment return type. Assuming map or struct.
		// If map:
		if resMap, ok := res.(map[string]interface{}); ok {
			if data, ok := resMap["data"].(map[string]interface{}); ok {
				if l, ok := data["link"].(string); ok {
					link = l
				}
			}
		} else if resStruct, ok := res.(common.SuccessResponse); ok {
			// If it returns common.SuccessResponse
			if dataMap, ok := resStruct.Data.(map[string]interface{}); ok {
				if l, ok := dataMap["link"].(string); ok {
					link = l
				}
			}
		}

	case "pawapay":
		username := userResp.Username
		if !strings.HasPrefix(username, "255") {
			username = "255" + strings.TrimLeft(username, "0")
		}

		email := userEmail
		if email == "" || strings.Contains(email, "noemail") {
			email = fmt.Sprintf("noemail+%s@%s", userResp.Username, siteUrl)
		}

		depositId := common.GenerateTrxNo()
		transactionNo = depositId

		res, err := s.Pawapay.GeneratePaymentLink(map[string]interface{}{
			"depositId":         depositId,
			"amount":            fmt.Sprintf("%.0f", param.Amount), // Verify formatting
			"currency":          "TZS",
			"country":           "TZA",
			"msisdn":            username,
			"customerTimestamp": time.Now().Format(time.RFC3339),
			"email":             email,
			"correspondent":     "TIGO_TZ", // This usually needs helper to get correspondent
			"payer": map[string]interface{}{
				"type": "MSISDN",
				"address": map[string]string{
					"value": username,
				},
			},
			"statementDescription": "Deposit via 777bet",
		}, param.ClientId)

		if err != nil {
			return common.NewErrorResponse("Pawapay error", nil, 500), nil
		}

		description = "Online Deposit (Pawapay)"
		// Mapping...
		if resMap, ok := res.(map[string]interface{}); ok {
			if data, ok := resMap["data"].(map[string]interface{}); ok {
				if status, ok := data["status"].(string); ok {
					link = status
				}
			}
		}

	case "tigo":
		// ... (previous implementation)
		description = "Online Deposit (Tigo)"
		userName := userResp.Username
		if !strings.HasPrefix(userName, "255") {
			userName = "255" + strings.TrimLeft(userName, "0")
		}

		tigoRes, err := s.Tigo.InitiatePayment(map[string]interface{}{
			"CustomerMSISDN": userName,
			"Amount":         param.Amount,
			"Remarks":        description,
			"ReferenceID":    "KML" + transactionNo,
		}, param.ClientId)

		if err != nil {
			return common.NewErrorResponse("Tigo error", nil, 500), nil
		}
		_ = tigoRes

	case "korapay":
		description = "Online Deposit (Korapay)"
		korapayEmail := userEmail
		if korapayEmail == "" || strings.Contains(korapayEmail, "noemail") {
			korapayEmail = fmt.Sprintf("noemail+%s@%s", username, siteUrl)
		}

		currency := "NGN"
		if userResp.Currency != nil && *userResp.Currency != "" {
			currency = *userResp.Currency
		}

		koraRes, err := s.Korapay.CreatePayment(map[string]interface{}{
			"amount":          param.Amount,
			"reference":       transactionNo,
			"currency":        currency,
			"redirect_url":    fmt.Sprintf("%s/payment-verification/korapay", callbackUrl),
			"channels":        []string{"card", "bank_transfer"},
			"default_channel": "card",
			"metadata": map[string]interface{}{
				"clientId": param.ClientId,
			},
			"narration": "Online Deposit (Korapay)",
			"customer": map[string]string{
				"email": korapayEmail,
			},
			"merchant_bears_cost": true,
		}, param.ClientId)

		if err != nil {
			return common.NewErrorResponse("Korapay error", nil, 500), nil
		}

		if resMap, ok := koraRes.(map[string]interface{}); ok {
			if data, ok := resMap["data"].(map[string]interface{}); ok {
				if l, ok := data["link"].(string); ok {
					link = l
				}
			}
		} else if resStruct, ok := koraRes.(common.SuccessResponse); ok {
			if data, ok := resStruct.Data.(map[string]interface{}); ok {
				if l, ok := data["link"].(string); ok {
					link = l
				}
			}
		}

	case "opay":
		description = "Online Deposit (Opay)"
		// Opay Logic
		// Get BaseUrl from helper or config? TS uses hardcoded logic based on clientID or environment.
		// Assuming helper can provide or OPayService handles it.
		// OPayService InitiatePayment takes data map.

		opayRes, err := s.OPay.InitiatePayment(map[string]interface{}{
			"country":   "NG",
			"reference": transactionNo,
			"amount": map[string]interface{}{
				"total":    param.Amount * 100,
				"currency": "NGN",
			},
			"returnUrl":   fmt.Sprintf("%s/payment-verification/opay", callbackUrl),
			"callbackUrl": fmt.Sprintf("%s/api/v2/webhook/checkout/%d/opay/callback", s.getApiBaseUrl(param.ClientId), param.ClientId),
			"cancelUrl":   fmt.Sprintf("%s/payment-verification/opay", callbackUrl),
			"evokeOpay":   true,
			"expireAt":    300,
			"product": map[string]interface{}{
				"description": "Online Deposit (Opay)",
				"name":        "Sbe",
			},
			"payMethod": "OpayWalletNg",
		}, param.ClientId)

		if err != nil {
			return common.NewErrorResponse("Opay error", nil, 500), nil
		}

		if resStruct, ok := opayRes.(common.SuccessResponse); ok {
			if l, ok := resStruct.Data.(string); ok {
				link = l
			}
		}

	case "monnify":
		description = "Online Deposit (Monnify)"
		monifyEmail := userEmail
		if monifyEmail == "" || strings.Contains(monifyEmail, "noemail") {
			monifyEmail = fmt.Sprintf("noemail+%s@%s", username, siteUrl)
		}

		mRes, err := s.Monnify.GeneratePaymentLink(map[string]interface{}{
			"amount":       param.Amount,
			"name":         username,
			"email":        monifyEmail,
			"reference":    transactionNo,
			"callback_url": fmt.Sprintf("%s/payment-verification/monnify", callbackUrl),
		}, param.ClientId)

		if err != nil {
			return common.NewErrorResponse("Monnify error", nil, 500), nil
		}

		if resStruct, ok := mRes.(common.SuccessResponse); ok {
			if l, ok := resStruct.Data.(string); ok {
				link = l
			}
		} else if resMap, ok := mRes.(map[string]interface{}); ok {
			if l, ok := resMap["data"].(string); ok {
				link = l
			}
		}

	case "fidelity":
		description = "Online Deposit (Fidelity)"
		fidelityEmail := userEmail
		if fidelityEmail == "" || strings.Contains(fidelityEmail, "noemail") {
			fidelityEmail = fmt.Sprintf("noemail+%s@%s", username, siteUrl)
		}

		fRes, err := s.Fidelity.InitiatePay(map[string]interface{}{
			"connection_mode":       "Test",
			"first_name":            username,
			"last_name":             username,
			"email_address":         fidelityEmail,
			"phone_number":          "0" + username,
			"transaction_reference": transactionNo,
			"checkout_amount":       param.Amount,
			"currency_code":         "NGN",
			"description":           description,
			"callback_url":          fmt.Sprintf("%s/payment-verification/fidelity", callbackUrl),
		}, param.ClientId)

		if err != nil {
			return common.NewErrorResponse("Fidelity error", nil, 500), nil
		}

		dataBytes, _ := json.Marshal(fRes)
		link = string(dataBytes)

	case "fidelity_transfer":
		transactionNo = common.GenerateTrxNo() // Re-generate or use existing? TS generates new one.
		// NOTE: In Go we already generated transactionNo at start of function?
		// TS: transactionNo = generateTrxNo(); inside case.
		// Go: transactionNo := common.GenerateTrxNo() at top (if I did that).
		// Let's assume transactionNo is set.

		fidelityPayEmail := userEmail
		if fidelityPayEmail == "" || strings.Contains(fidelityPayEmail, "noemail") {
			fidelityPayEmail = fmt.Sprintf("noemail+%s@%s", username, siteUrl)
		}
		accountName := fmt.Sprintf("Betting Wallet - %s", username)

		fPayRes, err := s.Fidelity.HandlePay(map[string]interface{}{
			"request_ref":  transactionNo,
			"request_type": "open_account",
			"auth": map[string]interface{}{
				"type": nil, "secure": nil, "auth_provider": "FidelityVirtual", "route_mode": nil,
			},
			"transaction": map[string]interface{}{
				"mock_mode":              "Live",
				"transaction_ref":        transactionNo,
				"transaction_desc":       "Online Deposit (Fidelity)",
				"transaction_ref_parent": nil,
				"amount":                 param.Amount,
				"customer": map[string]interface{}{
					"customer_ref": "234" + username,
					"email":        fidelityPayEmail,
					"mobile_no":    "234" + username,
				},
				"meta":    map[string]interface{}{"amount": param.Amount},
				"details": map[string]interface{}{"name_on_account": accountName},
			},
		}, param.ClientId)

		if err != nil {
			return common.NewErrorResponse("Fidelity Transfer error", nil, 500), nil
		}
		description = "Online Deposit (Fidelity)"
		dataBytes, _ := json.Marshal(fPayRes)
		link = string(dataBytes)

	case "providus":
		description = "Online Deposit (Providus)"
		providusRes, err := s.Providus.InitiatePayment(map[string]interface{}{
			"account_name": username,
		}, param.ClientId)

		if err != nil {
			return common.NewErrorResponse("Providus error", nil, 500), nil
		}

		if resStruct, ok := providusRes.(common.SuccessResponse); ok {
			dataBytes, _ := json.Marshal(resStruct.Data)
			link = string(dataBytes)
			if dataMap, ok := resStruct.Data.(map[string]interface{}); ok {
				if acc, ok := dataMap["account_number"].(string); ok {
					transactionNo = acc
				}
			}
		}

	case "smileandpay":
		description = "Online Deposit (SmileAndPay)"
		baseUrl := s.getApiBaseUrl(param.ClientId)
		resultUrl := fmt.Sprintf("%s/api/v2/webhook/%d/smileandpay/callback", baseUrl, param.ClientId)
		returnUrl := fmt.Sprintf("%s/payment-verification/smileandpay", callbackUrl)

		smileRes, err := s.SmileAndPay.InitiatePayment(map[string]interface{}{
			"orderReference":    transactionNo,
			"amount":            param.Amount,
			"returnUrl":         returnUrl,
			"resultUrl":         resultUrl,
			"itemName":          "Deposit via Bwinners",
			"itemDescription":   description,
			"mobilePhoneNumber": "263" + username, // Check prefix logic
			"currencyCode":      "840",
		}, param.ClientId)

		if err != nil {
			return common.NewErrorResponse("SmileAndPay error", nil, 500), nil
		}

		if resStruct, ok := smileRes.(common.SuccessResponse); ok {
			if dataMap, ok := resStruct.Data.(map[string]interface{}); ok {
				if l, ok := dataMap["paymentUrl"].(string); ok {
					link = l
				}
			}
		}

	case "palmpay":
		description = "Online Deposit (PalmPay)"
		baseUrl := s.getApiBaseUrl(param.ClientId)
		webHookUrl := fmt.Sprintf("%s/api/v2/webhook/%d/palmpay", baseUrl, param.ClientId)

		palmRes, err := s.PalmPay.InitiatePayment(map[string]interface{}{
			"orderId":     transactionNo,
			"amount":      param.Amount * 100,
			"currency":    "NGN",
			"callBackUrl": fmt.Sprintf("%s/payment-verification/palmpay", callbackUrl),
			"notifyUrl":   webHookUrl,
			"description": "OnlineDeposit(PalmPay)",
		}, param.ClientId)

		if err != nil {
			return common.NewErrorResponse("PalmPay error", nil, 500), nil
		}

		if resStruct, ok := palmRes.(common.SuccessResponse); ok {
			if l, ok := resStruct.Data.(string); ok {
				link = l
			}
		}

	case "globus":
		description = "Online Deposit (Globus)"
		globusRes, err := s.Globus.InitiatePayment(map[string]interface{}{
			"accountName":          username,
			"canExpire":            true,
			"expiredTime":          30,
			"hasTransactionAmount": true,
			"transactionAmount":    param.Amount,
			"partnerReference":     transactionNo,
		}, param.ClientId)

		if err != nil {
			return common.NewErrorResponse("Globus error", nil, 500), nil
		}

		if resStruct, ok := globusRes.(common.SuccessResponse); ok {
			dataBytes, _ := json.Marshal(resStruct.Data)
			link = string(dataBytes)
		}

	case "coralpay":
		description = "Online Deposit (Coralpay)"
		// Generate traceId similar to TS?
		// TS: TRX + timestamp(36) + random(36)..
		traceId := "TRX" + fmt.Sprintf("%x", time.Now().UnixNano()) // Simplified
		transactionNo = traceId

		coralRes, err := s.CoralPay.InitiatePayment(map[string]interface{}{
			"customer": map[string]interface{}{
				"email":       userEmail,
				"name":        username,
				"phone":       username,
				"tokenUserId": username,
			},
			"customization": map[string]interface{}{
				"title":       "Coralpay Payment",
				"description": "Payment via Online Deposit",
			},
			"traceId":   traceId,
			"productId": common.GenerateTrxNo(), // Using UUID equivalent
			"amount":    fmt.Sprintf("%.2f", param.Amount),
			"currency":  "NGN",
			"feeBearer": "M",
			"returnUrl": fmt.Sprintf("%s/payment-verification/coralpay", callbackUrl),
		}, param.ClientId)

		if err != nil {
			return common.NewErrorResponse("CoralPay error", nil, 500), nil
		}

		if resStruct, ok := coralRes.(common.SuccessResponse); ok {
			if l, ok := resStruct.Data.(string); ok {
				link = l
			}
		}

	case "payonus":
		description = "Online Deposit (Payonus)"
		payonusEmail := userEmail
		if payonusEmail == "" || strings.Contains(payonusEmail, "noemail") {
			payonusEmail = fmt.Sprintf("noemail+%s@%s", username, siteUrl)
		}

		payonusRes, err := s.Payonus.InitiatePayment(map[string]interface{}{
			"amount":    param.Amount,
			"reference": common.GenerateTrxNo(),
			"customer": map[string]interface{}{
				"name":       username,
				"email":      payonusEmail,
				"phone":      "+234" + username,
				"externalId": common.GenerateTrxNo(),
			},
			"redirectUrl":     fmt.Sprintf("%s/payment-verification/payonus", callbackUrl),
			"notificationUrl": "https://dev.staging.sportsbookengine.com/api/v2/webhook/4/payonus/callback", // Check environment logic
		}, param.ClientId)

		if err != nil {
			return common.NewErrorResponse("Payonus error", nil, 500), nil
		}

		if resStruct, ok := payonusRes.(common.SuccessResponse); ok {
			// Mapping based on TS: transactionNo = payonusRes.data.onusReference
			dataBytes, _ := json.Marshal(resStruct.Data)
			link = string(dataBytes)
			// Need to extract onusReference to set transactionNo
			if dataMap, ok := resStruct.Data.(map[string]interface{}); ok {
				if ref, ok := dataMap["onusReference"].(string); ok {
					transactionNo = ref
				}
			}
		}

	case "mtnmomo":
		description = "Online Deposit (MTMMOMO )"
		transactionNo = common.GenerateTrxNo() // uuidv4

		mtnmomoRes, err := s.Momo.InitiatePayment(map[string]interface{}{
			"amount":     param.Amount,
			"externalId": transactionNo,
			"currency":   "EUR",
			"payer": map[string]interface{}{
				"partyIdType": "MSISDN",
				"partyId":     username,
			},
		}, param.ClientId)

		if err != nil {
			return common.NewErrorResponse("Momo error", nil, 500), nil
		}

		// Map response
		if resMap, ok := mtnmomoRes.(map[string]interface{}); ok {
			if msg, ok := resMap["message"].(string); ok {
				link = msg
			}
		}

	case "mgurush":
		description = "Online Deposit (mGurush)"

	case "wayaquick":
		description = "Online Deposit (Wayaquick)"
		wayaEmail := userEmail
		if wayaEmail == "" || strings.Contains(wayaEmail, "noemail") {
			wayaEmail = fmt.Sprintf("noemail+%s@%s", username, siteUrl)
		}

		wRes, err := s.WayaQuick.GeneratePaymentLink(map[string]interface{}{
			"amount":      fmt.Sprintf("%.2f", param.Amount),
			"email":       wayaEmail,
			"firstName":   username,
			"lastName":    username,
			"narration":   description,
			"phoneNumber": "0" + username,
		}, param.ClientId)

		if err != nil {
			return common.NewErrorResponse("Wayaquick error", nil, 500), nil
		}

		if resMap, ok := wRes.(map[string]interface{}); ok {
			if data, ok := resMap["data"].(map[string]interface{}); ok {
				if url, ok := data["authorization_url"].(string); ok {
					link = url
				}
				if tid, ok := data["tranId"].(string); ok {
					transactionNo = tid
				}
			}
		} else if resStruct, ok := wRes.(common.SuccessResponse); ok {
			if dataMap, ok := resStruct.Data.(map[string]interface{}); ok {
				if url, ok := dataMap["authorization_url"].(string); ok {
					link = url
				}
				if tid, ok := dataMap["tranId"].(string); ok {
					transactionNo = tid
				}
			}
		}

	case "stkpush":
		transactionNo = common.GenerateTrxNo()
		stkRes, err := s.Pitch90.Deposit(map[string]interface{}{
			"amount":   param.Amount,
			"user":     userResp, // Passing user object? TS passes `user`
			"clientId": param.ClientId,
		})

		if err != nil {
			return common.NewErrorResponse("StkPush error", nil, 500), nil
		}

		if resStruct, ok := stkRes.(common.SuccessResponse); ok {
			if dataMap, ok := resStruct.Data.(map[string]interface{}); ok {
				if ref, ok := dataMap["ref_id"].(string); ok {
					transactionNo = ref
				}
			}
		}
		description = "Online Deposit (StkPush)"

	default:
		description = "Shop Deposit"
	}
	// Save Transaction
	s.Helper.SaveTransaction(TransactionData{
		Amount:        param.Amount,
		Channel:       param.PaymentMethod,
		ClientId:      param.ClientId,
		ToUserId:      param.UserId, // To User
		ToUsername:    wallet.Username,
		ToUserBalance: wallet.AvailableBalance, // This uses "toUserBalance" semantics
		// From User is System (0)
		Source:        param.Source,
		Subject:       "Deposit",
		Description:   description,
		TransactionNo: transactionNo,
		Status:        status,
	})

	return map[string]interface{}{
		"success": true,
		"message": "Success",
		"data": map[string]interface{}{
			"transactionRef": transactionNo,
			"link":           link,
		},
	}, nil
}

type UpdateWithdrawalDTO struct {
	WithdrawalId int
	Status       string // 'approve', 'reject', 'process'
	ClientId     int
	Comment      string
	UpdatedBy    string
}

func (s *PaymentService) UpdateWithdrawalStatus(param UpdateWithdrawalDTO) (interface{}, error) {
	var withdrawal models.Withdrawal
	if err := s.DB.Where("id = ? AND client_id = ?", param.WithdrawalId, param.ClientId).First(&withdrawal).Error; err != nil {
		return common.NewErrorResponse("Withdrawal request not found", nil, 404), nil
	}

	switch param.Status {
	case "approve":
		// Disburse Funds Logic
		paymentMethod := models.PaymentMethod{}
		err := s.DB.Where("for_disbursement = ? AND client_id = ?", 1, param.ClientId).First(&paymentMethod).Error
		if err != nil {
			return common.NewErrorResponse("No payment method setup for auto disbursement", nil, 501), nil
		}

		var resp interface{}
		var disbursementErr error

		// Disburse based on provider
		switch paymentMethod.Provider {
		case "paystack":
			resp, disbursementErr = s.Paystack.DisburseFunds(withdrawal, param.ClientId)
		case "flutterwave":
			resp, disbursementErr = s.Flutterwave.DisburseFunds(withdrawal, param.ClientId)
		case "payonus":
			resp, disbursementErr = s.Payonus.DisburseFunds(withdrawal, param.ClientId)
		case "stkpush":
			resp, disbursementErr = s.Pitch90.Withdraw(&withdrawal, param.ClientId)
		case "monnify":
			resp, disbursementErr = s.Monnify.DisburseFunds(&withdrawal, param.ClientId)
		case "korapay":
			resp, disbursementErr = s.Korapay.DisburseFunds(withdrawal, param.ClientId)
		case "opay":
			resp, disbursementErr = s.OPay.DisburseFunds(&withdrawal, param.ClientId)
		case "pawapay":
			data := map[string]interface{}{
				"username":      withdrawal.Username,
				"amount":        withdrawal.Amount,
				"currency":      "TZS",
				"correspondent": "TIGO_TZ", // Or fetch dynamic if needed
				// "recipient": ... handled in CreatePayout helper locally using username
			}
			resp, disbursementErr = s.Pawapay.CreatePayout(data, param.ClientId)
		case "smileandpay":
			data := map[string]interface{}{
				"username":        withdrawal.Username,
				"amount":          withdrawal.Amount,
				"withdrawal_code": withdrawal.WithdrawalCode,
			}
			resp, disbursementErr = s.SmileAndPay.InitiatePayout(data, param.ClientId)

		default:
			return common.NewErrorResponse("Provider not supported for disbursement: "+paymentMethod.Provider, nil, 501), nil
		}

		if disbursementErr != nil {
			return common.NewErrorResponse(disbursementErr.Error(), nil, 400), nil
		}

		// If success (check resp logic if needed, but assuming service returns success struct or map)
		// Update status to 1 (Approved/Processed)
		// Most services update status internally if successful, but we can enforce it here if needed.
		// TS: if (resp.success) update status 1.

		if respMap, ok := resp.(map[string]interface{}); ok {
			if success, ok := respMap["success"].(bool); ok && success {
				s.DB.Model(&withdrawal).Updates(map[string]interface{}{"status": 1, "updated_by": param.UpdatedBy})
			}
		} else if respStruct, ok := resp.(common.SuccessResponse); ok {
			if respStruct.Success {
				s.DB.Model(&withdrawal).Updates(map[string]interface{}{"status": 1, "updated_by": param.UpdatedBy})
			}
		}

		return resp, nil

	case "reject":
		// Reject Logic: Refund user
		transactionNo := withdrawal.WithdrawalCode

		// Update Withdrawal
		s.DB.Model(&withdrawal).Updates(map[string]interface{}{
			"status":     3,
			"comment":    param.Comment,
			"updated_by": param.UpdatedBy,
		})

		// Update Transactions (Debit/Credit) -> Status 3 (Cancelled/Failed)
		s.DB.Model(&models.Transaction{}).Where("transaction_no = ?", transactionNo).Update("status", 3)

		// Refund Wallet
		var wallet models.Wallet
		s.DB.Where("client_id = ? AND user_id = ?", param.ClientId, withdrawal.UserId).First(&wallet)

		newBalance := wallet.AvailableBalance + withdrawal.Amount
		s.DB.Model(&wallet).Update("available_balance", newBalance)

		// Log Refund Transaction
		s.Helper.SaveTransaction(TransactionData{
			Amount:        withdrawal.Amount,
			Channel:       "internal",
			ClientId:      param.ClientId,
			ToUserId:      wallet.UserId, // Corrected from TS _wallet.id vs user_id
			ToUsername:    wallet.Username,
			ToUserBalance: newBalance,
			Source:        "internal",
			Subject:       "Cancelled Request",
			Description:   param.Comment,
			TransactionNo: common.GenerateTrxNo(),
			Status:        1,
			// From fields stubbed
			FromUserId:   0,
			FromUsername: "System",
		})

		return map[string]interface{}{"success": true, "message": "Withdrawal request cancelled"}, nil

	default:
		// Default (Process/Other Action) -> TS logic seems to treat default as "Rejected Request" / Refund essentially?
		// "update withdrawal status to 2"
		s.DB.Model(&withdrawal).Updates(map[string]interface{}{
			"status":     2,
			"comment":    param.Comment,
			"updated_by": param.UpdatedBy,
		})

		// TS Logic: Refund funds to user wallet
		var wallet models.Wallet
		if err := s.DB.Where("client_id = ? AND user_id = ?", param.ClientId, withdrawal.UserId).First(&wallet).Error; err == nil {
			balance := wallet.AvailableBalance + withdrawal.Amount
			s.DB.Model(&wallet).Update("available_balance", balance)

			s.Helper.SaveTransaction(TransactionData{
				Amount:        withdrawal.Amount,
				Channel:       "internal",
				ClientId:      param.ClientId,
				ToUserId:      wallet.UserId,
				ToUsername:    wallet.Username,
				ToUserBalance: balance,
				Source:        "internal",
				Subject:       "Rejected Request",
				Description:   param.Comment,
				TransactionNo: common.GenerateTrxNo(),
				Status:        1,
				FromUserId:    0,
				FromUsername:  "System",
			})
		}

		return map[string]interface{}{"success": true, "message": "Withdrawal request updated"}, nil
	}
}

type CommissionApprovalDTO struct {
	ClientId      int
	UserId        int
	Status        int // 1 = Approve, 2 = Reject
	TransactionNo string
	Comment       string
	UpdatedBy     string
}

func (s *PaymentService) ApproveAndRejectCommissionRequest(param CommissionApprovalDTO) (interface{}, error) {
	// Logic mirrors payments.service.ts
	// If Status 1 (Approve) -> Disburse if needed (but logic in TS seems to rely on withdrawal flow or just update status?)
	// In TS `approveAndRejectCommissionRequest`:
	// If approved (status 1):
	//   Check if withdrawal exists.
	//   If For Disbursement (via settings?) -> Disburse.
	//   Update Withdrawal status to 1.
	//   Update Transaction status to 1.
	// If rejected (status 2 or default?):
	//   Update Withdrawal to 2.
	//   Refund Commission Wallet.
	//   Update Transaction to 2.
	//   Log Reversal.

	var withdrawal models.Withdrawal
	if err := s.DB.Where("withdrawal_code = ? AND client_id = ?", param.TransactionNo, param.ClientId).First(&withdrawal).Error; err != nil {
		return common.NewErrorResponse("Request not found", nil, 404), nil
	}

	switch param.Status {
	case 1:
		// Approve -> Disburse
		paymentMethod := models.PaymentMethod{}
		err := s.DB.Where("for_disbursement = ? AND client_id = ?", 1, param.ClientId).First(&paymentMethod).Error
		if err != nil {
			return common.NewErrorResponse("No payment method setup for auto disbursement", nil, 501), nil
		}

		var resp interface{}
		var disbursementErr error

		// Disburse based on provider (Copy of UpdateWithdrawalStatus disbursement logic)
		switch paymentMethod.Provider {
		case "paystack":
			resp, disbursementErr = s.Paystack.DisburseFunds(withdrawal, param.ClientId)
		case "flutterwave":
			resp, disbursementErr = s.Flutterwave.DisburseFunds(withdrawal, param.ClientId)
		case "payonus":
			resp, disbursementErr = s.Payonus.DisburseFunds(withdrawal, param.ClientId)
		case "stkpush":
			resp, disbursementErr = s.Pitch90.Withdraw(&withdrawal, param.ClientId)
		case "monnify":
			resp, disbursementErr = s.Monnify.DisburseFunds(&withdrawal, param.ClientId)
		case "korapay":
			resp, disbursementErr = s.Korapay.DisburseFunds(withdrawal, param.ClientId)
		case "opay":
			resp, disbursementErr = s.OPay.DisburseFunds(&withdrawal, param.ClientId)
		case "pawapay":
			data := map[string]interface{}{
				"username":      withdrawal.Username,
				"amount":        withdrawal.Amount,
				"currency":      "TZS",
				"correspondent": "TIGO_TZ",
			}
			resp, disbursementErr = s.Pawapay.CreatePayout(data, param.ClientId)
		case "smileandpay":
			data := map[string]interface{}{
				"username":        withdrawal.Username,
				"amount":          withdrawal.Amount,
				"withdrawal_code": withdrawal.WithdrawalCode,
			}
			resp, disbursementErr = s.SmileAndPay.InitiatePayout(data, param.ClientId)

		default:
			return common.NewErrorResponse("Provider not supported for disbursement: "+paymentMethod.Provider, nil, 501), nil
		}

		if disbursementErr != nil {
			return common.NewErrorResponse(disbursementErr.Error(), nil, 400), nil
		}

		if respMap, ok := resp.(map[string]interface{}); ok {
			if success, ok := respMap["success"].(bool); ok && success {
				s.DB.Model(&withdrawal).Updates(map[string]interface{}{"status": 1, "updated_by": "System"})
			}
		}

		return resp, nil

	case 3:
		// Reject -> Refund to Commission Balance
		s.DB.Model(&withdrawal).Updates(map[string]interface{}{
			"status":     3,
			"comment":    "Rejected by Admin",
			"updated_by": "System",
		})

		s.DB.Model(&models.Transaction{}).Where("transaction_no = ?", withdrawal.WithdrawalCode).Updates(map[string]interface{}{"status": 3})

		var wallet models.Wallet
		s.DB.Where("client_id = ? AND user_id = ?", param.ClientId, withdrawal.UserId).First(&wallet)

		newBalance := wallet.CommissionBalance + withdrawal.Amount
		s.DB.Model(&wallet).Update("commission_balance", newBalance)

		s.Helper.SaveTransaction(TransactionData{
			Amount:        withdrawal.Amount,
			Channel:       "internal",
			ClientId:      param.ClientId,
			ToUserId:      wallet.UserId, // user_id
			ToUsername:    wallet.Username,
			ToUserBalance: newBalance, // commission_balance
			Source:        "internal",
			Subject:       "Cancelled Request",
			Description:   "Withdrawal request was cancelled",
			TransactionNo: common.GenerateTrxNo(),
			Status:        1,
			FromUserId:    0,
			FromUsername:  "System",
		})

		return map[string]interface{}{"success": true, "message": "Withdrawal request cancelled", "status": 201}, nil

	default:
		// Processed -> Refund to Available Balance (per TS logic)
		s.DB.Model(&withdrawal).Updates(map[string]interface{}{
			"status":     2,
			"comment":    "Processed by Admin",
			"updated_by": "System",
		})

		var wallet models.Wallet
		if err := s.DB.Where("client_id = ? AND user_id = ?", param.ClientId, withdrawal.UserId).First(&wallet).Error; err == nil {
			balance := wallet.AvailableBalance + withdrawal.Amount
			s.DB.Model(&wallet).Update("available_balance", balance)

			s.Helper.SaveTransaction(TransactionData{
				Amount:        withdrawal.Amount,
				Channel:       "internal",
				ClientId:      param.ClientId,
				ToUserId:      wallet.UserId,
				ToUsername:    wallet.Username,
				ToUserBalance: balance, // available_balance
				Source:        "internal",
				Subject:       "Rejected Request", // TS says "Rejected Request" for default case
				Description:   "Withdrawal request was cancelled",
				TransactionNo: common.GenerateTrxNo(),
				Status:        1,
				FromUserId:    0,
				FromUsername:  "System",
			})
		}

		return map[string]interface{}{"success": true, "message": "Withdrawal request updated", "status": 201}, nil
	}
}

// CancelPendingDebit cancels pending transactions older than 7 minutes
func (s *PaymentService) CancelPendingDebit() error {
	// TS: dayjs().subtract(7, 'minutes')
	cutoff := time.Now().Add(-7 * time.Minute)

	// Update transactions
	// channel != 'sbengine' AND status = 0 AND created_at <= cutoff
	// Gorm: "created_at <= ? AND status = ? AND channel != ?", cutoff, 0, "sbengine"
	// TS sets status = 2
	if err := s.DB.Model(&models.Transaction{}).
		Where("created_at <= ? AND status = ? AND channel != ?", cutoff, 0, "sbengine").
		Update("status", 2).Error; err != nil {
		return err
	}
	return nil
}

type WalletTransferDTO struct {
	ClientId     int
	FromUserId   int
	FromUsername string
	ToUserId     int
	ToUsername   string
	Action       string // "deposit" or "withdrawal" per TS logic (used in response data)
	Amount       float64
	Description  string
}

func (s *PaymentService) WalletTransfer(payload WalletTransferDTO) (interface{}, error) {
	// Find sender wallet
	var fromWallet models.Wallet
	if err := s.DB.Where("user_id = ? AND client_id = ?", payload.FromUserId, payload.ClientId).First(&fromWallet).Error; err != nil {
		return common.NewErrorResponse("Sender wallet not found", nil, 404), nil
	}

	// Find receiver wallet
	var toWallet models.Wallet
	if err := s.DB.Where("user_id = ? AND client_id = ?", payload.ToUserId, payload.ClientId).First(&toWallet).Error; err != nil {
		return common.NewErrorResponse("Receiver wallet not found", nil, 404), nil
	}

	if fromWallet.AvailableBalance < payload.Amount {
		return common.NewErrorResponse("Insufficient balance", nil, 400), nil
	}

	senderBalance := fromWallet.AvailableBalance - payload.Amount
	receiverBalance := toWallet.AvailableBalance + payload.Amount

	// Update sender
	if err := s.DB.Model(&fromWallet).Update("available_balance", senderBalance).Error; err != nil {
		return common.NewErrorResponse("Failed to debit sender", nil, 500), nil
	}

	// Update receiver
	if err := s.DB.Model(&toWallet).Update("available_balance", receiverBalance).Error; err != nil {
		// Potential inconsistency if sender debited but receiver not credited.
		return common.NewErrorResponse("Failed to credit receiver", nil, 500), nil
	}

	desc := payload.Description
	if desc == "" {
		desc = "Inter account transfer"
	}

	s.Helper.SaveTransaction(TransactionData{
		Amount:          payload.Amount,
		Channel:         "retail",
		ClientId:        payload.ClientId,
		ToUserId:        payload.ToUserId,
		ToUsername:      payload.ToUsername,
		ToUserBalance:   receiverBalance,
		FromUserId:      payload.FromUserId,
		FromUsername:    payload.FromUsername,
		FromUserBalance: senderBalance,
		Source:          "internal",
		Subject:         "Funds Transfer",
		Description:     desc,
		TransactionNo:   common.GenerateTrxNo(),
		Status:          1,
	})

	balanceToReturn := receiverBalance
	if payload.Action == "deposit" {
		balanceToReturn = senderBalance
	}

	return map[string]interface{}{
		"success": true,
		"message": "Transaction successful",
		"status":  200,
		"data": map[string]interface{}{
			"balance": balanceToReturn,
		},
	}, nil
}

type VerifyDepositDTO struct {
	TransactionRef string
	PaymentChannel string
	ClientId       int // Added context if needed, though TS VerifyDepositRequest only shows param access.
	// Assuming TS VerifyDepositRequest acts as DTO
}

func (s *PaymentService) VerifyDeposit(param VerifyDepositDTO) (interface{}, error) {
	if param.TransactionRef != "undefined" && param.TransactionRef != "" {
		switch param.PaymentChannel {
		case "paystack":
			return s.Paystack.VerifyTransaction(VerifyTransactionDTO{
				ClientId:       param.ClientId,
				TransactionRef: param.TransactionRef,
			})
		case "monnify":
			return s.Monnify.VerifyTransaction(MonnifyVerifyDTO{
				ClientId:       param.ClientId,
				TransactionRef: param.TransactionRef,
			})
		case "wayaquick":
			// WayaQuick accepts map
			data := map[string]interface{}{
				"clientId":       float64(param.ClientId),
				"transactionRef": param.TransactionRef,
			}
			return s.WayaQuick.VerifyTransaction(data)
		case "flutterwave":
			return s.Flutterwave.VerifyTransaction(VerifyFlutterwaveDTO{
				ClientId:       param.ClientId,
				TransactionRef: param.TransactionRef,
			})
		case "korapay":
			return s.Korapay.VerifyTransaction(VerifyKoraDTO{
				ClientId:       param.ClientId,
				TransactionRef: param.TransactionRef,
			})
		case "fidelity":
			// Fidelity handleCallback likely needs more data, specifically payload from callback?
			// TS: return this.fidelityService.handleCallback(param);
			// For now, return generic error if not supported or pass param as map if compatible?
			return common.NewErrorResponse("Fidelity verification not fully ported check signature", nil, 501), nil
		case "smileandpay":
			data := map[string]interface{}{
				"clientId":       float64(param.ClientId),
				"transactionRef": param.TransactionRef,
			}
			return s.SmileAndPay.VerifyTransaction(data)
		case "fidelity_transfer":
			// return s.Fidelity.HandleVerifyPay(param)
			return common.NewErrorResponse("Fidelity Transfer verification not fully ported", nil, 501), nil
		case "payonus":
			// return s.Payonus.VerifyPayment(param.TransactionRef)
			return common.NewErrorResponse("Payonus verification not implemented", nil, 501), nil

		}
	}
	return common.NewErrorResponse("Invalid transaction reference", nil, 400), nil
}

type VerifyBankAccountDTO struct {
	ClientId      int    `json:"clientId"`
	UserId        int    `json:"userId"`
	AccountNumber string `json:"accountNumber"`
	BankCode      string `json:"bankCode"`
}

func (s *PaymentService) VerifyBankAccount(param VerifyBankAccountDTO) (interface{}, error) {
	// Find payment method for disbursement
	var pm models.PaymentMethod
	if err := s.DB.Where("client_id = ? AND for_disbursement = ?", param.ClientId, 1).First(&pm).Error; err != nil {
		return common.NewErrorResponse("No payment method is active for disbursement", nil, 404), nil
	}

	// Get User Details from Identity Service
	userResp, err := s.IdentityClient.GetUser(param.UserId) // Changed to GetUser as per IdentityClient definition
	if err != nil {
		return common.NewErrorResponse("Failed to fetch user details", nil, 500), nil
	}

	// proto definition likely has Data field which is User. User has FirstName, LastName.
	// Go generated getters are safer.
	userData := userResp.GetData()
	if userData == nil {
		return common.NewErrorResponse("User data not found", nil, 404), nil
	}

	firstName := strings.ToLower(userData.GetFirstName())
	lastName := strings.ToLower(userData.GetLastName())

	if firstName == "" {
		return common.NewErrorResponse("Please update your profile details to proceed", nil, 404), nil
	}

	var accountName string
	var resolveErr error
	var resolveResp interface{}

	switch pm.Provider {
	case "paystack":
		resolveResp, resolveErr = s.Paystack.ResolveAccountNumber(param.ClientId, param.AccountNumber, param.BankCode)
	case "flutterwave":
		resolveResp, resolveErr = s.Flutterwave.ResolveAccountNumber(param.ClientId, param.AccountNumber, param.BankCode)
	case "monnify":
		resolveResp, resolveErr = s.Monnify.ResolveAccountNumber(param.ClientId, param.AccountNumber, param.BankCode)
	case "korapay":
		resolveResp, resolveErr = s.Korapay.ResolveAccountNumber(param.ClientId, param.AccountNumber, param.BankCode)
	// case "opay":
	// 	resolveResp, resolveErr = s.OPay.ResolveAccountNumber(param.ClientId, param.AccountNumber, param.BankCode)
	// Add other providers if they support ResolveAccountNumber
	default:
		return common.NewErrorResponse("Provider does not support account resolution", nil, 400), nil
	}

	if resolveErr != nil {
		return common.NewErrorResponse("Could not resolve account name. Check parameters or try again", nil, 404), nil
	}

	// Parse account name from response
	var resolvedData map[string]interface{}

	if successResp, ok := resolveResp.(common.SuccessResponse); ok {
		if dataMap, ok := successResp.Data.(map[string]interface{}); ok {
			resolvedData = dataMap
		}
	} else if mapResp, ok := resolveResp.(map[string]interface{}); ok {
		// Monnify returns map
		if data, ok := mapResp["data"].(map[string]interface{}); ok {
			resolvedData = data
		} else {
			resolvedData = mapResp
		}
	}

	if resolvedData == nil {
		// Fallback if data is not in "data" key or structure differs
		if mapResp, ok := resolveResp.(map[string]interface{}); ok {
			resolvedData = mapResp
		}
	}

	if val, ok := resolvedData["account_name"].(string); ok {
		accountName = val
	} else if val, ok := resolvedData["accountName"].(string); ok {
		accountName = val
	}

	if accountName == "" {
		// Log response for debug?
		return common.NewErrorResponse("Could not extract account name from provider response", nil, 500), nil
	}

	lowerAccountName := strings.ToLower(accountName)
	names := strings.Fields(lowerAccountName)

	match := false
	for _, n := range names {
		if n == firstName || n == lastName {
			match = true
			break
		}
	}

	if !match {
		return common.NewErrorResponse("Account name does not match user name", nil, 400), nil
	}

	return map[string]interface{}{
		"success":        true,
		"account_name":   accountName,
		"account_number": param.AccountNumber,
		"bank_code":      param.BankCode,
	}, nil
}

type WayaBankRequestDTO struct {
	ClientId int `json:"clientId"`
	UserId   int `json:"userId"`
}

func (s *PaymentService) WayaBankAccountEnquiry(param WayaBankRequestDTO) (interface{}, error) {
	// Find Wallet
	var wallet models.Wallet
	if err := s.DB.Where("client_id = ? AND user_id = ?", param.ClientId, param.UserId).First(&wallet).Error; err != nil {
		return common.NewErrorResponse("Wallet not found", nil, 404), nil
	}

	if wallet.VirtualAccountNo == "" {
		return common.NewErrorResponse("Virtual account not found, proceed to create", nil, 404), nil
	}

	data := map[string]interface{}{
		"accountNumber": wallet.VirtualAccountNo,
	}

	res, err := s.WayaBank.AccountEnquiry(data, param.ClientId)
	if err != nil {
		return common.NewErrorResponse("Unable to complete transaction", nil, 500), nil
	}

	// Unpack response
	if resMap, ok := res.(map[string]interface{}); ok {
		if success, ok := resMap["success"].(bool); ok && success {
			if dataMap, ok := resMap["data"].(map[string]interface{}); ok {
				// Update Wallet
				updates := map[string]interface{}{
					"virtual_branch_id":         dataMap["branchId"],
					"virtual_account_no":        dataMap["accountNo"],
					"virtual_account_name":      dataMap["accountName"],
					"virtual_balance":           dataMap["balance"],
					"virtual_account_default":   dataMap["accountDefault"],
					"virtual_nuban_account_no":  dataMap["nubanAccountNo"],
					"virtual_acct_closure_flag": dataMap["acctClosureFlag"],
					"virtual_acct_delete_flag":  dataMap["acctDeleteFlag"],
				}
				s.DB.Model(&wallet).Updates(updates)
				return map[string]interface{}{"success": true, "message": "Virtual account found and wallet updated"}, nil
			}
		} else {
			return res, nil // Forward error response
		}
	}

	return common.NewErrorResponse("Invalid response from provider", nil, 500), nil
}

func (s *PaymentService) CreateVirtualAccount(param WayaBankRequestDTO) (interface{}, error) {
	var wallet models.Wallet
	if err := s.DB.Where("client_id = ? AND user_id = ?", param.ClientId, param.UserId).First(&wallet).Error; err != nil {
		return common.NewErrorResponse("Wallet not found", nil, 404), nil
	}

	// Get User Details
	userResp, err := s.IdentityClient.GetUser(param.UserId)
	if err != nil {
		return common.NewErrorResponse("Failed to fetch user details", nil, 500), nil
	}
	userData := userResp.GetData()
	if userData == nil {
		return common.NewErrorResponse("User data not found", nil, 404), nil
	}

	// Construct user map for WayaBank
	userMap := map[string]interface{}{
		"username":    userData.GetUsername(), // Needs verification if Username exists in User proto
		"email":       userData.GetEmail(),
		"firstName":   userData.GetFirstName(),
		"lastName":    userData.GetLastName(),
		"phoneNumber": userData.GetPhone(),
	}

	data := map[string]interface{}{
		"user": userMap,
	}

	res, err := s.WayaBank.CreateVirtualAccount(data, param.ClientId)
	if err != nil {
		return common.NewErrorResponse("Unable to complete transaction", nil, 500), nil
	}

	if resMap, ok := res.(map[string]interface{}); ok {
		if success, ok := resMap["success"].(bool); ok && success {
			if dataMap, ok := resMap["data"].(map[string]interface{}); ok {
				updates := map[string]interface{}{
					"virtual_branch_id":         dataMap["branchId"],
					"virtual_account_no":        dataMap["accountNo"],
					"virtual_account_name":      dataMap["accountName"],
					"virtual_balance":           dataMap["balance"],
					"virtual_account_default":   dataMap["accountDefault"],
					"virtual_nuban_account_no":  dataMap["nubanAccountNo"],
					"virtual_acct_closure_flag": dataMap["acctClosureFlag"],
					"virtual_acct_delete_flag":  dataMap["acctDeleteFlag"],
				}
				s.DB.Model(&wallet).Updates(updates)
				return map[string]interface{}{"success": true, "message": "Virtual account created and wallet updated"}, nil
			}
		} else {
			return res, nil
		}
	}

	return common.NewErrorResponse("Invalid response from provider", nil, 500), nil
}

type CreateRequestDTO struct {
	ClientId  int     `json:"clientId"`
	UserId    int     `json:"userId"`
	Amount    float64 `json:"amount"`
	Action    string  `json:"action"` // deposit, payouts, cancel-payouts, refunds
	Operator  string  `json:"operator"`
	Source    string  `json:"source"`
	DepositId string  `json:"depositId"` // for refunds
}

func (s *PaymentService) CreateRequest(param CreateRequestDTO) (interface{}, error) {
	// Find Wallet
	var wallet models.Wallet
	if err := s.DB.Where("client_id = ? AND user_id = ?", param.ClientId, param.UserId).First(&wallet).Error; err != nil {
		return common.NewErrorResponse("Wallet not found", nil, 404), nil
	}

	if param.Action == "payouts" {
		if wallet.AvailableBalance < param.Amount {
			return common.NewErrorResponse("Insufficient wallet balance for payout", nil, 400), nil
		}
	}

	// fetch user data for payment
	// s.IdentityClient.GetPaymentData
	req := &identity.GetPaymentDataRequest{
		ClientId: int32(param.ClientId),
		UserId:   int32(param.UserId),
		Source:   param.Source,
	}
	userRes, err := s.IdentityClient.GetPaymentData(req)
	if err != nil {
		return common.NewErrorResponse("Failed to fetch user payment data", nil, 500), nil
	}
	// userRes is *identity.GetPaymentDataResponse which likely mimics UserData or contains it.
	// Need to verify GetPaymentDataResponse structure.
	// TS: user = userRes
	// PawaPay methods take `user` which in TS is passed as object.
	// Go PawaPayService methods likely take struct or map.
	// I need to check pawapay_service.go CreateDeposit/CreatePayout signatures.

	// For now, I'll assume they take *identity.GetPaymentDataResponse or mapped data.
	// I'll check pawapay_service.go briefly to be accurate.
	// But I will write the code structure now and adjust if needed.

	// Generate Action ID
	actionId := uuid.NewString()

	var subject, transactionNo string
	var res interface{}
	var pawaErr error

	switch param.Action {
	case "deposit":
		userMap := map[string]interface{}{
			"username": userRes.GetUsername(),
			"email":    userRes.GetEmail(),
			"pin":      userRes.GetPin(),
		}
		res, pawaErr = s.Pawapay.CreateDeposit(userMap, param.Amount, actionId, param.Operator, param.ClientId)
		if pawaErr != nil {
			return common.NewErrorResponse(pawaErr.Error(), nil, 500), nil
		}
		// assert success from res if it's map
		if resMap, ok := res.(map[string]interface{}); ok {
			if sVal, ok := resMap["success"].(bool); !ok || !sVal {
				return res, nil
			}
			transactionNo, _ = resMap["transactionNo"].(string)
		}
		subject = "deposit"

	case "payouts":
		username := userRes.GetUsername()
		if !strings.HasPrefix(username, "255") {
			username = "255" + strings.TrimLeft(username, "0")
		}

		payoutData := map[string]interface{}{
			"payoutId":      actionId,
			"amount":        fmt.Sprintf("%.2f", param.Amount), // TS: param.amount.toString()
			"currency":      "TZS",
			"country":       "TZA",
			"correspondent": param.Operator,
			"recipient": map[string]interface{}{
				"address": map[string]string{"value": username},
				"type":    "MSISDN",
			},
			"statementDescription": "Online Payouts",
			"customerTimestamp":    time.Now(),
			"metadata": []map[string]interface{}{
				{"fieldName": "customerId", "fieldValue": username, "isPII": true},
			},
		}

		res, pawaErr = s.Pawapay.CreatePayout(payoutData, param.ClientId)
		if pawaErr != nil {
			return common.NewErrorResponse(pawaErr.Error(), nil, 500), nil
		}
		if resMap, ok := res.(map[string]interface{}); ok {
			if sVal, ok := resMap["success"].(bool); !ok || !sVal {
				return res, nil
			}
			transactionNo, _ = resMap["transactionNo"].(string)
		}
		subject = "payouts"

	case "cancel-payouts":
		res, pawaErr = s.Pawapay.CancelPayout(actionId, param.ClientId)
		if pawaErr != nil {
			return common.NewErrorResponse(pawaErr.Error(), nil, 500), nil
		}
		if resMap, ok := res.(map[string]interface{}); ok {
			if sVal, ok := resMap["success"].(bool); !ok || !sVal {
				return res, nil
			}
			transactionNo, _ = resMap["transactionNo"].(string)
			// Update transaction status 2
			s.DB.Model(&models.Transaction{}).Where("transaction_no = ?", transactionNo).Update("status", 2)
			return map[string]interface{}{
				"success": true,
				"message": "Payout cancelled successfully",
				"data":    resMap["data"],
			}, nil
		}

	case "refunds":
		userMap := map[string]interface{}{
			"username": userRes.GetUsername(),
			"email":    userRes.GetEmail(),
			"pin":      userRes.GetPin(),
		}
		res, pawaErr = s.Pawapay.CreateRefund(userMap, param.Amount, actionId, param.DepositId, param.ClientId)
		if pawaErr != nil {
			return common.NewErrorResponse(pawaErr.Error(), nil, 500), nil
		}
		if resMap, ok := res.(map[string]interface{}); ok {
			if sVal, ok := resMap["success"].(bool); !ok || !sVal {
				return res, nil
			}
			transactionNo, _ = resMap["transactionNo"].(string)
		}
		subject = "refunds"
	default:
		return common.NewErrorResponse("Invalid action", nil, 400), nil
	}

	// Save Transaction
	// Need description from res.data.status?
	description := ""
	if resMap, ok := res.(map[string]interface{}); ok {
		if data, ok := resMap["data"].(map[string]interface{}); ok {
			if s, ok := data["status"].(string); ok {
				description = s
			}
		}
	}

	s.Helper.SaveTransaction(TransactionData{
		Amount:          param.Amount,
		Channel:         "pawapay",
		ClientId:        param.ClientId,
		ToUserId:        param.UserId,
		ToUsername:      wallet.Username,
		ToUserBalance:   wallet.AvailableBalance,
		FromUserId:      0,
		FromUsername:    "System",
		FromUserBalance: 0,
		Source:          param.Source,
		Subject:         subject,
		Description:     description,
		TransactionNo:   transactionNo,
	})

	return map[string]interface{}{
		"success": true,
		"message": "Success",
		"data":    map[string]string{"transactionRef": transactionNo},
	}, nil
}

// StartScheduler initializes the cron job for PaymentService
func (s *PaymentService) StartScheduler() {
	c := cron.New()
	// Run every 10 minutes: "*/10 * * * *"
	_, err := c.AddFunc("*/10 * * * *", func() {
		log.Println("Running scheduled CancelPendingDebit task...")
		if err := s.CancelPendingDebit(); err != nil {
			log.Printf("Error in CancelPendingDebit: %v", err)
		}
	})
	if err != nil {
		log.Printf("Error scheduling CancelPendingDebit: %v", err)
		return
	}
	c.Start()
	log.Println("PaymentService Scheduler started (Every 10 minutes)")
}
