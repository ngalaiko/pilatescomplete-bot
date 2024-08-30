package pilatescomplete

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
)

var ErrInvalidLoginOrPassword = errors.New("invalid login or password")

var CookieName = "CAKEPHP"

type Client struct {
	httpClient http.Client
}

func NewClient() *Client {
	return &Client{
		httpClient: http.Client{},
	}
}

type LoginData struct {
	Login    string
	Password string
}

func (c Client) Login(data LoginData) (*http.Cookie, error) {
	log.Printf("[INFO] pilatescompleteapi: login")

	body := fmt.Sprintf("_method=POST&data[User][email]=%s&data[User][password]=%s\n", data.Login, data.Password)

	request, err := http.NewRequest(http.MethodPost, "https://pilatescomplete.wondr.se/", strings.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	request.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("Content-Length", strconv.Itoa(len(body)))
	request.Header.Set("User-Agent", "hello ?")
	request.Header.Set("Accept", "*/*")
	request.Header.Set("Host", "pilatescomplete.wondr.se")

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
