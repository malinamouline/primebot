package bot

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"hard90-bot/internal/models"
	"hard90-bot/internal/storage"
)

type WeightStats struct {
	HasData       bool
	StartWeight   float64
	CurrentWeight float64
	CurrentWeek   int
	TargetWeight  float64
	WeeklyChange  float64
	TotalLoss     float64
	OnTrack       bool
	HasWeeklyData bool
	LastLogDate   time.Time
}

func ParseWeightInput(text string) (float64, error) {
	clean := strings.TrimSpace(strings.Replace(text, ",", ".", 1))
	weight, err := strconv.ParseFloat(clean, 64)
	if err != nil || weight <= 0 || weight > 500 {
		return 0, fmt.Errorf("invalid weight")
	}
	return weight, nil
}

func looksLikeWeightInput(text string) bool {
	parts := strings.Fields(strings.TrimSpace(text))
	if len(parts) != 1 {
		return false
	}
	_, err := ParseWeightInput(parts[0])
	return err == nil
}

func CalcWeightStats(store *storage.Store, user *models.User, state ChallengeState) (*WeightStats, error) {
	startWeight, err := store.GetStartWeight(user.ID)
	if err != nil {
		return &WeightStats{HasData: false}, nil
	}

	latest, err := store.GetLatestWeight(user.ID)
	if err != nil {
		return &WeightStats{HasData: false}, nil
	}

	currentWeek := (state.DayNumber-1)/7 + 1
	targetWeight := startWeight - float64(currentWeek)*models.WeeklyWeightLossGoal
	totalLoss := startWeight - latest.WeightKg

	stats := &WeightStats{
		HasData:       true,
		StartWeight:   startWeight,
		CurrentWeight: latest.WeightKg,
		CurrentWeek:   currentWeek,
		TargetWeight:  targetWeight,
		TotalLoss:     totalLoss,
		OnTrack:       latest.WeightKg <= targetWeight+0.2,
		LastLogDate:   latest.LogDate,
		HasWeeklyData: true,
		WeeklyChange:  startWeight - latest.WeightKg,
	}

	if currentWeek > 1 {
		prevWeekStart := state.StartDate.AddDate(0, 0, (currentWeek-2)*7)
		prevWeekEnd := prevWeekStart.AddDate(0, 0, 6)
		prevLog, err := store.GetWeightForWeek(user.ID, prevWeekStart, prevWeekEnd)
		if err == nil {
			stats.WeeklyChange = prevLog.WeightKg - latest.WeightKg
		}
	}

	return stats, nil
}

func formatWeightMessage(stats *WeightStats) string {
	if !stats.HasData {
		return "⚖️ *Вес*\n\nПока нет записей.\nЖми *⚖️ Вес* и отправь число, например `81.0`"
	}

	lines := []string{
		"⚖️ *Вес*",
		"",
		fmt.Sprintf("Старт: *%.1f* кг", stats.StartWeight),
		fmt.Sprintf("Сейчас: *%.1f* кг", stats.CurrentWeight),
		fmt.Sprintf("Сброшено: *%.1f* кг", stats.TotalLoss),
		"",
		fmt.Sprintf("Неделя *%d* • цель: *−%.0f кг/нед*", stats.CurrentWeek, models.WeeklyWeightLossGoal),
		fmt.Sprintf("К концу недели: *≤ %.1f* кг", stats.TargetWeight),
	}

	if stats.HasWeeklyData {
		sign := "−"
		if stats.WeeklyChange < 0 {
			sign = "+"
		}
		lines = append(lines, fmt.Sprintf("За неделю: *%s%.1f* кг", sign, abs(stats.WeeklyChange)))
	}

	if stats.OnTrack {
		lines = append(lines, "", "✅ По плану! Так держать.")
	} else {
		diff := stats.CurrentWeight - stats.TargetWeight
		lines = append(lines, "", fmt.Sprintf("⚠️ Выше цели на *%.1f* кг — не паникуй, главное консистентность.", diff))
	}

	lines = append(lines, "", fmt.Sprintf("_Последняя запись: %s_", stats.LastLogDate.Format("02.01.2006")))
	return joinLines(lines)
}

func formatWeightSection(stats *WeightStats) string {
	if !stats.HasData {
		return "⚖️ Вес: ещё не записан"
	}

	status := "✅"
	if !stats.OnTrack {
		status = "⚠️"
	}
	return fmt.Sprintf("%s Вес: *%.1f* кг (сброшено *%.1f*, цель ≤ *%.1f*)", status, stats.CurrentWeight, stats.TotalLoss, stats.TargetWeight)
}

func abs(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}
