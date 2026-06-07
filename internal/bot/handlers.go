package bot

import (
	"fmt"
	"log"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"hard90-bot/internal/models"
	"hard90-bot/internal/storage"
)

const rulesText = `📖 *Лето 90 — твой личный челлендж*

*Цель:* прожить оставшиеся *90 дней лета* осознанно.
*Главное:* консистентность, а не сложность.

Каждый день в *21:00* отмечай 5 пунктов:

1. 🎮 Без игр
2. 🚶 Прогулка
3. 🏋️ Тренировка
4. 🥗 Калории в норме
5. 💻 Проект

*Стрик* — сколько дней подряд ты закрыл все 5 пунктов.
Первая цель — *7 дней* подряд. Потом 14, 21...

*Вес* — цель *−1 кг в неделю*. Взвешивайся раз в неделю, просто введи число.

Кнопки:
• *📊 Мой прогресс* — статус и стрик
• *✅ Отметить за сегодня* — галочки
• *📅 Прошлые дни* — заполнить прошлую неделю
• *⚖️ Вес* — записать вес (просто число)
• *📅 Дата старта* — изменить день 1`

type Handler struct {
	api              *tgbotapi.BotAPI
	store            *storage.Store
	loc              *time.Location
	awaitingWeight   map[int64]bool
	awaitingStartDate map[int64]bool
}

func NewHandler(api *tgbotapi.BotAPI, store *storage.Store, loc *time.Location) *Handler {
	return &Handler{
		api:               api,
		store:             store,
		loc:               loc,
		awaitingWeight:    make(map[int64]bool),
		awaitingStartDate: make(map[int64]bool),
	}
}

func (h *Handler) HandleUpdate(update tgbotapi.Update) {
	if update.CallbackQuery != nil {
		h.handleCallback(update.CallbackQuery)
		return
	}

	if update.Message == nil {
		return
	}

	msg := update.Message
	if msg.Text == "" && !msg.IsCommand() {
		return
	}

	user, err := h.store.GetOrCreateUser(msg.From.ID, msg.From.UserName)
	if err != nil {
		log.Printf("get user: %v", err)
		h.reply(msg.Chat.ID, "Ошибка базы данных. Попробуй позже.")
		return
	}

	state := CalcChallengeState(user.StartDate, time.Now().In(h.loc))

	if h.awaitingStartDate[msg.Chat.ID] {
		h.handleStartDateInput(msg.Chat.ID, user, msg.Text)
		return
	}

	if h.awaitingWeight[msg.Chat.ID] && msg.Text != btnWeight {
		h.handleWeightInput(msg.Chat.ID, user, state, msg.Text)
		return
	}

	switch {
	case msg.IsCommand() && msg.Command() == "start":
		h.sendWelcome(msg.Chat.ID, state.Started)
	case msg.Text == btnStart:
		h.requestStartDate(msg.Chat.ID, state, false)
	case msg.Text == btnChangeStart:
		h.requestStartDate(msg.Chat.ID, state, true)
	case msg.Text == btnProgress:
		h.sendProgress(msg.Chat.ID, user, state)
	case msg.Text == btnMarkToday:
		h.sendMarkToday(msg.Chat.ID, user, state)
	case msg.Text == btnPastDays:
		h.sendPastDays(msg.Chat.ID, state)
	case msg.Text == btnWeight:
		h.requestWeight(msg.Chat.ID, state)
	case msg.Text == btnRules:
		h.replyMarkdown(msg.Chat.ID, rulesText)
	default:
		if state.Started && looksLikeWeightInput(msg.Text) {
			h.handleWeightInput(msg.Chat.ID, user, state, msg.Text)
			return
		}
		h.reply(msg.Chat.ID, "Не понял. Используй кнопки внизу 👇")
	}
}

func (h *Handler) requestStartDate(chatID int64, state ChallengeState, isChange bool) {
	if isChange {
		if !state.Started {
			h.reply(chatID, "Сначала начни челлендж.")
			return
		}
	} else if state.Started {
		h.reply(chatID, fmt.Sprintf(
			"Челлендж уже идёт с %s. День %d/%d.\n\nЧтобы изменить — жми *📅 Дата старта*.",
			state.StartDate.Format("02.01.2006"),
			state.DayNumber,
			models.TotalDays,
		))
		return
	}

	h.awaitingStartDate[chatID] = true
	text := "📅 Когда был *день 1* челленджа?\n\nВведи дату, например: `01.06.2026`\nИли напиши `сегодня`"
	if isChange {
		text = "📅 Введи новую дату *дня 1*, например: `01.06.2026`"
	}
	h.replyMarkdown(chatID, text)
}

