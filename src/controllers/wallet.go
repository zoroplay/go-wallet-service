package controllers

import (
	"database/sql"
	"fmt"
	"log"
	"math/rand"
	"strconv"
	"time"

	"github.com/sirupsen/logrus"
	pbWallet "github.com/zoroplay/go-wallet-service/grpc/protobuf"
	"github.com/zoroplay/go-wallet-service/models"
)

func CreditUser(db *sql.DB, in *pbWallet.CreditUserRequest) (success bool, status int32, message string, data *pbWallet.Wallet) {
	var walletType string = "Main"
	// var err error
	_ = walletType
	log.Printf("Crediting user  %d in client %d ", in.UserId, in.ClientId)

	var walletBalance string = "available_balance"
	var balance float64 = 0.00
	var updateQuery string
	var userId = in.UserId
	var row models.Wallet

	amount, err := strconv.ParseFloat(in.Amount, 32)
	if err != nil {

		logrus.Panic(err)
	}

	queryString := fmt.Sprintf("SELECT balance, available_balance, sport_bonus_balance, " +
		"virtual_bonus_balance, casino_bonus_balance, trust_balance " +
		" FROM wallets " +
		" WHERE user_id = ? ")

	err = db.QueryRow(queryString, userId).Scan(&row.Balance, &row.AvailableBalance, &row.SportBonusBalance, &row.CasinoBonusBalance,
		&row.VirtualBonusBalance, &row.TrustBalance)

	if err != nil {

		log.Printf("error getting user wallet with id %d  %s", userId, err.Error())
		return false, 404, "User not found", nil
	}

	switch in.Wallet {
	case "sport-bonus":
		walletBalance = "sport_bonus_balance"
		walletType = "Sport Bonus"
		break
	case "virtual":
		walletBalance = "virtual_bonus_balance"
		walletType = "Virtual Bonus"
		break
	case "casino":
		walletBalance = "casino_bonus_balance"
		walletType = "Casino Bonus"
		break
	case "trust":
		walletBalance = "trust_balance"
		walletType = "Trust"
		break
	default:
		walletBalance = "available_balance"
		break
	}

	// CRITICAL FIX: Use atomic SQL increment to prevent race conditions
	// This ensures concurrent credits don't overwrite each other
	// Instead of: Read balance -> Calculate -> Write (race condition!)
	// We do: Atomically increment (safe for concurrency!)
	updateQuery = fmt.Sprintf("UPDATE wallets SET %s = %s + %f WHERE user_id = %d AND client_id = %d",
		walletBalance, walletBalance, amount, userId, in.ClientId)

	_, err = db.Exec(updateQuery)

	if err != nil {

		log.Printf("got error updating wallet\n%s\n%s", updateQuery, err.Error())
		return false, 500, "Unable to update user wallet", nil
	}

	// Fetch updated balance after atomic increment
	err = db.QueryRow(queryString, userId).Scan(&row.Balance, &row.AvailableBalance, &row.SportBonusBalance, &row.CasinoBonusBalance,
		&row.VirtualBonusBalance, &row.TrustBalance)

	if err != nil {
		log.Printf("error fetching updated wallet with id %d  %s", userId, err.Error())
		return false, 500, "Unable to fetch updated wallet", nil
	}

	// Get the updated balance for the correct wallet type
	switch in.Wallet {
	case "sport-bonus":
		balance = row.SportBonusBalance
	case "virtual":
		balance = row.VirtualBonusBalance
	case "casino":
		balance = row.CasinoBonusBalance
	case "trust":
		balance = row.TrustBalance
	default:
		balance = row.AvailableBalance
	}

	var transaction_no = generateTrxNo()

	//save transactions
	stmt, err := db.Prepare("INSERT INTO transactions (client_id,user_id,username,transaction_no,amount,tranasaction_type,subject,description,source,channel,balance, status, created_at) VALUE (?,?,?,?,?,'credit',?,?,?,?,?,1,NOW())")
	if err != nil {
		log.Printf("error preparing query %s ", err.Error())
		return false, 500, "Error saving transaction", nil
	}
	defer stmt.Close()

	_, err = stmt.Exec(in.ClientId, in.UserId, in.Username, transaction_no, amount, in.Subject, in.Description, in.Source, in.Channel, balance)
	if err != nil {

		log.Printf("error preparing query %s ", err.Error())
		return false, 500, "Error saving transaction", nil
	}

	var wallet = &pbWallet.Wallet{
		UserId:              userId,
		Balance:             balance,
		AvailableBalance:    row.AvailableBalance,
		TrustBalance:        row.TrustBalance,
		SportBonusBalance:   row.SportBonusBalance,
		VirtualBonusBalance: row.VirtualBonusBalance,
		CasinoBonusBalance:  row.CasinoBonusBalance,
	}

	return true, 200, "Wallet Credited", wallet
}

