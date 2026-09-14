package balance

import (
	"context"
	"log/slog"
	"time"

	"app/models/currency"
)

type Operation string

const (
	OperationDeposit = "deposit" // Пополнение (+)
	OperationRefund  = "refund"  // Возврат (+)
	OperationCharge  = "charge"  // Списание (-)
)

type TransactionRepository interface {
	Create(ctx context.Context, transaction *BalanceTransaction) (uint, error)
	CreateOrIgnore(ctx context.Context, transaction *BalanceTransaction) (uint, error)
	GetAmountForUser(ctx context.Context, userID uint) (int, error)
	GetTransactions(ctx context.Context, userID uint) ([]BalanceTransaction, error)
}

type BalanceTransaction struct {
	ID        uint `gorm:"primarykey"`
	CreatedAt time.Time
	UserID    uint
	OrderID   *uint
	PaymentID *uint
	Amount    int64
	Operation Operation
}

type Service struct {
	log  *slog.Logger
	repo TransactionRepository
}

func (s *Service) GetTransactions(ctx context.Context, userID uint) ([]BalanceTransaction, error) {
	return s.repo.GetTransactions(ctx, userID)
}

func NewService(
	log *slog.Logger,
	repo TransactionRepository,
) *Service {
	return &Service{
		log:  log,
		repo: repo,
	}
}

func (s *Service) CreateTransaction(ctx context.Context, transaction *BalanceTransaction) error {
	_, err := s.repo.Create(ctx, transaction)
	return err
}

func (s *Service) CreateDeposit(ctx context.Context, userID uint, paymentID uint, amount currency.Amount) error {
	return s.CreateOrIgnore(ctx, &BalanceTransaction{
		UserID:    userID,
		PaymentID: &paymentID,
		Amount:    int64(amount),
		Operation: OperationDeposit,
	})
}

func (s *Service) CreateRefund(ctx context.Context, userID uint, orderID uint, amount currency.Amount) error {
	return s.CreateOrIgnore(ctx, &BalanceTransaction{
		UserID:    userID,
		OrderID:   &orderID,
		Amount:    int64(amount),
		Operation: OperationRefund,
	})
}

func (s *Service) CreateCharge(ctx context.Context, userID uint, orderID uint, amount currency.Amount) error {
	return s.CreateOrIgnore(ctx, &BalanceTransaction{
		UserID:    userID,
		OrderID:   &orderID,
		Amount:    -int64(amount),
		Operation: OperationCharge,
	})
}

func (s *Service) CreateOrIgnore(ctx context.Context, balanceTransaction *BalanceTransaction) error {
	id, err := s.repo.CreateOrIgnore(ctx, balanceTransaction)
	s.log.Info(
		"Transaction created",
		"id", id,
		"operation", balanceTransaction.Operation,
		"orderID", balanceTransaction.OrderID,
		"paymentID", balanceTransaction.PaymentID,
		"amount", balanceTransaction.Amount,
	)

	return err
}

func (s *Service) GetAmountForUser(ctx context.Context, userID uint) (int, error) {
	return s.repo.GetAmountForUser(ctx, userID)
}
