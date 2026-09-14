package web

import (
	"net/http"
	"strconv"

	"app/actions"
	productPkg "app/models/product"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
)

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

	productSKU := make([]productPkg.SKU, 0, len(data.Sku))
	for _, sku := range data.Sku {
		productSKU = append(productSKU, productPkg.SKU(sku))
	}

	createOrder := actions.CreateOrder{
		Orders:   *api.orders,
		Products: *api.products,
		Payments: *api.payments,
		DB:       api.DB,
	}

	orders, err := createOrder.Do(r.Context(), data.UserId, productSKU)
	if err != nil {
		api.log.Error(err.Error(), "userId", data.UserId, "sku", data.Sku)
		render.Status(r, http.StatusInternalServerError)
		render.JSON(w, r, map[string]string{
			"error": err.Error(),
		})
		return
	}

	api.log.Info("Orders created", "groupID", orders[0].GroupID)

	render.Status(r, http.StatusCreated)
	render.JSON(w, r, map[string]any{
		"orders": orders,
	})
}

func (api *API) cancelOrder(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	orderIDParam := chi.URLParam(r, "orderId")
	orderID, err := strconv.Atoi(orderIDParam)
	if err != nil {
		api.internalServerError(w, r, err)
		return
	}

	err = (&actions.CancelOrder{
		Log:      api.log,
		DB:       api.DB,
		Orders:   api.orders,
		Balance:  api.balanceService,
		Payments: api.payments,
	}).Do(ctx, uint(orderID))
	if err != nil {
		api.internalServerError(w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
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

	orders, err := api.issues.GetIssuesByOrderExtID(ctx, order.ExtID)

	if err != nil {
		api.internalServerError(w, r, err)
		return
	}

	render.Status(r, http.StatusOK)
	render.JSON(w, r, orders)
}