func DebitUser(db *sql.DB, in *pbWallet.DebitUserRequest) (success bool, status int32, message string, data *pbWallet.Wallet) {
	var walletType string = "Main"
	// var err error
	_ = walletType

	log.Printf("Debiting user  %d in client %d ", in.UserId, in.ClientId)

	var walletBalance string = "available_balance"
	var balance float64 = 0.00
	var updateQuery string
	var userId = in.UserId
	var row models.Wallet

	amount, err := strconv.ParseFloat(in.Amount, 32)
	if err != nil {

		logrus.Panic(err)
	}

	queryString := fmt.Sprintf("SELECT balance, available_balance, sport_bonus_balance, " +
		"virtual_bonus_balance, casino_bonus_balance, trust_balance " +
		" FROM wallets " +
		" WHERE user_id = ? ")

	err = db.QueryRow(queryString, userId).Scan(&row.Balance, &row.AvailableBalance, &row.SportBonusBalance, &row.CasinoBonusBalance,
		&row.VirtualBonusBalance, &row.TrustBalance)

	if err != nil {

		log.Printf("error getting user wallet with id %d  %s", userId, err.Error())
		return false, 404, "User not found", nil
	}

	// Check sufficient balance before debit
	switch in.Wallet {
	case "sport-bonus":
		if row.SportBonusBalance < amount {
			return false, 400, "Insufficient balance", nil
		}
		walletBalance = "sport_bonus_balance"
		walletType = "Sport Bonus"
		break
	case "virtual":
		if row.VirtualBonusBalance < amount {
			return false, 400, "Insufficient balance", nil
		}
		walletBalance = "virtual_bonus_balance"
		walletType = "Virtual Bonus"
		break
	case "casino":
		if row.CasinoBonusBalance < amount {
			return false, 400, "Insufficient balance", nil
		}
		walletBalance = "casino_bonus_balance"
		walletType = "Casino Bonus"
		break
	case "trust":
		if row.TrustBalance < amount {
			return false, 400, "Insufficient balance", nil
		}
		walletBalance = "trust_balance"
		walletType = "Trust"
		break
	default:
		if row.AvailableBalance < amount {
			return false, 400, "Insufficient balance", nil
		}
		walletBalance = "available_balance"
		break
	}

	// CRITICAL FIX: Use atomic SQL decrement to prevent race conditions
	// Same fix as CreditUser - atomic operations prevent lost updates
	updateQuery = fmt.Sprintf("UPDATE wallets SET %s = %s - %f WHERE user_id = %d AND client_id = %d",
		walletBalance, walletBalance, amount, userId, in.ClientId)

	_, err = db.Exec(updateQuery)

	if err != nil {

		log.Printf("got error updating wallet\n%s\n%s", updateQuery, err.Error())
		return false, 500, "Unable to update user wallet", nil
	}

	// Fetch updated balance after atomic decrement
	err = db.QueryRow(queryString, userId).Scan(&row.Balance, &row.AvailableBalance, &row.SportBonusBalance, &row.CasinoBonusBalance,
		&row.VirtualBonusBalance, &row.TrustBalance)

	if err != nil {
		log.Printf("error fetching updated wallet with id %d  %s", userId, err.Error())
		return false, 500, "Unable to fetch updated wallet", nil
	}

	// Get the updated balance for the correct wallet type
	switch in.Wallet {
	case "sport-bonus":
		balance = row.SportBonusBalance
	case "virtual":
		balance = row.VirtualBonusBalance
	case "casino":
		balance = row.CasinoBonusBalance
	case "trust":
		balance = row.TrustBalance
	default:
		balance = row.AvailableBalance
	}

	var transaction_no = generateTrxNo()

	//save transactions
	stmt, err := db.Prepare("INSERT INTO transactions (client_id,user_id,username,transaction_no,amount,tranasaction_type,subject,description,source,channel,balance, status, created_at) VALUE (?,?,?,?,?,'debit',?,?,?,?,?,1,NOW())")
	if err != nil {
		log.Printf("error preparing query %s ", err.Error())
		return false, 500, "Error saving transaction", nil
	}
	defer stmt.Close()

	_, err = stmt.Exec(in.ClientId, in.UserId, in.Username, transaction_no, amount, in.Subject, in.Description, in.Source, in.Channel, balance)
	if err != nil {

		log.Printf("error preparing query %s ", err.Error())
		return false, 500, "Error saving transaction", nil
	}

	var wallet = &pbWallet.Wallet{
		UserId:              userId,
		Balance:             balance,
		AvailableBalance:    row.AvailableBalance,
		TrustBalance:        row.TrustBalance,
		SportBonusBalance:   row.SportBonusBalance,
		VirtualBonusBalance: row.VirtualBonusBalance,
		CasinoBonusBalance:  row.CasinoBonusBalance,
	}

	return true, 200, "Wallet Debited", wallet
}

