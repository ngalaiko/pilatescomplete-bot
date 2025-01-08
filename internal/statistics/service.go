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

func (s *Service) CalculateYearWeek(ctx context.Context, year int, week int) (*WeekStatistics, error) {
	entries, err := s.calculateEnteries(ctx)
	if err != nil {
		return nil, fmt.Errorf("calculate entries: %w", err)
	}
	stats := &WeekStatistics{}
	weekStart := getDateFromISOWeek(year, week)
	daysIndex := map[int]int{}
	for d := weekStart; d.Before(weekStart.AddDate(0, 0, 7)); d = d.AddDate(0, 0, 1) {
		day := d.Day()
		if _, ok := daysIndex[day]; !ok {
			daysIndex[day] = len(daysIndex)
			stats.Days = append(stats.Days, Day{
				Number: day,
			})
		}
	}

	classesByName := map[string]int{}
	now := time.Now()
	for _, entry := range entries {
		if entry.Time.After(now) {
			continue
		}
		entryYear, entryWeek := entry.Time.ISOWeek()
		if entryYear != year {
			continue
		}
		if entryWeek != week {
			continue
		}
		stats.Total++
		stats.Days[daysIndex[entry.Time.Day()]].Total++
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

func (s *Service) CalculateYearMonth(ctx context.Context, year int, month time.Month) (*MonthStatistics, error) {
	entries, err := s.calculateEnteries(ctx)
	if err != nil {
		return nil, fmt.Errorf("calculate entries: %w", err)
	}
	stats := &MonthStatistics{}
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

func (s *Service) CalculateYear(ctx context.Context, year int) (*YearStatistics, error) {
	entries, err := s.calculateEnteries(ctx)
	if err != nil {
		return nil, fmt.Errorf("calculate entries: %w", err)
	}
	stats := &YearStatistics{
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

// getDateFromISOWeek returns the date of Monday for the given ISO week
func getDateFromISOWeek(year int, week int) time.Time {
	// Start with January 1st of the given year
	jan1 := time.Date(year, 1, 1, 0, 0, 0, 0, time.UTC)

	// Get the weekday of January 1st (0 = Sunday, 1 = Monday, etc.)
	weekday := int(jan1.Weekday())
	if weekday == 0 {
		weekday = 7
	}

	// Calculate the date of Monday in week 1
	// If Jan 1 is Tue,Wed,Thu,Fri, week 1 starts on Dec 31,30,29,28
	// If Jan 1 is Sat,Sun,Mon, week 1 starts on Jan 2,3,4
	week1Start := jan1
	if weekday <= 4 {
		// Jan 1 is in week 1, go back to Monday
		week1Start = week1Start.AddDate(0, 0, -(weekday - 1))
	} else {
		// Jan 1 is in last week of previous year, go forward to Monday
		week1Start = week1Start.AddDate(0, 0, 8-weekday)
	}

	// Add the necessary weeks
	return week1Start.AddDate(0, 0, (week-1)*7)
}
