package pilatescomplete

import (
	"testing"
)

func Test_parseCookie(t *testing.T) {
	rawCookie := "CAKEPHP=14c543c162c889462dd1b09789eafa8d; expires=Mon, 02-Sep-2024 05:11:12 GMT; Max-Age=43200; path=/; secure; HttpOnly"
	cookie := parseCookie(rawCookie)
	if cookie.Name != "CAKEPHP" {
		t.Fatalf("cookie.Name: expected \"CAKEPHP\" got %q", cookie.Name)
	}
	if cookie.Value != "14c543c162c889462dd1b09789eafa8d" {
		t.Fatalf("cookie.Name: expected \"14c543c162c889462dd1b09789eafa8d\" got %q", cookie.Value)
	}
	if cookie.Expires.String() != "2024-09-02 05:11:12 +0000 GMT" {
		t.Fatalf("cookie.Name: expected \"2024-09-02 05:11:12 +0000 GMT\" got %q", cookie.Expires)
	}
}
