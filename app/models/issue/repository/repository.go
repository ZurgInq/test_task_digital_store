package repository

import (
	"context"
	"fmt"

	"app/db"
	issuePkg "app/models/issue"

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

func (r *repo) Create(ctx context.Context, issue *issuePkg.Issue) (uint, error) {
	db := r.db(ctx)

	err := gorm.G[issuePkg.Issue](db).Create(ctx, issue)
	if err != nil {
		return 0, fmt.Errorf("create issue: %w", err)
	}

	return issue.ID, nil
}

func (r *repo) GetById(ctx context.Context, id uint) (issuePkg.Issue, error) {
	db := r.db(ctx)

	issue, err := gorm.G[issuePkg.Issue](db).Where("id = ?", id).First(ctx)
	if err != nil {
		return issuePkg.Issue{}, fmt.Errorf("get issue by id %d: %w", id, err)
	}

	return issue, nil
}

func (r *repo) GetByOrderExtID(ctx context.Context, orderExtID string) ([]issuePkg.Issue, error) {
	db := r.db(ctx)

	issues, err := gorm.G[issuePkg.Issue](db).Where("order_ext_id = ?", orderExtID).Find(ctx)
	if err != nil {
		return nil, fmt.Errorf("get issue by orderExtID %s: %w", orderExtID, err)
	}

	return issues, nil
}

func (r *repo) UpdateById(ctx context.Context, id uint, update issuePkg.Issue) error {
	db := r.db(ctx)

	_, err := gorm.G[issuePkg.Issue](db).Where("id = ?", id).Updates(ctx, update)
	if err != nil {
		return fmt.Errorf("update issue: %w", err)
	}

	return nil
}

func (r *repo) GetFirstIncomplete(ctx context.Context, orderIDExt string) (*issuePkg.Issue, error) {
	db := r.db(ctx)

	issues, err := gorm.G[issuePkg.Issue](db.Debug()).
		Where(`order_ext_id = ? AND response_code = ?`, orderIDExt, 0).
		Limit(1).
		Find(ctx)
	if err != nil {
		return nil, fmt.Errorf("get first incomplete issue: %w", err)
	}

	if len(issues) == 0 {
		return nil, nil
	}

	return &issues[0], nil
}
