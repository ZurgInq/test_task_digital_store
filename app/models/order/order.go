package order

import (
	"app/models/product"
	"app/queue"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"gorm.io/gorm"
)

type Status string

const (
	StatusCreated        = "created"
	StatusPaid           = "paid"
	StatusDelivering     = "delivering"
	StatusDelivered      = "delivered"
	StatusPaymentFailed  = "payment_failed"
	StatusOutOfStock     = "out_of_stock"
	StatusDeliveryFailed = "delivery_failed"
)

// Заказ
type Order struct {
	gorm.Model

	ExtID      *string // ИД для внешних сервисов. Генерируется во время initPayment
	UserId     uint
	Status     Status
	ProductIds []uint `gorm:"serializer:json"`
	Code       string // Выданный код
}

type OrderRepository interface {
	Create(ctx context.Context, order *Order) (uint, error)
	GetByID(ctx context.Context, id uint) (Order, error)
	UpdateByID(ctx context.Context, ID uint, order Order) error
	UpdateStatusByExtID(ctx context.Context, extID string, oldStatus Status, newStatus Status) (bool, error)
	UpdateExtID(ctx context.Context, ID uint, extID string) error
	GetAll(ctx context.Context, offset int, limit int) ([]Order, error)
	GetByExtID(ctx context.Context, extID string) (*Order, error)
	GetOrdersByStatus(ctx context.Context, status []Status, offset int, limit int) ([]Order, error)
}

type Service struct {
	repo     OrderRepository
	products *product.Service
	queue    *queue.MockQueue
	log      *slog.Logger
}

const (
	orderPaidQueue    = "order-paid"
	orderCreatedQueue = "order-created"
)

func NewService(
	repo OrderRepository,
	productsRepo *product.Service,
	queue *queue.MockQueue,
	log *slog.Logger,
) *Service {
	return &Service{
		repo:     repo,
		products: productsRepo,
		queue:    queue,
		log:      log,
	}
}

func (s *Service) CreateOrder(ctx context.Context, userId uint, sku []product.SKU, extID *string) (Order, error) {
	productIds, err := s.products.GetIdsBySKU(ctx, sku)
	if err != nil {
		return Order{}, fmt.Errorf("get products: %w", err)
	}

	if len(productIds) == 0 {
		return Order{}, fmt.Errorf("products not found")
	}

	order := &Order{
		UserId:     userId,
		Status:     StatusCreated,
		ProductIds: productIds,
		ExtID:      extID,
	}
	_, err = s.repo.Create(ctx, order)

	if err != nil {
		return *order, fmt.Errorf("create order: %w", err)
	}

	return *order, nil
}

func (s *Service) UpdateExtID(ctx context.Context, id uint, extID string) error {
	return s.repo.UpdateExtID(ctx, id, extID)
}

func (s *Service) UpdateByID(ctx context.Context, id uint, order Order) error {
	return s.repo.UpdateByID(ctx, id, order)
}

func (s *Service) UpdateStatus(
	ctx context.Context,
	extID string,
	oldStatus Status,
	newStatus Status,
) (bool, error) {
	return s.repo.UpdateStatusByExtID(ctx, extID, oldStatus, newStatus)
}

func (s *Service) GetByID(ctx context.Context, id uint) (Order, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *Service) GetAll(ctx context.Context, offset int, limit int) ([]Order, error) {
	return s.repo.GetAll(ctx, offset, limit)
}

func (s *Service) GetOrdersByStatus(ctx context.Context, status []Status, offset int, limit int) ([]Order, error) {
	return s.repo.GetOrdersByStatus(ctx, status, offset, limit)
}

func (s *Service) GetByExtID(ctx context.Context, extID string) (*Order, error) {
	return s.repo.GetByExtID(ctx, extID)
}

func (s *Service) SaveOrderPaidEvent(ctx context.Context, order Order) error {
	orderData, err := json.Marshal(order)
	if err != nil {
		return fmt.Errorf("save order paid event: %w", err)
	}

	return s.queue.Publish(orderPaidQueue, string(orderData))
}

func (s *Service) SaveOrderCreatedEvent(ctx context.Context, order Order) error {
	orderData, err := json.Marshal(order)
	if err != nil {
		return fmt.Errorf("save order paid event: %w", err)
	}

	return s.queue.Publish(orderCreatedQueue, string(orderData))
}

func (s *Service) PollOrderCreatedEvents(
	ctx context.Context,
	pollInterval time.Duration,
	callback func(context.Context, Order) error,
) {
	queue.PollEvents(
		ctx,
		s.log,
		s.queue,
		orderCreatedQueue,
		pollInterval,
		callback,
	)
}

func (s *Service) PollOrderPaidEvents(
	ctx context.Context,
	pollInterval time.Duration,
	callback func(context.Context, Order) error,
) {
	queue.PollEvents(
		ctx,
		s.log,
		s.queue,
		orderPaidQueue,
		pollInterval,
		callback,
	)
}
