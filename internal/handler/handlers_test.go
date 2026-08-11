//go:build !windows
// +build !windows

package handler

// Internal test (same package) so we can use DTO types from dto.go.

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/runut/fmcg-wallet/internal/domain/ledger"
	apperrors "github.com/runut/fmcg-wallet/internal/platform/errors"
	"github.com/runut/fmcg-wallet/internal/platform/httpx"
	"github.com/runut/fmcg-wallet/internal/platform/money"
)

// =============================================================================
// Mocks
// =============================================================================

type mockTransferAPI struct {
	mu       sync.Mutex
	results  map[string]ledger.Transaction
	lastCall ledger.TransferInput
}

func newMockTransferAPI() *mockTransferAPI {
	return &mockTransferAPI{
		results: make(map[string]ledger.Transaction),
	}
}

func (m *mockTransferAPI) Transfer(_ context.Context, input ledger.TransferInput) (ledger.Transaction, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.lastCall = input
	if existing, ok := m.results[input.IdempotencyKey]; ok {
		return existing, nil
	}
	txID := uuid.NewString()
	now := time.Now().UTC()
	tx := ledger.Transaction{
		ID:             txID,
		IdempotencyKey: input.IdempotencyKey,
		Status:         ledger.TransactionStatusPosted,
		PostedAt:       &now,
		TenantID:       input.InitiatorID,
		Entries: []ledger.Entry{
			{
				ID:            uuid.NewString(),
				TransactionID: txID,
				AccountID:     input.FromAccountID,
				Amount:        input.Amount,
				Type:          ledger.EntryTypeDebit,
				PeriodID:      "00000000-0000-0000-0000-000000000001",
				Currency:      money.IDR,
				CreatedAt:     now,
			},
			{
				ID:            uuid.NewString(),
				TransactionID: txID,
				AccountID:     input.ToAccountID,
				Amount:        input.Amount,
				Type:          ledger.EntryTypeCredit,
				PeriodID:      "00000000-0000-0000-0000-000000000001",
				Currency:      money.IDR,
				CreatedAt:     now,
			},
		},
	}
	m.results[input.IdempotencyKey] = tx
	return tx, nil
}

type failingTransferAPI struct {
	err error
}

func (f *failingTransferAPI) Transfer(_ context.Context, _ ledger.TransferInput) (ledger.Transaction, error) {
	return ledger.Transaction{}, f.err
}

type mockAccountAPI struct {
	mu     sync.Mutex
	byID   map[string]ledger.Account
	byCode map[string]ledger.Account
}

func newMockAccountAPI() *mockAccountAPI {
	return &mockAccountAPI{
		byID:   make(map[string]ledger.Account),
		byCode: make(map[string]ledger.Account),
	}
}

func (m *mockAccountAPI) Create(_ context.Context, a ledger.Account) (ledger.Account, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if a.ID == "" {
		a.ID = uuid.NewString()
	}
	m.byID[a.ID] = a
	m.byCode[a.Code] = a
	return a, nil
}

func (m *mockAccountAPI) GetByID(_ context.Context, id string) (ledger.Account, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	a, ok := m.byID[id]
	if !ok {
		return ledger.Account{}, apperrors.ErrAccountNotFound
	}
	return a, nil
}

func (m *mockAccountAPI) ListEntries(_ context.Context, accountID string, limit int) ([]ledger.Entry, error) {
m.mu.Lock()
defer m.mu.Unlock()
out := make([]ledger.Entry, 0)
for i := 0; i < 3; i++ {
out = append(out, ledger.Entry{
ID:            uuid.NewString(),
TransactionID: uuid.NewString(),
AccountID:     accountID,
Amount:        money.NewFromMinor(int64(1000 * (i + 1))),
Type:          ledger.EntryTypeDebit,
PeriodID:      "00000000-0000-0000-0000-000000000001",
Currency:      money.IDR,
CreatedAt:     time.Now().Add(-time.Duration(i) * time.Hour),
})
}
if limit > 0 && limit < len(out) {
out = out[:limit]
}
return out, nil
}

func (m *mockAccountAPI) List(_ context.Context, _ ledger.AccountFilter) ([]ledger.Account, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]ledger.Account, 0, len(m.byID))
	for _, a := range m.byID {
		out = append(out, a)
	}
	return out, nil
}

// =============================================================================
// Response envelope helpers
// =============================================================================
//
// All success responses are wrapped in {"data": ..., "meta": ...}.
// Tests decode via these helpers.

type envelope struct {
	Data json.RawMessage `json:"data"`
	Meta *struct {
		RequestID string `json:"request_id"`
		Timestamp string `json:"timestamp"`
	} `json:"meta,omitempty"`
}

func decodeAccount(t *testing.T, body []byte) AccountResponse {
	t.Helper()
	var env envelope
	require.NoError(t, json.Unmarshal(body, &env))
	var acc AccountResponse
	require.NoError(t, json.Unmarshal(env.Data, &acc))
	return acc
}

