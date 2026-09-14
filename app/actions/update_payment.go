package actions

import (
	"context"
	"fmt"
	"log/slog"

	"app/db"
	balancePkg "app/models/balance"
	orderPkg "app/models/order"
	paymentPkg "app/models/payment"
)

type UpdatePayment struct {
	Log     *slog.Logger
	DB      db.UnitOfWork
	Repo    paymentPkg.PaymentRepository
	Orders  *orderPkg.Service
	Balance *balancePkg.Service
}

func (act *UpdatePayment) Do(ctx context.Context, paymentEvent paymentPkg.PaymentEvent) error {
	log := act.Log.With("payment_event.event_id", paymentEvent.EventID, "payment_event.order_id", paymentEvent.OrderID)
	log.Info("Start udpate payment")

	var newOrderStatus orderPkg.Status
	switch paymentEvent.Status {
	case paymentPkg.StatusFailed:
		newOrderStatus = orderPkg.StatusPaymentFailed
	case paymentPkg.StatusPaid:
		newOrderStatus = orderPkg.StatusPaid
	default:
		log.Error(fmt.Sprintf("Skip update payment: unexpected status %s", paymentEvent.Status))
		return nil
	}

	err := act.DB.Transaction(ctx, func(ctx context.Context) error {
		exists, err := act.findExistsByEventID(ctx, paymentEvent.EventID)
		log.Info("Check exists payment", "payment", exists)
		if err != nil {
			return fmt.Errorf("get exists payment by event id: %w", err)
		}

		if exists != nil {
			log.Info("Skip update payment: already exists", "payment.ID", exists.ID)
			return nil
		}

		payment, err := act.getAwaitPaymentByOrderGroupID(ctx, orderPkg.GroupID(paymentEvent.OrderID))
		if err != nil {
			return fmt.Errorf("get payment by order group id: %w", err)
		}

		isUpdated, err := act.Repo.UpdateStatusFromStatus(ctx, payment.ID, paymentPkg.StatusAwait, paymentEvent.Status)
		if err != nil {
			return fmt.Errorf("update payment status: %w", err)
		}

		if isUpdated {
			err = act.Repo.UpdateEventID(ctx, payment.ID, paymentEvent.EventID)
			if err != nil {
				return fmt.Errorf("update event_id: %w", err)
			}
		} else {
			log.Warn("Skip update payment: status not updated")
			return nil
		}

		updatedPayment, err := act.Repo.GetByID(ctx, payment.ID)
		if err != nil {
			return fmt.Errorf("get updated payment: %w", err)
		}

		if updatedPayment.Status == paymentPkg.StatusPaid {
			err = act.Balance.CreateDeposit(ctx, payment.UserID, payment.ID, payment.Amount)
			if err != nil {
				return fmt.Errorf("create deposit: %w", err)
			}
		}

		err = act.updateOrdersStatus(ctx, log, payment.OrderGroupID, newOrderStatus)
		if err != nil {
			return fmt.Errorf("update order: %w", err)
		}

		return nil
	})

	if err != nil {
		return fmt.Errorf("update payment transaction: %w", err)
	}

	orders, err := act.Orders.FindByGroupID(ctx, orderPkg.GroupID(paymentEvent.OrderID))
	if err != nil {
		return fmt.Errorf("get orders after update status: %w", err)
	}

	for _, order := range orders {
		if order.Status == orderPkg.StatusPaid {
			err := act.Orders.SaveOrderPaidEvent(ctx, order)
			if err != nil {
				log.Error(fmt.Sprintf("save order paid event: %s", err))
			}
		}
	}

	return nil
}

func (act *UpdatePayment) findExistsByEventID(ctx context.Context, eventID string) (*paymentPkg.Payment, error) {
	return act.Repo.FindByEventID(ctx, eventID)
}

func (act *UpdatePayment) getAwaitPaymentByOrderGroupID(ctx context.Context, orderGroupID orderPkg.GroupID) (paymentPkg.Payment, error) {
	return act.Repo.GetAwaitByOrderGroupID(ctx, orderGroupID)
}

func (act *UpdatePayment) updateOrdersStatus(
	ctx context.Context,
	log *slog.Logger,
	orderGroupID orderPkg.GroupID,
	newOrderStatus orderPkg.Status,
) error {
	orders, err := act.Orders.FindByGroupID(ctx, orderGroupID)
	if err != nil {
		return fmt.Errorf("find orders: %w", err)
	}

	for _, order := range orders {
		isUpdated, err := act.Orders.UpdateStatus(ctx, order.ExtID, orderPkg.StatusCreated, newOrderStatus)
		if err != nil {
			return fmt.Errorf("update status: %w", err)
		}

		if isUpdated {
			log.Info("Order status updated", "orderID", order.ID, "status", newOrderStatus)
		} else {
			act.Log.Info("Skip update order status", "orderID", order.ID)
		}

	}

	return nil
}
