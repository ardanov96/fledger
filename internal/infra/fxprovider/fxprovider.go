// Package fxprovider provides FX rate provider implementations.
//
// HTTPProvider fetches rates from a configurable JSON endpoint
// (default: exchangerate-api.com / open.er-api.com format).
// StubProvider returns deterministic rates for tests and dev mode
// when no provider URL is configured.
//
// Both implement currency.FxRateProvider. HTTPProvider additionally
// implements currency.FxRateBatchProvider (1 round-trip per base).
package fxprovider

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/shopspring/decimal"

	"github.com/runut/fmcg-wallet/internal/domain/currency"
)

// =============================================================================
// HTTPProvider
// =============================================================================

// HTTPProviderConfig configures the HTTP provider.
type HTTPProviderConfig struct {
	// URL is the endpoint to call. The default URL is for exchangerate-api.com:
	//   https://open.er-api.com/v6/latest/{BASE}
	// The provider URL is allowed to contain `{BASE}` as a placeholder.
	// If no placeholder is present, the base is appended.
	URL string

	// APIKey is optional. If set, sent as `apikey` header (X-API-Key) for
	// providers that require authentication. exchangerate-api.com expects
	// the API key in the URL path; for that case set URL with `{KEY}` placeholder.
	APIKey string

	// Timeout per request. Default 10s.
	Timeout time.Duration
}

// HTTPProvider fetches FX rates from a JSON HTTP API.
type HTTPProvider struct {
	cfg    HTTPProviderConfig
	client *http.Client
}

// NewHTTPProvider constructs an HTTPProvider.
func NewHTTPProvider(cfg HTTPProviderConfig) *HTTPProvider {
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	return &HTTPProvider{
		cfg:    cfg,
		client: &http.Client{Timeout: timeout},
	}
}

// httpResponse is the standard "open.er-api.com" / exchangerate format.
// `rates` keyed by ISO 4217 code.
type httpResponse struct {
	Base  string            `json:"base_code"`
	Rates map[string]string `json:"rates"`
	// Some providers (exchangerate-api.com v6) use different keys.
	Result string `json:"result"` // "success" on open.er-api.com
}

// FetchRate implements currency.FxRateProvider.
func (p *HTTPProvider) FetchRate(ctx context.Context, base, target string) (decimal.Decimal, error) {
	rates, err := p.FetchAllRates(ctx, base)
	if err != nil {
		return decimal.Zero, err
	}
	v, ok := rates[target]
	if !ok {
		return decimal.Zero, fmt.Errorf("%w: base=%s target=%s", currency.ErrFxProviderRateMissing, base, target)
	}
	return v, nil
}

// FetchAllRates implements currency.FxRateBatchProvider.
func (p *HTTPProvider) FetchAllRates(ctx context.Context, base string) (map[string]decimal.Decimal, error) {
	url := p.buildURL(base)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("%w: build request: %v", currency.ErrFxProviderUnavailable, err)
	}
	if p.cfg.APIKey != "" {
		req.Header.Set("X-API-Key", p.cfg.APIKey)
		req.Header.Set("apikey", p.cfg.APIKey)
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: http: %v", currency.ErrFxProviderUnavailable, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("%w: http status %d", currency.ErrFxProviderUnavailable, resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20)) // cap at 1MB
	if err != nil {
		return nil, fmt.Errorf("%w: read body: %v", currency.ErrFxProviderUnavailable, err)
	}

	var parsed httpResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("%w: parse json: %v", currency.ErrFxProviderInvalid, err)
	}
	if parsed.Rates == nil {
		return nil, fmt.Errorf("%w: rates field missing", currency.ErrFxProviderInvalid)
	}

	out := make(map[string]decimal.Decimal, len(parsed.Rates))
	for k, v := range parsed.Rates {
		d, err := decimal.NewFromString(v)
		if err != nil {
			// Skip invalid entries instead of failing the whole call.
			continue
		}
		out[k] = d
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("%w: no parseable rates", currency.ErrFxProviderInvalid)
	}
	return out, nil
}

// buildURL constructs the request URL. Supports {BASE} placeholder.
func (p *HTTPProvider) buildURL(base string) string {
	if p.cfg.URL == "" {
		return "https://open.er-api.com/v6/latest/" + base
	}
	if strings.Contains(p.cfg.URL, "{BASE}") {
		return strings.ReplaceAll(p.cfg.URL, "{BASE}", base)
	}
	return strings.TrimRight(p.cfg.URL, "/") + "/" + base
}

// Compile-time guard.
var (
	_ currency.FxRateProvider     = (*HTTPProvider)(nil)
	_ currency.FxRateBatchProvider = (*HTTPProvider)(nil)
)

// =============================================================================
// StubProvider — deterministic rates for dev/test
// =============================================================================

// StubProvider returns fixed rates regardless of input. Useful when:
//   - Worker runs in dev mode without network access
//   - Integration tests need predictable rates
//   - Demo environment without API keys
type StubProvider struct {
	rates map[string]decimal.Decimal // key = "BASE/TARGET"
}

// NewStubProvider constructs a StubProvider with the given rates.
// Any pair not in the map returns ErrFxProviderRateMissing.
func NewStubProvider(rates map[string]decimal.Decimal) *StubProvider {
	return &StubProvider{rates: rates}
}

// FetchRate implements currency.FxRateProvider.
func (p *StubProvider) FetchRate(_ context.Context, base, target string) (decimal.Decimal, error) {
	if base == target {
		return decimal.NewFromInt(1), nil
	}
	v, ok := p.rates[base+"/"+target]
	if !ok {
		return decimal.Zero, fmt.Errorf("%w: base=%s target=%s", currency.ErrFxProviderRateMissing, base, target)
	}
	return v, nil
}

// FetchAllRates implements currency.FxRateBatchProvider.
func (p *StubProvider) FetchAllRates(_ context.Context, base string) (map[string]decimal.Decimal, error) {
	out := make(map[string]decimal.Decimal)
	for k, v := range p.rates {
		prefix := base + "/"
		if strings.HasPrefix(k, prefix) {
			target := strings.TrimPrefix(k, prefix)
			out[target] = v
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("%w: no rates for base=%s", currency.ErrFxProviderRateMissing, base)
	}
	return out, nil
}

var (
	_ currency.FxRateProvider     = (*StubProvider)(nil)
	_ currency.FxRateBatchProvider = (*StubProvider)(nil)
)
