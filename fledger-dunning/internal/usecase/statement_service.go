// Package usecase ??? StatementService renders + dispatches the monthly PDF
// rekening-koran to the customer.
package usecase

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math/rand"
	"time"

	"github.com/google/uuid"

	apperrors "github.com/fledger/fledger-dunning/internal/platform/errors"
	"github.com/fledger/fledger-dunning/internal/domain"
	"github.com/fledger/fledger-dunning/internal/platform/pdf"
	"github.com/fledger/fledger-dunning/internal/platform/whatsapp"
	"github.com/fledger/fledger-dunning/internal/repository/postgres"
)

// StatementService handles PDF generation + WhatsApp document dispatch.
type StatementService struct {
	statements *postgres.StatementRepo
	contacts   *postgres.StoreContactRepo
	configs    *postgres.ConfigRepo
	queues     *postgres.QueueRepo
	logs       *postgres.MessageLogRepo
	audits     *postgres.AuditRepo
	provider   whatsapp.Provider
	log        *slog.Logger
}

func NewStatementService(
	stmts *postgres.StatementRepo, contacts *postgres.StoreContactRepo,
	cfg *postgres.ConfigRepo, queues *postgres.QueueRepo,
	logs *postgres.MessageLogRepo, audits *postgres.AuditRepo,
	provider whatsapp.Provider,
) *StatementService {
	return &StatementService{
		statements: stmts, contacts: contacts, configs: cfg,
		queues: queues, logs: logs, audits: audits, provider: provider,
		log: slog.Default(),
	}
}

func (s *StatementService) SetLogger(l *slog.Logger) {
	if l != nil {
		s.log = l
	}
}

// GenerateInput is the body of POST /v1/dunning/statements/generate.
type GenerateInput struct {
	TenantID       string
	StoreID       string
	StatementMonth string // YYYY-MM
	ActorID       string
	IP            string
}

// GenerateResult is the success response.
type GenerateResult struct {
	Statement domain.Statement `json:"statement"`
}

// Generate renders the PDF for one store/month and persists the row.
func (s *StatementService) Generate(ctx context.Context, in GenerateInput) (*GenerateResult, error) {
	if in.TenantID == "" || in.StoreID == "" {
		return nil, fmt.Errorf("%w: tenant_id and store_id required", apperrors.ErrInvalidInput)
	}
	if in.StatementMonth == "" {
		in.StatementMonth = time.Now().UTC().Format("2006-01")
	}

	contact, err := s.contacts.GetByStoreID(ctx, in.TenantID, in.StoreID)
	if err != nil {
		return nil, err
	}

	// For the demo we synthesise a small set of line items. In production
	// this would call Fledger Core for the real history.
	lines := synthLineItems(in.StoreID, in.StatementMonth, contact.StoreName)
	closing := sumClosing(lines)

	st := domain.Statement{
		TenantID:            in.TenantID,
		StoreID:             in.StoreID,
		StatementMonth:      in.StatementMonth,
		TotalInvoicedMinor:  sumDebit(lines),
		TotalPaidMinor:      sumCredit(lines),
		TotalReturnedMinor:  0,
		ClosingBalanceMinor: closing,
		PhoneNumber:         contact.Phone,
		DispatchStatus:      domain.StatementGenerated,
	}
	pdfPath, err := pdf.Render(st, contact.StoreName, contact.OwnerName, contact.Phone, lines)
	if err != nil {
		return nil, fmt.Errorf("render pdf: %w", err)
	}
	st.PDFFilePath = pdfPath

	saved, err := s.statements.Insert(ctx, st)
	if err != nil {
		return nil, err
	}
	_ = s.audits.Append(ctx, in.TenantID, in.ActorID, "STATEMENT_GENERATE", saved.ID, in.IP,
		map[string]any{
			"store_id":        in.StoreID,
			"statement_month": in.StatementMonth,
			"closing_minor":   closing,
		})
	s.log.Info("statement.generated",
		"tenant", in.TenantID,
		"store", in.StoreID,
		"month", in.StatementMonth,
		"path", pdfPath,
	)
	return &GenerateResult{Statement: saved}, nil
}

// SendInput is the body of POST /v1/dunning/statements/{id}/send.
type SendInput struct {
	TenantID string
	StatementID string
	ActorID   string
	IP        string
}

