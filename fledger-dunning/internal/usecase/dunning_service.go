// Package usecase ??? WhatsApp dispatch service.
//
// The service schedules the 5-stage cadence, dispatches messages with the
// configured jitter delay, and records every outbound attempt to the
// dunning_message_logs table.
package usecase

import (
	"context"
	"fmt"
	"log/slog"
	"math/rand"
	"strings"
	"time"

	"github.com/google/uuid"

	apperrors "github.com/fledger/fledger-dunning/internal/platform/errors"
	"github.com/fledger/fledger-dunning/internal/domain"
	"github.com/fledger/fledger-dunning/internal/platform/whatsapp"
	"github.com/fledger/fledger-dunning/internal/repository/postgres"
)

// DunningService is the central orchestrator.
type DunningService struct {
	configs   *postgres.ConfigRepo
	sessions  *postgres.SessionRepo
	contacts  *postgres.StoreContactRepo
	queues    *postgres.QueueRepo
	logs      *postgres.MessageLogRepo
	audits    *postgres.AuditRepo
	providers whatsapp.Provider
	jitterMin int
	jitterMax int
	log       *slog.Logger
}

// NewDunningService wires the dispatch service.
func NewDunningService(
	configs *postgres.ConfigRepo, sessions *postgres.SessionRepo,
	contacts *postgres.StoreContactRepo, queues *postgres.QueueRepo,
	logs *postgres.MessageLogRepo, audits *postgres.AuditRepo,
	provider whatsapp.Provider, jitterMin, jitterMax int,
) *DunningService {
	if jitterMin <= 0 {
		jitterMin = 3
	}
	if jitterMax <= jitterMin {
		jitterMax = jitterMin + 5
	}
	return &DunningService{
		configs: configs, sessions: sessions, contacts: contacts,
		queues: queues, logs: logs, audits: audits,
		providers: provider, jitterMin: jitterMin, jitterMax: jitterMax,
		log:       slog.Default(),
	}
}

// SetLogger overrides the default logger.
func (s *DunningService) SetLogger(l *slog.Logger) {
	if l != nil {
		s.log = l
	}
}

// IngestInput is the body of POST /v1/dunning/queues/ingest-invoice.
type IngestInput struct {
	TenantID         string
	InvoiceID        string
	InvoiceNumber    string
	StoreID          string
	PhoneNumber      string
	DueDate          time.Time
	AmountDueMinor   int64
	PaymentLinkURL   string
	StoreName        string // optional (used to render the WhatsApp body)
}

// IngestOutput is the success response.
type IngestOutput struct {
	InvoiceID       string                   `json:"invoice_id"`
	SchedulesCreated []CreatedSchedule       `json:"schedules_created"`
}

// CreatedSchedule describes one persisted queue row.
type CreatedSchedule struct {
	Stage       domain.DunningStage `json:"stage"`
	ScheduledAt time.Time          `json:"scheduled_at"`
	ID          string             `json:"id"`
}

