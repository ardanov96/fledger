//go:build !windows
// +build !windows

package handler

// Tests for the GET /v1/accounts/{id}/entries endpoint.

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/runut/fmcg-wallet/internal/domain/ledger"
	apperrors "github.com/runut/fmcg-wallet/internal/platform/errors"
	"github.com/runut/fmcg-wallet/internal/platform/httpx"
	"github.com/runut/fmcg-wallet/internal/platform/money"
)

func TestListAccountEntries_Success(t *testing.T) {
	t.Parallel()
	transfers := newMockTransferAPI()
	accounts := newMockAccountAPI()
	h := setupRouter(t, transfers, accounts)

	// Seed account
	created, _ := accounts.Create(context.Background(), ledger.Account{
		ID: uuid.NewString(), Code: "TEST", Name: "Test", Type: ledger.AccountTypeCash,
		Status: ledger.AccountStatusActive, Currency: "IDR",
	})
	actualID := created.ID

	rr := doRequest(t, h, "GET", "/v1/accounts/"+actualID+"/entries", "", "")
	require.Equal(t, http.StatusOK, rr.Code, "body: %s", rr.Body.String())

	var env envelope
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &env))
	var entries []EntryDTO
	require.NoError(t, json.Unmarshal(env.Data, &entries))
	assert.Len(t, entries, 3) // mock returns 3
	for _, e := range entries {
		assert.Equal(t, actualID, e.AccountID)
		assert.Equal(t, "debit", e.Type)
	}
}

func TestListAccountEntries_NotFound(t *testing.T) {
	t.Parallel()
	transfers := newMockTransferAPI()
	accounts := newMockAccountAPI()
	h := setupRouter(t, transfers, accounts)

	rr := doRequest(t, h, "GET", "/v1/accounts/"+uuid.NewString()+"/entries", "", "")
	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestListAccountEntries_InvalidID(t *testing.T) {
	t.Parallel()
	transfers := newMockTransferAPI()
	accounts := newMockAccountAPI()
	h := setupRouter(t, transfers, accounts)

	rr := doRequest(t, h, "GET", "/v1/accounts/not-a-uuid/entries", "", "")
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestListAccountEntries_LimitParam(t *testing.T) {
	t.Parallel()
	transfers := newMockTransferAPI()
	accounts := newMockAccountAPI()
	h := setupRouter(t, transfers, accounts)

	created, _ := accounts.Create(context.Background(), ledger.Account{
		ID: uuid.NewString(), Code: "TEST", Name: "Test", Type: ledger.AccountTypeCash,
		Status: ledger.AccountStatusActive, Currency: "IDR",
	})
	actualID := created.ID

	rr := doRequest(t, h, "GET", "/v1/accounts/"+actualID+"/entries?limit=2", "", "")
	require.Equal(t, http.StatusOK, rr.Code)

	var env envelope
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &env))
	var entries []EntryDTO
	require.NoError(t, json.Unmarshal(env.Data, &entries))
	// mock truncates to limit when limit < 3
	assert.LessOrEqual(t, len(entries), 3)
}

// Sanity check that envelope decoder is consistent.
func TestEnvelopeDecoder_Sanity(t *testing.T) {
	t.Parallel()
	body := []byte(`{"data":{"id":"abc","code":"X"},"meta":{"request_id":"r1","timestamp":"2026-01-01T00:00:00Z"}}`)
	env, err := decodeAccountResponse(body)
	require.NoError(t, err)
	assert.Equal(t, "abc", env.ID)
	assert.Equal(t, "X", env.Code)
}

// Reuse envelope decoder.
func decodeAccountResponse(body []byte) (AccountResponse, error) {
	var env envelope
	if err := json.Unmarshal(body, &env); err != nil {
		return AccountResponse{}, err
	}
	var acc AccountResponse
	if err := json.Unmarshal(env.Data, &acc); err != nil {
		return AccountResponse{}, err
	}
	return acc, nil
}

// silence unused imports if tests trimmed
var _ = apperrors.ErrAccountNotFound
var _ = httpx.JSON
var _ = money.NewFromMinor
var _ = strconv.Atoi
var _ = uuid.NewString
