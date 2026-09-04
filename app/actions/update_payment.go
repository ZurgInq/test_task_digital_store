package actions

import (
	"context"
	"fmt"
	"log/slog"

	"app/db"
	orderPkg "app/models/order"
	paymentPkg "app/models/payment"
)

type UpdatePayment struct {
	Log    *slog.Logger
	Repo   paymentPkg.PaymentRepository
	Orders *orderPkg.Service
	DB     db.UnitOfWork
}

func (act *UpdatePayment) Do(ctx context.Context, payment paymentPkg.Payment) error {
	return act.DB.Transaction(ctx, func(ctx context.Context) error {
		log := act.Log.With("eventId", payment.EventID, "orderId", payment.OrderID)
		log.Info("Start udpate payment")

		exists, err := act.getExists(ctx, payment)
		if err != nil {
			return err
		}

		if exists == nil {
			log.Info("Start create payment")

			payment.Model.CreatedAt = payment.CreatedAt
			_, err = act.Repo.Create(ctx, &payment)
			if err != nil {
				return fmt.Errorf("create payment: %w", err)
			}
			log.Info("Payment created", "id", payment.ID)
		} else {
			log.Info("Skip create payment: already exists")
		}

		order, err := act.updateOrderStatus(ctx, payment)
		if err != nil {
			return fmt.Errorf("update order: %w", err)
		}

		if order == nil {
			return fmt.Errorf("order not found extID=%s", payment.OrderID)
		}

		if order.Status == orderPkg.StatusPaid {
			err = act.Orders.SaveOrderPaidEvent(ctx, *order)
			if err != nil {
				return fmt.Errorf("save order paid event: %w", err)
			}
		}

		return nil
	})
}

func (act *UpdatePayment) getExists(ctx context.Context, payment paymentPkg.Payment) (*paymentPkg.Payment, error) {
	order, err := act.Orders.GetByExtID(ctx, payment.OrderID)
	if err != nil {
		return nil, fmt.Errorf("get by extID %s: %w", payment.OrderID, err)
	}

	if order == nil {
		return nil, fmt.Errorf("order not found extID=%s", payment.OrderID)
	}

	exists, err := act.Repo.GetByEventID(ctx, payment.EventID)
	if err != nil {
		return nil, fmt.Errorf("exists by event id: %w", err)
	}

	return exists, nil
}

func (act *UpdatePayment) updateOrderStatus(ctx context.Context, payment paymentPkg.Payment) (*orderPkg.Order, error) {
	var newOrderStatus orderPkg.Status
	switch payment.Status {
	case paymentPkg.StatusFailed:
		newOrderStatus = orderPkg.StatusPaymentFailed
	case paymentPkg.StatusPaid:
		newOrderStatus = orderPkg.StatusPaid
	default:
		return nil, fmt.Errorf("unexpected status %s", payment.Status)
	}

	isUpdated, err := act.Orders.UpdateStatus(ctx, payment.OrderID, orderPkg.StatusCreated, newOrderStatus)
	if err != nil {
		return nil, fmt.Errorf("update status: %w", err)
	}

	if isUpdated {
		act.Log.Info("Order status updated", "orderID", payment.OrderID, "status", newOrderStatus)
	} else {
		act.Log.Info("Skip update order status", "eventId", payment.EventID)
	}

	return act.Orders.GetByExtID(ctx, payment.OrderID)
}