// Ingest creates 5 queue rows (PRE_DUE_H3, DUE_DATE, OVERDUE_H3, OVERDUE_H7, OVERDUE_H14)
// from a single ingested invoice. The 5 scheduled_at timestamps are computed
// from the invoice's due_date using the tenant's cadence_days.
func (s *DunningService) Ingest(ctx context.Context, in IngestInput) (*IngestOutput, error) {
	if in.TenantID == "" {
		return nil, fmt.Errorf("%w: tenant_id required", apperrors.ErrInvalidInput)
	}
	if in.InvoiceID == "" {
		return nil, fmt.Errorf("%w: invoice_id required", apperrors.ErrInvalidInput)
	}
	if in.InvoiceNumber == "" {
		return nil, fmt.Errorf("%w: invoice_number required", apperrors.ErrInvalidInput)
	}
	if in.StoreID == "" {
		return nil, fmt.Errorf("%w: store_id required", apperrors.ErrInvalidInput)
	}
	if in.PhoneNumber == "" {
		return nil, fmt.Errorf("%w: phone_number required", apperrors.ErrInvalidInput)
	}
	if in.AmountDueMinor <= 0 {
		return nil, fmt.Errorf("%w: amount_due_minor must be > 0", apperrors.ErrInvalidInput)
	}
	if in.DueDate.IsZero() {
		return nil, fmt.Errorf("%w: due_date required", apperrors.ErrInvalidInput)
	}
	if in.PaymentLinkURL == "" {
		return nil, fmt.Errorf("%w: payment_link_url required", apperrors.ErrInvalidInput)
	}

	cfg, err := s.configs.Get(ctx, in.TenantID)
	if err != nil {
		return nil, err
	}
	if len(cfg.CadenceDays) == 0 {
		cfg.CadenceDays = domain.DefaultCadence
	}

	// Derive the 5 stage names from cadence. Pad/truncate to 5 entries.
	stages := stagesFromCadence(cfg.CadenceDays)

	storeName := in.StoreName
	if storeName == "" {
		storeName = in.StoreID
	}

	out := &IngestOutput{InvoiceID: in.InvoiceID, SchedulesCreated: make([]CreatedSchedule, 0, len(stages))}
	for _, st := range stages {
		scheduled := in.DueDate.AddDate(0, 0, st.OffsetDays).Truncate(24 * time.Hour).Add(9 * time.Hour)
		// If the BASE due_date is already in the past, skip pre-due reminders
		// (too late) but bump due/overdue stages to fire NOW.
		baseDue := in.DueDate
		if st.OffsetDays >= 0 && baseDue.Before(time.Now().UTC()) {
			scheduled = time.Now().UTC()
		} else if scheduled.Before(time.Now().UTC()) {
			scheduled = time.Now().UTC().Add(5 * time.Minute)
		}
		body := paymentMessage(stageToDunningStage(st.Name), storeName, in.InvoiceNumber, in.AmountDueMinor, in.DueDate, in.PaymentLinkURL)
		q := domain.QueueItem{
			TenantID:       in.TenantID,
			InvoiceID:     in.InvoiceID,
			InvoiceNumber: in.InvoiceNumber,
			StoreID:       in.StoreID,
			PhoneNumber:   in.PhoneNumber,
			Stage:         stageToDunningStage(st.Name),
			DueDate:       in.DueDate,
			AmountDueMinor: in.AmountDueMinor,
			PaymentLinkURL: in.PaymentLinkURL,
			MessageBody:   body,
			Status:        domain.StatusQueued,
			ScheduledAt:   scheduled,
		}
		saved, err := s.queues.Insert(ctx, q)
		if err != nil {
			return nil, fmt.Errorf("insert queue stage %s: %w", st.Name, err)
		}
		out.SchedulesCreated = append(out.SchedulesCreated, CreatedSchedule{
			Stage:       saved.Stage,
			ScheduledAt: saved.ScheduledAt,
			ID:          saved.ID,
		})
	}

	_ = s.audits.Append(ctx, in.TenantID, "system", "INGEST_INVOICE", in.InvoiceID, "",
		map[string]any{
			"invoice_number":  in.InvoiceNumber,
			"amount_due_minor": in.AmountDueMinor,
			"due_date":         in.DueDate.Format("2006-01-02"),
			"schedules":       len(out.SchedulesCreated),
		})
	s.log.Info("dunning.ingest",
		"tenant", in.TenantID,
		"invoice", in.InvoiceID,
		"schedules", len(out.SchedulesCreated),
	)
	return out, nil
}

// CancelByInvoiceInput is the body of POST /v1/dunning/queues/cancel-invoice.
type CancelByInvoiceInput struct {
	TenantID  string
	InvoiceID string
	ActorID   string
	IP        string
}