func GetBalance(db *sql.DB, in *pbWallet.GetBalanceRequest) (success bool, status int32, message string, data *pbWallet.Wallet) {

	var userId = in.UserId
	var row models.Wallet

	log.Printf("Getting balance for  %d in client %d ", in.UserId, in.ClientId)

	queryString := fmt.Sprintf("SELECT balance, available_balance, sport_bonus_balance, " +
		"virtual_bonus_balance, casino_bonus_balance, trust_balance " +
		" FROM wallets " +
		" WHERE user_id = ? ")

	err := db.QueryRow(queryString, userId).Scan(&row.Balance, &row.AvailableBalance, &row.SportBonusBalance, &row.VirtualBonusBalance,
		&row.CasinoBonusBalance, &row.TrustBalance)

	if err != nil {

		log.Printf("error getting user wallet with id %d  %s", userId, err.Error())
		return false, 404, "User not found", nil
	}

	var wallet = &pbWallet.Wallet{
		UserId:              userId,
		Balance:             row.Balance,
		AvailableBalance:    row.AvailableBalance,
		TrustBalance:        row.TrustBalance,
		SportBonusBalance:   row.SportBonusBalance,
		VirtualBonusBalance: row.VirtualBonusBalance,
		CasinoBonusBalance:  row.CasinoBonusBalance,
	}

	return true, 200, "Wallet retreived", wallet
}

func generateTrxNo() string {
	const charset = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

	var seededRand *rand.Rand = rand.New(rand.NewSource(time.Now().UnixNano()))
	b := make([]byte, 7)
	for i := range b {
		b[i] = charset[seededRand.Intn(len(charset))]
	}

	return string(b)
}

// func GetProbability(db *sql.DB, producerID, matchID, marketID int64, specifier, outcomeID string) (probability float64) {

// 	var p sql.NullFloat64

// 	var table string

// 	if producerID == 3 {

// 		table = "odds_prematch"

// 	} else {

// 		table = "odds_live"

// 	}

// 	queryString := fmt.Sprintf("SELECT probability "+
// 		" FROM %s "+
// 		" WHERE match_id = ? AND market_id = ? AND specifier = ? AND outcome_id = ?  ", table)

// 	err := db.QueryRow(queryString, matchID, marketID, specifier, outcomeID).
// 		Scan(&p)
// 	if err != nil {

// 		log.Printf("error checking odds for event %d  %s", matchID, err.Error())
// 		return 0
// 	}

// 	if !p.Valid {

// 		return 0
// 	}

// 	return p.Float64
// }
