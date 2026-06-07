package bot

import (
	"log"
	"time"

	"hard90-bot/internal/storage"
)

type ReminderService struct {
	handler *Handler
	store   *storage.Store
	loc     *time.Location
	hour    int
	minute  int
}

func NewReminderService(handler *Handler, store *storage.Store, loc *time.Location, hour, minute int) *ReminderService {
	return &ReminderService{
		handler: handler,
		store:   store,
		loc:     loc,
		hour:    hour,
		minute:  minute,
	}
}

func (r *ReminderService) Start() {
	go func() {
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()

		for now := range ticker.C {
			r.tick(now.In(r.loc))
		}
	}()

	log.Printf("reminders enabled at %02d:%02d", r.hour, r.minute)
}

func (r *ReminderService) tick(now time.Time) {
	if now.Hour() != r.hour || now.Minute() != r.minute {
		return
	}

	today := dateOnly(now, r.loc)

	users, err := r.store.GetActiveUsers()
	if err != nil {
		log.Printf("reminder users: %v", err)
		return
	}

	for _, user := range users {
		if user.LastReminderDate != nil && dateOnly(*user.LastReminderDate, r.loc).Equal(today) {
			continue
		}

		state := CalcChallengeState(user.StartDate, now)
		if !state.Started || state.Finished {
			continue
		}

		if err := r.handler.SendReminder(user, state); err != nil {
			log.Printf("reminder send to %d: %v", user.TelegramID, err)
			continue
		}

		if err := r.store.SetLastReminderDate(user.ID, today); err != nil {
			log.Printf("reminder save date: %v", err)
		}
	}
}
