package invoq

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"testing"
)

const webhookSecret = "whsec_test_123"
const webhookTimestamp int64 = 1_710_000_000
const webhookBody = `{"id":"evt_test","type":"webhook.ping","data":{"project":{"id":"proj_test"}}}`
const webhookHeader = "t=1710000000,v1=eeafd628acb4e854f5fd942644490b313220dcc7906303d0c8572050ee7795ff"

func TestVerifyWebhookVerifiesBackendCompatiblePayloadSignatures(t *testing.T) {
	event, err := verifyWebhookWithNow([]byte(webhookBody), webhookHeader, webhookSecret, webhookTimestamp)
	if err != nil {
		t.Fatal(err)
	}

	if event["id"] != "evt_test" {
		t.Fatalf("unexpected event id: %#v", event["id"])
	}
	if event["type"] != "webhook.ping" {
		t.Fatalf("unexpected event type: %#v", event["type"])
	}
}

func TestVerifyWebhookVerifiesBytePayloadsAndHeaders(t *testing.T) {
	bytes := hexToBytes(t, "7b226964223a226576745f6279746573222c2274797065223a22776562686f6f6b2e70696e67222c2264617461223a7b2270726f6a656374223a7b226964223a2270726f6a5f6279746573227d7d7d")
	header := "t=1710000001,v1=1ee237dd9e509e515eca754c3a34da3536e8c76cfc8ce1fd0a4e74d1366d20e2"
	headers := http.Header{
		"Invoq-Signature": []string{header},
	}

	event, err := verifyWebhookWithNow(bytes, signatureHeader(headers), webhookSecret, 1_710_000_001)
	if err != nil {
		t.Fatal(err)
	}

	if event["id"] != "evt_bytes" {
		t.Fatalf("unexpected event id: %#v", event["id"])
	}
}

func TestVerifyWebhookAcceptsMultiValueHeaders(t *testing.T) {
	headers := http.Header{
		"invoq-signature": []string{
			"t=1710000000",
			"v1=eeafd628acb4e854f5fd942644490b313220dcc7906303d0c8572050ee7795ff",
		},
	}

	event, err := verifyWebhookWithNow([]byte(webhookBody), signatureHeader(headers), webhookSecret, webhookTimestamp)
	if err != nil {
		t.Fatal(err)
	}

	if event["id"] != "evt_test" {
		t.Fatalf("unexpected event id: %#v", event["id"])
	}
}

func TestVerifyWebhookRejectsInvalidSignatureInputs(t *testing.T) {
	_, err := verifyWebhookWithNow([]byte(webhookBody), "", webhookSecret, webhookTimestamp)
	assertSignatureError(t, err, SignatureErrorMissingSignature)

	_, err = verifyWebhookWithNow([]byte(webhookBody), "v1=abc", webhookSecret, webhookTimestamp)
	assertSignatureError(t, err, SignatureErrorInvalidSignatureHeader)

	_, err = verifyWebhookWithNow([]byte(webhookBody), webhookHeader, webhookSecret, webhookTimestamp+301)
	assertSignatureError(t, err, SignatureErrorTimestampOutsideTolerance)

	_, err = verifyWebhookWithNow([]byte(webhookBody), webhookHeader, "wrong", webhookTimestamp)
	assertSignatureError(t, err, SignatureErrorSignatureMismatch)

	_, err = verifyWebhookWithNow([]byte(webhookBody), webhookHeader, "", webhookTimestamp)
	assertSignatureError(t, err, SignatureErrorInvalidSignatureHeader)
}

func TestVerifyWebhookRejectsInvalidPayloadsAfterSignature(t *testing.T) {
	_, err := verifyWebhookWithNow([]byte("not json"), signWebhookTestPayload("not json", webhookTimestamp), webhookSecret, webhookTimestamp)
	assertSignatureError(t, err, SignatureErrorInvalidPayload)

	_, err = verifyWebhookWithNow([]byte("[]"), signWebhookTestPayload("[]", webhookTimestamp), webhookSecret, webhookTimestamp)
	assertSignatureError(t, err, SignatureErrorInvalidPayload)

	_, err = verifyWebhookWithNow([]byte(`{"id":"evt"}`), signWebhookTestPayload(`{"id":"evt"}`, webhookTimestamp), webhookSecret, webhookTimestamp)
	assertSignatureError(t, err, SignatureErrorInvalidPayload)
}

