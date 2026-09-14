package repository

import (
	"context"
	"fmt"

	"app/db"
	productPkg "app/models/product"

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

func (r *repo) Create(ctx context.Context, product *productPkg.Product) (uint, error) {
	db := r.db.DB(ctx)

	err := gorm.G[productPkg.Product](db).Create(ctx, product)
	if err != nil {
		return 0, fmt.Errorf("Create product: %w", err)
	}

	return product.ID, nil
}

func (r *repo) GetByID(ctx context.Context, id uint) (productPkg.Product, error) {
	result := gorm.WithResult()
	db := r.db.DB(ctx)

	order, err := gorm.G[productPkg.Product](db, result).Where("id = ?", id).First(ctx)
	if err != nil {
		return order, fmt.Errorf("get product by id %d: %w", id, err)
	}

	return order, nil
}

func (r *repo) GetIdsBySKU(ctx context.Context, sku []productPkg.SKU) ([]uint, error) {
	db := r.db.DB(ctx)
	products := make([]productPkg.Product, 0, len(sku))

	err := db.WithContext(ctx).Select("id").Where("sku IN ?", sku).Limit(len(sku)).Find(&products).Error
	if err != nil {
		return nil, fmt.Errorf("get ids by sku: %w", err)
	}

	ids := make([]uint, 0, len(products))
	for _, p := range products {
		ids = append(ids, p.ID)
	}

	return ids, nil
}

func (r *repo) GetBySKUs(ctx context.Context, sku []productPkg.SKU) ([]productPkg.Product, error) {
	db := r.db.DB(ctx)
	products := make([]productPkg.Product, 0, len(sku))

	err := db.WithContext(ctx).Where("sku IN ?", sku).Limit(len(sku)).Find(&products).Error
	if err != nil {
		return nil, fmt.Errorf("get ids by sku: %w", err)
	}

	return products, nil
}

func (r *repo) GetBySKU(ctx context.Context, sku productPkg.SKU) (productPkg.Product, error) {
	db := r.db.DB(ctx)
	var product productPkg.Product

	err := db.WithContext(ctx).Where("sku = ?", sku).First(product).Error
	if err != nil {
		return product, fmt.Errorf("get by sku: %w", err)
	}

	return product, nil
}
