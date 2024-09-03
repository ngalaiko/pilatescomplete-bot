package authentication

import (
	"context"
	"errors"
	"fmt"

	"github.com/pilatescomplete-bot/internal/credentials"
	"github.com/pilatescomplete-bot/internal/pilatescomplete"
	"github.com/pilatescomplete-bot/internal/tokens"
)

type Service struct {
	tokensStore            *tokens.Store
	credentialsStore *credentials.Store
	apiClient        *pilatescomplete.APIClient
}

func NewService(
	tokensStore *tokens.Store,
	credentialsStore *credentials.Store,
	apiClient *pilatescomplete.APIClient,
) *Service {
	return &Service{
		tokensStore:            tokensStore,
		credentialsStore: credentialsStore,
		apiClient:        apiClient,
	}
}

func (s *Service) AuthenticateContext(ctx context.Context, credentialsID credentials.ID) (context.Context, error) {
	token, err := s.tokensStore.FindByID(ctx, credentialsID)
	if errors.Is(err, tokens.ErrNotFound) {
		creds, err := s.credentialsStore.FindByID(ctx, credentialsID)
		if err != nil {
			return ctx, fmt.Errorf("find credentials %q: %w", credentialsID, err)
		}

		cookie, err := s.apiClient.Login(ctx, pilatescomplete.LoginData{
			Login:    creds.Login,
			Password: creds.Password,
		})
		if err != nil {
			return ctx, fmt.Errorf("login: %w", err)
		}

		token = &tokens.Token{
			CredentialsID: creds.ID,
			Token:         cookie.Value,
			Expires:       cookie.Expires,
		}

		if err := s.tokensStore.Insert(ctx, token); err != nil {
			return ctx, fmt.Errorf("insert token: %w", err)
		}
	} else if err != nil {
		return ctx, fmt.Errorf("find token by credentialsID %q: %w")
	}
	return tokens.NewContext(ctx, token), nil
}
