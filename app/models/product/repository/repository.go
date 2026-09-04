package repository

import (
	"context"
	"fmt"

	"app/db"
	modelsProduct "app/models/product"

	"gorm.io/gorm"
)

type repo struct {
	db *db.DB
}

func New(
	db *db.DB,
) *repo {
	return &repo{
		db: db,
	}
}

func (r *repo) Create(ctx context.Context, order *modelsProduct.Product) (uint, error) {
	result := gorm.WithResult()
	db := r.db.DB(ctx)

	err := gorm.G[modelsProduct.Product](db, result).Create(ctx, order)
	if err != nil {
		return 0, fmt.Errorf("Create product: %w", err)
	}

	return order.ID, result.Error
}

func (r *repo) GetById(ctx context.Context, id uint) (modelsProduct.Product, error) {
	result := gorm.WithResult()
	db := r.db.DB(ctx)

	order, err := gorm.G[modelsProduct.Product](db, result).Where("id = ?", id).First(ctx)
	if err != nil {
		return order, fmt.Errorf("get product by id %d: %w", id, err)
	}

	return order, nil
}

func (r *repo) GetIdsBySKU(ctx context.Context, sku []modelsProduct.SKU) ([]uint, error) {
	db := r.db.DB(ctx)
	products := make([]modelsProduct.Product, 0, len(sku))

	err := db.WithContext(ctx).Debug().Select("id").Where("sku IN ?", sku).Limit(len(sku)).Find(&products).Error
	if err != nil {
		return nil, fmt.Errorf("get ids by sku: %w", err)
	}

	ids := make([]uint, 0, len(products))
	for _, p := range products {
		ids = append(ids, p.ID)
	}

	return ids, nil
}
