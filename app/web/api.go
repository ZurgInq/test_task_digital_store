package web

import (
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"slices"
	"strconv"

	"app/db"
	balancePkg "app/models/balance"
	issuePkg "app/models/issue"
	orderPkg "app/models/order"
	paymentPkg "app/models/payment"
	productPkg "app/models/product"
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
	log            *slog.Logger
	DB             db.UnitOfWork
	orders         *orderPkg.Service
	payments       *paymentPkg.Service
	queue          *queue.MockQueue
	issues         *issuePkg.Service
	products       *productPkg.Service
	balanceService *balancePkg.Service
	paymentAddr    string
}

func New(
	log *slog.Logger,
	db db.UnitOfWork,
	orderService *orderPkg.Service,
	paymentService *paymentPkg.Service,
	queue *queue.MockQueue,
	issues *issuePkg.Service,
	products *productPkg.Service,
	balanceService *balancePkg.Service,
	paymentAddr string,
) *API {
	return &API{
		log:            log,
		DB:             db,
		orders:         orderService,
		payments:       paymentService,
		queue:          queue,
		issues:         issues,
		products:       products,
		paymentAddr:    paymentAddr,
		balanceService: balanceService,
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
		r.Post("/{orderId}/status/cancel", api.cancelOrder)
		r.Get("/{orderId}/issues", api.getOrdersIssues)
	})

	// users
	r.Route("/api/users", func(r chi.Router) {
		r.Get("/{userId}/balance", api.getUserBalance)
		r.Get("/{userId}/transactions", api.getUserTransactions)
	})

	// payments
	r.Route("/webhook/payments", func(r chi.Router) {
		r.Post("/", api.webhookPayment)
	})

	// recovery
	r.Route("/api/recovery", func(r chi.Router) {
		r.Post("/orders", api.recoveryOrders)
	})

	r.Route("/api/payments", func(r chi.Router) {
		// test payments
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

func (api *API) getUserBalance(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	userIDParam := chi.URLParam(r, "userId")
	userID, err := strconv.Atoi(userIDParam)
	if err != nil {
		api.internalServerError(w, r, err)
		return
	}

	amount, err := api.balanceService.GetAmountForUser(ctx, uint(userID))
	if err != nil {
		api.internalServerError(w, r, err)
		return
	}

	render.JSON(w, r, map[string]any{
		"user_id": userID,
		"balance": amount,
	})
}

func (api *API) getUserTransactions(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	userIDParam := chi.URLParam(r, "userId")
	userID, err := strconv.Atoi(userIDParam)
	if err != nil {
		api.internalServerError(w, r, err)
		return
	}

	amount, err := api.balanceService.GetTransactions(ctx, uint(userID))
	if err != nil {
		api.internalServerError(w, r, err)
		return
	}

	render.JSON(w, r, map[string]any{
		"user_id":      userID,
		"transactions": amount,
	})
}

func (api *API) recoveryOrders(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	totalRecovery := 0
	recoveryInitPayment := 0
	recoveryDelivering := 0
	recoveryDeliveryFailed := 0
	recoveryOutOfStock := 0

	statuses := []orderPkg.Status{
		orderPkg.StatusCreated,
		orderPkg.StatusPaid,
		orderPkg.StatusDelivering,
		orderPkg.StatusDeliveryFailed,
		orderPkg.StatusOutOfStock,
	}

	orders, err := api.orders.GetOrdersByStatus(ctx, statuses, 0, 500)
	if err != nil {
		api.internalServerError(w, r, err)
		return
	}

	for _, order := range orders {
		// recovery init payments
		if order.Status == orderPkg.StatusCreated {
			payment, err := api.payments.GetPaidByOrderGroupID(ctx, order.GroupID)
			if err != nil {
				api.log.Error("get payment: " + err.Error())
				continue
			}

			if payment.Status == paymentPkg.StatusInit {
				err = api.payments.SaveInitEvent(ctx, payment)
				if err != nil {
					api.internalServerError(w, r, err)
					api.log.Error("save init payment event: " + err.Error())
					continue
				}
				totalRecovery++
				recoveryInitPayment++
			}
		}

		if slices.Contains([]orderPkg.Status{
			orderPkg.StatusPaid,
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
		"recovery_init_payment":    recoveryInitPayment,
		"recovery_delivering":      recoveryDelivering,
		"recovery_delivery_failed": recoveryDeliveryFailed,
		"recovery_out_of_stock":    recoveryOutOfStock,
	})
}

func (api *API) internalServerError(w http.ResponseWriter, r *http.Request, err error) {
	api.log.Error(err.Error())
	render.Status(r, http.StatusInternalServerError)
	render.JSON(w, r, map[string]string{
		"error": err.Error(),
	})
}
