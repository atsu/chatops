package db

import (
	"database/sql"

	_ "github.com/mattn/go-sqlite3"
)

const (
	TableInitQuery = "CREATE TABLE IF NOT EXISTS tokens (teamId TEXT PRIMARY KEY, botToken TEXT, webHookUrl TEXT)"
)

type Database interface {
	Init() error
	InsertSlackBot(teamId, botToken, webHookUrl string) error
	GetSlackBot(teamId string) (string, string, error)
	GetAllSlackBots() ([]SlackBot, error)
}

type SqliteDb struct {
	db   *sql.DB
	file string
}

func NewSqliteDB(file string) *SqliteDb {
	return &SqliteDb{
		file: file,
	}
}

func (sdb *SqliteDb) Init() error {
	if db, err := sql.Open("sqlite3", sdb.file); err != nil {
		return err
	} else {
		statement, err := db.Prepare(TableInitQuery)
		if err != nil {
			return err
		}
		if _, err := statement.Exec(); err != nil {
			return err
		}
		sdb.db = db
	}
	return nil
}

func (sdb *SqliteDb) InsertSlackBot(teamId, botToken, webHookUrl string) error {
	if query, err := sdb.db.Prepare("REPLACE INTO tokens (teamId , botToken, webHookUrl) VALUES (?, ?, ?)"); err != nil {
		return err
	} else {
		if _, err := query.Exec(teamId, botToken, webHookUrl); err != nil {
			return err
		}
	}
	return nil
}

func (sdb *SqliteDb) GetSlackBot(teamId string) (string, string, error) {
	row := sdb.db.QueryRow("SELECT botToken, webHookUrl FROM tokens WHERE teamId = :teamId", sql.Named("teamId", teamId))
	botToken, webHookUrl := "", ""
	err := row.Scan(&botToken, &webHookUrl)
	return botToken, webHookUrl, err
}

type SlackBot struct {
	TeamId     string
	BotToken   string
	WebHookUrl string
}

func (sdb *SqliteDb) GetAllSlackBots() ([]SlackBot, error) {
	bots := make([]SlackBot, 0)
	rows, err := sdb.db.Query("SELECT * FROM tokens")
	if err != nil {
		return bots, err
	}
	for rows.Next() {
		bt := SlackBot{}
		if err = rows.Scan(&bt.TeamId, &bt.BotToken, &bt.WebHookUrl); err != nil {
			return bots, err
		}
		bots = append(bots, bt)
	}
	return bots, err
}
