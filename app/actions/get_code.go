package actions

import (
	"context"
	"fmt"
	"log/slog"
	"slices"

	issuePkg "app/models/issue"
	orderPkg "app/models/order"
)

type GetCodeForOrder struct {
	Issues *issuePkg.Service
	Orders *orderPkg.Service
	Log    *slog.Logger
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
		_, err := act.Orders.UpdateStatus(ctx, *order.ExtID, order.Status, orderPkg.StatusDelivering)
		if err != nil {
			return fmt.Errorf("update order status from %s to %s: %w", order.Status, orderPkg.StatusDelivering, err)
		}
	}

	log.Info("Start request code")
	issue, err := act.Issues.RequestCodeForOrder(ctx, order)
	if err != nil {
		return fmt.Errorf("get issue for order extID=%s: %w", *order.ExtID, err)
	}

	var newOrderStatus orderPkg.Status
	if issue.Status == issuePkg.StatusOk {
		newOrderStatus = orderPkg.StatusDelivered
		log.Info("Success request code")
	} else if issue.ErrReason == issuePkg.ErrReasonOutOfStock {
		newOrderStatus = orderPkg.StatusOutOfStock
	} else {
		newOrderStatus = orderPkg.StatusDeliveryFailed
	}

	err = act.Orders.UpdateByID(ctx, order.ID, orderPkg.Order{
		Status: newOrderStatus,
		Code:   issue.Code,
	})
	if err != nil {
		return fmt.Errorf("update order status from %s to %s: %w", orderPkg.StatusDelivering, newOrderStatus, err)
	}

	return nil
}
