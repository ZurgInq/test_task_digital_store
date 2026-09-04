package main

import (
	"fmt"
	"log/slog"
	"math/rand/v2"
	"net/http"
	"strings"
	"time"

	"github.com/caarlos0/env/v11"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/render"
	"go.rtnl.ai/x/randstr"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type Issue struct {
	gorm.Model

	RequestID string
	OrderID   string
	CodeID    *uint `gorm:"unique"`
}

type Code struct {
	gorm.Model

	Code    string `gorm:"unique"`
	OrderID string
	IssueID *uint
}

type IssueReq struct {
	RequestID string `json:"request_id"`
	Sku       string `json:"sku"`
	OrderID   string `json:"order_id"`
}

type IssueResp struct {
	Status    string `json:"status"`
	RequestID string `json:"request_id"`
	Code      string `json:"code"`
}

func (i *IssueResp) Render(w http.ResponseWriter, r *http.Request) error {
	render.Status(r, http.StatusOK)
	return nil
}

type Config struct {
	Host           string `env:"APP_HOST"`
	Port           string `env:"APP_PORT" envDefault:"3002"`
	TestTimeoutSec int    `env:"APP_TEST_TIMEOUT_SEC" envDefault:"10"` // Эмуляция долгой обработки запроса
	TestTimeoutP   int    `env:"APP_TEST_TIMEOUT_P" envDefault:"0"`    // Процент ответов с таймаутом APP_TEST_TIMEOUT_SEC
	TestErrorsP    int    `env:"APP_TEST_ERRORS_P" envDefault:"0"`     // Процент ответов с ошибкой 500
}

func getTestError(timeoutP int, errorP int) string {
	if timeoutP <= 0 && errorP <= 0 {
		return ""
	}

	r := rand.IntN(100)

	switch {
	case r < timeoutP:
		return "timeout"
	case r < timeoutP+errorP:
		return "error"
	default:
		return ""
	}
}

func main() {
	log := slog.Default()

	cfg := Config{}
	err := env.Parse(&cfg)
	if err != nil {
		panic(err)
	}

	db := NewDB()
	Migrate(db)

	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.URLFormat)
	r.Use(render.SetContentType(render.ContentTypeJSON))

	r.Route("/issue", func(r chi.Router) {
		// Выдача кода
		r.Post("/", func(w http.ResponseWriter, r *http.Request) {

			switch getTestError(cfg.TestTimeoutP, cfg.TestErrorsP) {
			case "error":
				internalServerError(w, r, fmt.Errorf("test error"))
				return
			case "timeout":
				time.Sleep(time.Duration(cfg.TestTimeoutSec) * time.Second)
			}

			issueReq := &IssueReq{}
			render.Decode(r, issueReq)

			err := db.Transaction(func(db *gorm.DB) error {
				exists := &Issue{}
				err := db.Debug().
					Where("request_id = ? AND order_id = ?", issueReq.RequestID, issueReq.OrderID).
					Limit(1).
					Find(exists).Error

				if err != nil {
					return err
				}

				if exists.ID != 0 {
					code := &Code{}
					err := db.Where("issue_id = ?", exists.ID).First(code).Error
					if err != nil {
						return err
					}

					renderIssueOK(w, r, *exists, *code)
					return nil
				}

				issue := Issue{
					RequestID: issueReq.RequestID,
					OrderID:   issueReq.OrderID,
				}

				err = db.Create(&issue).Error
				if err != nil {
					return err
				}

				log.Info("Issue created", "issue", issue)

				// naive atomic update
				result := db.Debug().
					Table("codes").
					Where(
						"id = (?)",
						db.Table("codes as c").Select("c.id").Where("c.issue_id IS NULL").Limit(1),
					).
					Update("issue_id", issue.ID)

				if result.RowsAffected == 0 {
					renderIssueOutOfStock(w, r)
					return nil
				}

				if err := result.Error; err != nil {
					return err
				}

				code := &Code{}
				err = db.Where("issue_id = ?", issue.ID).First(code).Error
				if err != nil {
					return err
				}

				renderIssueOK(w, r, issue, *code)
				return nil
			})

			if err != nil {
				internalServerError(w, r, err)
			}
		})
	})

	r.Route("/issues", func(r chi.Router) {
		// Список запросов на получение кодов
		r.Get("/", func(w http.ResponseWriter, r *http.Request) {
			issues := make([]Issue, 0)

			err := db.Limit(1000).Find(&issues).Error
			if err != nil {
				internalServerError(w, r, err)
				return
			}

			render.JSON(w, r, issues)
		})
	})

	r.Route("/codes", func(r chi.Router) {
		// Список всех кодов
		r.Get("/", func(w http.ResponseWriter, r *http.Request) {
			codes := make([]Code, 0)
			db.Limit(1000).Find(&codes)

			render.JSON(w, r, codes)
		})
	})

	r.Route("/generate/codes", func(r chi.Router) {
		// Генерация нового пула кодов
		r.Post("/", func(w http.ResponseWriter, r *http.Request) {
			codes := make([]*Code, 0, 10)

			for range 10 {
				codeVal := strings.ToUpper(
					randstr.AlphaNumeric(4) + "-" +
						randstr.AlphaNumeric(4) + "-" +
						randstr.AlphaNumeric(4),
				)

				codes = append(codes, &Code{
					Code:    codeVal,
					OrderID: "",
				})
			}

			err := db.Create(codes).Error
			if err != nil {
				internalServerError(w, r, err)
			}

			w.WriteHeader(http.StatusCreated)
			render.JSON(w, r, codes)
		})
	})

	log.Info("Config", "cfg", cfg)
	addr := fmt.Sprintf("%s:%s", cfg.Host, cfg.Port)
	log.Info("Start server", "addr", addr)

	err = http.ListenAndServe(addr, r)
	if err != nil {
		panic(fmt.Sprintf("Start server: %s", err))
	}
}

func NewDB() *gorm.DB {
	db, err := gorm.Open(sqlite.Open("codes.db"), &gorm.Config{})
	if err != nil {
		panic("failed to connect database")
	}

	return db
}

func Migrate(db *gorm.DB) {
	err := db.AutoMigrate(
		Issue{},
		Code{},
	)

	if err != nil {
		panic(fmt.Sprintf("Automigrate: %s", err))
	}
}

func internalServerError(w http.ResponseWriter, r *http.Request, err error) {
	slog.Error(err.Error())
	render.Status(r, http.StatusInternalServerError)
	render.JSON(w, r, map[string]any{
		"error": err.Error(),
	})
}

func renderIssueOK(w http.ResponseWriter, r *http.Request, issue Issue, code Code) {
	render.JSON(w, r, &IssueResp{
		Status:    "ok",
		RequestID: issue.RequestID,
		Code:      code.Code,
	})
}

func renderIssueOutOfStock(w http.ResponseWriter, r *http.Request) {
	render.Status(r, http.StatusConflict)
	render.JSON(w, r, map[string]any{
		"status": "error",
		"reason": "out_of_stock",
	})
}
