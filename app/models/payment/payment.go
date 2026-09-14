package payment

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	currencyPkg "app/models/currency"
	orderPkg "app/models/order"
	"app/queue"

	"gorm.io/gorm"
)

type Status string

const (
	StatusInit   Status = "init"   // Платёж создан, запрос в сервис платежей
	StatusAwait  Status = "await"  // Ждём подтверждение от внешнего сервиса
	StatusPaid   Status = "paid"   // Оплачено
	StatusFailed Status = "failed" // Оплата отменена
)

const (
	paymentInitQueue   string = "payments:init"
	paymentUpdateQueue string = "payments:update"
)

type PaymentRepository interface {
	Create(ctx context.Context, payment *Payment) (uint, error)
	GetByID(ctx context.Context, id uint) (Payment, error)
	GetPaidByOrderGroupID(ctx context.Context, orderGroupID orderPkg.GroupID) (Payment, error)
	GetAwaitByOrderGroupID(ctx context.Context, orderGroupID orderPkg.GroupID) (Payment, error)
	ExistsByEventID(ctx context.Context, eventID string) (bool, error)
	GetByEventID(ctx context.Context, eventID string) (Payment, error)
	FindByEventID(ctx context.Context, eventID string) (*Payment, error)
	UpdateStatusFromStatus(ctx context.Context, id uint, from Status, to Status) (bool, error)
	UpdateEventID(ctx context.Context, id uint, eventID string) error
}

// Платёж
type Payment struct {
	gorm.Model

	EventID      *string // ИД для идемпотентности
	UserID       uint
	OrderGroupID orderPkg.GroupID
	Status       Status // Статус платежа paid/failed
	Amount       currencyPkg.Amount
	Currency     currencyPkg.Currency
}

// Событие оплаты
type PaymentEvent struct {
	EventID   string // ИД для идемпотентности
	OrderID   string
	Status    Status // Статус платежа paid/failed
	Amount    currencyPkg.Amount
	Currency  currencyPkg.Currency
	CreatedAt time.Time
}

type Service struct {
	log    *slog.Logger
	queue  *queue.MockQueue
	repo   PaymentRepository
	orders *orderPkg.Service
}

func New(
	repo PaymentRepository,
	log *slog.Logger,
	orders *orderPkg.Service,
	queue *queue.MockQueue,
) *Service {
	return &Service{
		repo:   repo,
		log:    log,
		queue:  queue,
		orders: orders,
	}
}

func (s *Service) Create(
	ctx context.Context,
	userID uint,
	orderGroupID orderPkg.GroupID,
	paymentAmount currencyPkg.Amount,
	currency currencyPkg.Currency,
) (uint, error) {
	return s.repo.Create(ctx, &Payment{
		UserID:       userID,
		OrderGroupID: orderGroupID,
		Status:       StatusInit,
		Amount:       paymentAmount,
		Currency:     currency,
	})
}

func (s *Service) GetByID(ctx context.Context, paymentID uint) (Payment, error) {
	return s.repo.GetByID(ctx, paymentID)
}

func (s *Service) GetPaidByOrderGroupID(ctx context.Context, orderGroupID orderPkg.GroupID) (Payment, error) {
	return s.repo.GetPaidByOrderGroupID(ctx, orderGroupID)
}

func (s *Service) GetAwaitByOrderGroupID(ctx context.Context, orderGroupID orderPkg.GroupID) (Payment, error) {
	return s.repo.GetAwaitByOrderGroupID(ctx, orderGroupID)
}

func (s *Service) StatusToAwait(ctx context.Context, id uint, fromStatus Status, toStatus Status) (bool, error) {
	return s.repo.UpdateStatusFromStatus(ctx, id, fromStatus, toStatus)
}

func (s Service) SaveInitEvent(ctx context.Context, payment Payment) error {
	data, err := json.Marshal(payment)
	if err != nil {
		return fmt.Errorf("marshal payment data: %w", err)
	}

	queue := paymentInitQueue
	s.log.Info("publish payment to queue", "queue", queue, "data", string(data))
	return s.queue.Publish(queue, string(data))
}

func (s *Service) PollInitEvents(
	ctx context.Context,
	pollInterval time.Duration,
	callback func(context.Context, Payment) error,
) {
	queue.PollEvents(
		ctx,
		s.log,
		s.queue,
		paymentInitQueue,
		pollInterval,
		callback,
	)
}

func (s *Service) SaveUpdateEvent(ctx context.Context, event PaymentEvent) error {
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal event: %w", err)
	}

	s.log.Info("publish event to queue", "queue", paymentUpdateQueue, "data", string(data))
	return s.queue.Publish(paymentUpdateQueue, string(data))
}

func (s *Service) PollUpdateEvents(
	ctx context.Context,
	pollInterval time.Duration,
	callback func(context.Context, PaymentEvent) error,
) {
	queue.PollEvents(
		ctx,
		s.log,
		s.queue,
		paymentUpdateQueue,
		pollInterval,
		callback,
	)
}
