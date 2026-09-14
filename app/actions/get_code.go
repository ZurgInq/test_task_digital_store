package actions

import (
	"context"
	"fmt"
	"log/slog"
	"slices"

	"app/db"
	balancePkg "app/models/balance"
	issuePkg "app/models/issue"
	orderPkg "app/models/order"
)

type GetCodeForOrder struct {
	Log     *slog.Logger
	DB      db.UnitOfWork
	Issues  *issuePkg.Service
	Orders  *orderPkg.Service
	Balance *balancePkg.Service
}

func (act *GetCodeForOrder) Do(ctx context.Context, order orderPkg.Order) error {
	log := act.Log.With("orderID", order.ID)
	log.Info("Start get code for order")

	var validOrdersStatus = []orderPkg.Status{
		orderPkg.StatusPaid,
		orderPkg.StatusDelivering,
		orderPkg.StatusDeliveryFailed,
		orderPkg.StatusOutOfStock,
	}

	if !slices.Contains(validOrdersStatus, order.Status) {
		return fmt.Errorf("invalid order status %s", order.Status)
	}

	if order.Status != orderPkg.StatusDelivering {
		_, err := act.Orders.UpdateStatus(ctx, order.ExtID, order.Status, orderPkg.StatusDelivering)
		if err != nil {
			return fmt.Errorf("update order status from %s to %s: %w", order.Status, orderPkg.StatusDelivering, err)
		}
	}

	log.Info("Start request code")
	issue, err := act.Issues.RequestCodeForOrder(ctx, order)
	if err != nil {
		return fmt.Errorf("get issue for order extID=%s: %w", order.ExtID, err)
	}

	err = act.DB.Transaction(ctx, func(ctx context.Context) error {
		newOrderStatus, orderCode, err := act.getStatusAndCode(ctx, log, issue)
		if err != nil {
			return fmt.Errorf("get order status and code from issue: %w", err)
		}

		err = act.Orders.UpdateByID(ctx, order.ID, orderPkg.Order{
			Status: newOrderStatus,
			Code:   orderCode,
		})
		if err != nil {
			return fmt.Errorf("update order status from %s to %s: %w", orderPkg.StatusDelivering, newOrderStatus, err)
		}

		err = act.Balance.CreateCharge(ctx, order.UserID, order.ID, order.Amount)
		if err != nil {
			return fmt.Errorf("create charge for order %d: %w", order.ID, err)
		}

		return nil
	})

	if err != nil {
		return fmt.Errorf("get code transaction: %w", err)
	}

	return nil
}

func (act *GetCodeForOrder) getStatusAndCode(ctx context.Context, log *slog.Logger, issue issuePkg.Issue) (orderPkg.Status, string, error) {
	var orderCode string
	var newOrderStatus orderPkg.Status

	if issue.Status == issuePkg.StatusOk {
		newOrderStatus = orderPkg.StatusDelivered
		log.Info("Success request code")
	} else if issue.ErrReason == issuePkg.ErrReasonOutOfStock {
		newOrderStatus = orderPkg.StatusOutOfStock
	} else {
		newOrderStatus = orderPkg.StatusDeliveryFailed
	}

	if issue.Code != "" {
		isCodeExists, err := act.Orders.ExistsByCode(ctx, issue.Code)
		if err != nil {
			return newOrderStatus, "", fmt.Errorf("get order by code %s:%w", issue.Code, err)
		}

		if isCodeExists {
			log.Warn("Delivery failed: detect duplicate code", "code", issue.Code)
			return orderPkg.StatusDeliveryFailed, "", nil
		}

		if !isCodeExists {
			orderCode = issue.Code
		} else {
			newOrderStatus = orderPkg.StatusDeliveryFailed
			orderCode = ""
		}
	}

	return newOrderStatus, orderCode, nil
}
