package bot

import (
	"fmt"
	"time"

	"hard90-bot/internal/models"
)

type ChallengeState struct {
	Started   bool
	DayNumber int
	LogDate   time.Time
	StartDate time.Time
	Finished  bool
	DaysLeft  int
}

func CalcChallengeState(startDate *time.Time, now time.Time) ChallengeState {
	if startDate == nil {
		return ChallengeState{Started: false}
	}

	start := dateOnly(*startDate, now.Location())
	today := dateOnly(now, now.Location())

	if today.Before(start) {
		return ChallengeState{
			Started:   true,
			StartDate: start,
			DayNumber: 0,
			LogDate:   today,
		}
	}

	dayNumber := int(today.Sub(start).Hours()/24) + 1
	finished := dayNumber > models.TotalDays
	if finished {
		dayNumber = models.TotalDays
	}

	daysLeft := models.TotalDays - dayNumber
	if daysLeft < 0 {
		daysLeft = 0
	}

	return ChallengeState{
		Started:   true,
		StartDate: start,
		DayNumber: dayNumber,
		LogDate:   today,
		Finished:  finished,
		DaysLeft:  daysLeft,
	}
}

func CalcStreak(logs []*models.DailyLog, currentDay int) int {
	byDay := make(map[int]*models.DailyLog, len(logs))
	for _, log := range logs {
		byDay[log.DayNumber] = log
	}

	streak := 0
	for day := currentDay; day >= 1; day-- {
		log, ok := byDay[day]
		if !ok || !log.IsPerfect() {
			break
		}
		streak++
	}

	return streak
}

func dateOnly(t time.Time, loc *time.Location) time.Time {
	y, m, d := t.In(loc).Date()
	return time.Date(y, m, d, 0, 0, 0, 0, loc)
}

func taskLabel(task models.Task) string {
	switch task {
	case models.TaskNoGames:
		return "Без игр"
	case models.TaskWalk:
		return "Прогулка"
	case models.TaskWorkout:
		return "Тренировка"
	case models.TaskCalories:
		return "Калории в норме"
	case models.TaskProject:
		return "Проект"
	default:
		return string(task)
	}
}

func taskIcon(task models.Task) string {
	switch task {
	case models.TaskNoGames:
		return "🎮"
	case models.TaskWalk:
		return "🚶"
	case models.TaskWorkout:
		return "🏋️"
	case models.TaskCalories:
		return "🥗"
	case models.TaskProject:
		return "💻"
	default:
		return "•"
	}
}

func streakMessage(streak int) string {
	switch {
	case streak == 0:
		return "💡 Начни стрик сегодня — *консистентность* важнее сложности"
	case streak < models.WeekStreakGoal:
		left := models.WeekStreakGoal - streak
		return fmt.Sprintf("🔥 Стрик *%d* дн. — до недели осталось *%d*!", streak, left)
	case streak == models.WeekStreakGoal:
		return fmt.Sprintf("🎉 *%d дней подряд!* Недельный стрик — ты молодец!", streak)
	case streak < 14:
		return fmt.Sprintf("🔥 Стрик *%d* дн. — неделя позади, не останавливайся!", streak)
	case streak < 30:
		return fmt.Sprintf("💪 Стрик *%d* дн. — это уже привычка!", streak)
	default:
		return fmt.Sprintf("🏆 Стрик *%d* дн. — легенда!", streak)
	}
}

func completionMessage(dayNumber, streak int) string {
	switch {
	case streak == 1:
		return fmt.Sprintf("✅ День %d закрыт! Завтра — день %d, не сбивай ритм.", dayNumber, dayNumber+1)
	case streak == models.WeekStreakGoal:
		return "🎉 НЕДЕЛЯ СТРИКА! Именно ради этого и затевали. Так держать!"
	case streak%7 == 0:
		return fmt.Sprintf("🔥 %d дней подряд! Ещё одна неделя в кармане.", streak)
	default:
		return fmt.Sprintf("✅ Всё отмечено! Стрик: %d 🔥", streak)
	}
}

func formatProgressMessage(state ChallengeState, log *models.DailyLog, perfectDays, streak int, weightSection string) string {
	if !state.Started {
		return "Челлендж ещё не начат.\n\nНажми «🚀 Начать лето», чтобы стартовать с сегодняшнего дня."
	}

	if state.Finished {
		return fmt.Sprintf(
			"🎉 Лето прожито! Все *%d* дня позади.\n\nИдеальных дней: *%d*\nФинальный стрик: *%d* 🔥",
			models.TotalDays, perfectDays, streak,
		)
	}

	lines := []string{
		fmt.Sprintf("☀️ *%s* — День *%d/%d*", models.ChallengeName, state.DayNumber, models.TotalDays),
		fmt.Sprintf("📅 %s", state.LogDate.Format("02.01.2006")),
		"",
		"*Сегодня:*",
	}

	for _, task := range models.AllTasks {
		mark := "⬜"
		if log.IsDone(task) {
			mark = "✅"
		}
		lines = append(lines, fmt.Sprintf("%s %s %s", mark, taskIcon(task), taskLabel(task)))
	}

	percent := 0
	if len(models.AllTasks) > 0 {
		percent = log.CompletedCount() * 100 / len(models.AllTasks)
	}

	lines = append(lines,
		"",
		fmt.Sprintf("Прогресс дня: *%d/%d* (%d%%)", log.CompletedCount(), len(models.AllTasks), percent),
		streakMessage(streak),
		fmt.Sprintf("Идеальных дней: *%d*", perfectDays),
		weightSection,
		fmt.Sprintf("Осталось дней лета: *%d*", state.DaysLeft),
	)

	if log.IsPerfect() {
		lines = append(lines, "", completionMessage(state.DayNumber, streak))
	} else {
		remaining := len(log.RemainingTasks())
		lines = append(lines, "", fmt.Sprintf("⏰ Отметься в *21:00* — осталось *%d* пункт(а)", remaining))
	}

	return joinLines(lines)
}

func formatReminderMessage(state ChallengeState, log *models.DailyLog, streak int) string {
	remaining := log.RemainingTasks()
	lines := []string{
		"⏰ *21:00* — время отметиться!",
		fmt.Sprintf("День *%d/%d* • Стрик: *%d* 🔥", state.DayNumber, models.TotalDays, streak),
		"",
	}

	if len(remaining) == 0 {
		lines = append(lines, "✅ Сегодня всё уже отмечено. Красавчик!")
		return joinLines(lines)
	}

	lines = append(lines, "*Осталось:*")
	for _, task := range remaining {
		lines = append(lines, fmt.Sprintf("⬜ %s %s", taskIcon(task), taskLabel(task)))
	}

	if streak > 0 && streak < models.WeekStreakGoal {
		lines = append(lines, "", fmt.Sprintf("Не сбивай стрик — до недели *%d* дн.", models.WeekStreakGoal-streak))
	}

	lines = append(lines, "", "Жми кнопку ниже 👇")
	return joinLines(lines)
}

func joinLines(lines []string) string {
	result := ""
	for i, line := range lines {
		if i > 0 {
			result += "\n"
		}
		result += line
	}
	return result
}
