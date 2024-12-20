package tokens

import (
	"time"

	"github.com/pilatescomplete-bot/internal/keys"
)

type Token struct {
	CredentialsID string    `json:"credentials_id"`
	Token         string    `json:"token"`
	Expires       time.Time `json:"time"`
}

func (c Token) Encode(key *keys.Key) (*EncodedToken, error) {
	encoded, err := key.Encrypt([]byte(c.Token))
	if err != nil {
		return nil, err
	}
	return &EncodedToken{
		CredentialsID: c.CredentialsID,
		Token:         encoded,
		Expires:       c.Expires,
	}, nil
}

type EncodedToken struct {
	CredentialsID string    `json:"credentials_id"`
	Token         []byte    `json:"token"`
	Expires       time.Time `json:"time"`
}

func (e EncodedToken) Decode(key *keys.Key) (*Token, error) {
	token, err := key.Decrypt(e.Token)
	if err != nil {
		return nil, err
	}
	return &Token{
		CredentialsID: e.CredentialsID,
		Token:         string(token),
		Expires:       e.Expires,
	}, nil
}
