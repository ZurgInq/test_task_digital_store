package actions

import (
	"context"
	"fmt"

	orderPkg "app/models/order"
	"app/models/product"
)

type CreateOrder struct {
	Orders orderPkg.Service
}

type paymentsReq struct {
	OrderID  string `json:"order_id"`
	Amount   int    `json:"amount"`
	Currency string `json:"currency"`
}

func (act *CreateOrder) Do(ctx context.Context, userId uint, sku []product.SKU) (orderPkg.Order, error) {
	var order orderPkg.Order

	order, err := act.Orders.CreateOrder(ctx, userId, sku, nil)
	if err != nil {
		return order, fmt.Errorf("action create order: %w", err)
	}

	err = act.Orders.SaveOrderCreatedEvent(ctx, order)
	if err != nil {
		return order, fmt.Errorf("init payment: %w", err)
	}

	return order, nil
}
