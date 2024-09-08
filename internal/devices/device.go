package devices

import (
	"net/http"
	"time"
)

type Device struct {
	CredentialsID string
}

func FromCookies(cookies []*http.Cookie) (*Device, bool) {
	d := &Device{}
	for _, cookie := range cookies {
		switch cookie.Name {
		case "credentials_id":
			d.CredentialsID = cookie.Value
		}
	}
	if len(d.CredentialsID) == 0 {
		return nil, false
	}
	return d, true
}

var (
	minute = time.Second * 60
	hour   = minute * 60
	day    = hour * 24
)

func (d Device) ToCookies(secure bool) []*http.Cookie {
	return []*http.Cookie{
		{
			Name:     "credentials_id",
			Value:    string(d.CredentialsID),
			HttpOnly: true,
			SameSite: http.SameSiteLaxMode,
			Expires:  time.Now().Add(30 * day),
			Secure:   secure,
		},
	}
}
