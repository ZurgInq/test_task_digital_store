package repository

import (
	"context"
	"fmt"

	"app/db"
	balancePkg "app/models/balance"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type repo struct {
	uow db.UnitOfWork
}

func New(
	db db.UnitOfWork,
) *repo {
	return &repo{
		uow: db,
	}
}

func (r *repo) db(ctx context.Context) *gorm.DB {
	return r.uow.DB(ctx).WithContext(ctx)
}

func (r *repo) GetAmountForUser(ctx context.Context, userID uint) (int, error) {
	db := r.db(ctx)

	var amount int
	err := db.Table("balance_transactions").Select("COALESCE(SUM(amount), 0)").Where("user_id = ?", userID).Scan(&amount).Error
	if err != nil {
		return 0, fmt.Errorf("sum amount for user_id = %d: %w", userID, err)
	}

	return amount, nil
}

func (r *repo) GetTransactions(ctx context.Context, userID uint) ([]balancePkg.BalanceTransaction, error) {
	db := r.db(ctx)

	transactions, err := gorm.G[balancePkg.BalanceTransaction](db).Where("user_id = ?", userID).Order("id").Find(ctx)
	if err != nil {
		return nil, fmt.Errorf("get transactions for userID %d: %w", userID, err)
	}

	return transactions, nil
}

func (r *repo) Create(
	ctx context.Context,
	transaction *balancePkg.BalanceTransaction,
) (uint, error) {
	result := gorm.WithResult()
	db := r.db(ctx)

	err := gorm.G[balancePkg.BalanceTransaction](db, result).Create(ctx, transaction)
	if err != nil {
		return 0, fmt.Errorf("create transaction: %w", err)
	}

	return transaction.ID, nil
}

func (r *repo) CreateOrIgnore(
	ctx context.Context,
	transaction *balancePkg.BalanceTransaction,
) (uint, error) {
	db := r.db(ctx)

	err := db.WithContext(ctx).Clauses(clause.OnConflict{
		DoNothing: true,
	}).Create(transaction).Error

	if err != nil {
		return 0, fmt.Errorf("create transaction: %w", err)
	}

	return transaction.ID, nil
}