func decodeTransfer(t *testing.T, body []byte) TransferResponse {
	t.Helper()
	var env envelope
	require.NoError(t, json.Unmarshal(body, &env))
	var t1 TransferResponse
	require.NoError(t, json.Unmarshal(env.Data, &t1))
	return t1
}

func decodeAccountList(t *testing.T, body []byte) []AccountResponse {
	t.Helper()
	var env envelope
	require.NoError(t, json.Unmarshal(body, &env))
	var out []AccountResponse
	require.NoError(t, json.Unmarshal(env.Data, &out))
	return out
}

type errorEnvelope struct {
	Error *struct {
		Code    string         `json:"code"`
		Message string         `json:"message"`
		Details map[string]any `json:"details,omitempty"`
	} `json:"error"`
}

func assertErrorCode(t *testing.T, body []byte, expected string) {
	t.Helper()
	var env errorEnvelope
	require.NoError(t, json.Unmarshal(body, &env))
	require.NotNil(t, env.Error, "expected error envelope, got: %s", string(body))
	assert.Equal(t, expected, env.Error.Code)
}

// =============================================================================
// Tests
// =============================================================================

func setupRouter(t *testing.T, transfers TransferAPI, accounts AccountAPI) http.Handler {
	t.Helper()
	h := New(transfers, accounts)
	r := chi.NewRouter()
	r.Use(httpx.RequestIDMiddleware)
	r.Route("/v1", func(r chi.Router) {
		h.RegisterRoutes(r)
	})
	return r
}

func doRequest(t *testing.T, h http.Handler, method, path, body, idemKey string) *httptest.ResponseRecorder {
	t.Helper()
	var reqBody *strings.Reader
	if body != "" {
		reqBody = strings.NewReader(body)
	} else {
		reqBody = strings.NewReader("")
	}
	req := httptest.NewRequest(method, path, reqBody)
	if idemKey != "" {
		req.Header.Set("Idempotency-Key", idemKey)
	}
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	return rr
}

func TestCreateAccount_Success(t *testing.T) {
	t.Parallel()
	transfers := newMockTransferAPI()
	accounts := newMockAccountAPI()
	h := setupRouter(t, transfers, accounts)

	body := `{"code": "HQ-001", "name": "HQ Treasury", "type": "hq", "currency": "IDR"}`
	rr := doRequest(t, h, "POST", "/v1/accounts", body, "")
	require.Equal(t, http.StatusCreated, rr.Code)

	resp := decodeAccount(t, rr.Body.Bytes())
	assert.Equal(t, "HQ-001", resp.Code)
	assert.Equal(t, "HQ Treasury", resp.Name)
	assert.Equal(t, "hq", resp.Type)
	assert.Equal(t, "IDR", resp.Currency)
	assert.Equal(t, "active", resp.Status)
	assert.Equal(t, int64(0), resp.BalanceMinor)
}

func TestCreateAccount_ValidationErrors(t *testing.T) {
	t.Parallel()
	transfers := newMockTransferAPI()
	accounts := newMockAccountAPI()
	h := setupRouter(t, transfers, accounts)

	tests := []struct {
		name     string
		body     string
		wantCode string
	}{
		{"empty code", `{"code": "", "name": "X", "type": "hq", "currency": "IDR"}`, "VALIDATION_FAILED"},
		{"invalid type", `{"code": "A", "name": "X", "type": "unknown", "currency": "IDR"}`, "VALIDATION_FAILED"},
		{"empty currency", `{"code": "A", "name": "X", "type": "hq", "currency": ""}`, "VALIDATION_FAILED"},
		{"bad currency len", `{"code": "A", "name": "X", "type": "hq", "currency": "DOLLAR"}`, "VALIDATION_FAILED"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			rr := doRequest(t, h, "POST", "/v1/accounts", tt.body, "")
			assert.Equal(t, http.StatusBadRequest, rr.Code, "body: %s", rr.Body.String())
			assertErrorCode(t, rr.Body.Bytes(), tt.wantCode)
		})
	}
}

func TestGetAccount_Success(t *testing.T) {
	t.Parallel()
	transfers := newMockTransferAPI()
	accounts := newMockAccountAPI()
	created, _ := accounts.Create(context.Background(), ledger.Account{
		ID: uuid.NewString(), Code: "TEST", Name: "Test", Type: ledger.AccountTypeCash,
		Status: ledger.AccountStatusActive, Currency: "IDR",
	})
	actualID := created.ID

	h := setupRouter(t, transfers, accounts)
	rr := doRequest(t, h, "GET", "/v1/accounts/"+actualID, "", "")
	require.Equal(t, http.StatusOK, rr.Code)
	resp := decodeAccount(t, rr.Body.Bytes())
	assert.Equal(t, actualID, resp.ID)
	assert.Equal(t, "TEST", resp.Code)
}