// CancelByInvoice marks every QUEUED row for the invoice as CANCELLED_BY_PAYMENT
// (atomic transaction) and sends a thank-you WhatsApp message.
func (s *DunningService) CancelByInvoice(ctx context.Context, in CancelByInvoiceInput) (int, error) {
	if in.TenantID == "" || in.InvoiceID == "" {
		return 0, fmt.Errorf("%w: tenant_id and invoice_id required", apperrors.ErrInvalidInput)
	}
	count, err := s.queues.CancelByInvoice(ctx, in.TenantID, in.InvoiceID)
	if err != nil {
		return 0, err
	}
	_ = s.audits.Append(ctx, in.TenantID, in.ActorID, "CANCEL_BY_INVOICE", in.InvoiceID, in.IP,
		map[string]any{"cancelled_count": count})

	// Send a thank-you via the configured provider (best-effort, no DB write
	// failure).
	if count > 0 {
		// We need the contact phone; fall back to the most-recent cancelled row.
		rows, lerr := s.queues.ListByTenant(ctx, in.TenantID, string(domain.StatusCancelledByPayment), 1)
		if lerr == nil && len(rows) > 0 && rows[0].InvoiceID == in.InvoiceID {
			msg := paidConfirmation(rows[0].StoreID, rows[0].InvoiceNumber, rows[0].AmountDueMinor)
			_ = s.dispatchText(ctx, rows[0].TenantID, rows[0].PhoneNumber, msg, nil)
		}
	}
	return count, nil
}

// DispatchDue loops over due queue rows, sends them with jitter, and updates
// their status. The function is used by the cron-run endpoint and the
// background worker.
func (s *DunningService) DispatchDue(ctx context.Context, batch int) (sent, failed int, err error) {
	if batch <= 0 {
		batch = 8
	}
	due, err := s.queues.FetchDue(ctx, batch)
	if err != nil {
		return 0, 0, err
	}
	for _, q := range due {
		jitter := time.Duration(s.jitterMin+rand.Intn(s.jitterMax-s.jitterMin+1)) * time.Second
		time.Sleep(jitter)
		if err := s.dispatchOne(ctx, q); err != nil {
			failed++
			_ = s.queues.MarkFailed(ctx, q.ID, err.Error(), 5*time.Minute)
			s.log.Warn("dunning.dispatch.failed", "queue_id", q.ID, "err", err.Error())
			continue
		}
		_ = s.queues.MarkSent(ctx, q.ID)
		_ = s.audits.Append(ctx, q.TenantID, "system", "DISPATCH_SENT", q.ID, "",
			map[string]any{"stage": string(q.Stage), "jitter_seconds": int(jitter.Seconds())})
		sent++
	}
	return sent, failed, nil
}

// dispatchOne sends a single queue row to the WhatsApp provider and records
// the outbound message in the log table.
func (s *DunningService) dispatchOne(ctx context.Context, q domain.QueueItem) error {
	if s.providers == nil {
		return fmt.Errorf("no WhatsApp provider configured")
	}
	qid := q.ID
	pid, err := s.providers.SendTextMessage(ctx, whatsapp.MessageRequest{
		TenantID:   q.TenantID,
		PhoneNumber: q.PhoneNumber,
		Body:        q.MessageBody,
		QueueID:     &qid,
	})
	if err != nil {
		return err
	}
	if _, err := s.logs.Insert(ctx, domain.MessageLog{
		TenantID:         q.TenantID,
		QueueID:          &qid,
		Direction:        domain.DirOutbound,
		PhoneNumber:      q.PhoneNumber,
		MessageType:      domain.MsgText,
		ProviderMessageID: pid,
		Status:           domain.MsgStatusSent,
		Payload: map[string]any{
			"stage":   string(q.Stage),
			"invoice": q.InvoiceNumber,
		},
	}); err != nil {
		s.log.Warn("dunning.dispatch.log_insert_failed", "err", err.Error())
	}
	return nil
}

