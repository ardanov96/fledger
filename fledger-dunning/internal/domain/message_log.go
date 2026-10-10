// Package domain: WhatsApp message log.
package domain

import "time"

// MessageDirection enumerates inbound / outbound.
type MessageDirection string

const (
	DirOutbound MessageDirection = "OUTBOUND"
	DirInbound  MessageDirection = "INBOUND"
)

// MessageType enumerates supported message kinds.
type MessageType string

const (
	MsgText         MessageType = "TEXT"
	MsgDocumentPDF  MessageType = "DOCUMENT_PDF"
	MsgPaymentLink  MessageType = "PAYMENT_LINK"
)

// MessageStatus enumerates delivery status.
type MessageStatus string

const (
	MsgStatusSent      MessageStatus = "SENT"
	MsgStatusDelivered MessageStatus = "DELIVERED"
	MsgStatusRead      MessageStatus = "READ"
	MsgStatusFailed    MessageStatus = "FAILED"
)

// MessageLog is one row in dunning_message_logs.
type MessageLog struct {
	ID                 string          `json:"id"`
	TenantID           string          `json:"tenant_id"`
	QueueID            *string         `json:"queue_id,omitempty"`
	Direction          MessageDirection `json:"direction"`
	PhoneNumber        string          `json:"phone_number"`
	MessageType        MessageType     `json:"message_type"`
	ProviderMessageID  string          `json:"provider_message_id,omitempty"`
	Status             MessageStatus   `json:"status"`
	Payload            map[string]any   `json:"payload,omitempty"`
	CreatedAt          time.Time       `json:"created_at"`
}