// Package domain: WhatsApp pairing session.
package domain

import "time"

// SessionStatus enumerates the gateway lifecycle.
type SessionStatus string

const (
	SessionDisconnected   SessionStatus = "DISCONNECTED"
	SessionScanQR          SessionStatus = "SCAN_QR"
	SessionConnected       SessionStatus = "CONNECTED"
	SessionPairingTimeout  SessionStatus = "PAIRING_TIMEOUT"
)

// Session is one WhatsApp gateway session.
type Session struct {
	ID               string        `json:"id"`
	TenantID         string        `json:"tenant_id"`
	SessionName      string        `json:"session_name"`
	ConnectionStatus SessionStatus `json:"connection_status"`
	QRCodeData       string        `json:"qr_code_data,omitempty"`
	PhoneConnected   string        `json:"phone_connected,omitempty"`
	LastHeartbeat    *time.Time    `json:"last_heartbeat,omitempty"`
	UpdatedAt        time.Time     `json:"updated_at"`
}