// dispatchText is a fire-and-forget helper for thank-you / system messages.
func (s *DunningService) dispatchText(ctx context.Context, tenantID, phone, body string, queueID *string) error {
	if s.providers == nil {
		return fmt.Errorf("no WhatsApp provider configured")
	}
	pid, err := s.providers.SendTextMessage(ctx, whatsapp.MessageRequest{
		TenantID:   tenantID,
		PhoneNumber: phone,
		Body:        body,
		QueueID:     queueID,
	})
	if err != nil {
		return err
	}
	qid := ""
	if queueID != nil {
		qid = *queueID
	}
	_, _ = s.logs.Insert(ctx, domain.MessageLog{
		TenantID:         tenantID,
		QueueID:          nilStr(qid),
		Direction:        domain.DirOutbound,
		PhoneNumber:      phone,
		MessageType:      domain.MsgText,
		ProviderMessageID: pid,
		Status:           domain.MsgStatusSent,
		Payload:          map[string]any{"context": "system_thank_you"},
	})
	return nil
}

func nilStr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// DrainLoop runs DispatchDue on a ticker.
func (s *DunningService) DrainLoop(ctx context.Context, interval time.Duration, batch int) {
	if interval <= 0 {
		interval = 5 * time.Second
	}
	t := time.NewTicker(interval)
	defer t.Stop()
	_, _, _ = s.DispatchDue(ctx, batch)
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			sent, failed, err := s.DispatchDue(ctx, batch)
			if err != nil {
				s.log.Error("dunning.drain", "err", err.Error())
				continue
			}
			if sent > 0 || failed > 0 {
				s.log.Info("dunning.drain.tick", "sent", sent, "failed", failed)
			}
		}
	}
}

// SendTestMessage lets the operator validate the gateway from the UI.
func (s *DunningService) SendTestMessage(ctx context.Context, tenantID, phone, body string) (string, error) {
	pid, err := s.providers.SendTextMessage(ctx, whatsapp.MessageRequest{
		TenantID:   tenantID,
		PhoneNumber: phone,
		Body:        body,
	})
	if err != nil {
		return "", err
	}
	_, _ = s.logs.Insert(ctx, domain.MessageLog{
		TenantID:         tenantID,
		Direction:        domain.DirOutbound,
		PhoneNumber:      phone,
		MessageType:      domain.MsgText,
		ProviderMessageID: pid,
		Status:           domain.MsgStatusSent,
		Payload:          map[string]any{"context": "operator_test"},
	})
	return pid, nil
}

// QueueCounts returns dashboard counts.
func (s *DunningService) QueueCounts(ctx context.Context, tenantID string) (postgres.QueueCounts, error) {
	return s.queues.CountByStatus(ctx, tenantID)
}

// ListQueues returns recent queue items.
func (s *DunningService) ListQueues(ctx context.Context, tenantID, status string, limit int) ([]domain.QueueItem, error) {
	return s.queues.ListByTenant(ctx, tenantID, status, limit)
}

// GetQueueByID returns a specific queue item by ID.
func (s *DunningService) GetQueueByID(ctx context.Context, tenantID, id string) (domain.QueueItem, error) {
	return s.queues.GetByID(ctx, tenantID, id)
}

// DispatchOneByID forces immediate delivery of a specific queue row by ID.
func (s *DunningService) DispatchOneByID(ctx context.Context, tenantID, id string) error {
	q, err := s.queues.GetByID(ctx, tenantID, id)
	if err != nil {
		return err
	}
	if q.Status == domain.StatusSent {
		return fmt.Errorf("%w: message already sent", apperrors.ErrConflict)
	}
	if q.Status == domain.StatusCancelledByPayment {
		return fmt.Errorf("%w: queue already cancelled by payment", apperrors.ErrConflict)
	}
	if err := s.queues.MarkProcessing(ctx, tenantID, id); err != nil {
		return fmt.Errorf("mark processing: %w", err)
	}
	if err := s.dispatchOne(ctx, q); err != nil {
		_ = s.queues.MarkFailed(ctx, q.ID, err.Error(), 5*time.Minute)
		return fmt.Errorf("dispatch message: %w", err)
	}
	_ = s.queues.MarkSent(ctx, q.ID)
	_ = s.audits.Append(ctx, q.TenantID, "operator", "DISPATCH_SENT_MANUAL", q.ID, "",
		map[string]any{"stage": string(q.Stage), "manual": true})
	return nil
}

