package repository

import (
	"context"
	"errors"
	"fmt"

	"app/db"
	paymentPkg "app/models/payment"

	"gorm.io/gorm"
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

func (r *repo) Create(ctx context.Context, order *paymentPkg.Payment) (uint, error) {
	result := gorm.WithResult()
	db := r.db(ctx)

	err := gorm.G[paymentPkg.Payment](db, result).Create(ctx, order)
	if err != nil {
		return 0, fmt.Errorf("create payment: %w", err)
	}

	return order.ID, nil
}

func (r *repo) GetById(ctx context.Context, id uint) (paymentPkg.Payment, error) {
	result := gorm.WithResult()
	db := r.db(ctx)

	order, err := gorm.G[paymentPkg.Payment](db, result).Where("id = ?", id).First(ctx)
	if err != nil {
		return order, fmt.Errorf("get payment by id %d: %w", id, err)
	}

	return order, nil
}

func (r *repo) ExistsByEventID(ctx context.Context, eventID string) (bool, error) {
	result := gorm.WithResult()
	db := r.db(ctx)

	_, err := gorm.G[paymentPkg.Payment](db, result).Select("id").Where("event_id = ?", eventID).First(ctx)
	if err != nil {
		return false, fmt.Errorf("ExistsByEventID %s: %w", eventID, err)
	}

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return false, nil
	}

	return true, nil
}

func (r *repo) GetByEventID(ctx context.Context, eventID string) (*paymentPkg.Payment, error) {
	result := gorm.WithResult()
	db := r.db(ctx)

	payments, err := gorm.G[paymentPkg.Payment](db, result).Where("event_id = ?", eventID).Limit(1).Find(ctx)
	if err != nil {
		return nil, fmt.Errorf("GetByEventID %s: %w", eventID, err)
	}

	if len(payments) == 0 {
		return nil, nil
	}

	return &payments[0], nil
}
