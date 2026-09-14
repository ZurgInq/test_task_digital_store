package web

import (
	"app/models/currency"
	paymentPkg "app/models/payment"
	"fmt"
	"net/http"
	"slices"
	"time"

	"github.com/go-chi/render"
)

type CreatePaymentReq struct {
	EventID   string `json:"event_id"`
	OrderID   string `json:"order_id"`
	Status    string `json:"status"`
	Amount    int    `json:"amount"`
	Currency  string `json:"currency"`
	CreatedAt string `json:"created_at"`
}

func (api *API) webhookPayment(w http.ResponseWriter, r *http.Request) {
	data := &CreatePaymentReq{}

	if err := render.DefaultDecoder(r, data); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	createdAt, err := time.Parse(time.RFC3339, data.CreatedAt)
	if err != nil {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, map[string]any{
			"error": fmt.Sprintf("parse created_at: %s", err),
		})
		return
	}

	validStatuses := []string{
		string(paymentPkg.StatusPaid),
		string(paymentPkg.StatusFailed),
	}

	if !slices.Contains(validStatuses, data.Status) {
		api.log.Error("Unexpected payment status", "payment", data)
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, map[string]any{
			"error": "Unexpected payment status",
		})
		return
	}

	err = api.payments.SaveUpdateEvent(r.Context(), paymentPkg.PaymentEvent{
		EventID:   data.EventID,
		OrderID:   data.OrderID,
		Status:    paymentPkg.Status(data.Status),
		Amount:    currency.Amount(data.Amount),
		Currency:  currency.Currency(data.Currency),
		CreatedAt: createdAt,
	})

	if err != nil {
		api.log.Error(err.Error(), "payment", data)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusCreated)
}

func (api *API) changePaymentStatus(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	resp, err := http.Post(api.paymentAddr+path, "application/json", nil)
	if err != nil {
		api.internalServerError(w, r, err)
		return
	}

	w.WriteHeader(resp.StatusCode)
}
