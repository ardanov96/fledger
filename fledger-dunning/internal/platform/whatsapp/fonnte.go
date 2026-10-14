// Package whatsapp provides real and mock implementations of the WhatsApp gateway.
package whatsapp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fledger/fledger-dunning/internal/domain"
)

// FonnteDeviceDetail provides extended device metrics returned by Fonnte.
type FonnteDeviceDetail struct {
	Device       string `json:"device"`
	DeviceStatus string `json:"device_status"`
	Name         string `json:"name"`
	Package      string `json:"package"`
	Quota        string `json:"quota"`
	Messages     int    `json:"messages"`
	Expired      string `json:"expired"`
}

// FonnteProvider implements the Provider interface backed by the official
// Fonnte WhatsApp API (https://api.fonnte.com).
type FonnteProvider struct {
	mu      sync.RWMutex
	token   string
	baseURL string
	client  *http.Client
	log     Logger
}

// NewFonnteProvider creates a new Fonnte gateway instance.
func NewFonnteProvider(token, baseURL string, log Logger) *FonnteProvider {
	if baseURL == "" {
		baseURL = "https://api.fonnte.com"
	}
	baseURL = strings.TrimRight(baseURL, "/")
	return &FonnteProvider{
		token:   token,
		baseURL: baseURL,
		client: &http.Client{
			Timeout: 20 * time.Second,
		},
		log: log,
	}
}

// Name returns the provider identifier.
func (f *FonnteProvider) Name() string {
	return "FONNTE"
}

// SetToken updates the Fonnte authentication token dynamically at runtime.
func (f *FonnteProvider) SetToken(token string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.token = strings.TrimSpace(token)
}

// GetToken returns the current active token.
func (f *FonnteProvider) GetToken() string {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.token
}

// SendTextMessage dispatches a text message via POST /send.
func (f *FonnteProvider) SendTextMessage(ctx context.Context, req MessageRequest) (string, error) {
	token := f.GetToken()
	if token == "" {
		return "", errors.New("fonnte: token not configured (set FONNTE_TOKEN)")
	}
	if req.PhoneNumber == "" {
		return "", errors.New("fonnte: phone_number required")
	}
	if req.Body == "" {
		return "", errors.New("fonnte: body required")
	}

	target := cleanPhoneNumber(req.PhoneNumber)

	data := url.Values{}
	data.Set("target", target)
	data.Set("message", req.Body)
	data.Set("countryCode", "62")

	httpReq, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		f.baseURL+"/send",
		strings.NewReader(data.Encode()),
	)
	if err != nil {
		return "", fmt.Errorf("fonnte: create request: %w", err)
	}

	httpReq.Header.Set("Authorization", token)
	httpReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := f.client.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("fonnte: dispatch error: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)

	var res struct {
		Status  bool     `json:"status"`
		Target  []string `json:"target"`
		ID      []string `json:"id"`
		Process string   `json:"process"`
		Reason  string   `json:"reason"`
		Detail  string   `json:"detail"`
	}

	if err := json.Unmarshal(bodyBytes, &res); err != nil {
		if resp.StatusCode >= 400 {
			return "", fmt.Errorf("fonnte: api error (%d): %s", resp.StatusCode, string(bodyBytes))
		}
		return "", fmt.Errorf("fonnte: invalid response format: %w", err)
	}

	if !res.Status {
		msg := res.Reason
		if msg == "" {
			msg = res.Detail
		}
		if msg == "" {
			msg = string(bodyBytes)
		}
		return "", fmt.Errorf("fonnte rejected send: %s", msg)
	}

	msgID := ""
	if len(res.ID) > 0 && res.ID[0] != "" {
		msgID = res.ID[0]
	} else {
		msgID = fmt.Sprintf("fonnte-%d", time.Now().UnixNano())
	}

	if f.log != nil {
		f.log.Info("fonnte.send_text.success",
			"target", target,
			"msg_id", msgID,
			"process", res.Process,
		)
	}

	return msgID, nil
}

