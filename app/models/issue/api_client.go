package issue

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"time"

	"github.com/cenkalti/backoff/v5"
)

// backoff settings
var (
	BackoffInitialInterval = 1 * time.Second
	BackoffMaxTries        = uint(5)
)

// http client settings
var (
	ClientTLSHandshakeTimeout = 2 * time.Second
	ClientDialTimeout         = 2 * time.Second
	ClientDialKeepAlive       = 30 * time.Second
	ClientTimeout             = 5 * time.Second
)

// http client for api
var defaultApiHttpClient = &http.Client{
	Transport: &http.Transport{
		DialContext: (&net.Dialer{
			Timeout:   ClientDialTimeout,   // Connection timeout
			KeepAlive: ClientDialKeepAlive, // TCP keepalive interval
		}).DialContext,
		TLSHandshakeTimeout: ClientTLSHandshakeTimeout,
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 10,
	},
	Timeout: ClientTimeout,
}

type IssueApiClient struct {
	http *http.Client
	log  *slog.Logger
}

func NewIssueApiClient(http *http.Client, log *slog.Logger) *IssueApiClient {
	if http == nil {
		http = defaultApiHttpClient
	}

	return &IssueApiClient{
		http: http,
		log:  log,
	}
}

func (c *IssueApiClient) postIssue(ctx context.Context, issue *Issue) (*http.Response, *IssueResponse, error) {
	reqBody, err := json.Marshal(IssueRequest{
		RequestID: issue.RequestID,
		SKU:       issue.SKU,
		OrderID:   issue.OrderExtID,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("marshal issue: %w", err)
	}

	isSuccessFn := func(statusCode int) bool {
		return statusCode >= 200 && statusCode < 300
	}

	isClientErrorFn := func(statusCode int) bool {
		return statusCode >= 400 && statusCode < 500
	}

	log := c.log.With("reqID", issue.RequestID, "orderExtID", issue.OrderExtID)

	getCode := func() (*http.Response, error) {
		apiAddr := issue.ApiAddr + "/issue"

		log.Info(fmt.Sprintf("HTTP POST %s", apiAddr))

		resp, err := c.http.Post(apiAddr, "application/json", bytes.NewReader(reqBody))
		if err != nil {
			log.Error(fmt.Sprintf("HTTP POST err %s", err))
			return nil, fmt.Errorf("post issue: %w", err)
		} else {
			log.Info("HTTP POST response", "http.code", resp.StatusCode)
		}

		if isSuccessFn(resp.StatusCode) || isClientErrorFn(resp.StatusCode) {
			return resp, nil
		} else {
			err := fmt.Errorf("invalid response status code %d", resp.StatusCode)
			if resp.StatusCode >= 500 {
				return resp, err
			}
			return resp, backoff.Permanent(err)
		}
	}

	expBackoff := backoff.NewExponentialBackOff()
	expBackoff.InitialInterval = BackoffInitialInterval

	backoffRetryOpts := []backoff.RetryOption{
		backoff.WithBackOff(expBackoff),
		backoff.WithMaxTries(BackoffMaxTries),
	}
	resp, err := backoff.Retry(ctx, getCode, backoffRetryOpts...)
	if err != nil {
		return resp, nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if isSuccessFn(resp.StatusCode) || isClientErrorFn(resp.StatusCode) {
		respBody, err := io.ReadAll(resp.Body)
		if err != nil {
			return resp, nil, fmt.Errorf("read body: %w", err)
		}

		var issueResponse IssueResponse
		err = json.Unmarshal(respBody, &issueResponse)
		if err != nil {
			return resp, nil, err
		}

		return resp, &issueResponse, err
	} else {
		return resp, nil, fmt.Errorf("server error with status code: %d", resp.StatusCode)
	}
}

func canRetryWithFallback(issue Issue) bool {
	return issue.Code == "" && (issue.RequestStatus == RequestStatusUnavailable || issue.ResponseCode >= 500)
}

func isServerUnavailableErr(err error) bool {
	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		return !urlErr.Timeout() || isDialError(urlErr)
	}

	return false

}

func isDialError(urlErr *url.Error) bool {
	var netOpErr *net.OpError
	if errors.As(urlErr.Err, &netOpErr) {
		if netOpErr.Op == "dial" {
			return true
		}
	}

	return false
}
