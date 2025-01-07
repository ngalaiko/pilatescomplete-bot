package notifications

import (
	"github.com/pilatescomplete-bot/internal/pilatescomplete"
)

type NotificationType uint

const (
	NotificationTypeUnknown NotificationType = iota
	NotificationTypeBooked
	NotificationTypeUnbooked
)

type Notification struct {
	ID   string
	Type NotificationType
	Body string
}

func notificationsFromAPI(notifications *pilatescomplete.ListNotificationsResponse) ([]*Notification, error) {
	out := make([]*Notification, len(notifications.Notification))
	for i := range notifications.Notification {
		event := notifications.Notification[i]
		out[i] = &Notification{
			ID:   event.Notification.ID,
			Type: typeFromAPI(event.Notification.Type),
			Body: event.Notification.Notification,
		}
	}
	return out, nil
}

func typeFromAPI(typ pilatescomplete.NotificationType) NotificationType {
	switch typ {
	case pilatescomplete.NoticicationTypeBooked:
		return NotificationTypeBooked
	case pilatescomplete.NoticicationTypeUnbooked:
		return NotificationTypeUnbooked
	default:
		return NotificationTypeUnknown
	}
}
