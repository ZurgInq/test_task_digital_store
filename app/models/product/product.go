package product

import (
	"app/models/currency"
	"context"

	"gorm.io/gorm"
)

type Type string
type SKU string

const (
	TypeTopUp        = "topup"
	TypeKey          = "key"
	TypeSubscription = "subscription"
	TypeGiftcard     = "giftcard"
)

type Product struct {
	gorm.Model

	SKU      SKU
	Name     string
	Type     Type
	Price    currency.Amount
	Currency currency.Currency
	Image    string
}

type ProductRepository interface {
	Create(ctx context.Context, order *Product) (uint, error)
	GetIdsBySKU(ctx context.Context, sku []SKU) ([]uint, error)
	GetBySKU(ctx context.Context, sku SKU) (Product, error)
	GetBySKUs(ctx context.Context, sku []SKU) ([]Product, error)
	GetByID(ctx context.Context, id uint) (Product, error)
}

type Service struct {
	repo ProductRepository
}

func (s *Service) GetBySKUs(ctx context.Context, skuList []SKU) ([]Product, error) {
	return s.repo.GetBySKUs(ctx, skuList)
}

func NewService(
	repo ProductRepository,
) *Service {
	return &Service{
		repo: repo,
	}
}

func (s *Service) GetIdsBySKU(ctx context.Context, sku []SKU) ([]uint, error) {
	return s.repo.GetIdsBySKU(ctx, sku)
}

func (s *Service) GetByID(ctx context.Context, id uint) (Product, error) {
	return s.repo.GetByID(ctx, id)
}
