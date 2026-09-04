package main

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"app/actions"
	dbPkg "app/db"
	issuePkg "app/models/issue"
	issueRepositoryPkg "app/models/issue/repository"
	"app/models/order"
	orderRepoPkg "app/models/order/repository"
	"app/models/payment"
	paymentRepoPkg "app/models/payment/repository"
	"app/models/product"
	productRepoPkg "app/models/product/repository"
	"app/queue"
	webPkg "app/web"

	"github.com/caarlos0/env/v11"
)

type Config struct {
	Host                 string `env:"APP_HOST"`
	Port                 string `env:"APP_PORT" envDefault:"3000"`
	SeedDataDir          string `env:"SEED_DATA_DIR" envDefault:"seed_data"`
	PaymentAddr          string `env:"PAYMENT_ADDR" envDefault:"http://localhost:3001"`
	IssueApiAddrMain     string `env:"ISSUE_API_ADDR_MAIN" envDefault:"http://localhost:3002"`
	IssueApiAddrFallback string `env:"ISSUE_API_ADDR_FALLBACK" envDefault:"http://localhost:3003"`
}

func main() {
	log := slog.Default()
	slog.Info("Start app")

	cfg := Config{}
	err := env.Parse(&cfg)
	if err != nil {
		panic(err)
	}

	slog.Info("Config parsed", "cfg", cfg)

	db := dbPkg.NewDB("app.db")

	if cfg.SeedDataDir != "" {
		err := seedsProducts(db, cfg.SeedDataDir)
		if err != nil {
			panic(err)
		}
	}

	queue := queue.NewMockQueue()

	productRepo := productRepoPkg.New(db)
	productService := product.NewService(productRepo)

	orderRepo := orderRepoPkg.New(db)
	orderService := order.NewService(
		orderRepo,
		productService,
		queue,
		log,
	)

	paymentRepo := paymentRepoPkg.New(db)
	paymentService := payment.New(
		paymentRepo,
		log,
		orderService,
		queue,
	)

	issueRepo := issueRepositoryPkg.New(db)
	issueService := issuePkg.NewService(
		log,
		orderService,
		issueRepo,
		issuePkg.NewIssueApiClient(nil, log),
		cfg.IssueApiAddrMain,
		cfg.IssueApiAddrFallback,
	)

	initPaymentAct := actions.InitPayment{
		Orders:      orderService,
		PaymentAddr: cfg.PaymentAddr,
	}

	updatePaymentAct := actions.UpdatePayment{
		Log:    log,
		Repo:   paymentRepo,
		Orders: orderService,
		DB:     db,
	}

	getCodeForOrderAction := actions.GetCodeForOrder{
		Issues: issueService,
		Orders: orderService,
		Log:    log,
	}

	api := webPkg.New(
		log,
		orderService,
		paymentService,
		queue,
		issueService,
		cfg.PaymentAddr,
	)

	paymentService.PollCreatePaymentEvents(context.Background(), 2*time.Second, updatePaymentAct.Do)
	orderService.PollOrderPaidEvents(context.Background(), 2*time.Second, getCodeForOrderAction.Do)
	orderService.PollOrderCreatedEvents(context.Background(), 2*time.Second, initPaymentAct.Do)

	slog.Info("Start web server")
	err = api.Start(webPkg.Options{
		Host: cfg.Host,
		Port: cfg.Port,
	})

	if err != nil {
		panic(fmt.Sprintf("Start server: %s", err))
	}
}
