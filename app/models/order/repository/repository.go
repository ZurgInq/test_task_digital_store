package repository

import (
	"context"
	"fmt"

	"app/db"
	orderPkg "app/models/order"

	"gorm.io/gorm"
)

type repo struct {
	db db.UnitOfWork
}

func New(
	db db.UnitOfWork,
) *repo {
	return &repo{
		db: db,
	}
}

func (r *repo) Create(ctx context.Context, order *orderPkg.Order) (uint, error) {
	db := r.db.DB(ctx)

	err := gorm.G[orderPkg.Order](db).Create(ctx, order)
	if err != nil {
		return 0, fmt.Errorf("create order: %w", err)
	}

	return order.ID, nil
}

func (r *repo) GetByID(ctx context.Context, id uint) (orderPkg.Order, error) {
	result := gorm.WithResult()
	db := r.db.DB(ctx)

	order, err := gorm.G[orderPkg.Order](db, result).Where("id = ?", id).First(ctx)
	if err != nil {
		return order, fmt.Errorf("get order by id %d: %w", id, err)
	}

	return order, nil
}

func (r *repo) GetAll(ctx context.Context, offset int, limit int) ([]orderPkg.Order, error) {
	result := gorm.WithResult()
	db := r.db.DB(ctx)

	orders, err := gorm.G[orderPkg.Order](db, result).Offset(offset).Limit(limit).Find(ctx)
	if err != nil {
		return orders, fmt.Errorf("get all: %w", err)
	}

	return orders, nil
}

func (r *repo) GetByExtID(ctx context.Context, extID string) (*orderPkg.Order, error) {
	result := gorm.WithResult()
	db := r.db.DB(ctx).WithContext(ctx)

	orders, err := gorm.G[orderPkg.Order](db, result).Where("ext_id = ?", extID).Limit(1).Find(ctx)
	if err != nil {
		return nil, fmt.Errorf("get all: %w", err)
	}

	if len(orders) == 0 {
		return nil, nil
	}

	return &orders[0], nil
}

func (r *repo) GetFirstByGroupID(ctx context.Context, groupID orderPkg.GroupID) (orderPkg.Order, error) {
	result := gorm.WithResult()
	db := r.db.DB(ctx).WithContext(ctx)

	order, err := gorm.G[orderPkg.Order](db, result).Where("group_id = ?", groupID).First(ctx)
	if err != nil {
		return order, fmt.Errorf("get first by group id: %w", err)
	}

	return order, nil
}

func (r *repo) FindByGroupID(ctx context.Context, groupID orderPkg.GroupID) ([]orderPkg.Order, error) {
	result := gorm.WithResult()
	db := r.db.DB(ctx).WithContext(ctx)

	orders, err := gorm.G[orderPkg.Order](db, result).Where("group_id = ?", groupID).Find(ctx)
	if err != nil {
		return nil, fmt.Errorf("get first by group id: %w", err)
	}

	return orders, nil
}

func (r *repo) UpdateByID(ctx context.Context, ID uint, order orderPkg.Order) error {
	result := gorm.WithResult()
	db := r.db.DB(ctx)

	affected, err := gorm.G[orderPkg.Order](db, result).Where("id = ?", ID).Updates(ctx, order)
	if err != nil {
		return fmt.Errorf("update by id %d: %w", ID, err)
	}

	if affected == 0 {
		return gorm.ErrRecordNotFound
	}

	return nil
}

func (r *repo) UpdateStatusByExtID(ctx context.Context, extID string, oldStatus orderPkg.Status, newStatus orderPkg.Status) (bool, error) {
	result := gorm.WithResult()
	db := r.db.DB(ctx)

	affected, err := gorm.G[orderPkg.Order](db, result).
		Where("ext_id = ?", extID).
		Where("status = ?", oldStatus).
		Update(ctx, "status", newStatus)
	if err != nil {
		return false, fmt.Errorf("update by extID %s: %w", extID, err)
	}

	return affected > 0, nil
}

func (r *repo) GetOrdersByStatus(ctx context.Context, statuses []orderPkg.Status, offset int, limit int) ([]orderPkg.Order, error) {
	result := gorm.WithResult()
	db := r.db.DB(ctx)

	orders, err := gorm.G[orderPkg.Order](db, result).
		Where("ext_id = ? OR status IN (?)", "", statuses).
		Offset(offset).
		Limit(limit).
		Find(ctx)
	if err != nil {
		return orders, fmt.Errorf("get failed init payment: %w", err)
	}

	return orders, nil
}

func (r *repo) FindIDByCode(ctx context.Context, code string) (*uint, error) {
	result := gorm.WithResult()
	db := r.db.DB(ctx)

	orders, err := gorm.G[orderPkg.Order](db, result).
		Select("id").
		Where("code = ?", code).
		Limit(1).
		Find(ctx)

	if err != nil {
		return nil, fmt.Errorf("get id by code: %w", err)
	}

	if len(orders) == 0 {
		return nil, nil
	}

	return &orders[0].ID, nil
}
