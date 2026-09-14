package actions

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"

	paymentPkg "app/models/payment"
)

type InitPayment struct {
	Log         *slog.Logger
	Payments    *paymentPkg.Service
	PaymentAddr string
}

func (act *InitPayment) Do(ctx context.Context, payment paymentPkg.Payment) error {
	log := act.Log.With("payment.ID", payment.ID, "payment.OrderGroupID", payment.OrderGroupID)

	if payment.Status != paymentPkg.StatusInit {
		log.Info("Skip init payment by status", "status", string(payment.Status))
		return nil
	}

	if err := act.initPayment(payment); err != nil {
		return fmt.Errorf("init payment: %w", err)
	}

	isUpdated, err := act.Payments.StatusToAwait(ctx, payment.ID, paymentPkg.StatusInit, paymentPkg.StatusAwait)
	if err != nil {
		return fmt.Errorf("change payment status to await")
	}

	if isUpdated {
		log.Info("Change payment status to await")
	} else {
		log.Info("Skip update payment status: no updated records")
	}

	return nil
}

func (act *InitPayment) initPayment(payment paymentPkg.Payment) error {
	paymentReq := &paymentsReq{
		OrderID:  string(payment.OrderGroupID),
		Amount:   int(payment.Amount),
		Currency: string(payment.Currency),
	}
	reqBody, err := json.Marshal(paymentReq)
	if err != nil {
		return fmt.Errorf("marshal payment: %w", err)
	}

	resp, err := http.Post(act.PaymentAddr+"/api/payments", "application/json", bytes.NewReader(reqBody))
	if err != nil {
		return fmt.Errorf("post payment request: %w", err)
	}

	if resp.StatusCode > 300 || resp.StatusCode < 200 {
		return fmt.Errorf("invalid payment response status code for init payment: %d", resp.StatusCode)
	}

	return nil
}
