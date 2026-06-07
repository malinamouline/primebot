package bot

import (
	"fmt"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"hard90-bot/internal/models"
)

const (
	btnProgress   = "📊 Мой прогресс"
	btnMarkToday  = "✅ Отметить за сегодня"
	btnPastDays   = "📅 Прошлые дни"
	btnWeight     = "⚖️ Вес"
	btnChangeStart = "📅 Дата старта"
	btnStart      = "🚀 Начать лето"
	btnRules      = "📖 Правила"
)

func mainKeyboard(started bool) tgbotapi.ReplyKeyboardMarkup {
	rows := [][]tgbotapi.KeyboardButton{
		{
			tgbotapi.NewKeyboardButton(btnProgress),
			tgbotapi.NewKeyboardButton(btnMarkToday),
		},
		{
			tgbotapi.NewKeyboardButton(btnWeight),
			tgbotapi.NewKeyboardButton(btnPastDays),
		},
		{tgbotapi.NewKeyboardButton(btnRules)},
	}

	if started {
		rows = append(rows, []tgbotapi.KeyboardButton{
			tgbotapi.NewKeyboardButton(btnChangeStart),
		})
	}

	if !started {
		rows = append([][]tgbotapi.KeyboardButton{
			{tgbotapi.NewKeyboardButton(btnStart)},
		}, rows...)
	}

	kb := tgbotapi.NewReplyKeyboard(rows...)
	kb.ResizeKeyboard = true
	return kb
}

func tasksKeyboard(dayNumber int, log *models.DailyLog) tgbotapi.InlineKeyboardMarkup {
	var rows [][]tgbotapi.InlineKeyboardButton

	for _, task := range models.AllTasks {
		mark := "⬜"
		if log.IsDone(task) {
			mark = "✅"
		}
		label := fmt.Sprintf("%s %s %s", mark, taskIcon(task), taskLabel(task))
		callback := fmt.Sprintf("toggle:%d:%s", dayNumber, task)
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(label, callback),
		))
	}

	rows = append(rows, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("🔄 Обновить", fmt.Sprintf("refresh:%d", dayNumber)),
	))

	return tgbotapi.NewInlineKeyboardMarkup(rows...)
}

func pastDaysKeyboard(currentDay int, startDate time.Time) tgbotapi.InlineKeyboardMarkup {
	var rows [][]tgbotapi.InlineKeyboardButton
	var row []tgbotapi.InlineKeyboardButton

	for day := 1; day <= currentDay; day++ {
		d := dayDate(ChallengeState{StartDate: startDate, DayNumber: currentDay}, day)
		label := fmt.Sprintf("День %d (%s)", day, d.Format("02.01"))
		row = append(row, tgbotapi.NewInlineKeyboardButtonData(label, fmt.Sprintf("pickday:%d", day)))
		if len(row) == 2 {
			rows = append(rows, row)
			row = nil
		}
	}
	if len(row) > 0 {
		rows = append(rows, row)
	}

	return tgbotapi.NewInlineKeyboardMarkup(rows...)
}

func reminderKeyboard() tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("✅ Отметить сейчас", "reminder:mark"),
		),
	)
}
