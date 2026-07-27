package invoq

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
)

const webhookSecret = "whsec_test_123"
const webhookTimestamp int64 = 1_710_000_000
const webhookBody = `{"id":"wdel_test","type":"invoice.paid","mode":"test","created_at":"2026-06-15T00:00:00.000Z","data":{"invoice":{"id":"inv_test","mode":"test","status":"paid","amount":"149.0000","currency":"USD","amount_paid":"149.000000000000000000","reference_id":"order_123","payment_revision":1,"fully_paid_at":"2026-06-15T00:00:00.000Z"}}}`
const webhookHeader = "t=1710000000,v1=9ac6c61350b8de2b094013727bb2c1088d853729ac32426bc5e2076d9b7b5b53"

func TestVerifyWebhookVerifiesBackendCompatiblePayloadSignatures(t *testing.T) {
	event, err := verifyWebhookWithNow([]byte(webhookBody), webhookHeader, webhookSecret, webhookTimestamp)
	if err != nil {
		t.Fatal(err)
	}

	if event["id"] != "wdel_test" {
		t.Fatalf("unexpected event id: %#v", event["id"])
	}
	if event["type"] != "invoice.paid" {
		t.Fatalf("unexpected event type: %#v", event["type"])
	}

	// A verified payload decodes its numbers as json.Number, so the guards have
	// to accept that as well as the Go numbers a hand-built event carries.
	if !IsInvoicePaid(event) {
		t.Fatal("expected a verified payload to pass the invoice.paid guard")
	}

	invoicePaidEvent, ok := AsInvoicePaidEvent(event)
	if !ok {
		t.Fatal("expected typed invoice.paid event")
	}
	if invoicePaidEvent.Data.Invoice.PaymentRevision != 1 {
		t.Fatalf("unexpected payment revision: %d", invoicePaidEvent.Data.Invoice.PaymentRevision)
	}
}

func TestVerifyWebhookVerifiesBytePayloadsAndHeaders(t *testing.T) {
	bytes := hexToBytes(t, "7b226964223a227764656c5f6279746573222c2274797065223a22696e766f6963652e7061796d656e745f7265766572736564222c226d6f6465223a2274657374222c22637265617465645f6174223a22323032362d30362d31355430303a31303a30302e3030305a222c2264617461223a7b22696e766f696365223a7b226964223a22696e765f74657374222c226d6f6465223a2274657374222c22737461747573223a227265766965775f7265717569726564222c22616d6f756e74223a223134392e30303030222c2263757272656e6379223a22555344222c22616d6f756e745f70616964223a22302e303030303030303030303030303030303030222c227265666572656e63655f6964223a226f726465725f313233222c227061796d656e745f7265766973696f6e223a322c2266756c6c795f706169645f6174223a6e756c6c7d7d7d")
	header := "t=1710000001,v1=be0ae31fc29369bb6b09b29450eb5a440cec7a8bf92f7171baf794191d1f53bc"
	headers := http.Header{
		"Invoq-Signature": []string{header},
	}

	event, err := verifyWebhookWithNow(bytes, signatureHeader(headers), webhookSecret, 1_710_000_001)
	if err != nil {
		t.Fatal(err)
	}

	if event["id"] != "wdel_bytes" {
		t.Fatalf("unexpected event id: %#v", event["id"])
	}
	if !IsInvoicePaymentReversed(event) {
		t.Fatal("expected a verified payload to pass the invoice.payment_reversed guard")
	}
}

