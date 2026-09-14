package actions

import (
	"context"
	"fmt"
	"uuid"

	"app/db"
	currencyPkg "app/models/currency"
	orderPkg "app/models/order"
	paymentPkg "app/models/payment"
	productsPkg "app/models/product"

	"go.rtnl.ai/x/randstr"
)

type CreateOrder struct {
	Orders   orderPkg.Service
	Products productsPkg.Service
	Payments paymentPkg.Service
	DB       db.UnitOfWork
}

type paymentsReq struct {
	OrderID  string `json:"order_id"`
	Amount   int    `json:"amount"`
	Currency string `json:"currency"`
}

func (act *CreateOrder) Do(ctx context.Context, userID uint, skuList []productsPkg.SKU) ([]orderPkg.Order, error) {
	products, err := act.getProducts(ctx, skuList)
	if err != nil {
		return nil, err
	}

	groupID := orderPkg.GroupID(uuid.NewV7().String())

	var paymentID uint
	var orders []orderPkg.Order

	err = act.DB.Transaction(ctx, func(ctx context.Context) error {
		var err error
		var paymentAmount currencyPkg.Amount

		orders, paymentAmount, err = act.createProductsOrders(ctx, userID, groupID, products)
		if err != nil {
			return fmt.Errorf("create products orders: %w", err)
		}

		paymentID, err = act.createPayment(ctx, userID, groupID, paymentAmount)
		if err != nil {
			return fmt.Errorf("create payment: %w", err)
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("create order transaction: %w", err)
	}

	payment, err := act.Payments.GetByID(ctx, paymentID)
	if err != nil {
		return nil, fmt.Errorf("get payment: %w", err)
	}

	err = act.Payments.SaveInitEvent(ctx, payment)
	if err != nil {
		return nil, fmt.Errorf("payment init: %w", err)
	}

	return orders, nil
}

func (act *CreateOrder) createProductsOrders(
	ctx context.Context,
	userID uint,
	groupID orderPkg.GroupID,
	products []productsPkg.Product,
) ([]orderPkg.Order, currencyPkg.Amount, error) {
	var paymentAmount currencyPkg.Amount
	orders := make([]orderPkg.Order, 0, len(products))
	for _, product := range products {
		order, err := act.createOrder(ctx, userID, groupID, product)
		if err != nil {
			return nil, paymentAmount, fmt.Errorf("create order for product: %w", err)
		}

		orders = append(orders, order)
		paymentAmount += currencyPkg.Amount(product.Price)
	}

	return orders, paymentAmount, nil
}

func (act *CreateOrder) createOrder(
	ctx context.Context,
	userID uint,
	groupID orderPkg.GroupID,
	product productsPkg.Product,
) (orderPkg.Order, error) {
	extID := fmt.Sprintf("ord_%s", randstr.AlphaNumeric(12))
	order, err := act.Orders.CreateOrder(ctx, userID, groupID, product.ID, extID, product.Price)
	if err != nil {
		return order, err
	}

	return order, nil
}

func (act *CreateOrder) createPayment(ctx context.Context, userID uint, groupID orderPkg.GroupID, paymentAmount currencyPkg.Amount) (uint, error) {
	return act.Payments.Create(ctx, userID, groupID, paymentAmount, currencyPkg.Currency("RUB"))
}

func (act *CreateOrder) getProducts(ctx context.Context, skuList []productsPkg.SKU) ([]productsPkg.Product, error) {
	if len(skuList) == 0 {
		return nil, fmt.Errorf("empty sku list")
	}

	products, err := act.Products.GetBySKUs(ctx, skuList)
	if err != nil {
		return nil, fmt.Errorf("get products: %w", err)
	}

	if len(products) == 0 {
		return nil, fmt.Errorf("products not found")
	}

	return products, nil
}
