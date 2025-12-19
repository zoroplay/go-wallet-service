package services

import (
	"context"
	"fmt"
	"wallet-service/internal/models"
	"wallet-service/proto/identity"
	walletBuffer "wallet-service/proto/wallet"

	"google.golang.org/protobuf/types/known/structpb"
	"gorm.io/gorm"
)

type RetailService struct {
	DB       *gorm.DB
	Identity *IdentityClient
}

func NewRetailService(db *gorm.DB, identity *IdentityClient) *RetailService {
	return &RetailService{DB: db, Identity: identity}
}

func (s *RetailService) ListRetailTransactions(data *walletBuffer.RetailDataRequest) (*walletBuffer.CommonResponseArray, error) {
	if data.AgentId == nil || data.ClientId == 0 {
		return &walletBuffer.CommonResponseArray{
			Success: false,
			Status:  400,
			Message: "Invalid request parameters",
		}, nil
	}

	// Fetch users for this agent
	resp, err := s.Identity.ListAgentUsers(&identity.GetAgentUsersRequest{
		ClientId: data.ClientId,
		UserId:   data.AgentId,
	})
	if err != nil {
		return &walletBuffer.CommonResponseArray{
			Success: false,
			Status:  500,
			Message: fmt.Sprintf("Error fetching agent users: %v", err),
		}, nil
	}

	var userIds []int32
	for _, st := range resp.Data {
		if idVal, ok := st.Fields["id"]; ok {
			userIds = append(userIds, int32(idVal.GetNumberValue()))
		}
	}

	if len(userIds) == 0 {
		return &walletBuffer.CommonResponseArray{
			Success: true,
			Status:  200,
			Message: "No users found for this agent",
		}, nil
	}

	// Query transactions
	type TransactionTotal struct {
		UserId       int32   `gorm:"column:userId"`
		TotalDeposit float64 `gorm:"column:totalDeposit"`
	}

	var deposits []TransactionTotal
	depositQuery := s.DB.Model(&models.Transaction{}).
		Select("user_id as userId, SUM(amount) as totalDeposit").
		Where("user_id IN ?", userIds).
		Where("client_id = ?", data.ClientId).
		Where("tranasaction_type = ?", "credit").
		Where("subject = ?", "Funds Transfer").
		Group("user_id")

	if data.From != nil && data.To != nil {
		depositQuery = depositQuery.Where("created_at BETWEEN ? AND ?", *data.From, *data.To)
	}
	depositQuery.Scan(&deposits)

	type WithdrawalTotal struct {
		UserId          int32   `gorm:"column:userId"`
		TotalWithdrawal float64 `gorm:"column:totalWithdrawal"`
	}
	var withdrawals []WithdrawalTotal
	withdrawalQuery := s.DB.Model(&models.Transaction{}).
		Select("user_id as userId, SUM(amount) as totalWithdrawal").
		Where("user_id IN ?", userIds).
		Where("client_id = ?", data.ClientId).
		Where("tranasaction_type = ?", "debit").
		Where("subject = ?", "shop").
		Group("user_id")

	if data.From != nil && data.To != nil {
		withdrawalQuery = withdrawalQuery.Where("created_at BETWEEN ? AND ?", *data.From, *data.To)
	}
	withdrawalQuery.Scan(&withdrawals)

	// Merge results
	var resultList []*structpb.Struct
	for _, st := range resp.Data {
		idField, ok := st.Fields["id"]
		if !ok {
			continue
		}
		id := int32(idField.GetNumberValue())

		var totalDep, totalWith float64
		for _, d := range deposits {
			if d.UserId == id {
				totalDep = d.TotalDeposit
				break
			}
		}
		for _, w := range withdrawals {
			if w.UserId == id {
				totalWith = w.TotalWithdrawal
				break
			}
		}

		st.Fields["totalDeposit"] = structpb.NewNumberValue(totalDep)
		st.Fields["totalWithdrawal"] = structpb.NewNumberValue(totalWith)
		resultList = append(resultList, st)
	}

	return &walletBuffer.CommonResponseArray{
		Success: true,
		Status:  200,
		Message: "Retail transactions retrieved successfully",
		Data:    resultList,
	}, nil
}

