package database

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
)

func openTest(t *testing.T) *Store {
	t.Helper()
	s, err := OpenSQLite(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestCRUD(t *testing.T) {
	s, ctx := openTest(t), context.Background()

	if got, err := s.List(ctx); err != nil || got == nil || len(got) != 0 {
		t.Fatalf("empty list = %v, %v; want non-nil empty", got, err)
	}

	a := Task{Name: "backup", Service: "db", Time: "02:00", Duration: "01:30", DaysOfWeek: "Mon,Fri", IsRecurring: true, Hosts: "h1,h2"}
	b := Task{Name: "patch", Service: "AWX", Time: "18:00", Date: "2026-11-05"}
	for _, task := range []*Task{&a, &b} {
		if err := s.Create(ctx, task); err != nil {
			t.Fatal(err)
		}
	}
	if a.ID == 0 || b.ID <= a.ID {
		t.Fatalf("ids not assigned in order: %d, %d", a.ID, b.ID)
	}

	got, err := s.Get(ctx, b.ID)
	if err != nil || got != b {
		t.Fatalf("Get = %+v, %v; want %+v", got, err, b)
	}

	a.Description = "nightly"
	a.IsRecurring = false
	if err := s.Update(ctx, a.ID, &a); err != nil {
		t.Fatal(err)
	}
	list, err := s.List(ctx)
	if err != nil || len(list) != 2 || list[0] != a || list[1] != b {
		t.Fatalf("List = %+v, %v", list, err)
	}

	if err := s.Delete(ctx, a.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Get(ctx, a.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Get deleted = %v; want ErrNotFound", err)
	}
}

func TestNotFound(t *testing.T) {
	s, ctx := openTest(t), context.Background()
	if err := s.Update(ctx, 42, &Task{Name: "x"}); !errors.Is(err, ErrNotFound) {
		t.Errorf("Update = %v", err)
	}
	if err := s.Delete(ctx, 42); !errors.Is(err, ErrNotFound) {
		t.Errorf("Delete = %v", err)
	}
	if _, err := s.Get(ctx, 42); !errors.Is(err, ErrNotFound) {
		t.Errorf("Get = %v", err)
	}
}

// modernc sqlite must bind $N by number, not by position.
func TestDollarPlaceholdersBindByNumber(t *testing.T) {
	s := openTest(t)
	var first, second string
	if err := s.db.QueryRow("SELECT $2, $1", "one", "two").Scan(&first, &second); err != nil {
		t.Fatal(err)
	}
	if first != "two" || second != "one" {
		t.Fatalf("got %q, %q", first, second)
	}
}

func TestUpgradeFromOldSchema(t *testing.T) {
	path := filepath.Join(t.TempDir(), "old.db")
	old, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	// Schema of the earliest releases: no duration, no schema_migrations.
	_, err = old.Exec(`CREATE TABLE tasks (
		id INTEGER PRIMARY KEY AUTOINCREMENT, name TEXT NOT NULL, service TEXT NOT NULL,
		time TEXT NOT NULL, days_of_week TEXT NOT NULL, is_recurring BOOLEAN NOT NULL,
		description TEXT, hosts TEXT);
		INSERT INTO tasks (name, service, time, days_of_week, is_recurring, description, hosts)
		VALUES ('Reports', 'Reporting', '08:30', 'Tue,Thu', 1, NULL, 'ldap');`)
	old.Close()
	if err != nil {
		t.Fatal(err)
	}

	for range 2 { // second open must be a no-op
		s, err := OpenSQLite(path)
		if err != nil {
			t.Fatal(err)
		}
		tasks, err := s.List(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		want := Task{ID: 1, Name: "Reports", Service: "Reporting", Time: "08:30", DaysOfWeek: "Tue,Thu", IsRecurring: true, Hosts: "ldap"}
		if len(tasks) != 1 || tasks[0] != want {
			t.Fatalf("tasks = %+v; want [%+v]", tasks, want)
		}
		var version int
		if err := s.db.QueryRow("SELECT MAX(version) FROM schema_migrations").Scan(&version); err != nil || version != len(migrations) {
			t.Fatalf("version = %d, %v", version, err)
		}
		s.Close()
	}
}
