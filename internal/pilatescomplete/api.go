package pilatescomplete

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/pilatescompletebot/internal/tokens"
)

var (
	ErrInvalidLoginOrPassword  = errors.New("invalid login or password")
	ErrTokenMissingFromContext = errors.New("token missing from context")
)

var CookieName = "CAKEPHP"

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

func (c APIClient) Login(data LoginData) (*http.Cookie, error) {
	log.Printf("[INFO] pilatescompleteapi: login")

	body := fmt.Sprintf("_method=POST&data[User][email]=%s&data[User][password]=%s\n", data.Login, data.Password)

	request, err := http.NewRequest(http.MethodPost, "https://pilatescomplete.wondr.se/", strings.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	request.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("Content-Length", strconv.Itoa(len(body)))

	response, err := c.httpClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer response.Body.Close()

	for _, cookie := range response.Cookies() {
		if cookie.Name == CookieName {
			return cookie, nil
		}
	}
	return nil, ErrInvalidLoginOrPassword
}

type ListEventsInput struct {
	From *time.Time
	To   *time.Time
}

type ListEventsResponse struct {
	Events []*Event `json:"activities"`
}

func (c APIClient) ListEvents(ctx context.Context, input ListEventsInput) (*ListEventsResponse, error) {
	token, ok := tokens.FromContext(ctx)
	if !ok {
		return nil, ErrTokenMissingFromContext
	}

	values := url.Values{}
	if input.From != nil {
		values.Add("from", input.From.Format(time.DateOnly))
	}
	if input.To != nil {
		values.Add("to", input.To.Format(time.DateOnly))
	}

	request, err := http.NewRequest(
		http.MethodGet,
		fmt.Sprintf("https://pilatescomplete.wondr.se/w_booking/activities/list?%s", values.Encode()),
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	request.Header.Set("Accept", "application/json")
	request.AddCookie(&http.Cookie{Name: CookieName, Value: token.Token})

	log.Printf("[INFO] %s %s", request.Method, request.URL)

	resp, err := c.httpClient.Do(request)
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
