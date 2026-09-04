package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path"

	"app/db"
	"app/models/currency"
	"app/models/product"
)

type productsSeed struct {
	Products []ProductSeed `json:"products"`
}

type ProductSeed struct {
	Sku      string `json:"sku"`
	Name     string `json:"name"`
	Type     string `json:"type"`
	Price    int64  `json:"price"`
	Currency string `json:"currency"`
	Image    string `json:"image"`
}

func seedsProducts(
	db *db.DB,
	seedDataDir string,
) error {
	slog.Info("Start seed products")
	ctx := context.Background()

	var count int64
	err := db.DB(ctx).Table("products").Count(&count).Error
	if err != nil {
		return err
	}

	if count != 0 {
		slog.Info("Skip seed: products table is not empty")
		return nil
	}

	seedFilename := path.Join(seedDataDir, "products.json")

	data, err := os.ReadFile(seedFilename)
	if err != nil {
		return fmt.Errorf("read products seed file %s: %w", seedFilename, err)
	}

	var seed productsSeed
	if err := json.Unmarshal(data, &seed); err != nil {
		return fmt.Errorf("parse products seed file: %w", err)
	}

	for _, item := range seed.Products {
		p := &product.Product{
			Sku:      product.SKU(item.Sku),
			Name:     item.Name,
			Type:     product.Type(item.Type),
			Price:    currency.Amount(item.Price),
			Currency: currency.Currency(item.Currency),
			Image:    item.Image,
		}

		err := db.DB(ctx).Create(&p).Error
		if err != nil {
			return fmt.Errorf("create product %q: %w", item.Sku, err)
		}

		slog.Info("Product created", "id", p.ID, "sku", item.Sku)
	}

	return nil
}
