package models

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
)

type AssetRepository struct {
	conn   queryConn
	rootDB *sql.DB
}

func NewAssetRepository(database *sql.DB) *AssetRepository {
	return &AssetRepository{conn: database, rootDB: database}
}

func (r *AssetRepository) WithTx(tx *sql.Tx) *AssetRepository {
	return &AssetRepository{conn: tx, rootDB: r.rootDB}
}

func (r *AssetRepository) DB() *sql.DB {
	return r.rootDB
}

func (r *AssetRepository) Create(ctx context.Context, asset *Asset) error {
	if asset.ID == "" {
		asset.ID = uuid.NewString()
	}
	if asset.DisplayName == "" {
		return errors.New("display_name is required")
	}
	if asset.AssetType == "" {
		asset.AssetType = "unknown"
	}
	if asset.Status == "" {
		asset.Status = "active"
	}

	createdAt := asset.CreatedAt
	if createdAt.IsZero() {
		createdAt = nowUTC()
	}
	updatedAt := asset.UpdatedAt
	if updatedAt.IsZero() {
		updatedAt = createdAt
	}
	if asset.ManualDataJSON == "" {
		asset.ManualDataJSON = "{}"
	}
	if asset.DiscoveredDataJSON == "" {
		asset.DiscoveredDataJSON = "{}"
	}

	_, err := r.conn.ExecContext(ctx, `
		INSERT INTO assets (
			id, display_name, discovered_name, asset_type, status, primary_ip, primary_mac,
			mac_vendor, notes, manual_data_json, discovered_data_json, first_seen_at, last_seen_at,
			created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		asset.ID,
		asset.DisplayName,
		asset.DiscoveredName,
		asset.AssetType,
		asset.Status,
		asset.PrimaryIP,
		asset.PrimaryMAC,
		asset.MACVendor,
		asset.Notes,
		asset.ManualDataJSON,
		asset.DiscoveredDataJSON,
		formatNullableTime(asset.FirstSeenAt),
		formatNullableTime(asset.LastSeenAt),
		formatRequiredTime(createdAt),
		formatRequiredTime(updatedAt),
	)
	if err != nil {
		return fmt.Errorf("create asset: %w", err)
	}

	asset.CreatedAt = createdAt
	asset.UpdatedAt = updatedAt
	return nil
}

func (r *AssetRepository) List(ctx context.Context) ([]Asset, error) {
	rows, err := r.conn.QueryContext(ctx, `
		SELECT
			id, display_name, discovered_name, asset_type, status, primary_ip, primary_mac,
			mac_vendor, notes, manual_data_json, discovered_data_json,
			first_seen_at, last_seen_at, created_at, updated_at
		FROM assets
		ORDER BY display_name ASC, created_at ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("list assets: %w", err)
	}
	defer rows.Close()

	var assets []Asset
	for rows.Next() {
		asset, err := scanAsset(rows)
		if err != nil {
			return nil, err
		}
		assets = append(assets, asset)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate assets: %w", err)
	}
	return assets, nil
}

func (r *AssetRepository) Count(ctx context.Context) (int, error) {
	var count int
	if err := r.conn.QueryRowContext(ctx, `SELECT COUNT(1) FROM assets`).Scan(&count); err != nil {
		return 0, fmt.Errorf("count assets: %w", err)
	}
	return count, nil
}

func (r *AssetRepository) Get(ctx context.Context, id string) (*Asset, error) {
	row := r.conn.QueryRowContext(ctx, `
		SELECT
			id, display_name, discovered_name, asset_type, status, primary_ip, primary_mac,
			mac_vendor, notes, manual_data_json, discovered_data_json,
			first_seen_at, last_seen_at, created_at, updated_at
		FROM assets
		WHERE id = ?
	`, id)

	asset, err := scanAsset(row)
	if err != nil {
		return nil, err
	}
	return &asset, nil
}

