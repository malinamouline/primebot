package storage

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"

	"hard90-bot/internal/models"
)

type Store struct {
	db *sql.DB
}

func New(dbPath string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		return nil, fmt.Errorf("create db dir: %w", err)
	}

	dsn := fmt.Sprintf("file:%s?_pragma=foreign_keys(1)", dbPath)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}

	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		db.Close()
		return nil, err
	}

	return s, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) migrate() error {
	schema := `
CREATE TABLE IF NOT EXISTS users (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    telegram_id INTEGER NOT NULL UNIQUE,
    username TEXT NOT NULL DEFAULT '',
    start_date TEXT,
    start_weight REAL,
    last_reminder_date TEXT
);

CREATE TABLE IF NOT EXISTS weight_logs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    log_date TEXT NOT NULL,
    weight_kg REAL NOT NULL,
    UNIQUE(user_id, log_date)
);

CREATE TABLE IF NOT EXISTS daily_logs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    day_number INTEGER NOT NULL,
    log_date TEXT NOT NULL,
    no_games INTEGER NOT NULL DEFAULT 0,
    walk INTEGER NOT NULL DEFAULT 0,
    workout INTEGER NOT NULL DEFAULT 0,
    calories INTEGER NOT NULL DEFAULT 0,
    project INTEGER NOT NULL DEFAULT 0,
    UNIQUE(user_id, day_number)
);
`
	if _, err := s.db.Exec(schema); err != nil {
		return err
	}

	if err := s.migrateUserColumns(); err != nil {
		return err
	}

	return s.migrateFromOldSchema()
}

func (s *Store) migrateUserColumns() error {
	rows, err := s.db.Query(`PRAGMA table_info(users)`)
	if err != nil {
		return err
	}
	defer rows.Close()

	columns := make(map[string]bool)
	for rows.Next() {
		var cid int
		var name, ctype string
		var notnull, pk int
		var dflt sql.NullString
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err != nil {
			return err
		}
		columns[name] = true
	}
	if err := rows.Err(); err != nil {
		return err
	}

	if !columns["start_weight"] {
		_, err = s.db.Exec(`ALTER TABLE users ADD COLUMN start_weight REAL`)
		return err
	}
	return nil
}

func (s *Store) migrateFromOldSchema() error {
	rows, err := s.db.Query(`PRAGMA table_info(daily_logs)`)
	if err != nil {
		return err
	}
	defer rows.Close()

	columns := make(map[string]bool)
	for rows.Next() {
		var cid int
		var name, ctype string
		var notnull, pk int
		var dflt sql.NullString
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err != nil {
			return err
		}
		columns[name] = true
	}
	if err := rows.Err(); err != nil {
		return err
	}

	if columns["no_games"] || !columns["workout1"] {
		return nil
	}

	_, err = s.db.Exec(`DROP TABLE daily_logs`)
	if err != nil {
		return err
	}

	_, err = s.db.Exec(`
CREATE TABLE daily_logs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    day_number INTEGER NOT NULL,
    log_date TEXT NOT NULL,
    no_games INTEGER NOT NULL DEFAULT 0,
    walk INTEGER NOT NULL DEFAULT 0,
    workout INTEGER NOT NULL DEFAULT 0,
    calories INTEGER NOT NULL DEFAULT 0,
    project INTEGER NOT NULL DEFAULT 0,
    UNIQUE(user_id, day_number)
)`)
	return err
}

func (s *Store) GetOrCreateUser(telegramID int64, username string) (*models.User, error) {
	user, err := s.GetUserByTelegramID(telegramID)
	if err == nil {
		return user, nil
	}

	res, err := s.db.Exec(
		`INSERT INTO users (telegram_id, username) VALUES (?, ?)`,
		telegramID, username,
	)
	if err != nil {
		return nil, err
	}

	id, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}

	return &models.User{
		ID:         id,
		TelegramID: telegramID,
		Username:   username,
	}, nil
}

func (s *Store) GetUserByTelegramID(telegramID int64) (*models.User, error) {
	return s.scanUser(s.db.QueryRow(
		`SELECT id, telegram_id, username, start_date, start_weight, last_reminder_date FROM users WHERE telegram_id = ?`,
		telegramID,
	))
}

