package services

import (
	"context"
	"fmt"
	"os"
	"time"

	"wallet-service/proto/bonus"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type BonusClient struct {
	client     bonus.BonusServiceClient
	connection *grpc.ClientConn
}

func NewBonusClient() (*BonusClient, error) {
	bonusUrl := os.Getenv("BONUS_SERVICE_URL")
	if bonusUrl == "" {
		bonusUrl = "localhost:50054" // Default assumption, can be adjusted
	}

	conn, err := grpc.NewClient(bonusUrl, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("failed to connect to bonus service: %v", err)
	}

	client := bonus.NewBonusServiceClient(conn)
	return &BonusClient{
		client:     client,
		connection: conn,
	}, nil
}

func (c *BonusClient) Close() {
	if c.connection != nil {
		c.connection.Close()
	}
}

func (c *BonusClient) GetUserBonus(clientId int, userId int) (*bonus.CommonResponseObj, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req := &bonus.CheckDepositBonusRequest{
		ClientId: int32(clientId),
		UserId:   int32(userId),
	}

	return c.client.GetActiveUserBonus(ctx, req)
}

func (c *BonusClient) AwardBonus(req *bonus.AwardBonusRequest) (*bonus.UserBonusResponse, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	return c.client.AwardBonus(ctx, req)
}
