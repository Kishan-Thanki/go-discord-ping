package database

import (
	"database/sql"
	"log/slog"
	"time"

	_ "modernc.org/sqlite"
)

var DB *sql.DB

var (
	stmtGetUser         *sql.Stmt
	stmtInsertUser      *sql.Stmt
	stmtUpdateXP        *sql.Stmt
	stmtUpdateBalance   *sql.Stmt
	stmtUpdateDaily     *sql.Stmt
	stmtGetLeaderboard  *sql.Stmt
	stmtGetPrefix       *sql.Stmt
	stmtSetPrefix       *sql.Stmt
	stmtCreateReminder  *sql.Stmt
	stmtDeleteReminder  *sql.Stmt
	stmtGetReminders    *sql.Stmt
	stmtAddWarning      *sql.Stmt
	stmtGetWarningCount *sql.Stmt
)

func InitDB(dataSourceName string) error {
	var err error
	DB, err = sql.Open("sqlite", dataSourceName)
	if err != nil {
		return err
	}

	DB.SetMaxOpenConns(1)
	DB.SetMaxIdleConns(1)
	DB.SetConnMaxLifetime(0)

	pragmas := []string{
		"PRAGMA journal_mode = WAL;",
		"PRAGMA synchronous = NORMAL;",
		"PRAGMA temp_store = MEMORY;",
		"PRAGMA cache_size = -16000;", // Limit read cache to 16MB to save RAM
	}
	for _, p := range pragmas {
		if _, err := DB.Exec(p); err != nil {
			return err
		}
	}

	if err = createTables(); err != nil {
		return err
	}

	if err = prepareStatements(); err != nil {
		return err
	}

	slog.Info("Database initialized successfully with WAL & Prepared Statements")
	return nil
}

func CloseDB() {
	if stmtGetUser != nil {
		stmtGetUser.Close()
	}
	if stmtInsertUser != nil {
		stmtInsertUser.Close()
	}
	if stmtUpdateXP != nil {
		stmtUpdateXP.Close()
	}
	if stmtUpdateBalance != nil {
		stmtUpdateBalance.Close()
	}
	if stmtUpdateDaily != nil {
		stmtUpdateDaily.Close()
	}
	if stmtGetLeaderboard != nil {
		stmtGetLeaderboard.Close()
	}
	if stmtGetPrefix != nil {
		stmtGetPrefix.Close()
	}
	if stmtSetPrefix != nil {
		stmtSetPrefix.Close()
	}
	if stmtCreateReminder != nil {
		stmtCreateReminder.Close()
	}
	if stmtDeleteReminder != nil {
		stmtDeleteReminder.Close()
	}
	if stmtGetReminders != nil {
		stmtGetReminders.Close()
	}
	if stmtAddWarning != nil {
		stmtAddWarning.Close()
	}
	if stmtGetWarningCount != nil {
		stmtGetWarningCount.Close()
	}

	if DB != nil {
		slog.Info("Closing database connection")
		DB.Close()
	}
}