// Contacts exposes the contact repo for the handler.
func (s *DunningService) Contacts() *postgres.StoreContactRepo { return s.contacts }

// SessionStatus returns the current WhatsApp session.
func (s *DunningService) SessionStatus(ctx context.Context, tenantID string) (*domain.Session, error) {
	// Fall back to provider status if DB row is missing.
	sess, err := s.sessions.Get(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	if sess.ConnectionStatus == domain.SessionDisconnected {
		provider, perr := s.providers.GetStatus(ctx, tenantID)
		if perr == nil && provider.ConnectionStatus != "" {
			sess.ConnectionStatus = provider.ConnectionStatus
		}
	}
	return &sess, nil
}

// GenerateQR pairs a new session (mock: returns fake QR).
func (s *DunningService) GenerateQR(ctx context.Context, tenantID, sessionName string) (string, error) {
	qr, err := s.providers.GenerateQR(ctx, tenantID, sessionName)
	if err != nil {
		return "", err
	}
	_, _ = s.sessions.Upsert(ctx, tenantID, sessionName, string(domain.SessionScanQR), qr, "")
	return qr, nil
}

// Disconnect marks the session as DISCONNECTED.
func (s *DunningService) Disconnect(ctx context.Context, tenantID, actorID, ip string) error {
	_, err := s.sessions.Upsert(ctx, tenantID, "default", string(domain.SessionDisconnected), "", "")
	if err != nil {
		return err
	}
	_ = s.audits.Append(ctx, tenantID, actorID, "DISCONNECT_WA", tenantID, ip, nil)
	return nil
}

// GeneratePaymentLink returns the canonical deep-link used in messages.
func GeneratePaymentLink(payBaseURL, invoiceID string) string {
	if payBaseURL == "" {
		payBaseURL = "http://localhost:8083"
	}
	payBaseURL = strings.TrimRight(payBaseURL, "/")
	return fmt.Sprintf("%s/pay/%s", payBaseURL, invoiceID)
}

// stageDescriptor describes the 5 (or N) cadence stages.
type stageDescriptor struct {
	Name       string
	OffsetDays int
}

// stagesFromCadence returns stage descriptors in the order they fire.
// Names are derived from the offset: -3 PRE_DUE_H3, 0 DUE_DATE, +3/+7/+14
// OVERDUE_H{N}. Any extra offset gets a generic OVERDUE label.
func stagesFromCadence(cadence []int) []stageDescriptor {
	out := make([]stageDescriptor, 0, len(cadence))
	for _, d := range cadence {
		switch d {
		case -3:
			out = append(out, stageDescriptor{Name: "PRE_DUE_H3", OffsetDays: d})
		case 0:
			out = append(out, stageDescriptor{Name: "DUE_DATE", OffsetDays: d})
		case 3:
			out = append(out, stageDescriptor{Name: "OVERDUE_H3", OffsetDays: d})
		case 7:
			out = append(out, stageDescriptor{Name: "OVERDUE_H7", OffsetDays: d})
		case 14:
			out = append(out, stageDescriptor{Name: "OVERDUE_H14", OffsetDays: d})
		default:
			out = append(out, stageDescriptor{Name: fmt.Sprintf("OFFSET_%dD", d), OffsetDays: d})
		}
	}
	return out
}

func stageToDunningStage(name string) domain.DunningStage {
	switch name {
	case "PRE_DUE_H3":
		return domain.StagePreDueH3
	case "DUE_DATE":
		return domain.StageDueDate
	case "OVERDUE_H3":
		return domain.StageOverdueH3
	case "OVERDUE_H7":
		return domain.StageOverdueH7
	case "OVERDUE_H14":
		return domain.StageOverdueH14
	}
	if strings.HasPrefix(name, "OFFSET_") {
		return domain.StageManual
	}
	return domain.StageManual
}

// uuidPtr is a tiny helper used in tests and serialization.
func _unused_uuidPtr() *uuid.UUID { u := uuid.New(); return &u }