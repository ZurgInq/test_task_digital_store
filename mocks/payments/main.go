package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/render"
	"go.rtnl.ai/x/randstr"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type Payment struct {
	gorm.Model

	EventID  string `json:"event_id"`
	OrderID  string `json:"order_id"`
	Status   string `json:"status"`
	Amount   int    `json:"amount"`
	Currency string `json:"currency"`
}

type PaymentReq struct {
	OrderID  string `json:"order_id"`
	Amount   int    `json:"amount"`
	Currency string `json:"currency"`
}

func sendWebhook(
	callbackAddr string,
	payment *Payment,
) error {

	type WebhookReq struct {
		EventID   string `json:"event_id"`
		OrderID   string `json:"order_id"`
		Status    string `json:"status"`
		Amount    int    `json:"amount"`
		Currency  string `json:"currency"`
		CreatedAt string `json:"created_at"`
	}

	reqBody, err := json.Marshal(WebhookReq{
		EventID:   payment.EventID,
		OrderID:   payment.OrderID,
		Status:    payment.Status,
		Amount:    payment.Amount,
		Currency:  payment.Currency,
		CreatedAt: payment.Model.CreatedAt.Format(time.RFC3339),
	})
	if err != nil {
		return err
	}

	apiAddr := callbackAddr + "/webhook/payments"
	slog.Info("Send callback", "addr", apiAddr, "eventID", payment.EventID, "status", payment.Status, "body", string(reqBody))
	resp, err := http.Post(apiAddr, "application/json", bytes.NewReader(reqBody))
	if err != nil {
		return err
	}

	if resp.StatusCode < 200 || resp.StatusCode > 300 {
		respBody, _ := io.ReadAll(resp.Body)
		defer resp.Body.Close()

		return fmt.Errorf("Invalid callback response code: %d, body: %s", resp.StatusCode, respBody)
	}

	return nil
}

func main() {
	log := slog.Default()
	log.Info("Start app")

	port := os.Getenv("APP_PORT")
	if port == "" {
		port = "3001"
	}

	host := os.Getenv("APP_HOST")

	callbackAddr := os.Getenv("APP_CALLBACK_ADDR")
	if callbackAddr == "" {
		callbackAddr = "http://localhost:3000"
	}

	db := NewDB()
	Migrate(db)

	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.URLFormat)
	r.Use(render.SetContentType(render.ContentTypeJSON))

	r.Route("/api/payments", func(r chi.Router) {
		r.Post("/", func(w http.ResponseWriter, r *http.Request) {
			paymentReq := &PaymentReq{}
			render.Decode(r, paymentReq)

			exists := &Payment{}
			err := db.Where("order_id = ?", paymentReq.OrderID).Limit(1).Find(exists).Error

			if err != nil {
				log.Error(err.Error())
				render.Status(r, 500)
				render.PlainText(w, r, err.Error())
				return
			}

			if exists.ID != 0 {
				render.Status(r, 201)
				render.JSON(w, r, exists)
				return
			}

			payment := &Payment{
				EventID:  "",
				OrderID:  paymentReq.OrderID,
				Amount:   paymentReq.Amount,
				Currency: paymentReq.Currency,
				Status:   "new",
			}

			result := db.Create(payment)

			if err := result.Error; err != nil {
				log.Error(err.Error())
				render.Status(r, 500)
				render.PlainText(w, r, err.Error())
			}

			log.Info("Create payment", "payment.ID", payment.ID, "payment.OrderID", payment.OrderID)
			render.Status(r, 201)
			render.JSON(w, r, payment)
		})

		r.Post("/{orderID}/status/{status}", func(w http.ResponseWriter, r *http.Request) {
			orderIDParam := chi.URLParam(r, "orderID")
			statusParam := chi.URLParam(r, "status")

			payment := &Payment{}
			result := db.Where("order_id = ?", orderIDParam).First(payment)
			if err := result.Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					render.Status(r, 404)
					render.PlainText(w, r, err.Error())
					return
				}

				log.Error(err.Error())
				render.Status(r, 500)
				render.PlainText(w, r, err.Error())
				return
			}

			if payment.Status != "new" {
				render.Status(r, 400)
				render.JSON(w, r, map[string]string{
					"error":  "payment already complete",
					"status": payment.Status,
				})
				return
			}

			payment.Status = statusParam
			payment.EventID = "evt_" + randstr.AlphaNumeric(12)

			result = db.Save(payment)
			if err := result.Error; err != nil {
				render.Status(r, 500)
				render.JSON(w, r, nil)
				log.Error(err.Error())
				return
			}

			if err := sendWebhook(callbackAddr, payment); err != nil {
				render.Status(r, 500)
				render.JSON(w, r, nil)
				log.Error(err.Error())
				return
			}

			render.Status(r, 201)
			render.JSON(w, r, payment)
		})

		r.Post("/{orderID}/webhook", func(w http.ResponseWriter, r *http.Request) {
			orderIDParam := chi.URLParam(r, "orderID")
			payment := &Payment{}
			result := db.Where("order_id = ?", orderIDParam).First(payment)

			if err := result.Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					render.Status(r, 404)
					render.JSON(w, r, map[string]any{
						"error": err.Error(),
					})
					return
				}
				render.Status(r, 500)
				render.JSON(w, r, map[string]any{
					"error": err.Error(),
				})
				log.Error(err.Error())
				return
			}

			if err := sendWebhook(callbackAddr, payment); err != nil {
				render.Status(r, 500)
				render.JSON(w, r, map[string]any{
					"error": err.Error(),
				})
				log.Error(err.Error())
				return
			}

			render.Status(r, 201)
			render.JSON(w, r, payment)
		})
	})

	addr := fmt.Sprintf("%s:%s", host, port)
	log.Info("Start server", "addr", addr)

	err := http.ListenAndServe(addr, r)
	if err != nil {
		panic(fmt.Sprintf("Start server: %s", err))
	}
}

func NewDB() *gorm.DB {
	db, err := gorm.Open(sqlite.Open("payments.db"), &gorm.Config{})
	if err != nil {
		panic("failed to connect database")
	}

	return db
}

func Migrate(db *gorm.DB) {
	err := db.AutoMigrate(
		&Payment{},
	)

	if err != nil {
		panic(fmt.Sprintf("Automigrate: %s", err))
	}
}