func (r *AssetRepository) FindByPrimaryMAC(ctx context.Context, mac string) (*Asset, error) {
	if mac == "" {
		return nil, nil
	}
	row := r.conn.QueryRowContext(ctx, `
		SELECT
			id, display_name, discovered_name, asset_type, status, primary_ip, primary_mac,
			mac_vendor, notes, manual_data_json, discovered_data_json,
			first_seen_at, last_seen_at, created_at, updated_at
		FROM assets
		WHERE primary_mac = ?
		LIMIT 1
	`, mac)

	asset, err := scanAsset(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &asset, nil
}

func (r *AssetRepository) FindByPrimaryIP(ctx context.Context, ip string) (*Asset, error) {
	if ip == "" {
		return nil, nil
	}
	row := r.conn.QueryRowContext(ctx, `
		SELECT
			id, display_name, discovered_name, asset_type, status, primary_ip, primary_mac,
			mac_vendor, notes, manual_data_json, discovered_data_json,
			first_seen_at, last_seen_at, created_at, updated_at
		FROM assets
		WHERE primary_ip = ?
		LIMIT 1
	`, ip)

	asset, err := scanAsset(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &asset, nil
}

func (r *AssetRepository) FindByDiscoveredName(ctx context.Context, discoveredName string) ([]Asset, error) {
	if discoveredName == "" {
		return nil, nil
	}
	rows, err := r.conn.QueryContext(ctx, `
		SELECT
			id, display_name, discovered_name, asset_type, status, primary_ip, primary_mac,
			mac_vendor, notes, manual_data_json, discovered_data_json,
			first_seen_at, last_seen_at, created_at, updated_at
		FROM assets
		WHERE discovered_name = ?
		ORDER BY updated_at DESC
	`, discoveredName)
	if err != nil {
		return nil, fmt.Errorf("find assets by discovered name: %w", err)
	}
	defer rows.Close()

	var assets []Asset
	for rows.Next() {
		asset, err := scanAsset(rows)
		if err != nil {
			return nil, err
		}
		assets = append(assets, asset)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate assets by discovered name: %w", err)
	}
	return assets, nil
}

func (r *AssetRepository) UpdateBasicFields(ctx context.Context, update AssetUpdate) error {
	current, err := r.Get(ctx, update.ID)
	if err != nil {
		return fmt.Errorf("load asset before update: %w", err)
	}

	fields := []string{"notes"}
	displayName := current.DisplayName
	if update.DisplayName != "" {
		displayName = update.DisplayName
		fields = append(fields, "display_name")
	}
	assetType := current.AssetType
	if update.AssetType != "" {
		assetType = update.AssetType
		fields = append(fields, "asset_type")
	}

	updatedAt := nowUTC()
	manualDataJSON, err := mergeManualData(current.ManualDataJSON, fields, updatedAt)
	if err != nil {
		return err
	}

	_, err = r.conn.ExecContext(ctx, `
		UPDATE assets
		SET display_name = ?, asset_type = ?, notes = ?, manual_data_json = ?, updated_at = ?
		WHERE id = ?
	`,
		displayName,
		assetType,
		update.Notes,
		manualDataJSON,
		formatRequiredTime(updatedAt),
		update.ID,
	)
	if err != nil {
		return fmt.Errorf("update asset basics: %w", err)
	}

	return nil
}

func (r *AssetRepository) Delete(ctx context.Context, id string) error {
	return r.DeleteBatch(ctx, []string{id})
}

func (r *AssetRepository) DeleteBatch(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	placeholders := strings.Repeat("?,", len(ids))
	placeholders = placeholders[:len(placeholders)-1]
	args := make([]any, len(ids))
	for i, id := range ids {
		args[i] = id
	}

	tx, err := r.rootDB.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin delete tx: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx,
		fmt.Sprintf(`DELETE FROM services WHERE asset_id IN (%s)`, placeholders), args...); err != nil {
		return fmt.Errorf("delete services for assets: %w", err)
	}
	if _, err := tx.ExecContext(ctx,
		fmt.Sprintf(`DELETE FROM assets WHERE id IN (%s)`, placeholders), args...); err != nil {
		return fmt.Errorf("delete assets: %w", err)
	}
	return tx.Commit()
}

func (r *AssetRepository) SaveDiscoveredFields(ctx context.Context, asset *Asset) error {
	updatedAt := asset.UpdatedAt
	if updatedAt.IsZero() {
		updatedAt = nowUTC()
	}

	_, err := r.conn.ExecContext(ctx, `
		UPDATE assets
		SET discovered_name = ?, primary_ip = ?, primary_mac = ?, mac_vendor = ?,
			discovered_data_json = ?, first_seen_at = ?, last_seen_at = ?, updated_at = ?
		WHERE id = ?
	`,
		asset.DiscoveredName,
		asset.PrimaryIP,
		asset.PrimaryMAC,
		asset.MACVendor,
		asset.DiscoveredDataJSON,
		formatNullableTime(asset.FirstSeenAt),
		formatNullableTime(asset.LastSeenAt),
		formatRequiredTime(updatedAt),
		asset.ID,
	)
	if err != nil {
		return fmt.Errorf("save discovered asset fields: %w", err)
	}
	return nil
}

type assetScanner interface {
	Scan(dest ...any) error
}

func scanAsset(scanner assetScanner) (Asset, error) {
	var asset Asset
	var discoveredName sql.NullString
	var primaryIP sql.NullString
	var primaryMAC sql.NullString
	var macVendor sql.NullString
	var notes sql.NullString
	var firstSeen sql.NullString
	var lastSeen sql.NullString
	var createdAt sql.NullString
	var updatedAt sql.NullString

	err := scanner.Scan(
		&asset.ID,
		&asset.DisplayName,
		&discoveredName,
		&asset.AssetType,
		&asset.Status,
		&primaryIP,
		&primaryMAC,
		&macVendor,
		&notes,
		&asset.ManualDataJSON,
		&asset.DiscoveredDataJSON,
		&firstSeen,
		&lastSeen,
		&createdAt,
		&updatedAt,
	)
	if err != nil {
		return Asset{}, err
	}

	asset.DiscoveredName = discoveredName.String
	asset.PrimaryIP = primaryIP.String
	asset.PrimaryMAC = primaryMAC.String
	asset.MACVendor = macVendor.String
	asset.Notes = notes.String

	asset.FirstSeenAt, err = parseNullableTime(firstSeen)
	if err != nil {
		return Asset{}, err
	}
	asset.LastSeenAt, err = parseNullableTime(lastSeen)
	if err != nil {
		return Asset{}, err
	}
	asset.CreatedAt, err = parseNullableTime(createdAt)
	if err != nil {
		return Asset{}, err
	}
	asset.UpdatedAt, err = parseNullableTime(updatedAt)
	if err != nil {
		return Asset{}, err
	}

	return asset, nil
}
