// Package database stores tasks in SQLite or PostgreSQL using one SQL dialect
// ($N placeholders, INSERT ... RETURNING), which both engines accept.
package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net"
	"net/url"

	_ "github.com/jackc/pgx/v5/stdlib"
	_ "modernc.org/sqlite"
)

// ErrNotFound is returned when no task has the requested id.
var ErrNotFound = errors.New("task not found")

type Store struct {
	db       *sql.DB
	postgres bool // only migrations differ between engines
}

// OpenSQLite opens (creating if needed) the SQLite file at path and migrates it.
//
// WAL lets readers run concurrently with the single writer, and busy_timeout
// makes concurrent writers wait instead of failing with SQLITE_BUSY, so the
// pool is left unlimited rather than pinned to one connection.
func OpenSQLite(path string) (*Store, error) {
	dsn := path + "?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(ON)"
	return open("sqlite", dsn, false)
}

type PostgresConfig struct {
	Host, Port, User, Password, DBName, SSLMode string
}

// OpenPostgres connects to PostgreSQL and migrates the schema.
func OpenPostgres(cfg PostgresConfig) (*Store, error) {
	if cfg.SSLMode == "" {
		cfg.SSLMode = "disable"
	}
	u := url.URL{
		Scheme:   "postgres",
		User:     url.UserPassword(cfg.User, cfg.Password),
		Host:     net.JoinHostPort(cfg.Host, cfg.Port),
		Path:     "/" + cfg.DBName,
		RawQuery: url.Values{"sslmode": {cfg.SSLMode}}.Encode(),
	}
	return open("pgx", u.String(), true)
}

func open(driver, dsn string, postgres bool) (*Store, error) {
	db, err := sql.Open(driver, dsn)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", driver, err)
	}
	s := &Store{db: db, postgres: postgres}
	ctx := context.Background()
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("connect %s: %w", driver, err)
	}
	if err := s.migrate(ctx); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) Ping(ctx context.Context) error { return s.db.PingContext(ctx) }

func (s *Store) Close() error { return s.db.Close() }

const taskColumns = "id, name, service, time, duration, days_of_week, is_recurring, description, hosts, date"

type scanner interface{ Scan(...any) error }

func scanTask(row scanner) (Task, error) {
	var t Task
	var duration, description, hosts, date sql.NullString
	err := row.Scan(&t.ID, &t.Name, &t.Service, &t.Time, &duration, &t.DaysOfWeek, &t.IsRecurring, &description, &hosts, &date)
	t.Duration, t.Description, t.Hosts, t.Date = duration.String, description.String, hosts.String, date.String
	return t, err
}

func (s *Store) List(ctx context.Context) ([]Task, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT "+taskColumns+" FROM tasks ORDER BY id")
	if err != nil {
		return nil, fmt.Errorf("list tasks: %w", err)
	}
	defer rows.Close()
	tasks := []Task{}
	for rows.Next() {
		t, err := scanTask(rows)
		if err != nil {
			return nil, fmt.Errorf("scan task: %w", err)
		}
		tasks = append(tasks, t)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list tasks: %w", err)
	}
	return tasks, nil
}

func (s *Store) Get(ctx context.Context, id int) (Task, error) {
	t, err := scanTask(s.db.QueryRowContext(ctx, "SELECT "+taskColumns+" FROM tasks WHERE id = $1", id))
	if errors.Is(err, sql.ErrNoRows) {
		return Task{}, ErrNotFound
	}
	if err != nil {
		return Task{}, fmt.Errorf("get task %d: %w", id, err)
	}
	return t, nil
}

// Create inserts t and sets t.ID.
func (s *Store) Create(ctx context.Context, t *Task) error {
	err := s.db.QueryRowContext(ctx,
		`INSERT INTO tasks (name, service, time, duration, days_of_week, is_recurring, description, hosts, date)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9) RETURNING id`,
		t.Name, t.Service, t.Time, t.Duration, t.DaysOfWeek, t.IsRecurring, t.Description, t.Hosts, t.Date,
	).Scan(&t.ID)
	if err != nil {
		return fmt.Errorf("create task: %w", err)
	}
	return nil
}

// Update replaces task id with t and sets t.ID; ErrNotFound if id does not exist.
func (s *Store) Update(ctx context.Context, id int, t *Task) error {
	res, err := s.db.ExecContext(ctx,
		`UPDATE tasks SET name = $1, service = $2, time = $3, duration = $4, days_of_week = $5,
		 is_recurring = $6, description = $7, hosts = $8, date = $9 WHERE id = $10`,
		t.Name, t.Service, t.Time, t.Duration, t.DaysOfWeek, t.IsRecurring, t.Description, t.Hosts, t.Date, id,
	)
	if err := affectedOne(res, err); err != nil {
		return fmt.Errorf("update task %d: %w", id, err)
	}
	t.ID = id
	return nil
}

func (s *Store) Delete(ctx context.Context, id int) error {
	res, err := s.db.ExecContext(ctx, "DELETE FROM tasks WHERE id = $1", id)
	if err := affectedOne(res, err); err != nil {
		return fmt.Errorf("delete task %d: %w", id, err)
	}
	return nil
}

func affectedOne(res sql.Result, err error) error {
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}