func TestVerifyWebhookAcceptsMultiValueHeaders(t *testing.T) {
	headers := http.Header{
		"invoq-signature": []string{
			"t=1710000000",
			"v1=9ac6c61350b8de2b094013727bb2c1088d853729ac32426bc5e2076d9b7b5b53",
		},
	}

	event, err := verifyWebhookWithNow([]byte(webhookBody), signatureHeader(headers), webhookSecret, webhookTimestamp)
	if err != nil {
		t.Fatal(err)
	}

	if event["id"] != "wdel_test" {
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
	event := lifecycleTestEvent(WebhookEventTypeInvoicePaid, "paid")

	if !IsInvoicePaid(event) {
		t.Fatal("expected invoice.paid event")
	}
	if IsInvoicePaymentReversed(event) {
		t.Fatal("expected invoice.paid event to be rejected as a reversal")
	}

	invoicePaidEvent, ok := AsInvoicePaidEvent(event)
	if !ok {
		t.Fatal("expected typed invoice.paid event")
	}
	if invoicePaidEvent.Type != WebhookEventTypeInvoicePaid {
		t.Fatalf("unexpected event type: %s", invoicePaidEvent.Type)
	}
	if invoicePaidEvent.Data.Invoice.ReferenceID == nil || *invoicePaidEvent.Data.Invoice.ReferenceID != "order_123" {
		t.Fatalf("unexpected reference ID: %#v", invoicePaidEvent.Data.Invoice.ReferenceID)
	}
	if invoicePaidEvent.Data.Invoice.PaymentRevision != 1 {
		t.Fatalf("unexpected payment revision: %d", invoicePaidEvent.Data.Invoice.PaymentRevision)
	}

	invoice := testEventInvoice(t, event)
	invoice["reference_id"] = nil
	invoice["fully_paid_at"] = nil
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

	invoice["reference_id"] = "order_123"
	invoice["fully_paid_at"] = "2026-06-15T00:00:00.000Z"
	for _, status := range []string{"settling", "settled"} {
		invoice["status"] = status
		if !IsInvoicePaid(event) {
			t.Fatalf("expected %s invoice.paid event", status)
		}
	}

	missingAmountPaid := lifecycleTestEvent(WebhookEventTypeInvoicePaid, "paid")
	delete(testEventInvoice(t, missingAmountPaid), "amount_paid")
	if IsInvoicePaid(missingAmountPaid) {
		t.Fatal("expected incomplete invoice.paid event to be rejected")
	}

	// The paid guard fails closed: a status it does not recognize as paid never
	// authorizes fulfillment.
	for _, status := range []string{"review_required", "partially_paid", "some_future_status"} {
		rejected := lifecycleTestEvent(WebhookEventTypeInvoicePaid, status)
		testEventInvoice(t, rejected)["fully_paid_at"] = nil

		if IsInvoicePaid(rejected) {
			t.Fatalf("expected %s invoice.paid event to be rejected", status)
		}
	}
}

func TestLifecycleGuardsRequireAnIntegerPaymentRevision(t *testing.T) {
	for _, revision := range []any{nil, "1", 1.5, json.Number("1.5"), json.Number("")} {
		paid := lifecycleTestEvent(WebhookEventTypeInvoicePaid, "paid")
		testEventInvoice(t, paid)["payment_revision"] = revision
		if IsInvoicePaid(paid) {
			t.Fatalf("expected invoice.paid event with payment revision %#v to be rejected", revision)
		}

		reversed := lifecycleTestEvent(WebhookEventTypeInvoicePaymentReversed, "review_required")
		testEventInvoice(t, reversed)["payment_revision"] = revision
		if IsInvoicePaymentReversed(reversed) {
			t.Fatalf("expected invoice.payment_reversed event with payment revision %#v to be rejected", revision)
		}
	}

	// Verified payloads decode numbers as json.Number; hand-built events carry
	// Go numbers.
	for _, revision := range []any{1, int64(1), 1.0, json.Number("1")} {
		paid := lifecycleTestEvent(WebhookEventTypeInvoicePaid, "paid")
		testEventInvoice(t, paid)["payment_revision"] = revision
		if !IsInvoicePaid(paid) {
			t.Fatalf("expected invoice.paid event with payment revision %#v to be accepted", revision)
		}
	}

	missingRevision := lifecycleTestEvent(WebhookEventTypeInvoicePaid, "paid")
	delete(testEventInvoice(t, missingRevision), "payment_revision")
	if IsInvoicePaid(missingRevision) {
		t.Fatal("expected invoice.paid event without a payment revision to be rejected")
	}
}

func TestIsInvoicePaymentReversedAcceptsAnyStatus(t *testing.T) {
	// The reversal guard deliberately skips the status check the paid guard
	// makes: dropping a reversal leaves an order fulfilled on a payment that no
	// longer exists, so it fails open on a status this SDK version cannot name.
	for _, status := range []string{"review_required", "partially_paid", "unpaid", "some_future_status"} {
		event := lifecycleTestEvent(WebhookEventTypeInvoicePaymentReversed, status)
		invoice := testEventInvoice(t, event)
		invoice["amount_paid"] = "0.000000000000000000"
		invoice["payment_revision"] = 2
		invoice["fully_paid_at"] = nil

		if !IsInvoicePaymentReversed(event) {
			t.Fatalf("expected %s invoice.payment_reversed event", status)
		}
		if IsInvoicePaid(event) {
			t.Fatalf("expected %s invoice.payment_reversed event to be rejected as paid", status)
		}

		reversedEvent, ok := AsInvoicePaymentReversedEvent(event)
		if !ok {
			t.Fatalf("expected typed %s invoice.payment_reversed event", status)
		}
		if reversedEvent.Type != WebhookEventTypeInvoicePaymentReversed {
			t.Fatalf("unexpected event type: %s", reversedEvent.Type)
		}
		if reversedEvent.Data.Invoice.Status != InvoiceStatus(status) {
			t.Fatalf("unexpected invoice status: %s", reversedEvent.Data.Invoice.Status)
		}
		if reversedEvent.Data.Invoice.PaymentRevision != 2 {
			t.Fatalf("unexpected payment revision: %d", reversedEvent.Data.Invoice.PaymentRevision)
		}
		if reversedEvent.Data.Invoice.FullyPaidAt != nil {
			t.Fatalf("unexpected fully paid timestamp: %#v", reversedEvent.Data.Invoice.FullyPaidAt)
		}
	}

	// Failing open on the status is not failing open on the shape.
	malformed := lifecycleTestEvent(WebhookEventTypeInvoicePaymentReversed, "review_required")
	testEventInvoice(t, malformed)["currency"] = "EUR"
	if IsInvoicePaymentReversed(malformed) {
		t.Fatal("expected malformed invoice.payment_reversed event to be rejected")
	}
}

func TestPublicVerifyWebhookAcceptsSignatureStrings(t *testing.T) {
	_, err := VerifyWebhookWithSignature([]byte(webhookBody), webhookHeader, webhookSecret)
	assertSignatureError(t, err, SignatureErrorTimestampOutsideTolerance)
}

func lifecycleTestEvent(eventType WebhookEventType, status string) WebhookEvent {
	return WebhookEvent{
		"id":         "wdel_test",
		"type":       string(eventType),
		"mode":       "test",
		"created_at": "2026-06-15T00:00:00.000Z",
		"data": map[string]any{
			"invoice": map[string]any{
				"id":               "inv_test",
				"mode":             "test",
				"status":           status,
				"amount":           "149.0000",
				"currency":         "USD",
				"amount_paid":      "149.000000000000000000",
				"reference_id":     "order_123",
				"payment_revision": 1,
				"fully_paid_at":    "2026-06-15T00:00:00.000Z",
			},
		},
	}
}

func testEventInvoice(t *testing.T, event WebhookEvent) map[string]any {
	t.Helper()

	data, ok := event["data"].(map[string]any)
	if !ok {
		t.Fatalf("unexpected event data: %#v", event["data"])
	}

	invoice, ok := data["invoice"].(map[string]any)
	if !ok {
		t.Fatalf("unexpected event invoice: %#v", data["invoice"])
	}

	return invoice
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
