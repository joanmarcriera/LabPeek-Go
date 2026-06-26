package models

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

const (
	networkLabelKey           = "ui.network_label"
	discoveryDefaultTargetKey = "discovery.default_target"
	DefaultNetworkLabel       = "Lab network(default)"
	DefaultDiscoveryTarget    = "192.168.1.0/24"
)

type SettingsRepository struct {
	conn queryConn
}

func NewSettingsRepository(database *sql.DB) *SettingsRepository {
	return &SettingsRepository{conn: database}
}

func (r *SettingsRepository) Get(ctx context.Context, key string, fallback string) (string, error) {
	var value string
	err := r.conn.QueryRowContext(ctx, `SELECT value FROM settings WHERE key = ?`, key).Scan(&value)
	if err == nil {
		return value, nil
	}
	if err != sql.ErrNoRows {
		return "", fmt.Errorf("get setting %q: %w", key, err)
	}
	if err := r.Set(ctx, key, fallback); err != nil {
		return "", err
	}
	return fallback, nil
}

func (r *SettingsRepository) Set(ctx context.Context, key string, value string) error {
	_, err := r.conn.ExecContext(ctx, `
		INSERT INTO settings (key, value, updated_at)
		VALUES (?, ?, ?)
		ON CONFLICT(key) DO UPDATE SET
			value = excluded.value,
			updated_at = excluded.updated_at
	`, key, value, formatRequiredTime(nowUTC()))
	if err != nil {
		return fmt.Errorf("set setting %q: %w", key, err)
	}
	return nil
}

func (r *SettingsRepository) GetNetworkSettings(ctx context.Context) (NetworkSettings, error) {
	label, err := r.Get(ctx, networkLabelKey, DefaultNetworkLabel)
	if err != nil {
		return NetworkSettings{}, err
	}
	target, err := r.Get(ctx, discoveryDefaultTargetKey, DefaultDiscoveryTarget)
	if err != nil {
		return NetworkSettings{}, err
	}
	return NetworkSettings{
		Label:         label,
		DefaultTarget: target,
	}, nil
}

func (r *SettingsRepository) UpdateNetworkSettings(ctx context.Context, settings NetworkSettings) error {
	label := strings.TrimSpace(settings.Label)
	if label == "" {
		label = DefaultNetworkLabel
	}
	target := strings.TrimSpace(settings.DefaultTarget)
	if target == "" {
		target = DefaultDiscoveryTarget
	}
	if err := r.Set(ctx, networkLabelKey, label); err != nil {
		return err
	}
	if err := r.Set(ctx, discoveryDefaultTargetKey, target); err != nil {
		return err
	}
	return nil
}
