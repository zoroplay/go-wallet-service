package grpc

import (
	"context"
	"log"
	"net"
	"strconv"

	"wallet-service/internal/services"
	pb "wallet-service/proto/wallet"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Server struct {
	pb.UnimplementedWalletServiceServer
	Wallet      *services.WalletService
	Payment     *services.PaymentService
	Withdrawal  *services.WithdrawalService
	Deposit     *services.DepositService
	Reporting   *services.ReportingService
	Commission  *services.CommissionService
	Player      *services.PlayerService
	Paystack    *services.PaystackService
	Flutterwave *services.FlutterwaveService
	Monnify     *services.MonnifyService
	WayaBank    *services.WayaBankService
	WayaQuick   *services.WayaQuickService
	Kora        *services.KorapayService
	Pawa        *services.PawapayService
	Tigo        *services.TigoService
	Providus    *services.ProvidusService
	Fidelity    *services.FidelityService
	Smile       *services.SmileAndPayService
	Payonus     *services.PayonusService
	Dashboard   *services.DashboardService
}

// StartGRPCServer initializes and starts the gRPC server
func StartGRPCServer(
	port string,
	wallet *services.WalletService,
	payment *services.PaymentService,
	withdrawal *services.WithdrawalService,
	deposit *services.DepositService,
	reporting *services.ReportingService,
	commission *services.CommissionService,
	player *services.PlayerService,
	paystack *services.PaystackService,
	flutterwave *services.FlutterwaveService,
	monnify *services.MonnifyService,
	wayaBank *services.WayaBankService,
	wayaQuick *services.WayaQuickService,
	kora *services.KorapayService,
	pawa *services.PawapayService,
	tigo *services.TigoService,
	providus *services.ProvidusService,
	fidelity *services.FidelityService,
	smile *services.SmileAndPayService,
	payonus *services.PayonusService,
	dashboard *services.DashboardService,
) {
	lis, err := net.Listen("tcp", ":"+port)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	s := grpc.NewServer()
	pb.RegisterWalletServiceServer(s, &Server{
		Wallet:      wallet,
		Payment:     payment,
		Withdrawal:  withdrawal,
		Deposit:     deposit,
		Reporting:   reporting,
		Commission:  commission,
		Player:      player,
		Paystack:    paystack,
		Flutterwave: flutterwave,
		Monnify:     monnify,
		WayaBank:    wayaBank,
		WayaQuick:   wayaQuick,
		Kora:        kora,
		Pawa:        pawa,
		Tigo:        tigo,
		Providus:    providus,
		Fidelity:    fidelity,
		Smile:       smile,
		Payonus:     payonus,
		Dashboard:   dashboard,
	})

	log.Printf("gRPC server listening at %v", lis.Addr())
	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}

// Wallet Service Methods
func (s *Server) CreateWallet(ctx context.Context, req *pb.CreateWalletRequest) (*pb.WalletResponse, error) {
	amount := 0.0
	if req.Amount != nil {
		amount = float64(*req.Amount)
	}
	bonus := 0.0
	if req.Bonus != nil {
		bonus = float64(*req.Bonus)
	}

	resp, err := s.Wallet.CreateWallet(services.CreateWalletDTO{
		UserId:   int(req.UserId),
		Username: req.Username,
		ClientId: int(req.ClientId),
		Amount:   amount,
		Bonus:    bonus,
	})
	if err != nil {
		return &pb.WalletResponse{Success: false, Message: err.Error()}, nil
	}
	// Mapping response data stub
	return &pb.WalletResponse{Success: true, Message: resp.Message}, nil
}

func (s *Server) GetBalance(ctx context.Context, req *pb.GetBalanceRequest) (*pb.WalletResponse, error) {
	resp, err := s.Wallet.GetBalance(services.GetBalanceDTO{
		UserId:   int(req.UserId),
		ClientId: int(req.ClientId),
	})
	if err != nil {
		return &pb.WalletResponse{Success: false, Message: err.Error()}, nil
	}
	// Need to map data
	return &pb.WalletResponse{Success: true, Message: resp.Message}, nil
}

func (s *Server) CreditUser(ctx context.Context, req *pb.CreditUserRequest) (*pb.WalletResponse, error) {
	amount, _ := strconv.ParseFloat(req.Amount, 64)
	resp, err := s.Wallet.CreditUser(services.CreditUserDTO{
		UserId:        int(req.UserId),
		ClientId:      int(req.ClientId),
		Username:      req.Username,
		Amount:        amount,
		Wallet:        req.Wallet,
		Subject:       req.Subject,
		Description:   req.Description,
		Channel:       req.Channel,
		Source:        req.Source,
		TransactionNo: "",
	})
	if err != nil {
		return &pb.WalletResponse{Success: false, Message: err.Error()}, nil
	}
	return &pb.WalletResponse{Success: true, Message: resp.Message}, nil
}

func (s *Server) DebitUser(ctx context.Context, req *pb.DebitUserRequest) (*pb.WalletResponse, error) {
	amount, _ := strconv.ParseFloat(req.Amount, 64)
	_, err := s.Wallet.DebitUser(services.DebitUserDTO{
		UserId:      int(req.UserId),
		ClientId:    int(req.ClientId),
		Username:    req.Username,
		Amount:      amount,
		Wallet:      req.Wallet,
		Subject:     req.Subject,
		Description: req.Description,
		Channel:     req.Channel,
		Source:      req.Source,
	})
	if err != nil {
		return &pb.WalletResponse{Success: false, Message: err.Error()}, nil
	}
	return &pb.WalletResponse{Success: true, Message: "Debit successful"}, nil
}

func (s *Server) InititateDeposit(ctx context.Context, req *pb.InitiateDepositRequest) (*pb.InitiateDepositResponse, error) {
	resp, err := s.Payment.InitiateDeposit(services.InitiateDepositRequestDTO{
		ClientId:      int(req.ClientId),
		UserId:        int(req.UserId),
		Amount:        float64(req.Amount),
		Source:        req.Source,
		PaymentMethod: req.PaymentMethod,
	})
	if err != nil {
		return &pb.InitiateDepositResponse{Success: false, Message: err.Error()}, nil
	}

	respMap, _ := resp.(map[string]interface{})
	data, _ := respMap["data"].(map[string]interface{})

	link, _ := data["link"].(string)
	ref, _ := data["transactionRef"].(string)

	return &pb.InitiateDepositResponse{
		Success: true,
		Message: "Initiated",
		Data: &pb.InitiateDepositResponse_Data{
			Link:           &link,
			TransactionRef: &ref,
		},
	}, nil
}

func (s *Server) RequestWithdrawal(ctx context.Context, req *pb.WithdrawRequest) (*pb.WithdrawResponse, error) {
	// Type assertion handling for optional fields
	wType := ""
	if req.Type != nil {
		wType = *req.Type
	}
	bkCode := ""
	if req.BankCode != nil {
		bkCode = *req.BankCode
	}
	bkName := ""
	if req.BankName != nil {
		bkName = *req.BankName
	}

	resp, err := s.Withdrawal.RequestWithdrawal(services.WithdrawRequestDTO{
		ClientId:      int(req.ClientId),
		UserId:        int(req.UserId),
		Amount:        req.Amount,
		Type:          wType,
		AccountNumber: req.AccountNumber,
		AccountName:   req.AccountName,
		BankCode:      bkCode,
		BankName:      bkName,
	})
	if err != nil {
		return &pb.WithdrawResponse{Success: false, Message: err.Error()}, nil
	}

	respMap, _ := resp.(map[string]interface{})
	// Extract code if needed
	_ = respMap

	return &pb.WithdrawResponse{Success: true, Message: "Withdrawal requested"}, nil
}

func (s *Server) ListWithdrawals(ctx context.Context, req *pb.ListWithdrawalRequests) (*pb.CommonResponseObj, error) {
	// Stub using WithdrawalService
	// s.Withdrawal.ListWithdrawalRequest(...)
	return &pb.CommonResponseObj{Success: true, Message: "Listed"}, nil
}

func (s *Server) ListDeposits(ctx context.Context, req *pb.ListDepositRequests) (*pb.CommonResponseObj, error) {
	var status *int
	if req.Status != "" {
		sVal, err := strconv.Atoi(req.Status)
		if err == nil {
			status = &sVal
		}
	}
	_, err := s.Wallet.ListDeposits(services.ListDepositsDTO{
		ClientId:      int(req.ClientId),
		StartDate:     req.StartDate,
		EndDate:       req.EndDate,
		PaymentMethod: req.PaymentMethod,
		Status:        status,
		Username:      req.Username,
		TransactionId: req.TransactionId,
		Bank:          req.Bank,
		Page:          int(req.Page),
	})
	if err != nil {
		return &pb.CommonResponseObj{Success: false, Message: err.Error()}, nil
	}
	return &pb.CommonResponseObj{Success: true, Message: "Listed"}, nil
}

func (s *Server) UpdateWithdrawal(ctx context.Context, req *pb.UpdateWithdrawalRequest) (*pb.CommonResponseObj, error) {
	_, err := s.Payment.UpdateWithdrawalStatus(services.UpdateWithdrawalDTO{
		WithdrawalId: int(req.WithdrawalId),
		Status:       req.Action,
		ClientId:     int(req.ClientId),
		Comment:      req.Comment,
		UpdatedBy:    req.UpdatedBy,
	})
	if err != nil {
		return &pb.CommonResponseObj{Success: false, Message: err.Error()}, nil
	}
	return &pb.CommonResponseObj{Success: true, Message: "Updated"}, nil
}

func (s *Server) UpdateCommissionRequest(ctx context.Context, req *pb.UpdateAffiliateCommissionRequest) (*pb.CommonResponseObj, error) {
	_, err := s.Payment.ApproveAndRejectCommissionRequest(services.CommissionApprovalDTO{
		ClientId:      int(req.ClientId),
		UserId:        int(req.UserId),
		Status:        int(req.Status),
		TransactionNo: req.TransactionNo,
		Comment:       "", // Proto missing comment?
		UpdatedBy:     "", // Proto missing?
	})
	if err != nil {
		return &pb.CommonResponseObj{Success: false, Message: err.Error()}, nil
	}
	return &pb.CommonResponseObj{Success: true, Message: "Updated"}, nil
}

func (s *Server) FetchBetRange(ctx context.Context, req *pb.FetchBetRangeRequest) (*pb.FetchBetRangeResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method FetchBetRange not implemented")
}

func (s *Server) CreateVirtualAccount(ctx context.Context, req *pb.WayaBankRequest) (*pb.CommonResponseObj, error) {
	// Stub map logic
	// s.WayaBank.CreateVirtualAccount(...)
	return &pb.CommonResponseObj{Success: true, Message: "Virtual Account Stub"}, nil
}

// STUBS for Missing Services

func (s *Server) CashbookVerifyFinalTransaction(context.Context, *pb.FetchLastApprovedRequest) (*pb.CommonResponseObj, error) {
	return nil, status.Errorf(codes.Unimplemented, "method CashbookVerifyFinalTransaction not implemented")
}
func (s *Server) CashbookFetchLastApproved(context.Context, *pb.FetchLastApprovedRequest) (*pb.LastApprovedResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method CashbookFetchLastApproved not implemented")
}
func (s *Server) CashbookFetchSalesReport(context.Context, *pb.FetchSalesReportRequest) (*pb.SalesReportResponseArray, error) {
	return nil, status.Errorf(codes.Unimplemented, "method CashbookFetchSalesReport not implemented")
}
func (s *Server) CashbookFetchReport(context.Context, *pb.FetchReportRequest) (*pb.FetchReportResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method CashbookFetchReport not implemented")
}
func (s *Server) CashbookHandleReport(context.Context, *pb.HandleReportRequest) (*pb.LastApprovedResponseObj, error) {
	return nil, status.Errorf(codes.Unimplemented, "method CashbookHandleReport not implemented")
}
func (s *Server) CashbookFetchMonthlyShopReport(context.Context, *pb.FetchReportRequest) (*pb.CommonResponseObj, error) {
	return nil, status.Errorf(codes.Unimplemented, "method CashbookFetchMonthlyShopReport not implemented")
}
func (s *Server) CurrentReport(context.Context, *pb.FetchReportRequest) (*pb.CommonResponseObj, error) {
	return nil, status.Errorf(codes.Unimplemented, "method CurrentReport not implemented")
}
func (s *Server) CashbookApproveExpense(context.Context, *pb.CashbookApproveExpenseRequest) (*pb.ExpenseSingleResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method CashbookApproveExpense not implemented")
}
func (s *Server) CashbookCreateExpense(context.Context, *pb.CashbookCreateExpenseRequest) (*pb.ExpenseSingleResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method CashbookCreateExpense not implemented")
}
func (s *Server) CashbookFindAllExpense(context.Context, *pb.EmptyRequest) (*pb.ExpenseRepeatedResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method CashbookFindAllExpense not implemented")
}
func (s *Server) CashbookFindOneExpense(context.Context, *pb.CashbookIdRequest) (*pb.ExpenseSingleResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method CashbookFindOneExpense not implemented")
}
func (s *Server) CashbookDeleteOneExpense(context.Context, *pb.CashbookIdRequest) (*pb.ExpenseSingleResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method CashbookDeleteOneExpense not implemented")
}
func (s *Server) CashbookUpdateOneExpense(context.Context, *pb.CashbookCreateExpenseRequest) (*pb.ExpenseSingleResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method CashbookUpdateOneExpense not implemented")
}
func (s *Server) CashbookFindAllBranchExpense(context.Context, *pb.BranchRequest) (*pb.ExpenseRepeatedResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method CashbookFindAllBranchExpense not implemented")
}
func (s *Server) CashbookCreateExpenseType(context.Context, *pb.CashbookCreateExpenseTypeRequest) (*pb.ExpenseTypeSingleResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method CashbookCreateExpenseType not implemented")
}
func (s *Server) CashbookFindAllExpenseType(context.Context, *pb.EmptyRequest) (*pb.ExpenseTypeRepeatedResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method CashbookFindAllExpenseType not implemented")
}
func (s *Server) CashbookApproveCashIn(context.Context, *pb.CashbookApproveCashInOutRequest) (*pb.CashInOutSingleResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method CashbookApproveCashIn not implemented")
}
func (s *Server) CashbookCreateCashIn(context.Context, *pb.CashbookCreateCashInOutRequest) (*pb.CashInOutSingleResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method CashbookCreateCashIn not implemented")
}
func (s *Server) CashbookUpdateCashIn(context.Context, *pb.CashbookCreateCashInOutRequest) (*pb.CashInOutSingleResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method CashbookUpdateCashIn not implemented")
}
func (s *Server) CashbookDeleteOneCashIn(context.Context, *pb.CashbookIdRequest) (*pb.CashInOutSingleResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method CashbookDeleteOneCashIn not implemented")
}
func (s *Server) CashbookFindOneCashIn(context.Context, *pb.CashbookIdRequest) (*pb.CashInOutSingleResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method CashbookFindOneCashIn not implemented")
}
func (s *Server) CashbookFindAllCashIn(context.Context, *pb.EmptyRequest) (*pb.CashInOutRepeatedResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method CashbookFindAllCashIn not implemented")
}
func (s *Server) CashbookFindAllBranchCashIn(context.Context, *pb.BranchRequest) (*pb.CashInOutRepeatedResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method CashbookFindAllBranchCashIn not implemented")
}
func (s *Server) FindAllBranchApprovedCashinWDate(context.Context, *pb.BranchRequest) (*pb.CashInOutRepeatedResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method FindAllBranchApprovedCashinWDate not implemented")
}
func (s *Server) FindAllBranchPendingCashinWDate(context.Context, *pb.BranchRequest) (*pb.CashInOutRepeatedResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method FindAllBranchPendingCashinWDate not implemented")
}
func (s *Server) CashbookApproveCashOut(context.Context, *pb.CashbookApproveCashInOutRequest) (*pb.CashInOutSingleResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method CashbookApproveCashOut not implemented")
}
func (s *Server) CashbookCreateCashOut(context.Context, *pb.CashbookCreateCashInOutRequest) (*pb.CashInOutSingleResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method CashbookCreateCashOut not implemented")
}
func (s *Server) CashbookUpdateCashOut(context.Context, *pb.CashbookCreateCashInOutRequest) (*pb.CashInOutSingleResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method CashbookUpdateCashOut not implemented")
}
func (s *Server) CashbookDeleteOneCashOut(context.Context, *pb.CashbookIdRequest) (*pb.CashInOutSingleResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method CashbookDeleteOneCashOut not implemented")
}
func (s *Server) CashbookFindOneCashOut(context.Context, *pb.CashbookIdRequest) (*pb.CashInOutSingleResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method CashbookFindOneCashOut not implemented")
}
func (s *Server) CashbookFindAllCashOut(context.Context, *pb.EmptyRequest) (*pb.CashInOutRepeatedResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method CashbookFindAllCashOut not implemented")
}
func (s *Server) CashbookFindAllBranchCashOut(context.Context, *pb.BranchRequest) (*pb.CashInOutRepeatedResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method CashbookFindAllBranchCashOut not implemented")
}

// Add other missing stubs similarly to avoid compilation errors for interface satisfaction
