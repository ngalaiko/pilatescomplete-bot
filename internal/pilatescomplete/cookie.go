package pilatescomplete

import (
	"net/http"
	"strings"
	"time"
)

func parseCookie(rawCookie string) http.Cookie {
	cookie := http.Cookie{}
	for _, part := range strings.Split(rawCookie, ";") {
		part = strings.TrimSpace(part)
		keyValue := strings.Split(part, "=")
		key, value := keyValue[0], ""
		if len(keyValue) == 2 {
			value = keyValue[1]
		}
		switch key {
		case "expires":
			t, err := time.Parse("Mon, 02-Jan-2006 15:04:05 MST", value)
			if err == nil {
				cookie.Expires = t
			}
		case "Max-Age":
		case "path":
		case "secure":
		case "HttpOnly":
		default:
			cookie.Name = key
			cookie.Value = value
		}
	}
	return cookie
}