func createTables() error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS users (
			user_id    TEXT NOT NULL,
			guild_id   TEXT NOT NULL,
			xp         INTEGER DEFAULT 0,
			level      INTEGER DEFAULT 0,
			balance    INTEGER DEFAULT 0,
			last_daily TEXT DEFAULT '',
			PRIMARY KEY (user_id, guild_id)
		);`,
		`CREATE TABLE IF NOT EXISTS server_config (
			guild_id        TEXT PRIMARY KEY,
			prefix          TEXT DEFAULT '!',
			welcome_channel TEXT DEFAULT '',
			log_channel     TEXT DEFAULT ''
		);`,
		`CREATE TABLE IF NOT EXISTS reminders (
			id         INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id    TEXT NOT NULL,
			channel_id TEXT NOT NULL,
			guild_id   TEXT NOT NULL,
			message    TEXT NOT NULL,
			remind_at  TEXT NOT NULL,
			created_at TEXT NOT NULL
		);`,
		`CREATE TABLE IF NOT EXISTS warnings (
			id         INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id    TEXT NOT NULL,
			guild_id   TEXT NOT NULL,
			reason     TEXT NOT NULL,
			timestamp  TEXT NOT NULL
		);`,
	}

	for _, q := range queries {
		if _, err := DB.Exec(q); err != nil {
			return err
		}
	}
	return nil
}

func prepareStatements() error {
	var err error

	stmtGetUser, err = DB.Prepare("SELECT xp, level, balance, last_daily FROM users WHERE user_id = ? AND guild_id = ?")
	if err != nil {
		return err
	}

	stmtInsertUser, err = DB.Prepare("INSERT INTO users (user_id, guild_id) VALUES (?, ?)")
	if err != nil {
		return err
	}

	stmtUpdateXP, err = DB.Prepare("UPDATE users SET xp = ?, level = ? WHERE user_id = ? AND guild_id = ?")
	if err != nil {
		return err
	}

	stmtUpdateBalance, err = DB.Prepare("UPDATE users SET balance = ? WHERE user_id = ? AND guild_id = ?")
	if err != nil {
		return err
	}

	stmtUpdateDaily, err = DB.Prepare("UPDATE users SET balance = ?, last_daily = ? WHERE user_id = ? AND guild_id = ?")
	if err != nil {
		return err
	}

	stmtGetLeaderboard, err = DB.Prepare("SELECT user_id, xp, level FROM users WHERE guild_id = ? ORDER BY xp DESC LIMIT ?")
	if err != nil {
		return err
	}

	stmtGetPrefix, err = DB.Prepare("SELECT prefix FROM server_config WHERE guild_id = ?")
	if err != nil {
		return err
	}

	stmtSetPrefix, err = DB.Prepare("INSERT INTO server_config (guild_id, prefix) VALUES (?, ?) ON CONFLICT(guild_id) DO UPDATE SET prefix = ?")
	if err != nil {
		return err
	}

	stmtCreateReminder, err = DB.Prepare("INSERT INTO reminders (user_id, channel_id, guild_id, message, remind_at, created_at) VALUES (?, ?, ?, ?, ?, ?)")
	if err != nil {
		return err
	}

	stmtDeleteReminder, err = DB.Prepare("DELETE FROM reminders WHERE id = ?")
	if err != nil {
		return err
	}

	stmtGetReminders, err = DB.Prepare("SELECT id, user_id, channel_id, guild_id, message, remind_at FROM reminders")
	if err != nil {
		return err
	}

	stmtAddWarning, err = DB.Prepare("INSERT INTO warnings (user_id, guild_id, reason, timestamp) VALUES (?, ?, ?, ?)")
	if err != nil {
		return err
	}

	stmtGetWarningCount, err = DB.Prepare("SELECT COUNT(*) FROM warnings WHERE user_id = ? AND guild_id = ?")
	if err != nil {
		return err
	}

	return nil
}

// User represents a user record in the database
type User struct {
	UserID    string
	GuildID   string
	XP        int
	Level     int
	Balance   int
	LastDaily string
}

func GetUser(userID, guildID string) (*User, error) {
	u := &User{UserID: userID, GuildID: guildID}

	err := stmtGetUser.QueryRow(userID, guildID).Scan(&u.XP, &u.Level, &u.Balance, &u.LastDaily)

	if err == sql.ErrNoRows {
		_, err = stmtInsertUser.Exec(userID, guildID)
		if err != nil {
			return nil, err
		}
		return u, nil
	}
	if err != nil {
		return nil, err
	}

	return u, nil
}

func UpdateUserXP(userID, guildID string, xp, level int) error {
	_, err := stmtUpdateXP.Exec(xp, level, userID, guildID)
	return err
}

func UpdateUserBalance(userID, guildID string, balance int) error {
	_, err := stmtUpdateBalance.Exec(balance, userID, guildID)
	return err
}

func UpdateUserDaily(userID, guildID string, balance int, lastDaily string) error {
	_, err := stmtUpdateDaily.Exec(balance, lastDaily, userID, guildID)
	return err
}

func GetLeaderboard(guildID string, limit int) ([]*User, error) {
	rows, err := stmtGetLeaderboard.Query(guildID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []*User
	for rows.Next() {
		u := &User{GuildID: guildID}
		if err := rows.Scan(&u.UserID, &u.XP, &u.Level); err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	return users, nil
}

func GetPrefix(guildID string) (string, error) {
	var prefix string
	err := stmtGetPrefix.QueryRow(guildID).Scan(&prefix)
	if err == sql.ErrNoRows {
		return "!", nil
	}
	if err != nil {
		return "", err
	}
	return prefix, nil
}

func SetPrefix(guildID, prefix string) error {
	_, err := stmtSetPrefix.Exec(guildID, prefix, prefix)
	return err
}

type Reminder struct {
	ID        int
	UserID    string
	ChannelID string
	GuildID   string
	Message   string
	RemindAt  time.Time
}

func CreateReminder(userID, channelID, guildID, message string, remindAt time.Time) (int, error) {
	res, err := stmtCreateReminder.Exec(userID, channelID, guildID, message, remindAt.Format(time.RFC3339), time.Now().Format(time.RFC3339))
	if err != nil {
		return 0, err
	}
	id, err := res.LastInsertId()
	return int(id), err
}

func DeleteReminder(id int) error {
	_, err := stmtDeleteReminder.Exec(id)
	return err
}

func GetPendingReminders() ([]*Reminder, error) {
	rows, err := stmtGetReminders.Query()
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var reminders []*Reminder
	for rows.Next() {
		r := &Reminder{}
		var remindAtStr string
		if err := rows.Scan(&r.ID, &r.UserID, &r.ChannelID, &r.GuildID, &r.Message, &remindAtStr); err != nil {
			return nil, err
		}
		t, err := time.Parse(time.RFC3339, remindAtStr)
		if err != nil {
			continue
		}
		r.RemindAt = t
		reminders = append(reminders, r)
	}
	return reminders, nil
}

func AddWarning(userID, guildID, reason string) error {
	_, err := stmtAddWarning.Exec(userID, guildID, reason, time.Now().Format(time.RFC3339))
	return err
}

func GetWarningCount(userID, guildID string) (int, error) {
	var count int
	err := stmtGetWarningCount.QueryRow(userID, guildID).Scan(&count)
	return count, err
}