// SendDocument dispatches a document/file attachment (e.g. PDF e-Statement).
func (f *FonnteProvider) SendDocument(ctx context.Context, req DocumentRequest) (string, error) {
	token := f.GetToken()
	if token == "" {
		return "", errors.New("fonnte: token not configured (set FONNTE_TOKEN)")
	}
	if req.PhoneNumber == "" {
		return "", errors.New("fonnte: phone_number required")
	}
	if req.FilePath == "" {
		return "", errors.New("fonnte: file_path required")
	}

	target := cleanPhoneNumber(req.PhoneNumber)
	fileName := req.FileName
	if fileName == "" {
		fileName = filepath.Base(req.FilePath)
	}

	var reqBody io.Reader
	var contentType string

	// Check if FilePath is a URL vs local file on disk
	if strings.HasPrefix(req.FilePath, "http://") || strings.HasPrefix(req.FilePath, "https://") {
		data := url.Values{}
		data.Set("target", target)
		data.Set("url", req.FilePath)
		data.Set("filename", fileName)
		if req.Caption != "" {
			data.Set("message", req.Caption)
		}
		data.Set("countryCode", "62")
		reqBody = strings.NewReader(data.Encode())
		contentType = "application/x-www-form-urlencoded"
	} else {
		// Read local file and build multipart payload
		fileBytes, err := os.ReadFile(req.FilePath)
		if err != nil {
			return "", fmt.Errorf("fonnte: read file %q: %w", req.FilePath, err)
		}

		var buf bytes.Buffer
		writer := multipart.NewWriter(&buf)

		_ = writer.WriteField("target", target)
		_ = writer.WriteField("filename", fileName)
		_ = writer.WriteField("countryCode", "62")
		if req.Caption != "" {
			_ = writer.WriteField("message", req.Caption)
		}

		part, err := writer.CreateFormFile("file", fileName)
		if err != nil {
			return "", fmt.Errorf("fonnte: create form file: %w", err)
		}
		if _, err := part.Write(fileBytes); err != nil {
			return "", fmt.Errorf("fonnte: write file content: %w", err)
		}
		if err := writer.Close(); err != nil {
			return "", fmt.Errorf("fonnte: close multipart writer: %w", err)
		}

		reqBody = &buf
		contentType = writer.FormDataContentType()
	}

	httpReq, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		f.baseURL+"/send",
		reqBody,
	)
	if err != nil {
		return "", fmt.Errorf("fonnte: create request: %w", err)
	}

	httpReq.Header.Set("Authorization", token)
	httpReq.Header.Set("Content-Type", contentType)

	resp, err := f.client.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("fonnte: dispatch document error: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)

	var res struct {
		Status  bool     `json:"status"`
		Target  []string `json:"target"`
		ID      []string `json:"id"`
		Process string   `json:"process"`
		Reason  string   `json:"reason"`
		Detail  string   `json:"detail"`
	}

	if err := json.Unmarshal(bodyBytes, &res); err != nil {
		if resp.StatusCode >= 400 {
			return "", fmt.Errorf("fonnte: api error (%d): %s", resp.StatusCode, string(bodyBytes))
		}
		return "", fmt.Errorf("fonnte: invalid response format: %w", err)
	}

	if !res.Status {
		msg := res.Reason
		if msg == "" {
			msg = res.Detail
		}
		if msg == "" {
			msg = string(bodyBytes)
		}
		return "", fmt.Errorf("fonnte rejected document send: %s", msg)
	}

	msgID := ""
	if len(res.ID) > 0 && res.ID[0] != "" {
		msgID = res.ID[0]
	} else {
		msgID = fmt.Sprintf("fonnte-%d", time.Now().UnixNano())
	}

	if f.log != nil {
		f.log.Info("fonnte.send_document.success",
			"target", target,
			"file", fileName,
			"msg_id", msgID,
		)
	}

	return msgID, nil
}

