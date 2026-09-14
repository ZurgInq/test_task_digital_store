package issue

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"

	orderPkg "app/models/order"

	"go.rtnl.ai/x/randstr"
	"gorm.io/gorm"
)

type Status string

const (
	StatusOk    Status = "ok"
	StatusError Status = "error"
)

type RequestStatus string

const (
	RequestStatusOk          RequestStatus = "ok"
	RequestStatusTimeout     RequestStatus = "timeout"
	RequestStatusUnavailable RequestStatus = "unavailable"
)

const ErrReasonOutOfStock string = "out_of_stock"

// Результат запроса кода из сервиса поставщика
type Issue struct {
	gorm.Model

	RequestID     string        // ИД для идемпотентности
	SKU           string        //
	OrderExtID    string        // ИД заказа
	Code          string        // Выданный код
	Status        Status        // Результат api запроса ok/error
	ErrReason     string        // Код ошибки api
	ApiAddr       string        // Адрес сервиса api. Для повторных запросов в случае таймаутов.
	RequestStatus RequestStatus // Результат http запроса: ok/timeout/unavailable
	ResponseCode  int           // Код http
}

type IssueRequest struct {
	RequestID string `json:"request_id"`
	SKU       string `json:"sku"`
	OrderID   string `json:"order_id"`
}

type IssueResponse struct {
	Status    Status `json:"status"`
	RequestID string `json:"request_id"`
	Code      string `json:"code"`
	Reason    string `json:"reason"`
}

type Repository interface {
	Create(ctx context.Context, issue *Issue) (uint, error)
	GetById(ctx context.Context, id uint) (Issue, error)
	GetByOrderExtID(ctx context.Context, extID string) ([]Issue, error)
	UpdateById(ctx context.Context, id uint, issue Issue) error
	GetFirstIncomplete(ctx context.Context, orderIDExt string) (*Issue, error)
}

type Service struct {
	log                  *slog.Logger
	orders               *orderPkg.Service
	repo                 Repository
	apiClient            *IssueApiClient
	issueApiAddrMain     string
	issueApiAddrFallback string
}

func NewService(
	log *slog.Logger,
	orders *orderPkg.Service,
	repo Repository,
	apiClient *IssueApiClient,
	apiAddrMain string,
	apiAddrFallback string,
) *Service {
	if apiClient.http == nil {
		apiClient.http = defaultApiHttpClient
	}

	return &Service{
		log,
		orders,
		repo,
		apiClient,
		apiAddrMain,
		apiAddrFallback,
	}
}

func (s Service) WithApiAddr(
	main string,
	fallback string,
) *Service {
	s.issueApiAddrMain = main
	s.issueApiAddrFallback = fallback

	return &s
}

func (s *Service) GetIssuesByOrderExtID(ctx context.Context, orderExtID string) ([]Issue, error) {
	return s.repo.GetByOrderExtID(ctx, orderExtID)
}

func (s *Service) RequestCodeForOrder(ctx context.Context, order orderPkg.Order) (Issue, error) {
	var issue Issue

	incompleteIssue, err := s.repo.GetFirstIncomplete(ctx, order.ExtID)
	if err != nil {
		return issue, fmt.Errorf("get incomplete issue")
	}

	if incompleteIssue != nil {
		issue, err = s.requestCode(ctx, *incompleteIssue)
	} else {
		issue, err = s.requestCodeWithCreate(ctx, Issue{
			RequestID:  "req_" + randstr.AlphaNumeric(9),
			SKU:        "stub_xxx",
			OrderExtID: order.ExtID,
			ApiAddr:    s.issueApiAddrMain,
		})
	}

	if err != nil {
		return issue, fmt.Errorf("request code: %w", err)
	}

	if issue.Code == "" || canRetryWithFallback(issue) {
		if canRetryWithFallback(issue) && s.issueApiAddrFallback != "" {
			fallbackAddr := s.issueApiAddrFallback
			if issue.ApiAddr == fallbackAddr {
				fallbackAddr = s.issueApiAddrMain
			}

			issue, err = s.requestCodeWithUpdateAddr(ctx, issue, fallbackAddr)
			if err != nil {
				return issue, fmt.Errorf("request code for order from callback %s: %w", issue.ApiAddr, err)
			}
		}
	}

	if err != nil {
		return issue, fmt.Errorf("request code for order from %s: %w", issue.ApiAddr, err)
	}

	return issue, nil
}

func (s *Service) requestCodeWithCreate(ctx context.Context, issue Issue) (Issue, error) {
	if issue.ID == 0 {
		issueID, err := s.repo.Create(ctx, &issue)
		if err != nil {
			return issue, fmt.Errorf("create issue: %w", err)
		}
		issue.ID = issueID
	}

	return s.requestCode(ctx, issue)
}

func (s *Service) requestCodeWithUpdateAddr(ctx context.Context, issue Issue, apiAddr string) (Issue, error) {
	if issue.ApiAddr != apiAddr {
		err := s.repo.UpdateById(ctx, issue.ID, Issue{
			ApiAddr: apiAddr,
		})
		if err != nil {
			return issue, fmt.Errorf("update issue apiAddr: %w", err)
		}
		issue.ApiAddr = apiAddr
	}

	return s.requestCode(ctx, issue)
}

func (s *Service) requestCode(ctx context.Context, issue Issue) (Issue, error) {
	resp, issueResp, err := s.apiClient.postIssue(ctx, &issue)
	if err != nil {
		if issue.RequestStatus == "" {
			s.log.Info("Save issue error", "issue.ID", issue.ID, "err", err.Error())
			return s.saveIssueError(ctx, issue.ID, resp, err)
		}
		return issue, nil
	}

	if issueResp == nil {
		return issue, fmt.Errorf("empty issue response")
	}

	issue, err = s.saveIssueResponse(ctx, issue.ID, resp, issueResp)
	return issue, err
}

func (s *Service) saveIssueResponse(ctx context.Context, issueID uint, resp *http.Response, issueResp *IssueResponse) (Issue, error) {
	var issue Issue
	updateIssue := Issue{
		Status:        issueResp.Status,
		RequestStatus: RequestStatusOk,
	}

	if resp != nil {
		updateIssue.ResponseCode = resp.StatusCode
	}

	if issueResp.Status == StatusOk {
		updateIssue.Code = issueResp.Code
	} else {
		updateIssue.ErrReason = issueResp.Reason
	}

	err := s.repo.UpdateById(ctx, issueID, updateIssue)
	if err != nil {
		return issue, fmt.Errorf("update issue %d: %w", issueID, err)
	}

	issue, err = s.repo.GetById(ctx, issueID)
	if err != nil {
		return issue, fmt.Errorf("get updated issue: %w", err)
	}

	return issue, nil
}

func (s *Service) saveIssueError(ctx context.Context, issueID uint, resp *http.Response, err error) (Issue, error) {
	var reqStatus RequestStatus
	if isServerUnavailableErr(err) {
		reqStatus = RequestStatusUnavailable
	} else {
		reqStatus = RequestStatusTimeout
	}

	responseCode := 0
	if resp != nil {
		responseCode = resp.StatusCode
	}

	err = s.repo.UpdateById(ctx, issueID, Issue{
		RequestStatus: reqStatus,
		ResponseCode:  responseCode,
	})

	if err != nil {
		return Issue{}, fmt.Errorf("update issue %d: %w", issueID, err)
	}

	return s.repo.GetById(ctx, issueID)
}
