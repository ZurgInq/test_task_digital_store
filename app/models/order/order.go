package order

import (
	"app/models/currency"
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
type GroupID string

const (
	StatusCreated        = "created"
	StatusPaid           = "paid"
	StatusDelivering     = "delivering"
	StatusDelivered      = "delivered"
	StatusPaymentFailed  = "payment_failed"
	StatusOutOfStock     = "out_of_stock"
	StatusDeliveryFailed = "delivery_failed"
	StatusCancelled      = "cancelled"
)

// Заказ
type Order struct {
	gorm.Model

	GroupID   GroupID // ИД для группировки заказов из нескольких товаров
	ExtID     string  // ИД для внешних сервисов
	UserID    uint
	Status    Status
	ProductID uint
	Amount    currency.Amount // Зафиксированная сумма для отмены заказа
	Code      string          // Выданный код
}

type OrderRepository interface {
	Create(ctx context.Context, order *Order) (uint, error)
	GetByID(ctx context.Context, id uint) (Order, error)
	UpdateByID(ctx context.Context, ID uint, order Order) error
	UpdateStatusByExtID(ctx context.Context, extID string, oldStatus Status, newStatus Status) (bool, error)
	GetAll(ctx context.Context, offset int, limit int) ([]Order, error)
	GetByExtID(ctx context.Context, extID string) (*Order, error)
	GetOrdersByStatus(ctx context.Context, status []Status, offset int, limit int) ([]Order, error)
	GetFirstByGroupID(ctx context.Context, groupID GroupID) (Order, error)
	FindByGroupID(ctx context.Context, groupID GroupID) ([]Order, error)
	FindIDByCode(ctx context.Context, code string) (*uint, error)
}

type Service struct {
	repo     OrderRepository
	products *product.Service
	queue    *queue.MockQueue
	log      *slog.Logger
}

func (s *Service) ExistsByCode(ctx context.Context, code string) (bool, error) {
	id, err := s.repo.FindIDByCode(ctx, code)
	if err != nil {
		return false, err
	}

	return id != nil, nil
}

func (s *Service) FindByGroupID(ctx context.Context, orderGroupID GroupID) ([]Order, error) {
	return s.repo.FindByGroupID(ctx, orderGroupID)
}

const (
	orderPaidQueue = "orders:paid"
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

func (s *Service) CreateOrder(
	ctx context.Context,
	userId uint,
	groupID GroupID,
	productID uint,
	extID string,
	amount currency.Amount,
) (Order, error) {
	order := &Order{
		UserID:    userId,
		GroupID:   groupID,
		Status:    StatusCreated,
		ProductID: productID,
		ExtID:     extID,
		Amount:    amount,
	}
	_, err := s.repo.Create(ctx, order)
	if err != nil {
		return *order, fmt.Errorf("create order: %w", err)
	}

	return *order, nil
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

func (s *Service) GetFirstByGroupID(ctx context.Context, groupID GroupID) (Order, error) {
	return s.repo.GetFirstByGroupID(ctx, groupID)
}

func (s *Service) SaveOrderPaidEvent(ctx context.Context, order Order) error {
	orderData, err := json.Marshal(order)
	if err != nil {
		return fmt.Errorf("save order paid event: %w", err)
	}

	s.log.Info("Publish paid event", "orderID", order.ID)
	return s.queue.Publish(orderPaidQueue, string(orderData))
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
