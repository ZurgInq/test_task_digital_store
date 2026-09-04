package actions

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	orderPkg "app/models/order"

	"go.rtnl.ai/x/randstr"
)

type InitPayment struct {
	Orders      *orderPkg.Service
	PaymentAddr string
}

func (act *InitPayment) Do(ctx context.Context, order orderPkg.Order) error {
	extID := fmt.Sprintf("ord_%s", randstr.AlphaNumeric(12))
	if err := act.initPayment(extID); err != nil {
		return err
	}
	err := act.Orders.UpdateExtID(ctx, order.ID, extID)
	if err != nil {
		return fmt.Errorf("update extID id=%d, extID=%s: %w", order.ID, extID, err)
	}

	return nil
}

func (act *InitPayment) initPayment(extID string) error {
	payment := &paymentsReq{
		OrderID:  extID,
		Amount:   500,   // stub data
		Currency: "RUB", // stub data
	}
	reqBody, err := json.Marshal(payment)
	if err != nil {
		return fmt.Errorf("create order: marshal payment: %w", err)
	}

	resp, err := http.Post(act.PaymentAddr+"/api/payments", "application/json", bytes.NewReader(reqBody))
	if err != nil {
		return fmt.Errorf("create order: post payment request: %w", err)
	}

	if resp.StatusCode > 300 || resp.StatusCode < 200 {
		return fmt.Errorf("create order: invalid payment response status code: %d", resp.StatusCode)
	}

	return nil
}