// Send dispatches the PDF to the customer via WhatsApp Document API.
func (s *StatementService) Send(ctx context.Context, in SendInput) (*domain.Statement, error) {
	if in.TenantID == "" || in.StatementID == "" {
		return nil, fmt.Errorf("%w: tenant_id and statement_id required", apperrors.ErrInvalidInput)
	}
	st, err := s.statements.GetByID(ctx, in.TenantID, in.StatementID)
	if err != nil {
		return nil, err
	}

	if err := s.sendPDF(ctx, st); err != nil {
		_ = s.statements.MarkFailed(ctx, st.ID, err.Error())
		_ = s.audits.Append(ctx, in.TenantID, in.ActorID, "STATEMENT_SEND_FAILED", st.ID, in.IP, nil)
		return nil, err
	}
	if err := s.statements.MarkSent(ctx, st.ID); err != nil {
		return nil, err
	}
	_ = s.audits.Append(ctx, in.TenantID, in.ActorID, "STATEMENT_SENT", st.ID, in.IP, nil)
	out, err := s.statements.Get(ctx, in.TenantID, st.StoreID, st.StatementMonth)
	if err != nil {
		return nil, err
	}
	return &out, nil
}

// GetByID returns a single statement by ID.
func (s *StatementService) GetByID(ctx context.Context, tenantID, id string) (domain.Statement, error) {
	return s.statements.GetByID(ctx, tenantID, id)
}

// sendPDF posts the document via the WhatsApp provider.
func (s *StatementService) sendPDF(ctx context.Context, st domain.Statement) error {
	if s.provider == nil {
		return fmt.Errorf("no WhatsApp provider configured")
	}
	pid, err := s.provider.SendDocument(ctx, whatsapp.DocumentRequest{
		TenantID:    st.TenantID,
		PhoneNumber: st.PhoneNumber,
		FilePath:    st.PDFFilePath,
		FileName:    fmt.Sprintf("Rekening-Koran-%s-%s.pdf", st.StoreID, st.StatementMonth),
		MimeType:    "application/pdf",
		Caption:     fmt.Sprintf("Rekening Koran Toko %s Bulan %s ??? Saldo Akhir Rp %s", st.StoreID, st.StatementMonth, formatIDR(st.ClosingBalanceMinor)),
	})
	if err != nil {
		return err
	}
	_, _ = s.logs.Insert(ctx, domain.MessageLog{
		TenantID:         st.TenantID,
		Direction:        domain.DirOutbound,
		PhoneNumber:      st.PhoneNumber,
		MessageType:      domain.MsgDocumentPDF,
		ProviderMessageID: pid,
		Status:           domain.MsgStatusSent,
		Payload: map[string]any{
			"statement_id": st.ID,
			"path":         st.PDFFilePath,
		},
	})
	return nil
}

// List returns recent statements for a tenant.
func (s *StatementService) List(ctx context.Context, tenantID string, limit int) ([]domain.Statement, error) {
	return s.statements.ListByTenant(ctx, tenantID, limit)
}

// synthLineItems builds a deterministic small set of line items for the
// demo PDF. In production this would be replaced by an HTTP call to
// Fledger Core's /v1/aging or /v1/invoices endpoint.
func synthLineItems(storeID, month, storeName string) []domain.StatementLineItem {
	d, _ := time.Parse("2006-01", month)
	if d.IsZero() {
		d = time.Now().UTC()
	}
	rng := rand.New(rand.NewSource(int64(len(storeID) * 17)))
	var lines []domain.StatementLineItem
	balance := int64(0)
	for i := 0; i < 4; i++ {
		date := d.AddDate(0, 0, -7*(4-i))
		debit := int64(2500000 + rng.Intn(8)*500000)
		credit := int64(0)
		ref := fmt.Sprintf("INV-%s-%02d", month, i+1)
		desc := fmt.Sprintf("Faktur %s No. INV/%s/%04d", storeName, month, i+1)
		if i == 3 {
			credit = 2500000
			ref = "PAY-" + month
			desc = "Pembayaran via VA BCA"
		}
		balance += debit - credit
		lines = append(lines, domain.StatementLineItem{
			Date:         date,
			Reference:    ref,
			Description:  desc,
			DebitMinor:   debit,
			CreditMinor:  credit,
			BalanceMinor: balance,
		})
	}
	return lines
}

func sumDebit(lines []domain.StatementLineItem) int64 {
	var s int64
	for _, l := range lines {
		s += l.DebitMinor
	}
	return s
}

func sumCredit(lines []domain.StatementLineItem) int64 {
	var s int64
	for _, l := range lines {
		s += l.CreditMinor
	}
	return s
}

func sumClosing(lines []domain.StatementLineItem) int64 {
	if len(lines) == 0 {
		return 0
	}
	return lines[len(lines)-1].BalanceMinor
}

// uuidPtr is a small helper used in error wrapping.
func uuidPtr() *uuid.UUID { u := uuid.New(); return &u }

// notFound is a thin wrapper.
var notFound = errors.New("not found")