func (s *RetailService) ListClientRetailTransactions(data *walletBuffer.RetailDataRequest) (*walletBuffer.CommonResponseArray, error) {
	// 1. Fetch all agents for the client
	agentsRes, err := s.Identity.GetAgents(&identity.ClientIdRequest{ClientId: data.ClientId})
	if err != nil {
		return &walletBuffer.CommonResponseArray{
			Success: false,
			Status:  500,
			Message: fmt.Sprintf("Error fetching agents: %v", err),
		}, nil
	}

	var agentsList []*structpb.Value
	if d, ok := agentsRes.Data.Fields["data"]; ok {
		agentsList = d.GetListValue().Values
	}

	if len(agentsList) == 0 {
		return &walletBuffer.CommonResponseArray{
			Success: true,
			Status:  200,
			Message: "No agents found for this client",
		}, nil
	}

	// 2. Fetch users for all agents in parallel
	type AgentUsersResult struct {
		AgentId   int32
		AgentName string
		Users     []*structpb.Struct
	}
	resultsChan := make(chan AgentUsersResult, len(agentsList))
	
	ctx := context.Background()
	_ = ctx // to be used if needed

	for _, agentVal := range agentsList {
		agentObj := agentVal.GetStructValue()
		agentId := int32(agentObj.Fields["id"].GetNumberValue())
		agentName := agentObj.Fields["username"].GetStringValue()
		
		go func(aid int32, aname string) {
			resp, err := s.Identity.ListAgentUsers(&identity.GetAgentUsersRequest{
				ClientId: data.ClientId,
				UserId:   &aid,
			})
			if err != nil {
				resultsChan <- AgentUsersResult{AgentId: aid, AgentName: aname, Users: nil}
				return
			}
			resultsChan <- AgentUsersResult{AgentId: aid, AgentName: aname, Users: resp.Data}
		}(agentId, agentName)
	}

	// 3. Collect all users and their unique IDs
	var allUsers []map[string]interface{}
	userIdsSet := make(map[int32]bool)
	var userIds []int32

	for i := 0; i < len(agentsList); i++ {
		res := <-resultsChan
		for _, u := range res.Users {
			uid := int32(u.Fields["id"].GetNumberValue())
			
			uMap := make(map[string]interface{})
			for k, v := range u.Fields {
				uMap[k] = v.AsInterface()
			}
			uMap["agentId"] = res.AgentId
			uMap["agentName"] = res.AgentName
			
			allUsers = append(allUsers, uMap)
			if !userIdsSet[uid] {
				userIdsSet[uid] = true
				userIds = append(userIds, uid)
			}
		}
	}

	if len(userIds) == 0 {
		return &walletBuffer.CommonResponseArray{
			Success: true,
			Status:  200,
			Message: "No users found for these agents",
		}, nil
	}

	// 4. Batch fetch totals for all users
	type TransactionTotal struct {
		UserId          int32   `gorm:"column:userId"`
		TotalDeposit    float64 `gorm:"column:totalDeposit"`
		TotalWithdrawal float64 `gorm:"column:totalWithdrawal"`
	}

	var deposits []TransactionTotal
	tx := s.DB.Model(&models.Transaction{}).
		Select("user_id as userId, SUM(amount) as totalDeposit").
		Where("user_id IN ?", userIds).
		Where("client_id = ?", data.ClientId).
		Where("tranasaction_type = ?", "credit").
		Where("subject = ?", "Funds Transfer").
		Group("user_id")

	if data.From != nil && data.To != nil {
		tx = tx.Where("created_at BETWEEN ? AND ?", *data.From, *data.To)
	}
	tx.Scan(&deposits)

	var withdrawals []TransactionTotal
	tx = s.DB.Model(&models.Transaction{}).
		Select("user_id as userId, SUM(amount) as totalWithdrawal").
		Where("user_id IN ?", userIds).
		Where("client_id = ?", data.ClientId).
		Where("tranasaction_type = ?", "debit").
		Where("subject = ?", "shop").
		Group("user_id")

	if data.From != nil && data.To != nil {
		tx = tx.Where("created_at BETWEEN ? AND ?", *data.From, *data.To)
	}
	tx.Scan(&withdrawals)

	// 5. Merge totals back to users
	var finalData []*structpb.Struct
	for _, user := range allUsers {
		uid := int32(user["id"].(float64))
		
		var totalDep, totalWith float64
		for _, d := range deposits {
			if d.UserId == uid {
				totalDep = d.TotalDeposit
				break
			}
		}
		for _, w := range withdrawals {
			if w.UserId == uid {
				totalWith = w.TotalWithdrawal
				break
			}
		}

		user["totalDeposit"] = totalDep
		user["totalWithdrawal"] = totalWith
		
		st, _ := structpb.NewStruct(user)
		finalData = append(finalData, st)
	}

	return &walletBuffer.CommonResponseArray{
		Success: true,
		Status:  200,
		Message: "Client retail transactions retrieved successfully",
		Data:    finalData,
	}, nil
}