func (h *Handler) handleStartDateInput(chatID int64, user *models.User, text string) {
	delete(h.awaitingStartDate, chatID)

	startDate, err := parseStartDateInput(text, time.Now().In(h.loc), h.loc)
	if err != nil {
		h.awaitingStartDate[chatID] = true
		h.reply(chatID, "Не понял дату. Пример: 01.06.2026")
		return
	}

	if err := h.store.StartChallenge(user.ID, startDate); err != nil {
		log.Printf("start challenge: %v", err)
		h.reply(chatID, "Не удалось сохранить дату.")
		return
	}

	user.StartDate = &startDate
	state := CalcChallengeState(user.StartDate, time.Now().In(h.loc))

	h.sendWelcome(chatID, true)
	h.replyMarkdown(chatID, fmt.Sprintf(
		"☀️ *%s* — день 1: *%s*\nСегодня день *%d/%d*.\n\nЗаполни прошлые дни через *📅 Прошлые дни*, вес — через *⚖️ Вес*.",
		models.ChallengeName,
		startDate.Format("02.01.2006"),
		state.DayNumber,
		models.TotalDays,
	))
	h.sendProgress(chatID, user, state)
}

func (h *Handler) sendWelcome(chatID int64, started bool) {
	text := fmt.Sprintf(
		"Привет! Я помогу прожить *%s* — твой личный челлендж на оставшееся лето.\n\nОтмечай 5 пунктов каждый день и держи стрик.",
		models.ChallengeName,
	)
	if !started {
		text += "\n\nНажми *🚀 Начать лето*, чтобы стартовать."
	}

	msg := tgbotapi.NewMessage(chatID, text)
	msg.ParseMode = "Markdown"
	msg.ReplyMarkup = mainKeyboard(started)
	if _, err := h.api.Send(msg); err != nil {
		log.Printf("send welcome: %v", err)
	}
}

func (h *Handler) sendProgress(chatID int64, user *models.User, state ChallengeState) {
	if !state.Started {
		h.reply(chatID, "Сначала нажми «🚀 Начать лето».")
		return
	}

	dailyLog, perfectDays, streak, err := h.loadStats(user, state)
	if err != nil {
		log.Printf("load stats: %v", err)
		h.reply(chatID, "Не удалось загрузить прогресс.")
		return
	}

	weightStats, _ := CalcWeightStats(h.store, user, state)
	weightSection := "⚖️ Вес: ещё не записан"
	if weightStats != nil {
		weightSection = formatWeightSection(weightStats)
	}

	h.replyMarkdown(chatID, formatProgressMessage(state, dailyLog, perfectDays, streak, weightSection))
}

func (h *Handler) requestWeight(chatID int64, state ChallengeState) {
	if !state.Started {
		h.reply(chatID, "Сначала нажми «🚀 Начать лето».")
		return
	}

	h.awaitingWeight[chatID] = true
	h.reply(chatID, "⚖️ Сколько на весах? Просто число, например: 81.0")
}

func (h *Handler) handleWeightInput(chatID int64, user *models.User, state ChallengeState, text string) {
	delete(h.awaitingWeight, chatID)

	weight, err := ParseWeightInput(text)
	if err != nil {
		h.awaitingWeight[chatID] = true
		h.reply(chatID, "Не понял. Просто число, например: 81.0")
		return
	}

	logDate := state.LogDate
	if err := h.store.SaveWeight(user.ID, logDate, weight); err != nil {
		log.Printf("save weight: %v", err)
		h.reply(chatID, "Не удалось сохранить вес.")
		return
	}

	stats, err := CalcWeightStats(h.store, user, state)
	if err != nil || stats == nil || !stats.HasData {
		log.Printf("weight stats: %v", err)
		h.reply(chatID, fmt.Sprintf("Записал: %.1f кг ✅", weight))
		return
	}

	h.replyMarkdown(chatID, fmt.Sprintf("Записал: *%.1f* кг ✅\n\n%s", weight, formatWeightMessage(stats)))
}

