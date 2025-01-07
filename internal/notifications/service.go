package notifications

import (
	"context"
	"fmt"

	"github.com/pilatescomplete-bot/internal/pilatescomplete"
)

type Service struct {
	apiClient *pilatescomplete.APIClient
}

func NewService(
	apiClient *pilatescomplete.APIClient,
) *Service {
	return &Service{
		apiClient: apiClient,
	}
}

func (s *Service) ListNotifications(ctx context.Context) ([]*Notification, error) {
	return s.listNotiications(ctx, pilatescomplete.ListNotificationsInput{})
}

func (s *Service) listNotiications(ctx context.Context, input pilatescomplete.ListNotificationsInput) ([]*Notification, error) {
	apiResponse, err := s.apiClient.ListNotifications(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("list notifications: %w", err)
	}
	events, err := notificationsFromAPI(apiResponse)
	if err != nil {
		return nil, fmt.Errorf("events from api: %w", err)
	}
	return events, nil
}
