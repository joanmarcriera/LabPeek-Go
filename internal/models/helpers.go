package models

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

type queryConn interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

func nowUTC() time.Time {
	return time.Now().UTC()
}

func formatNullableTime(t time.Time) any {
	if t.IsZero() {
		return nil
	}
	return t.UTC().Format(time.RFC3339Nano)
}

func formatRequiredTime(t time.Time) string {
	return t.UTC().Format(time.RFC3339Nano)
}

func parseNullableTime(value sql.NullString) (time.Time, error) {
	if !value.Valid || value.String == "" {
		return time.Time{}, nil
	}
	parsed, err := time.Parse(time.RFC3339Nano, value.String)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse time %q: %w", value.String, err)
	}
	return parsed.UTC(), nil
}

func mergeManualData(existing string, fields []string, updatedAt time.Time) (string, error) {
	data := map[string]map[string]string{}
	if existing != "" {
		if err := json.Unmarshal([]byte(existing), &data); err != nil {
			return "", fmt.Errorf("parse manual data: %w", err)
		}
	}

	timestamp := updatedAt.UTC().Format(time.RFC3339Nano)
	for _, field := range fields {
		data[field] = map[string]string{
			"source":     "manual",
			"updated_at": timestamp,
		}
	}

	encoded, err := json.Marshal(data)
	if err != nil {
		return "", fmt.Errorf("marshal manual data: %w", err)
	}
	return string(encoded), nil
}
