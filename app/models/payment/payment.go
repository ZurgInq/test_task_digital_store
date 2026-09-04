package payment

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"app/models/currency"
	"app/models/order"
	"app/queue"

	"gorm.io/gorm"
)

type Status string

const (
	StatusPaid   Status = "paid"
	StatusFailed Status = "failed"
)

const (
	paymentQueue string = "payments"
)

type PaymentRepository interface {
	Create(ctx context.Context, payment *Payment) (uint, error)
	ExistsByEventID(ctx context.Context, eventID string) (bool, error)
	GetByEventID(ctx context.Context, eventID string) (*Payment, error)
}

// Результат оплаты
type Payment struct {
	gorm.Model

	EventID   string // ИД для идемпотентности
	OrderID   string // ИД заказа (orderExtId)
	Status    Status // Статус платежа paid/failed
	Amount    currency.Amount
	Currency  currency.Currency
	CreatedAt time.Time
}

type Service struct {
	log    *slog.Logger
	queue  *queue.MockQueue
	repo   PaymentRepository
	orders *order.Service
}

func New(
	repo PaymentRepository,
	log *slog.Logger,
	orders *order.Service,
	queue *queue.MockQueue,
) *Service {
	return &Service{
		repo:   repo,
		log:    log,
		queue:  queue,
		orders: orders,
	}
}

func (s *Service) SaveCreatePaymentEvent(ctx context.Context, payment Payment) error {
	data, err := json.Marshal(payment)
	if err != nil {
		return fmt.Errorf("marshal payment data: %w", err)
	}

	s.log.Info("publish payment to queue", "queue", paymentQueue, "data", string(data))
	return s.queue.Publish(paymentQueue, string(data))
}

func (s *Service) PollCreatePaymentEvents(
	ctx context.Context,
	pollInterval time.Duration,
	callback func(context.Context, Payment) error,
) {
	queue.PollEvents(
		ctx,
		s.log,
		s.queue,
		paymentQueue,
		pollInterval,
		callback,
	)
}
