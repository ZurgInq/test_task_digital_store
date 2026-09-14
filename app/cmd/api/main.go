package main

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"app/actions"
	dbPkg "app/db"
	balancePkg "app/models/balance"
	transactionRepoPkg "app/models/balance/repository"
	issuePkg "app/models/issue"
	issueRepositoryPkg "app/models/issue/repository"
	"app/models/order"
	orderRepoPkg "app/models/order/repository"
	"app/models/payment"
	paymentRepoPkg "app/models/payment/repository"
	"app/models/product"
	productRepoPkg "app/models/product/repository"
	queuePkg "app/queue"
	webPkg "app/web"

	"github.com/caarlos0/env/v11"
)

const (
	providerGiftCardQueueName = "get-issue:giftcard"
	providerAnyQueueName      = "get-issue:any"
)

var (
	queueProviders = map[product.Type]string{
		product.TypeGiftcard:     providerGiftCardQueueName,
		product.TypeTopUp:        providerAnyQueueName,
		product.TypeKey:          providerAnyQueueName,
		product.TypeSubscription: providerAnyQueueName,
	}
)

type Config struct {
	Host                    string `env:"APP_HOST"`
	Port                    string `env:"APP_PORT" envDefault:"3000"`
	SeedDataDir             string `env:"SEED_DATA_DIR" envDefault:"seed_data"`
	PaymentAddr             string `env:"PAYMENT_ADDR" envDefault:"http://localhost:3001"`
	IssueApiAddrMain        string `env:"ISSUE_API_ADDR_MAIN" envDefault:"http://localhost:3002"`
	IssueApiAddrFallback    string `env:"ISSUE_API_ADDR_FALLBACK" envDefault:"http://localhost:3003"`
	GiftCardApiAddrMain     string `env:"GIFT_CARD_API_ADDR_MAIN" envDefault:"http://localhost:3004"`
	GiftCardApiAddrFallback string `env:"GIFT_CARD_API_ADDR_FALLBACK" envDefault:""`
}

func main() {
	log := slog.Default()
	log.Info("Start app")

	cfg := Config{}
	err := env.Parse(&cfg)
	if err != nil {
		panic(err)
	}

	log.Info("Config parsed", "cfg", cfg)

	db := dbPkg.NewDB("app.db")

	if cfg.SeedDataDir != "" {
		err := seedsProducts(db, cfg.SeedDataDir)
		if err != nil {
			panic(err)
		}
	}

	queue := queuePkg.NewMockQueue(log)

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

	transactionRepo := transactionRepoPkg.New(db)
	balanceService := balancePkg.NewService(
		log,
		transactionRepo,
	)

	initPaymentAct := actions.InitPayment{
		Log:         log,
		Payments:    paymentService,
		PaymentAddr: cfg.PaymentAddr,
	}

	updatePaymentAct := actions.UpdatePayment{
		Log:     log,
		DB:      db,
		Repo:    paymentRepo,
		Orders:  orderService,
		Balance: balanceService,
	}

	chooseProviderAction := actions.ChooseProvider{
		Products:       productService,
		QueueProviders: queueProviders,
		Queue:          queue,
	}

	api := webPkg.New(
		log,
		db,
		orderService,
		paymentService,
		queue,
		issueService,
		productService,
		balanceService,
		cfg.PaymentAddr,
	)

	paymentService.PollInitEvents(context.Background(), 2*time.Second, initPaymentAct.Do)
	paymentService.PollUpdateEvents(context.Background(), 2*time.Second, updatePaymentAct.Do)
	orderService.PollOrderPaidEvents(context.Background(), 2*time.Second, chooseProviderAction.Do)

	getCodeForOrderAction := actions.GetCodeForOrder{
		DB:      db,
		Log:     log,
		Issues:  issueService,
		Orders:  orderService,
		Balance: balanceService,
	}

	// queue for all products
	queuePkg.PollEvents(
		context.Background(),
		log,
		queue,
		providerAnyQueueName,
		2*time.Second,
		getCodeForOrderAction.Do,
	)

	getCodeForOrderActionGiftCard := getCodeForOrderAction
	getCodeForOrderActionGiftCard.Issues = issueService.WithApiAddr(
		cfg.GiftCardApiAddrMain,
		cfg.GiftCardApiAddrFallback,
	)

	// queue for giftcard products
	queuePkg.PollEvents(
		context.Background(),
		log,
		queue,
		providerGiftCardQueueName,
		2*time.Second,
		getCodeForOrderActionGiftCard.Do,
	)

	log.Info("Start web server")
	err = api.Start(webPkg.Options{
		Host: cfg.Host,
		Port: cfg.Port,
	})

	if err != nil {
		panic(fmt.Sprintf("Start server: %s", err))
	}
}