func TestIsInvoicePaidChecksTheFullShapeBeforeDecoding(t *testing.T) {
	event := WebhookEvent{
		"id":         "evt_paid",
		"type":       "invoice.paid",
		"mode":       "test",
		"created_at": "2026-06-15T00:00:00.000Z",
		"data": map[string]any{
			"invoice": map[string]any{
				"id":            "inv_test",
				"mode":          "test",
				"status":        "paid",
				"amount":        "149",
				"currency":      "USD",
				"amount_paid":   "149",
				"reference_id":  "order_123",
				"fully_paid_at": "2026-06-15T00:00:00.000Z",
			},
		},
	}

	if !IsInvoicePaid(event) {
		t.Fatal("expected invoice.paid event")
	}

	invoicePaidEvent, ok := AsInvoicePaidEvent(event)
	if !ok {
		t.Fatal("expected typed invoice.paid event")
	}
	if invoicePaidEvent.Data.Invoice.ReferenceID == nil || *invoicePaidEvent.Data.Invoice.ReferenceID != "order_123" {
		t.Fatalf("unexpected reference ID: %#v", invoicePaidEvent.Data.Invoice.ReferenceID)
	}

	event["data"].(map[string]any)["invoice"].(map[string]any)["reference_id"] = nil
	event["data"].(map[string]any)["invoice"].(map[string]any)["fully_paid_at"] = nil
	if !IsInvoicePaid(event) {
		t.Fatal("expected invoice.paid event with null nullable fields")
	}
	invoicePaidEvent, ok = AsInvoicePaidEvent(event)
	if !ok {
		t.Fatal("expected typed invoice.paid event with null nullable fields")
	}
	if invoicePaidEvent.Data.Invoice.ReferenceID != nil {
		t.Fatalf("unexpected null reference ID: %#v", invoicePaidEvent.Data.Invoice.ReferenceID)
	}

	event["data"].(map[string]any)["invoice"].(map[string]any)["reference_id"] = "order_123"
	event["data"].(map[string]any)["invoice"].(map[string]any)["fully_paid_at"] = "2026-06-15T00:00:00.000Z"
	for _, status := range []string{"settling", "settled"} {
		event["data"].(map[string]any)["invoice"].(map[string]any)["status"] = status
		if !IsInvoicePaid(event) {
			t.Fatalf("expected %s invoice.paid event", status)
		}
	}

	missingAmountPaid := WebhookEvent{
		"id":         "evt_paid",
		"type":       "invoice.paid",
		"mode":       "test",
		"created_at": "2026-06-15T00:00:00.000Z",
		"data": map[string]any{
			"invoice": map[string]any{
				"id":            "inv_test",
				"mode":          "test",
				"status":        "paid",
				"amount":        "149",
				"currency":      "USD",
				"reference_id":  "order_123",
				"fully_paid_at": "2026-06-15T00:00:00.000Z",
			},
		},
	}

	if IsInvoicePaid(missingAmountPaid) {
		t.Fatal("expected incomplete invoice.paid event to be rejected")
	}

	reviewRequired := WebhookEvent{
		"id":         "evt_paid",
		"type":       "invoice.paid",
		"mode":       "test",
		"created_at": "2026-06-15T00:00:00.000Z",
		"data": map[string]any{
			"invoice": map[string]any{
				"id":            "inv_test",
				"mode":          "test",
				"status":        "review_required",
				"amount":        "149",
				"currency":      "USD",
				"amount_paid":   "149",
				"reference_id":  "order_123",
				"fully_paid_at": nil,
			},
		},
	}

	if IsInvoicePaid(reviewRequired) {
		t.Fatal("expected review_required invoice.paid event to be rejected")
	}
}

func TestPublicVerifyWebhookAcceptsSignatureStrings(t *testing.T) {
	_, err := VerifyWebhookWithSignature([]byte(webhookBody), webhookHeader, webhookSecret)
	assertSignatureError(t, err, SignatureErrorTimestampOutsideTolerance)
}

func assertSignatureError(t *testing.T, err error, code SignatureVerificationErrorCode) {
	t.Helper()

	signatureError, ok := err.(*SignatureVerificationError)
	if !ok {
		t.Fatalf("expected signature error, got %T", err)
	}
	if signatureError.Code != code {
		t.Fatalf("unexpected signature error code: %s", signatureError.Code)
	}
}

func signWebhookTestPayload(payload string, timestamp int64) string {
	mac := hmac.New(sha256.New, []byte(webhookSecret))
	mac.Write([]byte(fmt.Sprintf("%d.%s", timestamp, payload)))
	return fmt.Sprintf("t=%d,v1=%s", timestamp, hex.EncodeToString(mac.Sum(nil)))
}

func hexToBytes(t *testing.T, value string) []byte {
	t.Helper()

	bytes, err := hex.DecodeString(value)
	if err != nil {
		t.Fatal(err)
	}

	return bytes
}