func (s *Store) GetActiveUsers() ([]*models.User, error) {
	rows, err := s.db.Query(
		`SELECT id, telegram_id, username, start_date, start_weight, last_reminder_date FROM users WHERE start_date IS NOT NULL`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []*models.User
	for rows.Next() {
		user, err := s.scanUserRow(rows)
		if err != nil {
			return nil, err
		}
		users = append(users, user)
	}
	return users, rows.Err()
}

func (s *Store) scanUser(row *sql.Row) (*models.User, error) {
	var user models.User
	var startDate, lastReminder sql.NullString
	var startWeight sql.NullFloat64
	if err := row.Scan(&user.ID, &user.TelegramID, &user.Username, &startDate, &startWeight, &lastReminder); err != nil {
		return nil, err
	}
	if err := fillUserFields(&user, startDate, startWeight, lastReminder); err != nil {
		return nil, err
	}
	return &user, nil
}

func (s *Store) scanUserRow(rows *sql.Rows) (*models.User, error) {
	var user models.User
	var startDate, lastReminder sql.NullString
	var startWeight sql.NullFloat64
	if err := rows.Scan(&user.ID, &user.TelegramID, &user.Username, &startDate, &startWeight, &lastReminder); err != nil {
		return nil, err
	}
	if err := fillUserFields(&user, startDate, startWeight, lastReminder); err != nil {
		return nil, err
	}
	return &user, nil
}

func fillUserFields(user *models.User, startDate sql.NullString, startWeight sql.NullFloat64, lastReminder sql.NullString) error {
	if startDate.Valid {
		t, err := time.Parse("2006-01-02", startDate.String)
		if err != nil {
			return err
		}
		user.StartDate = &t
	}
	if startWeight.Valid {
		w := startWeight.Float64
		user.StartWeight = &w
	}
	if lastReminder.Valid {
		t, err := time.Parse("2006-01-02", lastReminder.String)
		if err != nil {
			return err
		}
		user.LastReminderDate = &t
	}
	return nil
}

func (s *Store) StartChallenge(userID int64, startDate time.Time) error {
	_, err := s.db.Exec(
		`UPDATE users SET start_date = ? WHERE id = ?`,
		startDate.Format("2006-01-02"), userID,
	)
	return err
}

func (s *Store) UpdateStartDate(userID int64, startDate time.Time) error {
	return s.StartChallenge(userID, startDate)
}

func (s *Store) SetLastReminderDate(userID int64, date time.Time) error {
	_, err := s.db.Exec(
		`UPDATE users SET last_reminder_date = ? WHERE id = ?`,
		date.Format("2006-01-02"), userID,
	)
	return err
}

func (s *Store) GetOrCreateDailyLog(userID int64, dayNumber int, logDate time.Time) (*models.DailyLog, error) {
	log, err := s.GetDailyLog(userID, dayNumber)
	if err == nil {
		return log, nil
	}

	res, err := s.db.Exec(
		`INSERT INTO daily_logs (user_id, day_number, log_date) VALUES (?, ?, ?)`,
		userID, dayNumber, logDate.Format("2006-01-02"),
	)
	if err != nil {
		return nil, err
	}

	id, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}

	return &models.DailyLog{
		ID:        id,
		UserID:    userID,
		DayNumber: dayNumber,
		LogDate:   logDate,
	}, nil
}

func (s *Store) GetDailyLog(userID int64, dayNumber int) (*models.DailyLog, error) {
	row := s.db.QueryRow(
		`SELECT id, user_id, day_number, log_date, no_games, walk, workout, calories, project
		 FROM daily_logs WHERE user_id = ? AND day_number = ?`,
		userID, dayNumber,
	)

	return scanDailyLog(row)
}

func (s *Store) UpdateTask(userID int64, dayNumber int, task models.Task, done bool) error {
	column, err := taskColumn(task)
	if err != nil {
		return err
	}

	value := 0
	if done {
		value = 1
	}

	query := fmt.Sprintf(
		`UPDATE daily_logs SET %s = ? WHERE user_id = ? AND day_number = ?`,
		column,
	)
	_, err = s.db.Exec(query, value, userID, dayNumber)
	return err
}

