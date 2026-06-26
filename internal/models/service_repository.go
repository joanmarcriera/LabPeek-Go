package models

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/google/uuid"
)

type ServiceRepository struct {
	conn   queryConn
	rootDB *sql.DB
}

func NewServiceRepository(database *sql.DB) *ServiceRepository {
	return &ServiceRepository{conn: database, rootDB: database}
}

func (r *ServiceRepository) WithTx(tx *sql.Tx) *ServiceRepository {
	return &ServiceRepository{conn: tx, rootDB: r.rootDB}
}

func (r *ServiceRepository) Create(ctx context.Context, service *Service) error {
	if service.ID == "" {
		service.ID = uuid.NewString()
	}
	if service.Transport == "" {
		service.Transport = "tcp"
	}
	if service.Status == "" {
		service.Status = "active"
	}

	createdAt := service.CreatedAt
	if createdAt.IsZero() {
		createdAt = nowUTC()
	}
	updatedAt := service.UpdatedAt
	if updatedAt.IsZero() {
		updatedAt = createdAt
	}

	_, err := r.conn.ExecContext(ctx, `
		INSERT INTO services (
			id, asset_id, display_name, ip_address, port, protocol, transport,
			service_name, product, version, url, status, first_seen_at, last_seen_at,
			created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		service.ID,
		service.AssetID,
		service.DisplayName,
		service.IPAddress,
		service.Port,
		service.Protocol,
		service.Transport,
		service.ServiceName,
		service.Product,
		service.Version,
		service.URL,
		service.Status,
		formatNullableTime(service.FirstSeenAt),
		formatNullableTime(service.LastSeenAt),
		formatRequiredTime(createdAt),
		formatRequiredTime(updatedAt),
	)
	if err != nil {
		return fmt.Errorf("create service: %w", err)
	}

	return nil
}

func (r *ServiceRepository) UpsertObserved(ctx context.Context, observed ObservedService) error {
	if observed.Transport == "" {
		observed.Transport = "tcp"
	}
	if observed.ObservedAt.IsZero() {
		observed.ObservedAt = nowUTC()
	}
	serviceID := uuid.NewString()
	timestamp := formatRequiredTime(observed.ObservedAt)

	_, err := r.conn.ExecContext(ctx, `
		INSERT INTO services (
			id, asset_id, display_name, ip_address, port, protocol, transport,
			service_name, product, version, status, first_seen_at, last_seen_at,
			created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 'active', ?, ?, ?, ?)
		ON CONFLICT(ip_address, port, protocol, transport) DO UPDATE SET
			asset_id = excluded.asset_id,
			display_name = excluded.display_name,
			service_name = excluded.service_name,
			product = excluded.product,
			version = excluded.version,
			status = 'active',
			last_seen_at = excluded.last_seen_at,
			updated_at = excluded.updated_at
	`,
		serviceID,
		observed.AssetID,
		observed.DisplayName,
		observed.IPAddress,
		observed.Port,
		observed.Protocol,
		observed.Transport,
		observed.ServiceName,
		observed.Product,
		observed.Version,
		timestamp,
		timestamp,
		timestamp,
		timestamp,
	)
	if err != nil {
		return fmt.Errorf("upsert observed service: %w", err)
	}
	return nil
}

func (r *ServiceRepository) List(ctx context.Context) ([]Service, error) {
	rows, err := r.conn.QueryContext(ctx, `
		SELECT
			id, asset_id, display_name, ip_address, port, protocol, transport,
			service_name, product, version, url, status,
			first_seen_at, last_seen_at, created_at, updated_at
		FROM services
		ORDER BY ip_address ASC, port ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("list services: %w", err)
	}
	defer rows.Close()

	var services []Service
	for rows.Next() {
		service, err := scanService(rows)
		if err != nil {
			return nil, err
		}
		services = append(services, service)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate services: %w", err)
	}
	return services, nil
}

func (r *ServiceRepository) ListByAsset(ctx context.Context, assetID string) ([]Service, error) {
	rows, err := r.conn.QueryContext(ctx, `
		SELECT
			id, asset_id, display_name, ip_address, port, protocol, transport,
			service_name, product, version, url, status,
			first_seen_at, last_seen_at, created_at, updated_at
		FROM services
		WHERE asset_id = ?
		ORDER BY port ASC
	`, assetID)
	if err != nil {
		return nil, fmt.Errorf("list services by asset: %w", err)
	}
	defer rows.Close()

	var services []Service
	for rows.Next() {
		service, err := scanService(rows)
		if err != nil {
			return nil, err
		}
		services = append(services, service)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate services by asset: %w", err)
	}
	return services, nil
}

func (r *ServiceRepository) Count(ctx context.Context) (int, error) {
	var count int
	if err := r.conn.QueryRowContext(ctx, `SELECT COUNT(1) FROM services`).Scan(&count); err != nil {
		return 0, fmt.Errorf("count services: %w", err)
	}
	return count, nil
}

func scanService(scanner assetScanner) (Service, error) {
	var service Service
	var assetID sql.NullString
	var product sql.NullString
	var version sql.NullString
	var url sql.NullString
	var firstSeen sql.NullString
	var lastSeen sql.NullString
	var createdAt sql.NullString
	var updatedAt sql.NullString

	err := scanner.Scan(
		&service.ID,
		&assetID,
		&service.DisplayName,
		&service.IPAddress,
		&service.Port,
		&service.Protocol,
		&service.Transport,
		&service.ServiceName,
		&product,
		&version,
		&url,
		&service.Status,
		&firstSeen,
		&lastSeen,
		&createdAt,
		&updatedAt,
	)
	if err != nil {
		return Service{}, err
	}

	service.AssetID = assetID.String
	service.Product = product.String
	service.Version = version.String
	service.URL = url.String

	service.FirstSeenAt, err = parseNullableTime(firstSeen)
	if err != nil {
		return Service{}, err
	}
	service.LastSeenAt, err = parseNullableTime(lastSeen)
	if err != nil {
		return Service{}, err
	}
	service.CreatedAt, err = parseNullableTime(createdAt)
	if err != nil {
		return Service{}, err
	}
	service.UpdatedAt, err = parseNullableTime(updatedAt)
	if err != nil {
		return Service{}, err
	}

	return service, nil
}
