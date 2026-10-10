package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	apperrors "github.com/fledger/fledger-dunning/internal/platform/errors"
	"github.com/fledger/fledger-dunning/internal/domain"
)

type ConfigRepo struct {
	pool *pgxpool.Pool
}

func NewConfigRepo(p *pgxpool.Pool) *ConfigRepo { return &ConfigRepo{pool: p} }

const cfgColumns = `id, tenant_id, cadence_days, wa_provider, statement_day_of_month,
	jitter_min_seconds, jitter_max_seconds, auto_cancel_on_payment,
	created_at, updated_at`

func scanConfig(row pgx.Row) (domain.DunningConfig, error) {
	var c domain.DunningConfig
	var cadenceBytes []byte
	if err := row.Scan(
		&c.ID, &c.TenantID, &cadenceBytes, &c.WAProvider, &c.StatementDayOfMonth,
		&c.JitterMinSeconds, &c.JitterMaxSeconds, &c.AutoCancelOnPayment,
		&c.CreatedAt, &c.UpdatedAt,
	); err != nil {
		return domain.DunningConfig{}, err
	}
	if len(cadenceBytes) > 0 {
		_ = json.Unmarshal(cadenceBytes, &c.CadenceDays)
	}
	if c.CadenceDays == nil {
		c.CadenceDays = domain.DefaultCadence
	}
	return c, nil
}

// Get returns the per-tenant config (creates the default if missing).
func (r *ConfigRepo) Get(ctx context.Context, tenantID string) (domain.DunningConfig, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT `+cfgColumns+` FROM dunning_configurations WHERE tenant_id = $1`, tenantID)
	c, err := scanConfig(row)
	if err == nil {
		return c, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return domain.DunningConfig{}, err
	}
	// Bootstrap a default config row.
	ins, ierr := r.Insert(ctx, tenantID, domain.DefaultCadence, "MOCK", 1, 3, 8, true)
	if ierr != nil {
		return domain.DunningConfig{}, ierr
	}
	return ins, nil
}

// Insert creates a new config (or updates via ON CONFLICT).
func (r *ConfigRepo) Insert(ctx context.Context, tenantID string, cadence []int, waProvider string, stmtDay, jitterMin, jitterMax int, autoCancel bool) (domain.DunningConfig, error) {
	if len(cadence) == 0 {
		cadence = domain.DefaultCadence
	}
	cadenceJSON, _ := json.Marshal(cadence)
	row := r.pool.QueryRow(ctx, `
		INSERT INTO dunning_configurations
		  (tenant_id, cadence_days, wa_provider, statement_day_of_month, jitter_min_seconds, jitter_max_seconds, auto_cancel_on_payment)
		VALUES ($1, $2::jsonb, $3, $4, $5, $6, $7)
		ON CONFLICT (tenant_id) DO UPDATE
		  SET cadence_days = EXCLUDED.cadence_days,
		      wa_provider = EXCLUDED.wa_provider,
		      statement_day_of_month = EXCLUDED.statement_day_of_month,
		      jitter_min_seconds = EXCLUDED.jitter_min_seconds,
		      jitter_max_seconds = EXCLUDED.jitter_max_seconds,
		      auto_cancel_on_payment = EXCLUDED.auto_cancel_on_payment,
		      updated_at = NOW()
		RETURNING `+cfgColumns,
		tenantID, cadenceJSON, waProvider, stmtDay, jitterMin, jitterMax, autoCancel,
	)
	c, err := scanConfig(row)
	if err != nil {
		return domain.DunningConfig{}, fmt.Errorf("insert config: %w", err)
	}
	return c, nil
}

// SessionRepo persists WhatsApp session state.
type SessionRepo struct {
	pool *pgxpool.Pool
}

func NewSessionRepo(p *pgxpool.Pool) *SessionRepo { return &SessionRepo{pool: p} }

const sessColumns = `id, tenant_id, session_name, connection_status, COALESCE(qr_code_data,''),
	COALESCE(phone_connected,''), last_heartbeat, updated_at`

func scanSession(row pgx.Row) (domain.Session, error) {
	var s domain.Session
	var status string
	if err := row.Scan(
		&s.ID, &s.TenantID, &s.SessionName, &status, &s.QRCodeData, &s.PhoneConnected,
		&s.LastHeartbeat, &s.UpdatedAt,
	); err != nil {
		return domain.Session{}, err
	}
	s.ConnectionStatus = domain.SessionStatus(status)
	return s, nil
}

// Get returns the current session for a tenant (or DISCONNECTED if none).
func (r *SessionRepo) Get(ctx context.Context, tenantID string) (domain.Session, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT `+sessColumns+` FROM dunning_whatsapp_sessions WHERE tenant_id = $1`, tenantID)
	s, err := scanSession(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			now := time.Now().UTC()
			return domain.Session{
				TenantID:         tenantID,
				SessionName:      "default",
				ConnectionStatus: domain.SessionDisconnected,
				UpdatedAt:        now,
			}, nil
		}
		return domain.Session{}, err
	}
	return s, nil
}

