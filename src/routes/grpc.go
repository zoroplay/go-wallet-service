package routes

import (
	"context"
	"log"

	"github.com/zoroplay/go-wallet-service/controllers"
	pbWallet "github.com/zoroplay/go-wallet-service/grpc/protobuf"
)

// GetProducerStatus
func (a *App) CreditUser(ctx context.Context, in *pbWallet.CreditUserRequest) (*pbWallet.WalletResponse, error) {

	log.Printf("CreditUser request")
	success, status, message, wallet := controllers.CreditUser(a.DB, in)

	return &pbWallet.WalletResponse{
		Status:  int32(status),
		Success: success,
		Message: message,
		Data:    wallet,
	}, nil

}

func (a *App) DebitUser(ctx context.Context, in *pbWallet.DebitUserRequest) (*pbWallet.WalletResponse, error) {

	log.Printf("DebitUser request")
	success, status, message, wallet := controllers.DebitUser(a.DB, in)

	return &pbWallet.WalletResponse{
		Status:  int32(status),
		Success: success,
		Message: message,
		Data:    wallet,
	}, nil
}

// Get Balance
func (a *App) GetBalance(ctx context.Context, in *pbWallet.GetBalanceRequest) (*pbWallet.WalletResponse, error) {

	log.Printf("Get User Balance request")
	success, status, message, wallet := controllers.GetBalance(a.DB, in)

	return &pbWallet.WalletResponse{
		Status:  int32(status),
		Success: success,
		Message: message,
		Data:    wallet,
	}, nil
}

// // GetOdds
// func (a *App) GetProbability(ctx context.Context, in *pbOdds.GetOddsRequest) (*pbOdds.Probability, error) {

// 	log.Printf("GetProbability | producerID %d | matchID %d | marketID %d | specifier %s | outcomeID %s", in.ProducerID,in.EventID,in.MarketID,in.Specifier,in.OutcomeID)
// 	prob := controllers.GetProbability(a.DB, int64(in.ProducerID),int64(in.EventID), int64(in.MarketID),in.Specifier,in.OutcomeID)

// 	return &pbOdds.Probability{
// 		Probability:       float32(prob),
// 	}, nil

// }
