// Package whatsapp is the provider abstraction for sending WhatsApp
// messages. The MockProvider records every send to the database so the
// system can be fully tested without scanning a real QR code.
package whatsapp

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"math/big"
	"sync"
	"time"

	"github.com/fledger/fledger-dunning/internal/domain"
)

// MessageRequest is the payload we hand to SendTextMessage.
type MessageRequest struct {
	TenantID     string
	PhoneNumber  string
	Body         string
	QueueID      *string
	IdempotencyKey string
}

// DocumentRequest is the payload for SendDocument (PDF e-statement).
type DocumentRequest struct {
	TenantID       string
	PhoneNumber    string
	FilePath       string
	FileName       string
	MimeType       string
	Caption        string
	IdempotencyKey string
}

// Provider is the abstract WhatsApp gateway.
type Provider interface {
	Name() string
	SendTextMessage(ctx context.Context, req MessageRequest) (providerMessageID string, err error)
	SendDocument(ctx context.Context, req DocumentRequest) (providerMessageID string, err error)
	GetStatus(ctx context.Context, tenantID string) (*domain.Session, error)
	GenerateQR(ctx context.Context, tenantID, sessionName string) (qrPayload string, err error)
}

// Logger is the optional logger hook the mock provider uses.
type Logger interface {
	Info(msg string, args ...any)
	Warn(msg string, args ...any)
}

// MockProvider is the default gateway used in dev / test / CI.
type MockProvider struct {
	mu     sync.Mutex
	qrByTenant map[string]string
	sessByTenant map[string]*domain.Session
	log    Logger
}

// NewMockProvider returns a fresh in-memory mock.
func NewMockProvider(log Logger) *MockProvider {
	if log == nil {
		log = slog.Default()
	}
	return &MockProvider{
		qrByTenant:   map[string]string{},
		sessByTenant: map[string]*domain.Session{},
		log:    log,
	}
}

func (m *MockProvider) Name() string { return "MOCK" }

// SendTextMessage records the message in memory and returns a fake
// provider id (random base32-style string).
func (m *MockProvider) SendTextMessage(ctx context.Context, req MessageRequest) (string, error) {
	if req.PhoneNumber == "" {
		return "", errors.New("whatsapp: phone_number required")
	}
	if req.Body == "" {
		return "", errors.New("whatsapp: body required")
	}
	id := randomMessageID()
	m.log.Info("mock.wa.send_text",
		"tenant", req.TenantID,
		"phone", req.PhoneNumber,
		"provider_id", id,
		"queue_id", derefStr(req.QueueID),
		"body_len", len(req.Body),
	)
	return id, nil
}

// SendDocument records a PDF dispatch.
func (m *MockProvider) SendDocument(ctx context.Context, req DocumentRequest) (string, error) {
	if req.PhoneNumber == "" {
		return "", errors.New("whatsapp: phone_number required")
	}
	if req.FilePath == "" {
		return "", errors.New("whatsapp: file_path required")
	}
	id := randomMessageID()
	m.log.Info("mock.wa.send_document",
		"tenant", req.TenantID,
		"phone", req.PhoneNumber,
		"file", req.FileName,
		"path", req.FilePath,
		"provider_id", id,
	)
	return id, nil
}

// GetStatus returns a fake CONNECTED session for any tenant (after pairing).
func (m *MockProvider) GetStatus(ctx context.Context, tenantID string) (*domain.Session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.sessByTenant[tenantID]
	if !ok {
		s = &domain.Session{
			ID:               fmt.Sprintf("sess-mock-%s", tenantID),
			TenantID:         tenantID,
			SessionName:      "official-distributor-wa",
			ConnectionStatus: domain.SessionConnected,
			PhoneConnected:   "6281198765432",
			UpdatedAt:        time.Now().UTC(),
		}
		m.sessByTenant[tenantID] = s
	}
	return s, nil
}

// GenerateQR returns a fake QR payload and stores it so the next status check
// returns SCAN_QR.
func (m *MockProvider) GenerateQR(ctx context.Context, tenantID, sessionName string) (string, error) {
	if sessionName == "" {
		sessionName = "default"
	}
	qr := randomQRPayload()
	m.mu.Lock()
	defer m.mu.Unlock()
	m.qrByTenant[tenantID] = qr
	m.sessByTenant[tenantID] = &domain.Session{
		ID:               fmt.Sprintf("sess-mock-%s", tenantID),
		TenantID:         tenantID,
		SessionName:      sessionName,
		ConnectionStatus: domain.SessionScanQR,
		QRCodeData:       qr,
		UpdatedAt:        time.Now().UTC(),
	}
	m.log.Info("mock.wa.qr_generated", "tenant", tenantID, "len", len(qr))
	return qr, nil
}

func derefStr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// randomMessageID returns a short random base64 string for the provider id.
func randomMessageID() string {
	var b [12]byte
	if _, err := rand.Read(b[:]); err != nil {
		return fmt.Sprintf("mid-%d", time.Now().UnixNano())
	}
	return "mid." + base64.RawURLEncoding.EncodeToString(b[:])
}

// randomQRPayload returns a fake base64 string that the UI can render as a
// QR code (any non-empty string is sufficient for the demo).
func randomQRPayload() string {
	n, _ := rand.Int(rand.Reader, big.NewInt(1<<62))
	return base64.RawURLEncoding.EncodeToString(n.Bytes())
}