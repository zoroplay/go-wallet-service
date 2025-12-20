package services

import (
	"context"
	"fmt"
	"os"
	"time"

	"wallet-service/pkg/common"
	"wallet-service/proto/identity"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type IdentityClient struct {
	client     identity.IdentityServiceClient
	connection *grpc.ClientConn
}

func NewIdentityClient() (*IdentityClient, error) {
	identityUrl := os.Getenv("IDENTITY_SERVICE_URL")
	if identityUrl == "" {
		identityUrl = "localhost:5002" // Default fallback
	}

	conn, err := grpc.NewClient(identityUrl, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("failed to connect to identity service: %v", err)
	}

	client := identity.NewIdentityServiceClient(conn)
	return &IdentityClient{
		client:     client,
		connection: conn,
	}, nil
}

func (c *IdentityClient) Close() {
	if c.connection != nil {
		c.connection.Close()
	}
}

func (c *IdentityClient) GetUser(userId int) (*identity.GetUserDetailsResponse, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req := &identity.GetUserDetailsRequest{
		UserId: int32(userId),
	}

	return c.client.GetUserDetails(ctx, req)
}

func (c *IdentityClient) GetTrackierKeys(itemId int) (*identity.CommonResponseObj, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req := &identity.SingleItemRequest{
		ItemId: int32(itemId),
	}

	return c.client.GetTrackierKeys(ctx, req)
}

func (c *IdentityClient) GetPaymentData(req *identity.GetPaymentDataRequest) (*identity.GetPaymentDataResponse, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return c.client.GetPaymentData(ctx, req)
}

func (c *IdentityClient) GetAffiliateUsers(req *identity.AffiliateRequest) (*identity.AffiliateResponse, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return c.client.GetAffiliateUsers(ctx, req)
}

func (c *IdentityClient) GetClientAffiliates(req *identity.ClientIdRequest) (*identity.CommonResponseArray, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return c.client.GetClientAffiliates(ctx, req)
}

func (c *IdentityClient) GetClientUsers(req *identity.ClientIdRequest) (*identity.CommonResponseArray, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return c.client.ClintUsers(ctx, req)
}

func (c *IdentityClient) GetAgents(req *identity.ClientIdRequest) (*identity.CommonResponseObj, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return c.client.FetchAgents(ctx, req)
}

func (c *IdentityClient) ListAgentUsers(req *identity.GetAgentUsersRequest) (*identity.CommonResponseArray, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return c.client.ListAgentUsers(ctx, req)
}

func (c *IdentityClient) GetWithdrawalSettings(clientId, userId int) (*identity.WithdrawalSettingsResponse, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req := &identity.GetWithdrawalSettingsRequest{
		ClientId: int32(clientId),
		UserId:   common.Int32Ptr(int32(userId)),
	}

	return c.client.GetWithdrawalSettings(ctx, req)
}
