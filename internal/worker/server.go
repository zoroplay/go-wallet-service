package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"wallet-service/internal/consumers"

	"github.com/hibiken/asynq"
)

type Worker struct {
	Processor *consumers.PaymentProcessor
}

func NewWorker(processor *consumers.PaymentProcessor) *Worker {
	return &Worker{
		Processor: processor,
	}
}

func (w *Worker) HandleShopDeposit(ctx context.Context, t *asynq.Task) error {
	var p consumers.ShopDepositDTO
	if err := json.Unmarshal(t.Payload(), &p); err != nil {
		return fmt.Errorf("json.Unmarshal failed: %v: %w", err, asynq.SkipRetry)
	}
	w.Processor.ProcessShopDeposit(p)
	return nil
}

func (w *Worker) HandleCredit(ctx context.Context, t *asynq.Task) error {
	var p consumers.CreditDTO
	if err := json.Unmarshal(t.Payload(), &p); err != nil {
		return fmt.Errorf("json.Unmarshal failed: %v: %w", err, asynq.SkipRetry)
	}
	w.Processor.ProcessCredit(p)
	return nil
}

func (w *Worker) HandleWithdrawalRequest(ctx context.Context, t *asynq.Task) error {
	var p consumers.WithdrawalJobDTO
	if err := json.Unmarshal(t.Payload(), &p); err != nil {
		return fmt.Errorf("json.Unmarshal failed: %v: %w", err, asynq.SkipRetry)
	}
	w.Processor.ProcessWithdrawal(p)
	return nil
}

func (w *Worker) HandleShopWithdrawal(ctx context.Context, t *asynq.Task) error {
	var p consumers.ShopWithdrawalDTO
	if err := json.Unmarshal(t.Payload(), &p); err != nil {
		return fmt.Errorf("json.Unmarshal failed: %v: %w", err, asynq.SkipRetry)
	}
	w.Processor.ProcessShopWithdrawal(p)
	return nil
}

func (w *Worker) HandleCommissionDeposit(ctx context.Context, t *asynq.Task) error {
	var p consumers.CommissionDTO
	if err := json.Unmarshal(t.Payload(), &p); err != nil {
		return fmt.Errorf("json.Unmarshal failed: %v: %w", err, asynq.SkipRetry)
	}
	w.Processor.ProcessCreditCommission(p)
	return nil
}

func (w *Worker) HandleCommissionDebit(ctx context.Context, t *asynq.Task) error {
	var p consumers.CommissionDTO
	if err := json.Unmarshal(t.Payload(), &p); err != nil {
		return fmt.Errorf("json.Unmarshal failed: %v: %w", err, asynq.SkipRetry)
	}
	w.Processor.ProcessDebitCommission(p)
	return nil
}

func (w *Worker) HandleCommissionWithdrawal(ctx context.Context, t *asynq.Task) error {
	var p consumers.CommissionDTO
	if err := json.Unmarshal(t.Payload(), &p); err != nil {
		return fmt.Errorf("json.Unmarshal failed: %v: %w", err, asynq.SkipRetry)
	}
	w.Processor.ProcessCommission(p)
	return nil
}

func (w *Worker) HandleCommissionReverse(ctx context.Context, t *asynq.Task) error {
	var p consumers.CommissionDTO
	if err := json.Unmarshal(t.Payload(), &p); err != nil {
		return fmt.Errorf("json.Unmarshal failed: %v: %w", err, asynq.SkipRetry)
	}
	w.Processor.ProcessReverseCommission(p)
	return nil
}

func (w *Worker) HandleAffiliateCommissionWithdrawal(ctx context.Context, t *asynq.Task) error {
	var p consumers.AffiliateCommissionDTO
	if err := json.Unmarshal(t.Payload(), &p); err != nil {
		return fmt.Errorf("json.Unmarshal failed: %v: %w", err, asynq.SkipRetry)
	}
	w.Processor.ProcessAffiliateCommission(p)
	return nil
}

func (w *Worker) HandleCreditPlayer(ctx context.Context, t *asynq.Task) error {
	var p consumers.CreditPlayerDTO
	if err := json.Unmarshal(t.Payload(), &p); err != nil {
		return fmt.Errorf("json.Unmarshal failed: %v: %w", err, asynq.SkipRetry)
	}
	w.Processor.ProcessCreditPlayer(p)
	return nil
}

func (w *Worker) HandleDebitUser(ctx context.Context, t *asynq.Task) error {
	var p consumers.CreditDTO
	if err := json.Unmarshal(t.Payload(), &p); err != nil {
		return fmt.Errorf("json.Unmarshal failed: %v: %w", err, asynq.SkipRetry)
	}
	w.Processor.DebitUser(p)
	return nil
}

func (w *Worker) HandleMobileMoneyPayout(ctx context.Context, t *asynq.Task) error {
	var p consumers.WithdrawalJobDTO
	if err := json.Unmarshal(t.Payload(), &p); err != nil {
		return fmt.Errorf("json.Unmarshal failed: %v: %w", err, asynq.SkipRetry)
	}
	w.Processor.ProcessMobileMoneyPayout(p)
	return nil
}

func (w *Worker) HandleSmileAndPayPayout(ctx context.Context, t *asynq.Task) error {
	var p consumers.WithdrawalJobDTO
	if err := json.Unmarshal(t.Payload(), &p); err != nil {
		return fmt.Errorf("json.Unmarshal failed: %v: %w", err, asynq.SkipRetry)
	}
	w.Processor.ProcessSmileAndPayPayout(p)
	return nil
}

func StartWorker(redisOpt asynq.RedisClientOpt, processor *consumers.PaymentProcessor) {
	srv := asynq.NewServer(
		redisOpt,
		asynq.Config{
			// Specify how many concurrent workers to use
			Concurrency: 10,
			// Optionally specify multiple queues with different priority.
			Queues: map[string]int{
				"critical": 6,
				"default":  3,
				"low":      1,
			},
			// See the godoc for other configuration options
		},
	)

	worker := NewWorker(processor)
	mux := asynq.NewServeMux()

	mux.HandleFunc(TypeShopDeposit, worker.HandleShopDeposit)
	mux.HandleFunc(TypeCredit, worker.HandleCredit)
	mux.HandleFunc(TypeWithdrawalRequest, worker.HandleWithdrawalRequest)
	mux.HandleFunc(TypeShopWithdrawal, worker.HandleShopWithdrawal)
	mux.HandleFunc(TypeCommissionDeposit, worker.HandleCommissionDeposit)
	mux.HandleFunc(TypeCommissionDebit, worker.HandleCommissionDebit)
	mux.HandleFunc(TypeCommissionWithdrawal, worker.HandleCommissionWithdrawal)
	mux.HandleFunc(TypeCommissionReverse, worker.HandleCommissionReverse)
	mux.HandleFunc(TypeAffiliateCommissionWithdrawal, worker.HandleAffiliateCommissionWithdrawal)
	mux.HandleFunc(TypeCreditPlayer, worker.HandleCreditPlayer)
	mux.HandleFunc(TypeDebitUser, worker.HandleDebitUser)
	mux.HandleFunc(TypeMobileMoneyPayout, worker.HandleMobileMoneyPayout)
	mux.HandleFunc(TypeSmileAndPayPayout, worker.HandleSmileAndPayPayout)

	if err := srv.Run(mux); err != nil {
		log.Fatalf("could not run server: %v", err)
	}
}
