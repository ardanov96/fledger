package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	apperrors "github.com/fledger/fledger-dunning/internal/platform/errors"
	"github.com/fledger/fledger-dunning/internal/domain"
)

type StatementRepo struct {
	pool *pgxpool.Pool
}

func NewStatementRepo(p *pgxpool.Pool) *StatementRepo { return &StatementRepo{pool: p} }

const stColumns = `id, tenant_id, store_id, statement_month, total_invoiced_minor, total_paid_minor,
	total_returned_minor, closing_balance_minor, pdf_file_path, phone_number,
	dispatch_status, sent_at, COALESCE(failure_reason,''), created_at`

func scanStatement(row pgx.Row) (domain.Statement, error) {
	var s domain.Statement
	var status string
	if err := row.Scan(
		&s.ID, &s.TenantID, &s.StoreID, &s.StatementMonth, &s.TotalInvoicedMinor,
		&s.TotalPaidMinor, &s.TotalReturnedMinor, &s.ClosingBalanceMinor,
		&s.PDFFilePath, &s.PhoneNumber, &status, &s.SentAt, &s.FailureReason, &s.CreatedAt,
	); err != nil {
		return domain.Statement{}, err
	}
	s.DispatchStatus = domain.StatementDispatchStatus(status)
	return s, nil
}

// Insert creates a new statement row (idempotent via UNIQUE on store+month).
func (r *StatementRepo) Insert(ctx context.Context, s domain.Statement) (domain.Statement, error) {
	row := r.pool.QueryRow(ctx, `
		INSERT INTO dunning_statements
		  (tenant_id, store_id, statement_month, total_invoiced_minor, total_paid_minor,
		   total_returned_minor, closing_balance_minor, pdf_file_path, phone_number,
		   dispatch_status)
		VALUES ($1::uuid,$2,$3,$4,$5,$6,$7,$8,$9,'GENERATED')
		ON CONFLICT (tenant_id, store_id, statement_month) DO UPDATE
		  SET total_invoiced_minor = EXCLUDED.total_invoiced_minor,
		      total_paid_minor = EXCLUDED.total_paid_minor,
		      total_returned_minor = EXCLUDED.total_returned_minor,
		      closing_balance_minor = EXCLUDED.closing_balance_minor,
		      pdf_file_path = EXCLUDED.pdf_file_path,
		      phone_number = EXCLUDED.phone_number,
		      dispatch_status = 'GENERATED',
		      created_at = NOW()
		RETURNING `+stColumns,
		s.TenantID, s.StoreID, s.StatementMonth, s.TotalInvoicedMinor, s.TotalPaidMinor,
		s.TotalReturnedMinor, s.ClosingBalanceMinor, s.PDFFilePath, s.PhoneNumber,
	)
	out, err := scanStatement(row)
	if err != nil {
		return domain.Statement{}, fmt.Errorf("insert statement: %w", err)
	}
	return out, nil
}

// MarkSent updates dispatch_status to SENT with sent_at.
func (r *StatementRepo) MarkSent(ctx context.Context, id string) error {
	_, err := r.pool.Exec(ctx, `UPDATE dunning_statements SET dispatch_status='SENT', sent_at=NOW() WHERE id=$1::uuid`, id)
	return err
}

// MarkFailed stores the failure_reason.
func (r *StatementRepo) MarkFailed(ctx context.Context, id, reason string) error {
	_, err := r.pool.Exec(ctx, `UPDATE dunning_statements SET dispatch_status='FAILED', failure_reason=$2 WHERE id=$1::uuid`, id, reason)
	return err
}