func (s *Store) CountPerfectDays(userID int64) (int, error) {
	row := s.db.QueryRow(
		`SELECT COUNT(*) FROM daily_logs
		 WHERE user_id = ?
		   AND no_games = 1 AND walk = 1 AND workout = 1
		   AND calories = 1 AND project = 1`,
		userID,
	)

	var count int
	if err := row.Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

func (s *Store) GetLogsUpToDay(userID int64, dayNumber int) ([]*models.DailyLog, error) {
	rows, err := s.db.Query(
		`SELECT id, user_id, day_number, log_date, no_games, walk, workout, calories, project
		 FROM daily_logs WHERE user_id = ? AND day_number <= ? ORDER BY day_number ASC`,
		userID, dayNumber,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var logs []*models.DailyLog
	for rows.Next() {
		log, err := scanDailyLogRow(rows)
		if err != nil {
			return nil, err
		}
		logs = append(logs, log)
	}

	return logs, rows.Err()
}

func scanDailyLog(row *sql.Row) (*models.DailyLog, error) {
	var log models.DailyLog
	var logDate string
	var noGames, walk, workout, calories, project int
	if err := row.Scan(
		&log.ID, &log.UserID, &log.DayNumber, &logDate,
		&noGames, &walk, &workout, &calories, &project,
	); err != nil {
		return nil, err
	}

	t, err := time.Parse("2006-01-02", logDate)
	if err != nil {
		return nil, err
	}

	log.LogDate = t
	log.NoGames = noGames == 1
	log.Walk = walk == 1
	log.Workout = workout == 1
	log.Calories = calories == 1
	log.Project = project == 1

	return &log, nil
}

func scanDailyLogRow(rows *sql.Rows) (*models.DailyLog, error) {
	var log models.DailyLog
	var logDate string
	var noGames, walk, workout, calories, project int
	if err := rows.Scan(
		&log.ID, &log.UserID, &log.DayNumber, &logDate,
		&noGames, &walk, &workout, &calories, &project,
	); err != nil {
		return nil, err
	}

	t, err := time.Parse("2006-01-02", logDate)
	if err != nil {
		return nil, err
	}

	log.LogDate = t
	log.NoGames = noGames == 1
	log.Walk = walk == 1
	log.Workout = workout == 1
	log.Calories = calories == 1
	log.Project = project == 1

	return &log, nil
}

func (s *Store) SaveWeight(userID int64, logDate time.Time, weight float64) error {
	dateStr := logDate.Format("2006-01-02")

	_, err := s.db.Exec(
		`INSERT INTO weight_logs (user_id, log_date, weight_kg) VALUES (?, ?, ?)
		 ON CONFLICT(user_id, log_date) DO UPDATE SET weight_kg = excluded.weight_kg`,
		userID, dateStr, weight,
	)
	if err != nil {
		return err
	}

	var startWeight sql.NullFloat64
	err = s.db.QueryRow(`SELECT start_weight FROM users WHERE id = ?`, userID).Scan(&startWeight)
	if err != nil {
		return err
	}

	return s.recalcStartWeight(userID)
}

func (s *Store) recalcStartWeight(userID int64) error {
	row := s.db.QueryRow(
		`SELECT weight_kg FROM weight_logs WHERE user_id = ? ORDER BY log_date ASC LIMIT 1`,
		userID,
	)
	var weight float64
	if err := row.Scan(&weight); err != nil {
		return err
	}
	_, err := s.db.Exec(`UPDATE users SET start_weight = ? WHERE id = ?`, weight, userID)
	return err
}

func (s *Store) GetStartWeight(userID int64) (float64, error) {
	var startWeight sql.NullFloat64
	err := s.db.QueryRow(`SELECT start_weight FROM users WHERE id = ?`, userID).Scan(&startWeight)
	if err != nil {
		return 0, err
	}
	if startWeight.Valid {
		return startWeight.Float64, nil
	}

	row := s.db.QueryRow(
		`SELECT weight_kg FROM weight_logs WHERE user_id = ? ORDER BY log_date ASC LIMIT 1`,
		userID,
	)
	var weight float64
	if err := row.Scan(&weight); err != nil {
		return 0, err
	}
	return weight, nil
}

func (s *Store) GetLatestWeight(userID int64) (*models.WeightLog, error) {
	row := s.db.QueryRow(
		`SELECT id, user_id, log_date, weight_kg FROM weight_logs
		 WHERE user_id = ? ORDER BY log_date DESC LIMIT 1`,
		userID,
	)
	return scanWeightLog(row)
}

func (s *Store) GetWeightForWeek(userID int64, weekStart, weekEnd time.Time) (*models.WeightLog, error) {
	row := s.db.QueryRow(
		`SELECT id, user_id, log_date, weight_kg FROM weight_logs
		 WHERE user_id = ? AND log_date >= ? AND log_date <= ?
		 ORDER BY log_date DESC LIMIT 1`,
		userID, weekStart.Format("2006-01-02"), weekEnd.Format("2006-01-02"),
	)
	return scanWeightLog(row)
}

func scanWeightLog(row *sql.Row) (*models.WeightLog, error) {
	var wl models.WeightLog
	var logDate string
	if err := row.Scan(&wl.ID, &wl.UserID, &logDate, &wl.WeightKg); err != nil {
		return nil, err
	}
	t, err := time.Parse("2006-01-02", logDate)
	if err != nil {
		return nil, err
	}
	wl.LogDate = t
	return &wl, nil
}

func taskColumn(task models.Task) (string, error) {
	switch task {
	case models.TaskNoGames:
		return "no_games", nil
	case models.TaskWalk:
		return "walk", nil
	case models.TaskWorkout:
		return "workout", nil
	case models.TaskCalories:
		return "calories", nil
	case models.TaskProject:
		return "project", nil
	default:
		return "", fmt.Errorf("unknown task: %s", task)
	}
}