func TestGetAccount_NotFound(t *testing.T) {
	t.Parallel()
	transfers := newMockTransferAPI()
	accounts := newMockAccountAPI()
	h := setupRouter(t, transfers, accounts)

	rr := doRequest(t, h, "GET", "/v1/accounts/"+uuid.NewString(), "", "")
	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestGetAccount_InvalidID(t *testing.T) {
	t.Parallel()
	transfers := newMockTransferAPI()
	accounts := newMockAccountAPI()
	h := setupRouter(t, transfers, accounts)

	rr := doRequest(t, h, "GET", "/v1/accounts/not-a-uuid", "", "")
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestCreateTransfer_Success(t *testing.T) {
	t.Parallel()
	transfers := newMockTransferAPI()
	accounts := newMockAccountAPI()
	h := setupRouter(t, transfers, accounts)

	srcID := uuid.NewString()
	dstID := uuid.NewString()
	body := `{
		"from_account_id": "` + srcID + `",
		"to_account_id": "` + dstID + `",
		"amount_minor": 100000,
		"currency": "IDR",
		"description": "Test transfer"
	}`
	rr := doRequest(t, h, "POST", "/v1/transfers", body, "key-1")
	require.Equal(t, http.StatusCreated, rr.Code, "body: %s", rr.Body.String())

	resp := decodeTransfer(t, rr.Body.Bytes())
	assert.NotEmpty(t, resp.TransactionID)
	assert.Equal(t, "posted", resp.Status)
	assert.Equal(t, int64(100000), resp.AmountMinor)
	assert.Equal(t, "IDR", resp.Currency)
	assert.Equal(t, srcID, resp.FromAccountID)
	assert.Equal(t, dstID, resp.ToAccountID)
	assert.Len(t, resp.Entries, 2)
}

func TestCreateTransfer_MissingIdempotencyKey(t *testing.T) {
	t.Parallel()
	transfers := newMockTransferAPI()
	accounts := newMockAccountAPI()
	h := setupRouter(t, transfers, accounts)

	body := `{
		"from_account_id": "` + uuid.NewString() + `",
		"to_account_id": "` + uuid.NewString() + `",
		"amount_minor": 1000,
		"currency": "IDR",
		"description": "Test"
	}`
	rr := doRequest(t, h, "POST", "/v1/transfers", body, "")
	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assertErrorCode(t, rr.Body.Bytes(), "IDEMPOTENCY_KEY_MISSING")
}

func TestCreateTransfer_InvalidAmount(t *testing.T) {
	t.Parallel()
	transfers := newMockTransferAPI()
	accounts := newMockAccountAPI()
	h := setupRouter(t, transfers, accounts)

	tests := []struct {
		name string
		body string
	}{
		{"zero amount", `{"from_account_id":"` + uuid.NewString() + `","to_account_id":"` + uuid.NewString() + `","amount_minor":0,"currency":"IDR","description":"X"}`},
		{"negative amount", `{"from_account_id":"` + uuid.NewString() + `","to_account_id":"` + uuid.NewString() + `","amount_minor":-100,"currency":"IDR","description":"X"}`},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			rr := doRequest(t, h, "POST", "/v1/transfers", tt.body, "key-bad")
			assert.Equal(t, http.StatusBadRequest, rr.Code, "body: %s", rr.Body.String())
		})
	}
}

func TestCreateTransfer_InsufficientBalance(t *testing.T) {
	t.Parallel()
	transfers := &failingTransferAPI{err: apperrors.ErrInsufficientBalance}
	accounts := newMockAccountAPI()
	h := setupRouter(t, transfers, accounts)

	body := `{
		"from_account_id": "` + uuid.NewString() + `",
		"to_account_id": "` + uuid.NewString() + `",
		"amount_minor": 100000,
		"currency": "IDR",
		"description": "Test"
	}`
	rr := doRequest(t, h, "POST", "/v1/transfers", body, "key-insufficient")
	assert.Equal(t, http.StatusUnprocessableEntity, rr.Code)
	assertErrorCode(t, rr.Body.Bytes(), "INSUFFICIENT_BALANCE")
}

func TestCreateTransfer_IdempotentReplay(t *testing.T) {
	t.Parallel()
	transfers := newMockTransferAPI()
	accounts := newMockAccountAPI()
	h := setupRouter(t, transfers, accounts)

	srcID := uuid.NewString()
	dstID := uuid.NewString()
	body := `{
		"from_account_id": "` + srcID + `",
		"to_account_id": "` + dstID + `",
		"amount_minor": 1000,
		"currency": "IDR",
		"description": "Test"
	}`

	rr1 := doRequest(t, h, "POST", "/v1/transfers", body, "idem-replay")
	require.Equal(t, http.StatusCreated, rr1.Code)
	resp1 := decodeTransfer(t, rr1.Body.Bytes())

	rr2 := doRequest(t, h, "POST", "/v1/transfers", body, "idem-replay")
	require.Equal(t, http.StatusCreated, rr2.Code)
	resp2 := decodeTransfer(t, rr2.Body.Bytes())

	assert.Equal(t, resp1.TransactionID, resp2.TransactionID)
}

func TestCreateTransfer_InvalidJSON(t *testing.T) {
	t.Parallel()
	transfers := newMockTransferAPI()
	accounts := newMockAccountAPI()
	h := setupRouter(t, transfers, accounts)

	rr := doRequest(t, h, "POST", "/v1/transfers", "not valid json", "key-bad-json")
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}
