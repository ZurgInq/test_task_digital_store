package actions

import (
	"context"
	"fmt"
	"log/slog"
	"slices"

	"app/db"
	balancePkg "app/models/balance"
	orderPkg "app/models/order"
	paymentPkg "app/models/payment"
)

type CancelOrder struct {
	Log      *slog.Logger
	DB       db.UnitOfWork
	Orders   *orderPkg.Service
	Balance  *balancePkg.Service
	Payments *paymentPkg.Service
}

func (act *CancelOrder) Do(ctx context.Context, orderID uint) error {
	log := act.Log.With("orderID", orderID)
	log.Info("Start cancel order")

	validStatuses := []orderPkg.Status{
		orderPkg.StatusDeliveryFailed,
		orderPkg.StatusOutOfStock,
	}

	err := act.DB.Transaction(ctx, func(ctx context.Context) error {
		order, err := act.Orders.GetByID(ctx, orderID)
		if err != nil {
			return fmt.Errorf("get order by id %d", orderID)
		}

		if order.Status == orderPkg.StatusCancelled {
			return nil
		}

		if !slices.Contains(validStatuses, order.Status) {
			return fmt.Errorf("invalid status %s", order.Status)
		}

		isUpdated, err := act.Orders.UpdateStatus(ctx, order.ExtID, order.Status, orderPkg.StatusCancelled)
		if err != nil {
			return fmt.Errorf("update order status from %s: %w", order.Status, err)
		}

		if !isUpdated {
			return nil
		}

		payment, err := act.Payments.GetPaidByOrderGroupID(ctx, order.GroupID)
		if err != nil {
			return fmt.Errorf("get payment for order: %w", err)
		}

		if payment.Status == paymentPkg.StatusPaid {
			err = act.Balance.CreateRefund(ctx, order.UserID, order.ID, order.Amount)
			if err != nil {
				return fmt.Errorf("create refund: %w", err)
			}
		}

		return nil
	})

	if err != nil {
		return fmt.Errorf("cancel order transaction: %w", err)
	}

	return nil
}