// Upsert persists a new session state (or updates the existing one).
func (r *SessionRepo) Upsert(ctx context.Context, tenantID, name, status, qrData, phone string) (domain.Session, error) {
	row := r.pool.QueryRow(ctx, `
		INSERT INTO dunning_whatsapp_sessions
		  (tenant_id, session_name, connection_status, qr_code_data, phone_connected, last_heartbeat)
		VALUES ($1, $2, $3, NULLIF($4,''), NULLIF($5,''), NOW())
		ON CONFLICT (tenant_id) DO UPDATE
		  SET session_name = EXCLUDED.session_name,
		      connection_status = EXCLUDED.connection_status,
		      qr_code_data = EXCLUDED.qr_code_data,
		      phone_connected = EXCLUDED.phone_connected,
		      last_heartbeat = NOW(),
		      updated_at = NOW()
		RETURNING `+sessColumns,
		tenantID, name, status, qrData, phone,
	)
	return scanSession(row)
}

// StoreContactRepo holds the master kontak WhatsApp toko.
type StoreContactRepo struct {
	pool *pgxpool.Pool
}

func NewStoreContactRepo(p *pgxpool.Pool) *StoreContactRepo { return &StoreContactRepo{pool: p} }

const contactColumns = `id, tenant_id, store_id, store_name, owner_name, phone_number,
	COALESCE(email,''), is_active, opt_out, created_at, updated_at`

func scanContact(row pgx.Row) (domain.StoreContact, error) {
	var c domain.StoreContact
	if err := row.Scan(
		&c.ID, &c.TenantID, &c.StoreID, &c.StoreName, &c.OwnerName, &c.Phone,
		&c.Email, &c.IsActive, &c.OptOut, &c.CreatedAt, &c.UpdatedAt,
	); err != nil {
		return domain.StoreContact{}, err
	}
	return c, nil
}

func (r *StoreContactRepo) Insert(ctx context.Context, c domain.StoreContact) (domain.StoreContact, error) {
	row := r.pool.QueryRow(ctx, `
		INSERT INTO dunning_store_contacts
		  (tenant_id, store_id, store_name, owner_name, phone_number, email, is_active, opt_out)
		VALUES ($1,$2,$3,$4,$5,NULLIF($6,''),$7,$8)
		ON CONFLICT (tenant_id, store_id) DO UPDATE
		  SET store_name = EXCLUDED.store_name,
		      owner_name = EXCLUDED.owner_name,
		      phone_number = EXCLUDED.phone_number,
		      email = EXCLUDED.email,
		      is_active = EXCLUDED.is_active,
		      opt_out = EXCLUDED.opt_out,
		      updated_at = NOW()
		RETURNING `+contactColumns,
		c.TenantID, c.StoreID, c.StoreName, c.OwnerName, c.Phone, c.Email,
		c.IsActive, c.OptOut,
	)
	out, err := scanContact(row)
	if err != nil {
		if isUniqueViolation(err) {
			return domain.StoreContact{}, fmt.Errorf("%w: store_id already exists", apperrors.ErrConflict)
		}
		return domain.StoreContact{}, fmt.Errorf("insert contact: %w", err)
	}
	return out, nil
}

func (r *StoreContactRepo) GetByStoreID(ctx context.Context, tenantID, storeID string) (domain.StoreContact, error) {
	row := r.pool.QueryRow(ctx, `SELECT `+contactColumns+` FROM dunning_store_contacts WHERE tenant_id = $1::uuid AND store_id = $2`, tenantID, storeID)
	c, err := scanContact(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.StoreContact{}, fmt.Errorf("%w: contact for store %s", apperrors.ErrNotFound, storeID)
		}
		return domain.StoreContact{}, err
	}
	return c, nil
}

func (r *StoreContactRepo) List(ctx context.Context, tenantID string) ([]domain.StoreContact, error) {
	rows, err := r.pool.Query(ctx, `SELECT `+contactColumns+` FROM dunning_store_contacts WHERE tenant_id = $1::uuid ORDER BY store_id`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]domain.StoreContact, 0, 4)
	for rows.Next() {
		c, err := scanContact(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}