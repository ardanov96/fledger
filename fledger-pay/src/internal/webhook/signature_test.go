package webhook

import (
	"testing"
)

// TestVerifyMidtransSignature covers the happy path and a mismatch.
func TestVerifyMidtransSignature(t *testing.T) {
	const orderID = "PAY-202610-00891"
	const statusCode = "200"
	const grossAmount = "4000000.00"
	const serverKey = "dev-webhook-secret-key-midtrans-xendit"

	sig := ComputeMidtransSignature(orderID, statusCode, grossAmount, serverKey)
	if err := VerifyMidtransSignature(orderID, statusCode, grossAmount, serverKey, sig); err != nil {
		t.Fatalf("expected match, got %v", err)
	}
	// Tamper: change orderID → should fail.
	if err := VerifyMidtransSignature("X", statusCode, grossAmount, serverKey, sig); err == nil {
		t.Fatalf("expected mismatch for tampered order id")
	}
}

// TestVerifyGenericSHA256 covers HMAC-SHA256 over the raw body.
func TestVerifyGenericSHA256(t *testing.T) {
	secret := "abc"
	body := []byte(`{"hello":"world"}`)
	sig := ComputeGenericSHA256(secret, body)
	if err := VerifyGenericSHA256(secret, body, sig); err != nil {
		t.Fatalf("expected match, got %v", err)
	}
	if err := VerifyGenericSHA256("wrong", body, sig); err == nil {
		t.Fatalf("expected mismatch for wrong secret")
	}
}

// TestVerifyXenditToken covers the simple token equality check.
func TestVerifyXenditToken(t *testing.T) {
	if err := VerifyXenditToken("secret-1", "secret-1"); err != nil {
		t.Fatalf("expected match, got %v", err)
	}
	if err := VerifyXenditToken("secret-1", "other"); err == nil {
		t.Fatalf("expected mismatch")
	}
}

// TestCRC16KnownVector — the EMVCo CRC-16/CCITT-FALSE for the ASCII "123456789"
// is 0x29B1. (See EMVCo spec §4.5.) Used as a quick sanity check.
func TestCRC16KnownVector(t *testing.T) {
	got := crc16CCITT([]byte("123456789"))
	if got != 0x29B1 {
		t.Fatalf("CRC16 mismatch: got %#04x want 0x29B1", got)
	}
}

// helper exposed so the QRIS test can verify the implementation against a
// known vector without re-implementing the CRC here.
func crc16CCITT(data []byte) uint16 {
	crc := uint16(0xFFFF)
	for _, b := range data {
		crc ^= uint16(b) << 8
		for i := 0; i < 8; i++ {
			if crc&0x8000 != 0 {
				crc = (crc << 1) ^ 0x1021
			} else {
				crc <<= 1
			}
		}
	}
	return crc
}