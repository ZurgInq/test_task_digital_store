package web

import (
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"slices"
	"strconv"
	"time"

	"app/actions"
	"app/models/currency"
	issuePkg "app/models/issue"
	orderPkg "app/models/order"
	"app/models/payment"
	"app/models/product"
	"app/queue"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/render"
)

type Options struct {
	Host string
	Port string
}

type API struct {
	log         *slog.Logger
	orders      *orderPkg.Service
	payments    *payment.Service
	queue       *queue.MockQueue
	issues      *issuePkg.Service
	paymentAddr string
}

func New(
	log *slog.Logger,
	orderService *orderPkg.Service,
	paymentService *payment.Service,
	queue *queue.MockQueue,
	issues *issuePkg.Service,
	paymentAddr string,
) *API {
	return &API{
		log:         log,
		orders:      orderService,
		payments:    paymentService,
		queue:       queue,
		issues:      issues,
		paymentAddr: paymentAddr,
	}
}

func (api *API) Start(opts Options) error {
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.URLFormat)
	r.Use(render.SetContentType(render.ContentTypeJSON))

	// orders
	r.Route("/api/orders", func(r chi.Router) {
		r.Post("/", api.createOrder)
		r.Get("/", api.getOrders)
		r.Get("/{orderId}", api.getOrder)
		r.Get("/{orderId}/issues", api.getOrdersIssues)
	})

	// payments
	r.Route("/webhook/payments", func(r chi.Router) {
		r.Post("/", api.createPayment)
	})

	// recovery
	r.Route("/api/recovery", func(r chi.Router) {
		r.Post("/orders", api.recoveryOrders)
	})

	// test payments
	r.Route("/api/payments", func(r chi.Router) {
		r.Post("/{orderExtID}/status/{status}", api.changePaymentStatus)
	})

	// test api page
	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		b, err := os.ReadFile("static/test.html")
		if err != nil {
			api.internalServerError(w, r, err)
		}
		render.HTML(w, r, string(b))
	})

	addr := fmt.Sprintf("%s:%s", opts.Host, opts.Port)
	api.log.Info("Start server", "addr", addr)
	return http.ListenAndServe(addr, r)
}

type CreateOrderReq struct {
	UserId uint     `json:"userId"`
	Sku    []string `json:"sku"`
}

func (api *API) getOrder(w http.ResponseWriter, r *http.Request) {
	orderIDParam := chi.URLParam(r, "orderId")
	orderID, err := strconv.Atoi(orderIDParam)
	if err != nil {
		api.internalServerError(w, r, err)
		return
	}

	order, err := api.orders.GetByID(r.Context(), uint(orderID))
	if err != nil {
		api.internalServerError(w, r, err)
		return
	}

	render.Status(r, http.StatusOK)
	render.JSON(w, r, order)
}

func (api *API) getOrders(w http.ResponseWriter, r *http.Request) {
	orders, err := api.orders.GetAll(r.Context(), 0, 1_000)
	if err != nil {
		api.log.Error(err.Error())
		render.Status(r, http.StatusInternalServerError)
		render.JSON(w, r, map[string]string{
			"error": err.Error(),
		})
		return
	}

	render.Status(r, http.StatusOK)
	render.JSON(w, r, orders)
}

func (api *API) createOrder(w http.ResponseWriter, r *http.Request) {
	data := &CreateOrderReq{}

	if err := render.DefaultDecoder(r, data); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	productSKU := make([]product.SKU, 0, len(data.Sku))
	for _, sku := range data.Sku {
		productSKU = append(productSKU, product.SKU(sku))
	}

	createOrder := actions.CreateOrder{
		Orders: *api.orders,
	}

	order, err := createOrder.Do(r.Context(), data.UserId, productSKU)
	if err != nil {
		api.log.Error(err.Error(), "userId", data.UserId, "sku", data.Sku)
		render.Status(r, http.StatusInternalServerError)
		render.JSON(w, r, map[string]string{
			"error": err.Error(),
		})
		return
	}

	api.log.Info("Order created", "id", order.ID)

	render.Status(r, http.StatusCreated)
	render.JSON(w, r, order)
}

type CreatePaymentReq struct {
	EventID   string `json:"event_id"`
	OrderID   string `json:"order_id"`
	Status    string `json:"status"`
	Amount    int    `json:"amount"`
	Currency  string `json:"currency"`
	CreatedAt string `json:"created_at"`
}

func (api *API) createPayment(w http.ResponseWriter, r *http.Request) {
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
		string(payment.StatusPaid),
		string(payment.StatusFailed),
	}

	if !slices.Contains(validStatuses, data.Status) {
		api.log.Error("Unexpected payment status", "payment", data)
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, map[string]any{
			"error": "Unexpected payment status",
		})
		return
	}

	err = api.payments.SaveCreatePaymentEvent(r.Context(), payment.Payment{
		EventID:   data.EventID,
		OrderID:   data.OrderID,
		Status:    payment.Status(data.Status),
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

func (api *API) recoveryOrders(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	totalRecovery := 0
	recoveryCreated := 0
	recoveryDelivering := 0
	recoveryDeliveryFailed := 0
	recoveryOutOfStock := 0

	statuses := []orderPkg.Status{
		orderPkg.StatusCreated,
		orderPkg.StatusDelivering,
		orderPkg.StatusDeliveryFailed,
		orderPkg.StatusOutOfStock,
	}

	orders, err := api.orders.GetOrdersByStatus(ctx, statuses, 0, 500)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	for _, order := range orders {
		if order.ExtID == nil {
			err = api.orders.SaveOrderCreatedEvent(ctx, order)
			if err != nil {
				api.log.Error("save order created event: " + err.Error())
				continue
			} else {
				totalRecovery++
				recoveryCreated++
			}
		}

		if slices.Contains([]orderPkg.Status{
			orderPkg.StatusDelivering,
			orderPkg.StatusDeliveryFailed,
			orderPkg.StatusOutOfStock,
		}, order.Status) {
			err = api.orders.SaveOrderPaidEvent(ctx, order)
			if err != nil {
				api.log.Error("save order paid event: " + err.Error())
				continue
			} else {
				totalRecovery++
			}

			switch order.Status {
			case orderPkg.StatusDelivering:
				recoveryDelivering++
			case orderPkg.StatusDeliveryFailed:
				recoveryDeliveryFailed++
			case orderPkg.StatusOutOfStock:
				recoveryOutOfStock++
			}
		}
	}

	render.JSON(w, r, map[string]any{
		"total_recovery":           totalRecovery,
		"recovery_created":         recoveryCreated,
		"recovery_delivering":      recoveryDelivering,
		"recovery_delivery_failed": recoveryDeliveryFailed,
		"recovery_out_of_stock":    recoveryOutOfStock,
	})
}

func (api *API) getOrdersIssues(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	orderIDParam := chi.URLParam(r, "orderId")
	orderID, err := strconv.Atoi(orderIDParam)
	if err != nil {
		api.internalServerError(w, r, err)
	}

	order, err := api.orders.GetByID(ctx, uint(orderID))
	if err != nil {
		api.internalServerError(w, r, err)
	}

	orders, err := api.issues.GetIssuesByOrderExtID(ctx, *order.ExtID)

	if err != nil {
		api.internalServerError(w, r, err)
		return
	}

	render.Status(r, http.StatusOK)
	render.JSON(w, r, orders)
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

func (api *API) internalServerError(w http.ResponseWriter, r *http.Request, err error) {
	api.log.Error(err.Error())
	render.Status(r, http.StatusInternalServerError)
	render.JSON(w, r, map[string]string{
		"error": err.Error(),
	})
}
