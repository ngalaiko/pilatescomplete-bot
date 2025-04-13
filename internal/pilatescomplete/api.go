package pilatescomplete

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"os/exec"
	"strings"

	"github.com/pilatescomplete-bot/internal/tokens"
)

var (
	ErrInvalidLoginOrPassword  = errors.New("invalid login or password")
	ErrTokenMissingFromContext = errors.New("token missing from context")
	ErrNotFound                = errors.New("not found")
)

var (
	cookieName = "CAKEPHP"
	userAgent  = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10.15; rv:129.0) Gecko/20100101 Firefox/129.0"
)

type APIClient struct {
	httpClient http.Client
}

func NewAPIClient() *APIClient {
	return &APIClient{
		httpClient: http.Client{},
	}
}

type LoginData struct {
	Login    string
	Password string
}

func (c APIClient) Login(ctx context.Context, data LoginData) (*http.Cookie, error) {
	slog.InfoContext(ctx, "login")
	values := url.Values{}

	values.Set("_method", http.MethodPost)
	values.Set("data[User][email]", data.Login)
	values.Set("data[User][password]", data.Password)
	body := values.Encode()

	cmd := exec.CommandContext(ctx, "curl",
		"https://pilatescomplete.wondr.se/",
		"--request", "POST",
		"--header", fmt.Sprintf("User-Agent: %s", userAgent),
		"--header", "Content-Type: application/x-www-form-urlencoded",
		"--header", "Accept: text/html",
		"--header", fmt.Sprintf("Content-Length: %d", len(body)),
		"--data-raw", body,
		"--silent",
		"--http1.1",
		"--include",
	)
	stdout := &bytes.Buffer{}
	cmd.Stdout = stdout
	if err := cmd.Run(); err != nil {
		return nil, err
	}

	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		line := scanner.Text()
		if rawCookie, ok := strings.CutPrefix(line, "set-cookie: "); ok {
			cookie := parseCookie(rawCookie)
			return &cookie, nil
		}
	}

	return nil, ErrInvalidLoginOrPassword
}

type ListEventsInput struct {
	ActivityID string
}

type ListEventsResponse struct {
	Events                   []*Event          `json:"activities"`
	ActicityTypeDescriptions map[string]string `json:"activityTypeDescriptions"`
}

func (c APIClient) ListEvents(ctx context.Context, input ListEventsInput) (*ListEventsResponse, error) {
	values := url.Values{}
	if input.ActivityID != "" {
		values.Set("activity", input.ActivityID)
	}
	req, err := http.NewRequest(
		http.MethodGet,
		fmt.Sprintf("https://pilatescomplete.wondr.se/w_booking/activities/list?%s", values.Encode()),
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", userAgent)

	if err := authenticateRequest(ctx, req); err != nil {
		return nil, err
	}
	slog.InfoContext(ctx, "api request", "method", req.Method, "url", req.URL)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	response := &ListEventsResponse{}
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return response, nil
}

type ErrorResponse struct {
	Message   string `json:"message"`
	ErrorCode string `json:"error_code"`
}

func (p ErrorResponse) Error() string {
	return fmt.Sprintf("%s: %s", p.ErrorCode, p.Message)
}

type APIResponse struct {
	Result string `json:"result"`
	ErrorResponse
}

var (
	ErrActivityBookingTooEarly = errors.New("too early to book the activity")
	ErrActivityAlreadyBooked   = errors.New("activity booking already exists")
	ErrAccessNotAllowed        = errors.New("you cant book any more activities this day")
	ErrOverbooked              = errors.New("you are currently booked on maximum allowed simultaneous bookings")
)

func (r APIResponse) Error() error {
	if r.Result != "error" {
		return nil
	}
	if r.ErrorCode == "USER_ALREADY_BOOKED" {
		return ErrActivityAlreadyBooked
	}
	if r.ErrorCode == "ACTIVITY_BOOKING_TO_EARLY" {
		return ErrActivityBookingTooEarly
	}
	if r.ErrorCode == "ACCESS_NOT_ALLOWED" {
		return ErrAccessNotAllowed
	}
	if r.ErrorCode == "USER_OVER_MAX_CONCURRENT_BOOKINGS" {
		return ErrOverbooked
	}
	return r.ErrorResponse
}

func (r APIResponse) IsOK() bool {
	return r.Result == "ok"
}

type participateResponse struct {
	APIResponse
	ActivityBooking
}

type cancelResponse struct {
	APIResponse
}

func (c APIClient) BookActivity(ctx context.Context, activityID string) (*ActivityBooking, error) {
	req, err := http.NewRequest(
		http.MethodPost,
		fmt.Sprintf("https://pilatescomplete.wondr.se/w_booking/activities/participate/%s/?force=1", activityID),
		strings.NewReader(`{"ActivityBooking":{"extras":{},"resources":{},"participants":1}}`),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", userAgent)

	if err := authenticateRequest(ctx, req); err != nil {
		return nil, err
	}
	slog.InfoContext(ctx, "api request", "method", req.Method, "url", req.URL)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	response := &participateResponse{}
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if err := response.Error(); err != nil {
		return nil, err
	}

	if !response.IsOK() {
		return nil, fmt.Errorf("%q: execpected result", response.Result)
	}

	return &response.ActivityBooking, nil
}

func (c APIClient) CancelBooking(ctx context.Context, activityBookingID string) error {
	req, err := http.NewRequest(
		http.MethodPost,
		fmt.Sprintf("https://pilatescomplete.wondr.se/w_booking/activities/cancel/%s/1?force=1", activityBookingID),
		strings.NewReader(`{"ActivityBooking":{"extras":{},"resources":{},"participants":1}}`),
	)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", userAgent)

	if err := authenticateRequest(ctx, req); err != nil {
		return err
	}
	slog.InfoContext(ctx, "api request", "method", req.Method, "url", req.URL)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	response := &cancelResponse{}
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	if err := response.Error(); err != nil {
		return err
	}

	if !response.IsOK() {
		return fmt.Errorf("%q: execpected result", response.Result)
	}

	return nil
}

func authenticateRequest(ctx context.Context, req *http.Request) error {
	token, ok := tokens.FromContext(ctx)
	if !ok {
		return ErrTokenMissingFromContext
	}
	req.Header.Set("Cookie", fmt.Sprintf("%s=%s", cookieName, token.Token))
	return nil
}

type ListNotificationsInput struct{}

type NotificationResponse struct {
	Notification Notification `json:"Notification"`
}

type ListNotificationsResponse struct {
	Notification []*NotificationResponse `json:"notification"`
}

func (c APIClient) ListNotifications(ctx context.Context, input ListNotificationsInput) (*ListNotificationsResponse, error) {
	values := url.Values{}
	values.Set("sort", "Notification.created")
	values.Set("direction", "ASC")
	req, err := http.NewRequest(
		http.MethodGet,
		fmt.Sprintf("https://pilatescomplete.wondr.se/notifications/list?%s", values.Encode()),
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", userAgent)

	if err := authenticateRequest(ctx, req); err != nil {
		return nil, err
	}
	slog.InfoContext(ctx, "api request", "method", req.Method, "url", req.URL)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	response := &ListNotificationsResponse{}
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return response, nil
}
