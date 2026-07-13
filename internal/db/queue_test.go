package db

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"time"
)

func TestParseQueueSchemaContract(t *testing.T) {
	for _, required := range []string{
		"PRIMARY KEY (match_id, dota_id)",
		"discord_id    VARCHAR(20) NOT NULL",
		"DELETE FROM parse_queue WHERE dota_id IS NULL OR discord_id IS NULL",
		"idx_parse_queue_pending_order",
	} {
		if !strings.Contains(schemaSQL, required) {
			t.Errorf("schema missing %q", required)
		}
	}
}

func TestParseQueueRepository(t *testing.T) {
	dsn := os.Getenv("TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("TEST_POSTGRES_DSN not set; PostgreSQL integration test skipped")
	}

	database, err := New(dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	schema := fmt.Sprintf("parse_queue_test_%d", time.Now().UnixNano())
	if _, err := database.Exec(`CREATE SCHEMA ` + schema); err != nil {
		t.Fatal(err)
	}
	defer database.Exec(`DROP SCHEMA ` + schema + ` CASCADE`)
	if _, err := database.Exec(`SET search_path TO ` + schema); err != nil {
		t.Fatal(err)
	}
	if err := database.RunMigrations(); err != nil {
		t.Fatal(err)
	}

	if err := database.EnqueueParse(100, 1, "discord-1"); err != nil {
		t.Fatal(err)
	}
	if err := database.EnqueueParse(100, 2, "discord-2"); err != nil {
		t.Fatal(err)
	}
	if err := database.EnqueueParse(101, 1, "discord-1"); err != nil {
		t.Fatal(err)
	}
	if err := database.EnqueueParse(101, 1, "discord-1"); err != nil {
		t.Fatalf("idempotent enqueue: %v", err)
	}

	rows, err := database.GetPendingParseQueue(10)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 3 {
		t.Fatalf("got %d queue rows, want 3", len(rows))
	}
	if rows[0].MatchID != 101 || rows[1].MatchID != 100 || rows[2].MatchID != 100 {
		t.Fatalf("queue not newest-first: %+v", rows)
	}

	if err := database.IncrementParseAttempt(100, 1); err != nil {
		t.Fatal(err)
	}
	rows, err = database.GetPendingParseQueue(10)
	if err != nil {
		t.Fatal(err)
	}
	var attempted *ParseQueueRow
	for i := range rows {
		if rows[i].MatchID == 100 && rows[i].DotaID == 1 {
			attempted = &rows[i]
		}
	}
	if attempted == nil || attempted.AttemptCount != 1 || !attempted.LastAttempt.Valid {
		t.Fatalf("attempt metadata not updated: %+v", attempted)
	}

	if err := database.MarkParseDone(100, 1); err != nil {
		t.Fatal(err)
	}
	rows, err = database.GetPendingParseQueue(10)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 {
		t.Fatalf("per-player completion removed wrong rows: %+v", rows)
	}
	for _, row := range rows {
		if row.MatchID == 100 && row.DotaID == 1 {
			t.Fatalf("completed row still pending: %+v", row)
		}
	}
}