func (h *Handler) sendPastDays(chatID int64, state ChallengeState) {
	if !state.Started {
		h.reply(chatID, "Сначала нажми «🚀 Начать лето».")
		return
	}

	if state.DayNumber <= 1 {
		h.reply(chatID, "Пока только день 1 — жми «✅ Отметить за сегодня».")
		return
	}

	msg := tgbotapi.NewMessage(chatID, "Выбери день для заполнения:")
	msg.ReplyMarkup = pastDaysKeyboard(state.DayNumber, state.StartDate)
	if _, err := h.api.Send(msg); err != nil {
		log.Printf("send past days: %v", err)
	}
}

func (h *Handler) sendMarkToday(chatID int64, user *models.User, state ChallengeState) {
	if !state.Started {
		h.reply(chatID, "Сначала нажми «🚀 Начать лето».")
		return
	}

	if state.Finished {
		h.reply(chatID, "Лето завершено! 🎉")
		return
	}

	dailyLog, err := h.store.GetOrCreateDailyLog(user.ID, state.DayNumber, state.LogDate)
	if err != nil {
		log.Printf("get daily log: %v", err)
		h.reply(chatID, "Не удалось открыть день.")
		return
	}

	h.sendTasksMessage(chatID, state.DayNumber, dailyLog)
}

func (h *Handler) sendTasksMessage(chatID int64, dayNumber int, dailyLog *models.DailyLog) {
	text := fmt.Sprintf(
		"Отметь за *день %d* (%s):",
		dayNumber,
		dailyLog.LogDate.Format("02.01.2006"),
	)

	msg := tgbotapi.NewMessage(chatID, text)
	msg.ParseMode = "Markdown"
	msg.ReplyMarkup = tasksKeyboard(dayNumber, dailyLog)
	if _, err := h.api.Send(msg); err != nil {
		log.Printf("send mark today: %v", err)
	}
}

func (h *Handler) handleCallback(cb *tgbotapi.CallbackQuery) {
	user, err := h.store.GetOrCreateUser(cb.From.ID, cb.From.UserName)
	if err != nil {
		log.Printf("callback user: %v", err)
		return
	}

	state := CalcChallengeState(user.StartDate, time.Now().In(h.loc))
	if !state.Started || state.Finished {
		h.answerCallback(cb.ID, "Челлендж не активен")
		return
	}

	data := cb.Data
	if data == "reminder:mark" {
		h.answerCallback(cb.ID, "Открываю отметки")
		dailyLog, err := h.store.GetOrCreateDailyLog(user.ID, state.DayNumber, state.LogDate)
		if err != nil {
			log.Printf("reminder mark: %v", err)
			return
		}
		h.sendTasksMessage(cb.Message.Chat.ID, state.DayNumber, dailyLog)
		return
	}

	if strings.HasPrefix(data, "pickday:") {
		var dayNumber int
		if _, err := fmt.Sscanf(strings.TrimPrefix(data, "pickday:"), "%d", &dayNumber); err != nil {
			h.answerCallback(cb.ID, "Ошибка")
			return
		}
		h.answerCallback(cb.ID, fmt.Sprintf("День %d", dayNumber))
		logDate := dayDate(state, dayNumber)
		dailyLog, err := h.store.GetOrCreateDailyLog(user.ID, dayNumber, logDate)
		if err != nil {
			log.Printf("pick day: %v", err)
			return
		}
		h.sendTasksMessage(cb.Message.Chat.ID, dayNumber, dailyLog)
		return
	}

	if strings.HasPrefix(data, "refresh:") {
		h.refreshTasksMessage(cb, data)
		return
	}

	if !strings.HasPrefix(data, "toggle:") {
		h.answerCallback(cb.ID, "Неизвестная команда")
		return
	}

	parts := strings.Split(data, ":")
	if len(parts) != 3 {
		h.answerCallback(cb.ID, "Ошибка данных")
		return
	}

	var dayNumber int
	if _, err := fmt.Sscanf(parts[1], "%d", &dayNumber); err != nil {
		h.answerCallback(cb.ID, "Ошибка дня")
		return
	}

	if dayNumber < 1 || dayNumber > state.DayNumber {
		h.answerCallback(cb.ID, "Нельзя отмечать этот день")
		return
	}

	task := models.Task(parts[2])
	logDate := dayDate(state, dayNumber)
	dailyLog, err := h.store.GetOrCreateDailyLog(user.ID, dayNumber, logDate)
	if err != nil {
		log.Printf("toggle get log: %v", err)
		h.answerCallback(cb.ID, "Ошибка базы")
		return
	}

	newValue := !dailyLog.IsDone(task)
	if err := h.store.UpdateTask(user.ID, dayNumber, task, newValue); err != nil {
		log.Printf("update task: %v", err)
		h.answerCallback(cb.ID, "Не удалось сохранить")
		return
	}

	dailyLog.SetDone(task, newValue)

	status := "снято"
	if newValue {
		status = "готово"
	}
	h.answerCallback(cb.ID, fmt.Sprintf("%s %s", taskIcon(task), status))

	h.editTasksMessage(cb, dayNumber, dailyLog)

	if dailyLog.IsPerfect() && dayNumber == state.DayNumber {
		_, _, streak, err := h.loadStats(user, state)
		if err == nil {
			h.replyMarkdown(cb.Message.Chat.ID, completionMessage(state.DayNumber, streak))
		}
	}
}

