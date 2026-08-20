package audit

import (
	"database/sql"
	"encoding/json"
	"time"
)

type Entry struct {
	ID         int64  `json:"id"`
	At         string `json:"at"`
	ActorType  string `json:"actor_type"`
	ActorID    string `json:"actor_id"`
	Action     string `json:"action"`
	Resource   string `json:"resource"`
	BeforeJSON string `json:"before,omitempty"`
	AfterJSON  string `json:"after,omitempty"`
}

type Log struct {
	db *sql.DB
}

func New(db *sql.DB) *Log {
	return &Log{db: db}
}

func (l *Log) Record(actorType, actorID, action, resource string, before, after any) error {
	var beforeJSON, afterJSON string
	if before != nil {
		b, err := json.Marshal(before)
		if err == nil {
			beforeJSON = string(b)
		}
	}
	if after != nil {
		b, err := json.Marshal(after)
		if err == nil {
			afterJSON = string(b)
		}
	}
	_, err := l.db.Exec(
		`INSERT INTO audit (at, actor_type, actor_id, action, resource, before_json, after_json)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		time.Now().UTC().Format(time.RFC3339),
		actorType, actorID, action, resource, beforeJSON, afterJSON,
	)
	return err
}

func (l *Log) List(limit, offset int) ([]Entry, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	rows, err := l.db.Query(
		`SELECT id, at, actor_type, actor_id, action, resource, before_json, after_json
		 FROM audit ORDER BY id DESC LIMIT ? OFFSET ?`,
		limit, offset,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Entry
	for rows.Next() {
		var e Entry
		if err := rows.Scan(&e.ID, &e.At, &e.ActorType, &e.ActorID, &e.Action, &e.Resource, &e.BeforeJSON, &e.AfterJSON); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}