// GetStatus checks device connectivity state via POST /device.
func (f *FonnteProvider) GetStatus(ctx context.Context, tenantID string) (*domain.Session, error) {
	token := f.GetToken()
	now := time.Now().UTC()

	if token == "" {
		return &domain.Session{
			ID:               fmt.Sprintf("sess-fonnte-%s", tenantID),
			TenantID:         tenantID,
			SessionName:      "Fonnte Gateway (Token Unset)",
			ConnectionStatus: domain.SessionDisconnected,
			UpdatedAt:        now,
		}, nil
	}

	httpReq, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		f.baseURL+"/device",
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("fonnte: create request: %w", err)
	}

	httpReq.Header.Set("Authorization", token)

	resp, err := f.client.Do(httpReq)
	if err != nil {
		if f.log != nil {
			f.log.Warn("fonnte.get_status.failed", "err", err.Error())
		}
		return &domain.Session{
			ID:               fmt.Sprintf("sess-fonnte-%s", tenantID),
			TenantID:         tenantID,
			SessionName:      "Fonnte Gateway (Network Error)",
			ConnectionStatus: domain.SessionDisconnected,
			UpdatedAt:        now,
		}, nil
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)

	var res struct {
		Status       bool   `json:"status"`
		Device       string `json:"device"`
		DeviceStatus string `json:"device_status"` // "connect" or "disconnect"
		Name         string `json:"name"`
		Package      string `json:"package"`
		Quota        any    `json:"quota"`
		Messages     any    `json:"messages"`
		Expired      string `json:"expired"`
		Reason       string `json:"reason"`
	}

	if err := json.Unmarshal(bodyBytes, &res); err != nil {
		return &domain.Session{
			ID:               fmt.Sprintf("sess-fonnte-%s", tenantID),
			TenantID:         tenantID,
			SessionName:      "Fonnte Gateway (Bad Response)",
			ConnectionStatus: domain.SessionDisconnected,
			UpdatedAt:        now,
		}, nil
	}

	connStatus := domain.SessionDisconnected
	if strings.EqualFold(res.DeviceStatus, "connect") {
		connStatus = domain.SessionConnected
	}

	sessionName := res.Name
	if sessionName == "" {
		sessionName = "Fonnte Device"
	}

	return &domain.Session{
		ID:               fmt.Sprintf("sess-fonnte-%s", tenantID),
		TenantID:         tenantID,
		SessionName:      sessionName,
		ConnectionStatus: connStatus,
		PhoneConnected:   res.Device,
		LastHeartbeat:    &now,
		UpdatedAt:        now,
	}, nil
}

// GenerateQR requests pairing QR from POST /qr.
func (f *FonnteProvider) GenerateQR(ctx context.Context, tenantID, sessionName string) (string, error) {
	token := f.GetToken()
	if token == "" {
		return "", errors.New("fonnte: token not configured (set FONNTE_TOKEN)")
	}

	data := url.Values{}
	data.Set("type", "qr")

	httpReq, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		f.baseURL+"/qr",
		strings.NewReader(data.Encode()),
	)
	if err != nil {
		return "", fmt.Errorf("fonnte: create qr request: %w", err)
	}

	httpReq.Header.Set("Authorization", token)
	httpReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := f.client.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("fonnte: fetch qr error: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)

	var res struct {
		Status bool   `json:"status"`
		URL    string `json:"url"` // base64 string or data uri
		Reason string `json:"reason"`
	}

	if err := json.Unmarshal(bodyBytes, &res); err != nil {
		return "", fmt.Errorf("fonnte: invalid qr response: %s", string(bodyBytes))
	}

	if !res.Status {
		return "", fmt.Errorf("fonnte: generate qr failed: %s", res.Reason)
	}

	qrPayload := res.URL
	if !strings.HasPrefix(qrPayload, "data:image") && !strings.HasPrefix(qrPayload, "http") {
		// Prepend standard data URI scheme for standard base64 PNG rendering
		qrPayload = "data:image/png;base64," + qrPayload
	}

	return qrPayload, nil
}

// cleanPhoneNumber sanitizes mobile phone numbers for WhatsApp delivery.
func cleanPhoneNumber(raw string) string {
	s := strings.TrimSpace(raw)
	s = strings.ReplaceAll(s, "+", "")
	s = strings.ReplaceAll(s, "-", "")
	s = strings.ReplaceAll(s, " ", "")
	s = strings.ReplaceAll(s, "(", "")
	s = strings.ReplaceAll(s, ")", "")
	return s
}
