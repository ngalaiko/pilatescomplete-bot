package credentials

import (
	gonanoid "github.com/matoous/go-nanoid/v2"
	"github.com/pilatescompletebot/internal/keys"
)

type ID string

func NewID() ID {
	return ID(gonanoid.Must())
}

type Credentials struct {
	ID       ID     `json:"id"`
	Login    string `json:"login"`
	Password string `json:"password"`
}

func (c Credentials) Encode(key *keys.Key) (*EncodedCredentials, error) {
	encoded, err := key.Encrypt([]byte(c.Password))
	if err != nil {
		return nil, err
	}
	return &EncodedCredentials{
		ID:       c.ID,
		Login:    c.Login,
		Password: encoded,
	}, nil
}

type EncodedCredentials struct {
	ID       ID     `json:"id"`
	Login    string `json:"login"`
	Password []byte `json:"password"`
}

func (e EncodedCredentials) Decode(key *keys.Key) (*Credentials, error) {
	password, err := key.Decrypt(e.Password)
	if err != nil {
		return nil, err
	}
	return &Credentials{
		ID:       e.ID,
		Login:    e.Login,
		Password: string(password),
	}, nil
}
