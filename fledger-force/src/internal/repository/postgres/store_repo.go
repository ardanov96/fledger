package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	apperrors "github.com/fledger/fledger-force/internal/platform/errors"
	"github.com/fledger/fledger-force/internal/domain/store"
)

type StoreRepo struct {
	pool *pgxpool.Pool
}

func NewStoreRepo(p *pgxpool.Pool) *StoreRepo { return &StoreRepo{pool: p} }

const storeColumns = `id, tenant_id, store_code, name, COALESCE(owner_name,''),
	COALESCE(phone,''), address, latitude, longitude, geofence_radius_meters,
	tier, credit_limit, status, created_at, updated_at`

func scanStore(row pgx.Row) (store.Store, error) {
	var s store.Store
	var tier string
	if err := row.Scan(
		&s.ID, &s.TenantID, &s.StoreCode, &s.Name, &s.OwnerName, &s.Phone,
		&s.Address, &s.Latitude, &s.Longitude, &s.GeofenceRadiusMeters,
		&tier, &s.CreditLimit, &s.Status, &s.CreatedAt, &s.UpdatedAt,
	); err != nil {
		return store.Store{}, err
	}
	s.Tier = store.Tier(tier)
	return s, nil
}

func (r *StoreRepo) Insert(ctx context.Context, s store.Store) (store.Store, error) {
	if s.GeofenceRadiusMeters <= 0 {
		s.GeofenceRadiusMeters = 100
	}
	if s.Tier == "" {
		s.Tier = store.TierRetail
	}
	row := r.pool.QueryRow(ctx, `
		INSERT INTO force_stores
		  (tenant_id, store_code, name, owner_name, phone, address, latitude,
		   longitude, geofence_radius_meters, tier, credit_limit, status)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,'ACTIVE')
		RETURNING `+storeColumns,
		s.TenantID, s.StoreCode, s.Name, s.OwnerName, s.Phone, s.Address,
		s.Latitude, s.Longitude, s.GeofenceRadiusMeters, string(s.Tier), s.CreditLimit,
	)
	out, err := scanStore(row)
	if err != nil {
		if isUniqueViolation(err) {
			return store.Store{}, fmt.Errorf("%w: store_code already exists", apperrors.ErrConflict)
		}
		return store.Store{}, fmt.Errorf("insert store: %w", err)
	}
	return out, nil
}

func (r *StoreRepo) Get(ctx context.Context, tenantID, id string) (store.Store, error) {
	row := r.pool.QueryRow(ctx, `SELECT `+storeColumns+` FROM force_stores WHERE id = $1 AND tenant_id = $2`, id, tenantID)
	s, err := scanStore(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return store.Store{}, fmt.Errorf("%w: store %s", apperrors.ErrNotFound, id)
		}
		return store.Store{}, err
	}
	return s, nil
}

func (r *StoreRepo) List(ctx context.Context, tenantID, tier string) ([]store.Store, error) {
	var (
		rows pgx.Rows
		err  error
	)
	if tier == "" {
		rows, err = r.pool.Query(ctx, `SELECT `+storeColumns+` FROM force_stores WHERE tenant_id = $1 ORDER BY store_code`, tenantID)
	} else {
		rows, err = r.pool.Query(ctx, `SELECT `+storeColumns+` FROM force_stores WHERE tenant_id = $1 AND tier = $2 ORDER BY store_code`, tenantID, tier)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]store.Store, 0, 8)
	for rows.Next() {
		v, err := scanStore(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}