// Get fetches a single statement.
func (r *StatementRepo) Get(ctx context.Context, tenantID, storeID, month string) (domain.Statement, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT `+stColumns+` FROM dunning_statements WHERE tenant_id = $1::uuid AND store_id = $2 AND statement_month = $3`,
		tenantID, storeID, month)
	s, err := scanStatement(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Statement{}, fmt.Errorf("%w: statement for %s/%s", apperrors.ErrNotFound, storeID, month)
		}
		return domain.Statement{}, err
	}
	return s, nil
}

// ListByTenant returns recent statements.
func (r *StatementRepo) ListByTenant(ctx context.Context, tenantID string, limit int) ([]domain.Statement, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := r.pool.Query(ctx,
		`SELECT `+stColumns+` FROM dunning_statements WHERE tenant_id = $1::uuid ORDER BY created_at DESC LIMIT $2`,
		tenantID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]domain.Statement, 0, 4)
	for rows.Next() {
		v, err := scanStatement(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

// MessageLogRepo persists outbound/inbound message log rows.
type MessageLogRepo struct {
	pool *pgxpool.Pool
}

func NewMessageLogRepo(p *pgxpool.Pool) *MessageLogRepo { return &MessageLogRepo{pool: p} }

const logColumns = `id, tenant_id, queue_id::text, direction, phone_number, message_type,
	COALESCE(provider_message_id,''), status, payload, created_at`

func scanLog(row pgx.Row) (domain.MessageLog, error) {
	var l domain.MessageLog
	var dir, mtype, status string
	var qid *string
	var payloadBytes []byte
	if err := row.Scan(
		&l.ID, &l.TenantID, &qid, &dir, &l.PhoneNumber, &mtype,
		&l.ProviderMessageID, &status, &payloadBytes, &l.CreatedAt,
	); err != nil {
		return domain.MessageLog{}, err
	}
	l.Direction = domain.MessageDirection(dir)
	l.MessageType = domain.MessageType(mtype)
	l.Status = domain.MessageStatus(status)
	l.QueueID = qid
	if len(payloadBytes) > 0 {
		_ = jsonUnmarshal(payloadBytes, &l.Payload)
	}
	return l, nil
}

// Insert creates a log row.
func (r *MessageLogRepo) Insert(ctx context.Context, l domain.MessageLog) (domain.MessageLog, error) {
	payload := l.Payload
	if payload == nil {
		payload = map[string]any{}
	}
	raw, _ := jsonMarshal(payload)
	row := r.pool.QueryRow(ctx, `
		INSERT INTO dunning_message_logs
		  (tenant_id, queue_id, direction, phone_number, message_type, provider_message_id, status, payload)
		VALUES ($1::uuid, NULLIF($2,'')::uuid, $3, $4, $5, NULLIF($6,''), $7, $8::jsonb)
		RETURNING `+logColumns,
		l.TenantID, derefStr(l.QueueID), string(l.Direction),
		l.PhoneNumber, string(l.MessageType), l.ProviderMessageID, string(l.Status), raw,
	)
	out, err := scanLog(row)
	if err != nil {
		return domain.MessageLog{}, fmt.Errorf("insert msg log: %w", err)
	}
	return out, nil
}

// ListByTenant returns recent logs.
func (r *MessageLogRepo) ListByTenant(ctx context.Context, tenantID string, limit int) ([]domain.MessageLog, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := r.pool.Query(ctx,
		`SELECT `+logColumns+` FROM dunning_message_logs WHERE tenant_id = $1::uuid ORDER BY created_at DESC LIMIT $2`,
		tenantID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]domain.MessageLog, 0, 8)
	for rows.Next() {
		v, err := scanLog(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

// AuditRepo persists operator audit rows.
type AuditRepo struct {
	pool *pgxpool.Pool
}

func NewAuditRepo(p *pgxpool.Pool) *AuditRepo { return &AuditRepo{pool: p} }

// Append writes one audit row.
func (r *AuditRepo) Append(ctx context.Context, tenantID, actorID, action, resourceID, ip string, details map[string]any) error {
	raw, _ := jsonMarshal(details)
	var ipArg any
	if ip != "" {
		ipArg = ip
	}
	_, err := r.pool.Exec(ctx, `
		INSERT INTO dunning_audit_logs
		  (tenant_id, actor_id, action, resource_id, details, ip_address)
		VALUES ($1::uuid, $2, $3, $4, $5::jsonb, $6)`,
		tenantID, actorID, action, resourceID, raw, ipArg)
	return err
}

func derefStr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}