package bot

import (
	"fmt"
	"strings"
	"time"
)

func parseStartDateInput(text string, now time.Time, loc *time.Location) (time.Time, error) {
	clean := strings.TrimSpace(strings.ToLower(text))
	today := dateOnly(now, loc)

	switch clean {
	case "сегодня", "today":
		return today, nil
	}

	formats := []string{"02.01.2006", "2.1.2006", "02.01.06", "2.1.06", "02.01", "2.1"}
	for _, layout := range formats {
		if t, err := time.ParseInLocation(layout, clean, loc); err == nil {
			if !strings.Contains(clean, ".") {
				continue
			}
			if layout == "02.01" || layout == "2.1" {
				t = time.Date(today.Year(), t.Month(), t.Day(), 0, 0, 0, 0, loc)
			}
			if layout == "02.01.06" || layout == "2.1.06" {
				year := 2000 + t.Year()%100
				t = time.Date(year, t.Month(), t.Day(), 0, 0, 0, 0, loc)
			}
			if t.After(today) {
				return time.Time{}, fmt.Errorf("date in future")
			}
			return t, nil
		}
	}

	return time.Time{}, fmt.Errorf("invalid date")
}

func parseWeightInputWithDate(text string, defaultDate time.Time, loc *time.Location) (float64, time.Time, error) {
	parts := strings.Fields(strings.TrimSpace(text))
	if len(parts) == 0 {
		return 0, defaultDate, fmt.Errorf("empty")
	}

	weight, err := ParseWeightInput(parts[0])
	if err != nil {
		return 0, defaultDate, err
	}

	if len(parts) == 1 {
		return weight, defaultDate, nil
	}

	date, err := parseStartDateInput(parts[1], time.Now().In(loc), loc)
	if err != nil {
		return 0, defaultDate, err
	}
	return weight, date, nil
}

func dayDate(state ChallengeState, dayNumber int) time.Time {
	return state.StartDate.AddDate(0, 0, dayNumber-1)
}