func (h *Handler) refreshTasksMessage(cb *tgbotapi.CallbackQuery, data string) {
	var dayNumber int
	if _, err := fmt.Sscanf(strings.TrimPrefix(data, "refresh:"), "%d", &dayNumber); err != nil {
		h.answerCallback(cb.ID, "Ошибка")
		return
	}

	user, err := h.store.GetOrCreateUser(cb.From.ID, cb.From.UserName)
	if err != nil {
		h.answerCallback(cb.ID, "Ошибка")
		return
	}
	state := CalcChallengeState(user.StartDate, time.Now().In(h.loc))
	logDate := dayDate(state, dayNumber)
	dailyLog, err := h.store.GetOrCreateDailyLog(user.ID, dayNumber, logDate)
	if err != nil {
		log.Printf("refresh log: %v", err)
		h.answerCallback(cb.ID, "Ошибка")
		return
	}

	h.answerCallback(cb.ID, "Обновлено")
	h.editTasksMessage(cb, dayNumber, dailyLog)
}

func (h *Handler) editTasksMessage(cb *tgbotapi.CallbackQuery, dayNumber int, dailyLog *models.DailyLog) {
	text := fmt.Sprintf(
		"Отметь за *день %d* (%s):",
		dayNumber,
		dailyLog.LogDate.Format("02.01.2006"),
	)

	edit := tgbotapi.NewEditMessageTextAndMarkup(
		cb.Message.Chat.ID,
		cb.Message.MessageID,
		text,
		tasksKeyboard(dayNumber, dailyLog),
	)
	edit.ParseMode = "Markdown"
	if _, err := h.api.Send(edit); err != nil {
		log.Printf("edit tasks: %v", err)
	}
}

func (h *Handler) SendReminder(user *models.User, state ChallengeState) error {
	dailyLog, _, streak, err := h.loadStats(user, state)
	if err != nil {
		return err
	}

	text := formatReminderMessage(state, dailyLog, streak)
	msg := tgbotapi.NewMessage(user.TelegramID, text)
	msg.ParseMode = "Markdown"
	if !dailyLog.IsPerfect() {
		msg.ReplyMarkup = reminderKeyboard()
	}

	_, err = h.api.Send(msg)
	return err
}

func (h *Handler) loadStats(user *models.User, state ChallengeState) (*models.DailyLog, int, int, error) {
	dailyLog, err := h.store.GetOrCreateDailyLog(user.ID, state.DayNumber, state.LogDate)
	if err != nil {
		return nil, 0, 0, err
	}

	perfectDays, err := h.store.CountPerfectDays(user.ID)
	if err != nil {
		return nil, 0, 0, err
	}

	logs, err := h.store.GetLogsUpToDay(user.ID, state.DayNumber)
	if err != nil {
		return nil, 0, 0, err
	}

	streak := CalcStreak(logs, state.DayNumber)
	return dailyLog, perfectDays, streak, nil
}

func (h *Handler) reply(chatID int64, text string) {
	msg := tgbotapi.NewMessage(chatID, text)
	if _, err := h.api.Send(msg); err != nil {
		log.Printf("send: %v", err)
	}
}

func (h *Handler) replyMarkdown(chatID int64, text string) {
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ParseMode = "Markdown"
	if _, err := h.api.Send(msg); err != nil {
		log.Printf("send markdown: %v", err)
	}
}

func (h *Handler) answerCallback(callbackID, text string) {
	cb := tgbotapi.NewCallback(callbackID, text)
	if _, err := h.api.Request(cb); err != nil {
		log.Printf("answer callback: %v", err)
	}
}
