package whatsapp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestFonnteProvider_Name(t *testing.T) {
	p := NewFonnteProvider("test-token", "", nil)
	if p.Name() != "FONNTE" {
		t.Fatalf("expected FONNTE, got %s", p.Name())
	}
}

func TestFonnteProvider_SendTextMessage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "valid-token" {
			http.Error(w, `{"status":false,"reason":"invalid token"}`, http.StatusUnauthorized)
			return
		}
		if r.URL.Path != "/send" {
			http.NotFound(w, r)
			return
		}
		_ = r.ParseForm()
		target := r.Form.Get("target")
		msg := r.Form.Get("message")
		if target == "" || msg == "" {
			http.Error(w, `{"status":false,"reason":"missing params"}`, http.StatusBadRequest)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"status":  true,
			"target":  []string{target},
			"id":      []string{"msg-fonnte-12345"},
			"process": "processing",
		})
	}))
	defer server.Close()

	ctx := context.Background()

	// 1. Missing token
	pNoToken := NewFonnteProvider("", server.URL, nil)
	_, err := pNoToken.SendTextMessage(ctx, MessageRequest{
		PhoneNumber: "08123456789",
		Body:        "Hello",
	})
	if err == nil {
		t.Fatal("expected error with empty token")
	}

	// 2. Success send
	p := NewFonnteProvider("valid-token", server.URL, nil)
	msgID, err := p.SendTextMessage(ctx, MessageRequest{
		PhoneNumber: "+62 812-3456-7890",
		Body:        "Test Invoice Reminder",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if msgID != "msg-fonnte-12345" {
		t.Fatalf("expected msg-fonnte-12345, got %s", msgID)
	}

	// 3. Dynamic token update
	p.SetToken("new-token")
	if p.GetToken() != "new-token" {
		t.Fatalf("expected new-token, got %s", p.GetToken())
	}
}

func TestFonnteProvider_SendDocument(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "valid-token" {
			http.Error(w, `{"status":false,"reason":"unauthorized"}`, http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"status": true,
			"id":     []string{"doc-fonnte-999"},
		})
	}))
	defer server.Close()

	tmpDir := t.TempDir()
	dummyFile := filepath.Join(tmpDir, "statement.pdf")
	_ = os.WriteFile(dummyFile, []byte("%PDF-1.4 mock content"), 0644)

	p := NewFonnteProvider("valid-token", server.URL, nil)
	docID, err := p.SendDocument(context.Background(), DocumentRequest{
		PhoneNumber: "08123456789",
		FilePath:    dummyFile,
		FileName:    "statement.pdf",
		Caption:     "Rekening Koran Bulanan",
	})
	if err != nil {
		t.Fatalf("unexpected document send error: %v", err)
	}
	if docID != "doc-fonnte-999" {
		t.Fatalf("expected doc-fonnte-999, got %s", docID)
	}
}

func TestFonnteProvider_GetStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/device" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"status":        true,
			"device":        "628123456789",
			"device_status": "connect",
			"name":          "FMCG Official Distributor",
			"messages":      150,
			"quota":         "9999",
		})
	}))
	defer server.Close()

	p := NewFonnteProvider("valid-token", server.URL, nil)
	sess, err := p.GetStatus(context.Background(), "tenant-001")
	if err != nil {
		t.Fatalf("unexpected get status error: %v", err)
	}
	if sess.ConnectionStatus != "CONNECTED" {
		t.Fatalf("expected CONNECTED, got %s", sess.ConnectionStatus)
	}
	if sess.PhoneConnected != "628123456789" {
		t.Fatalf("expected 628123456789, got %s", sess.PhoneConnected)
	}
}

func TestFonnteProvider_GenerateQR(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"status": true,
			"url":    "iVBORw0KGgoAAAANSUhEUgAA...",
		})
	}))
	defer server.Close()

	p := NewFonnteProvider("valid-token", server.URL, nil)
	qr, err := p.GenerateQR(context.Background(), "tenant-001", "default")
	if err != nil {
		t.Fatalf("unexpected generate qr error: %v", err)
	}
	if qr != "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAA..." {
		t.Fatalf("expected data uri prefix, got %s", qr)
	}
}
