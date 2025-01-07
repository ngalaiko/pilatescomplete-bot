package statistics

import (
	"bufio"
	"context"
	"fmt"
	"maps"
	"slices"
	"strings"
	"time"

	"github.com/pilatescomplete-bot/internal/notifications"
)

type Service struct {
	notificationsSerice *notifications.Service
}

func NewService(
	notificationsService *notifications.Service,
) *Service {
	return &Service{
		notificationsSerice: notificationsService,
	}
}

func (s *Service) CalculateYearMonth(ctx context.Context, year int, month time.Month) (*Month, error) {
	entries, err := s.calculateEnteries(ctx)
	if err != nil {
		return nil, fmt.Errorf("calculate entries: %w", err)
	}
	stats := &Month{}
	monthStart := time.Date(year, month, 1, 0, 0, 0, 0, time.UTC)
	weekIndex := map[int]int{}
	for d := monthStart; d.Before(monthStart.AddDate(0, 1, 0)); d = d.AddDate(0, 0, 1) {
		_, week := d.ISOWeek()
		if _, ok := weekIndex[week]; !ok {
			weekIndex[week] = len(weekIndex)
			stats.Weeks = append(stats.Weeks, Week{
				Number: week,
			})
		}
	}
	classesByName := map[string]int{}
	now := time.Now()
	for _, entry := range entries {
		if entry.Time.After(now) {
			continue
		}
		if entry.Time.Year() != year {
			continue
		}
		if entry.Time.Month() != month {
			continue
		}
		_, week := entry.Time.ISOWeek()
		stats.Total++
		stats.Weeks[weekIndex[week]].Total++
		classesByName[entry.DisplayName]++
	}

	for displayName, total := range classesByName {
		stats.Classes = append(stats.Classes, Class{
			Total:       total,
			DisplayName: displayName,
		})
	}
	slices.SortFunc(stats.Classes, func(a, b Class) int {
		return b.Total - a.Total
	})

	return stats, nil
}

func (s *Service) CalculateYear(ctx context.Context, year int) (*Year, error) {
	entries, err := s.calculateEnteries(ctx)
	if err != nil {
		return nil, fmt.Errorf("calculate entries: %w", err)
	}
	stats := &Year{
		Months: make([]int, 12),
	}
	classesByName := map[string]int{}
	now := time.Now()
	for _, entry := range entries {
		if entry.Time.After(now) {
			continue
		}
		if entry.Time.Year() != year {
			continue
		}
		stats.Total++
		stats.Months[entry.Time.Month()-1]++
		classesByName[entry.DisplayName]++
	}
	for displayName, total := range classesByName {
		stats.Classes = append(stats.Classes, Class{
			Total:       total,
			DisplayName: displayName,
		})
	}
	slices.SortFunc(stats.Classes, func(a, b Class) int {
		return b.Total - a.Total
	})
	return stats, nil
}

type entry struct {
	Time        time.Time
	DisplayName string
}

func (s *Service) calculateEnteries(ctx context.Context) ([]*entry, error) {
	nn, err := s.notificationsSerice.ListNotifications(ctx)
	if err != nil {
		return nil, fmt.Errorf("list notifications: %w", err)
	}
	entriesByDate := make(map[time.Time]*entry, len(nn))
	for _, notification := range nn {
		switch notification.Type {
		case notifications.NotificationTypeBooked:
			entry, err := parseBookedNotification(notification.Body)
			if err != nil {
				return nil, fmt.Errorf("failed to parse booked notification: %w", err)
			}
			entriesByDate[entry.Time] = entry
		case notifications.NotificationTypeUnbooked:
			entry, err := parseUnbookedNotification(notification.Body)
			if err != nil {
				return nil, fmt.Errorf("failed to parse unbooked notification: %w", err)
			}
			delete(entriesByDate, entry.Time)
		default:
			continue
		}
	}
	return slices.Collect(maps.Values(entriesByDate)), nil
}

func firstLine(str string) string {
	r := strings.NewReader(str)
	scanner := bufio.NewScanner(r)
	if scanner.Scan() {
		return scanner.Text()
	}
	if err := scanner.Err(); err != nil {
		return ""
	}
	return ""
}

func parseUnbookedNotification(notification string) (*entry, error) {
	const prefixLength = len("Du är nu avbokad på: ")
	const suffixLength = len(" hos Pilates Complete")
	const dateTimeLength = len("2024-09-01 11:15:00")
	header := firstLine(notification)
	value := header[prefixLength : len(header)-suffixLength]
	time, err := time.Parse(time.DateTime, value[len(value)-dateTimeLength:])
	if err != nil {
		return nil, fmt.Errorf("failed to parse date: %w", err)
	}
	displayName := value[:len(value)-dateTimeLength-1]
	return &entry{
		DisplayName: displayName,
		Time:        time,
	}, nil
}

func parseBookedNotification(notification string) (*entry, error) {
	const prefixLength = len("Du är nu bokad på: ")
	const suffixLength = len(" hos Pilates Complete")
	const dateTimeLength = len("2024-09-01 11:15:00")
	header := firstLine(notification)
	value := header[prefixLength : len(header)-suffixLength]
	time, err := time.Parse(time.DateTime, value[len(value)-dateTimeLength:])
	if err != nil {
		return nil, fmt.Errorf("failed to parse date: %w", err)
	}
	displayName := value[:len(value)-dateTimeLength-1]
	return &entry{
		DisplayName: displayName,
		Time:        time,
	}, nil
}
