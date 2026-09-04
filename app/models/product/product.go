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

	Sku      SKU
	Name     string
	Type     Type
	Price    currency.Amount
	Currency currency.Currency
	Image    string
}

type ProductRepository interface {
	Create(ctx context.Context, order *Product) (uint, error)
	GetIdsBySKU(ctx context.Context, sku []SKU) ([]uint, error)
}

type Service struct {
	repo ProductRepository
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
