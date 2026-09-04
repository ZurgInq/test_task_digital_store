package db

import (
	"context"
	"fmt"

	"app/models/issue"
	"app/models/order"
	"app/models/payment"
	"app/models/product"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type txKey struct{}

type UnitOfWork interface {
	DB(ctx context.Context) *gorm.DB
	Transaction(ctx context.Context, fn func(context.Context) error) error
}

type DB struct {
	gormDB *gorm.DB
}

func NewDB(dbName string) *DB {
	db, err := gorm.Open(sqlite.Open(dbName), &gorm.Config{})
	if err != nil {
		panic("failed to connect database")
	}

	return &DB{
		gormDB: db,
	}
}

func (d *DB) DB(ctx context.Context) *gorm.DB {
	tx, ok := ctx.Value(txKey{}).(*gorm.DB)
	if ok {
		return tx
	}

	return d.gormDB
}

func (d *DB) Transaction(ctx context.Context, fn func(context.Context) error) error {
	_, ok := ctx.Value(txKey{}).(*gorm.DB)
	if ok {
		return fn(ctx)
	}

	return d.gormDB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		txCtx := context.WithValue(ctx, txKey{}, tx)
		return fn(txCtx)
	})
}

func Migrate(db *DB) {
	err := db.gormDB.AutoMigrate(
		product.Product{},
		order.Order{},
		payment.Payment{},
		issue.Issue{},
	)
	if err != nil {
		panic(fmt.Sprintf("Automigrate: %s", err))
	}
}
