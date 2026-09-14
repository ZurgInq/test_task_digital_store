package actions

import (
	"context"
	"encoding/json"
	"fmt"

	orderPkg "app/models/order"
	productsPkg "app/models/product"
	"app/queue"
)

type ChooseProvider struct {
	Products       *productsPkg.Service
	QueueProviders map[productsPkg.Type]string // Маппинг типа продукта к нужной очереди провайдера
	Queue          *queue.MockQueue
}

func (act *ChooseProvider) Do(ctx context.Context, order orderPkg.Order) error {
	product, err := act.Products.GetByID(ctx, order.ProductID)
	if err != nil {
		return fmt.Errorf("get product by id %d: %w", order.ProductID, err)
	}

	queueName, ok := act.QueueProviders[product.Type]
	if !ok {
		return fmt.Errorf("provider not found for product type %s", product.Type)
	}

	data, err := json.Marshal(order)
	if err != nil {
		return fmt.Errorf("marshal order: %w", err)
	}

	err = act.Queue.Publish(queueName, string(data))
	if err != nil {
		return fmt.Errorf("publish to queue %s: %w", queueName, err)
	}

	return nil
}
