package models

import "time"

const (
	TotalDays           = 90
	ChallengeName       = "Лето 90"
	WeekStreakGoal      = 7
	WeeklyWeightLossGoal = 1.0 // кг в неделю
)

type Task string

const (
	TaskNoGames  Task = "no_games"
	TaskWalk     Task = "walk"
	TaskWorkout  Task = "workout"
	TaskCalories Task = "calories"
	TaskProject  Task = "project"
)

var AllTasks = []Task{
	TaskNoGames,
	TaskWalk,
	TaskWorkout,
	TaskCalories,
	TaskProject,
}

type User struct {
	ID               int64
	TelegramID       int64
	Username         string
	StartDate        *time.Time
	StartWeight      *float64
	LastReminderDate *time.Time
}

type WeightLog struct {
	ID       int64
	UserID   int64
	LogDate  time.Time
	WeightKg float64
}

type DailyLog struct {
	ID        int64
	UserID    int64
	DayNumber int
	LogDate   time.Time
	NoGames   bool
	Walk      bool
	Workout   bool
	Calories  bool
	Project   bool
}

func (l *DailyLog) IsDone(task Task) bool {
	switch task {
	case TaskNoGames:
		return l.NoGames
	case TaskWalk:
		return l.Walk
	case TaskWorkout:
		return l.Workout
	case TaskCalories:
		return l.Calories
	case TaskProject:
		return l.Project
	default:
		return false
	}
}

func (l *DailyLog) SetDone(task Task, done bool) {
	switch task {
	case TaskNoGames:
		l.NoGames = done
	case TaskWalk:
		l.Walk = done
	case TaskWorkout:
		l.Workout = done
	case TaskCalories:
		l.Calories = done
	case TaskProject:
		l.Project = done
	}
}

func (l *DailyLog) CompletedCount() int {
	count := 0
	for _, task := range AllTasks {
		if l.IsDone(task) {
			count++
		}
	}
	return count
}

func (l *DailyLog) IsPerfect() bool {
	return l.CompletedCount() == len(AllTasks)
}

func (l *DailyLog) RemainingTasks() []Task {
	var remaining []Task
	for _, task := range AllTasks {
		if !l.IsDone(task) {
			remaining = append(remaining, task)
		}
	}
	return remaining
}
