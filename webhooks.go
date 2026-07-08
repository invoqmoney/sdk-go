package invoq

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const defaultToleranceSeconds int64 = 300

// VerifyWebhook verifies an invoq webhook and returns the decoded event.
func VerifyWebhook(rawBody []byte, headers http.Header, webhookSecret string) (WebhookEvent, error) {
	return verifyWebhookWithNow(rawBody, signatureHeader(headers), webhookSecret, time.Now().Unix())
}

// VerifyWebhookWithSignature verifies an invoq webhook with an invoq-signature header value.
func VerifyWebhookWithSignature(rawBody []byte, signature string, webhookSecret string) (WebhookEvent, error) {
	return verifyWebhookWithNow(rawBody, signature, webhookSecret, time.Now().Unix())
}

// IsInvoicePaid returns whether a verified webhook event matches the invoice.paid shape.
func IsInvoicePaid(event WebhookEvent) bool {
	_, ok := AsInvoicePaidEvent(event)
	return ok
}

// AsInvoicePaidEvent decodes a verified invoice.paid webhook event.
func AsInvoicePaidEvent(event WebhookEvent) (*InvoicePaidEvent, bool) {
	if !hasInvoicePaidShape(event) {
		return nil, false
	}

	eventBytes, err := json.Marshal(event)
	if err != nil {
		return nil, false
	}

	var invoicePaidEvent InvoicePaidEvent
	if err := json.Unmarshal(eventBytes, &invoicePaidEvent); err != nil {
		return nil, false
	}

	if invoicePaidEvent.Type != "invoice.paid" {
		return nil, false
	}

	return &invoicePaidEvent, true
}

func verifyWebhookWithNow(rawBody []byte, signature string, webhookSecret string, nowSeconds int64) (WebhookEvent, error) {
	if signature == "" {
		return nil, signatureError(SignatureErrorMissingSignature, "Missing invoq-signature header.")
	}

	if webhookSecret == "" {
		return nil, signatureError(SignatureErrorInvalidSignatureHeader, "Webhook secret must be a non-empty string.")
	}

	parsed, err := parseSignatureHeader(signature)
	if err != nil {
		return nil, err
	}

	if absInt64(nowSeconds-parsed.timestampSeconds) > defaultToleranceSeconds {
		return nil, signatureError(SignatureErrorTimestampOutsideTolerance, "Webhook timestamp is outside the allowed tolerance.")
	}

	expectedSignature := hmacSHA256Hex(webhookSecret, parsed.timestamp, rawBody)
	if !hmac.Equal([]byte(expectedSignature), []byte(parsed.signature)) {
		return nil, signatureError(SignatureErrorSignatureMismatch, "Webhook signature mismatch.")
	}

	payload, err := decodeJSON(rawBody)
	if err != nil {
		return nil, signatureError(SignatureErrorInvalidPayload, "Webhook payload is not valid JSON.")
	}

	event, ok := payload.(map[string]any)
	if !ok {
		return nil, signatureError(SignatureErrorInvalidPayload, "Webhook payload must be an object with a string type.")
	}

	if _, ok := event["type"].(string); !ok {
		return nil, signatureError(SignatureErrorInvalidPayload, "Webhook payload must be an object with a string type.")
	}

	return WebhookEvent(event), nil
}

type parsedSignatureHeader struct {
	timestamp        string
	timestampSeconds int64
	signature        string
}

func parseSignatureHeader(signatureHeader string) (*parsedSignatureHeader, error) {
	parts := make(map[string]string)

	for _, part := range strings.Split(signatureHeader, ",") {
		separatorIndex := strings.Index(part, "=")
		if separatorIndex == -1 {
			return nil, signatureError(SignatureErrorInvalidSignatureHeader, "Invalid invoq-signature header.")
		}

		key := strings.TrimSpace(part[:separatorIndex])
		value := strings.TrimSpace(part[separatorIndex+1:])
		if key != "" && value != "" {
			parts[key] = value
		}
	}

	timestamp := parts["t"]
	signature := parts["v1"]
	if timestamp == "" || signature == "" || !isDigits(timestamp) {
		return nil, signatureError(SignatureErrorInvalidSignatureHeader, "Invalid invoq-signature header.")
	}

	signature = strings.ToLower(signature)
	if !isSignatureHex(signature) {
		return nil, signatureError(SignatureErrorInvalidSignatureHeader, "Invalid invoq-signature signature.")
	}

	timestampSeconds, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil {
		return nil, signatureError(SignatureErrorInvalidSignatureHeader, "Invalid invoq-signature header.")
	}

	return &parsedSignatureHeader{
		timestamp:        timestamp,
		timestampSeconds: timestampSeconds,
		signature:        signature,
	}, nil
}

func signatureHeader(headers http.Header) string {
	for key, values := range headers {
		if strings.EqualFold(key, "invoq-signature") {
			return strings.Join(values, ",")
		}
	}

	return ""
}

func hmacSHA256Hex(secret string, timestamp string, rawBody []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(timestamp))
	mac.Write([]byte("."))
	mac.Write(rawBody)

	return hex.EncodeToString(mac.Sum(nil))
}

func hasInvoicePaidShape(event WebhookEvent) bool {
	if event == nil {
		return false
	}

	if event["type"] != "invoice.paid" {
		return false
	}
	if _, ok := event["id"].(string); !ok {
		return false
	}
	if !isInvoiceMode(event["mode"]) {
		return false
	}
	if _, ok := event["created_at"].(string); !ok {
		return false
	}

	data, ok := event["data"].(map[string]any)
	if !ok {
		return false
	}

	invoice, ok := data["invoice"].(map[string]any)
	if !ok {
		return false
	}

	referenceID, hasReferenceID := invoice["reference_id"]
	fullyPaidAt, hasFullyPaidAt := invoice["fully_paid_at"]

	return stringField(invoice, "id") &&
		isInvoiceMode(invoice["mode"]) &&
		isInvoicePaidStatus(invoice["status"]) &&
		stringField(invoice, "amount") &&
		invoice["currency"] == "USD" &&
		stringField(invoice, "amount_paid") &&
		hasReferenceID &&
		(referenceID == nil || isString(referenceID)) &&
		hasFullyPaidAt &&
		(fullyPaidAt == nil || isString(fullyPaidAt))
}

func stringField(object map[string]any, key string) bool {
	_, ok := object[key].(string)
	return ok
}

func isString(value any) bool {
	_, ok := value.(string)
	return ok
}

func isInvoiceMode(value any) bool {
	return value == "test" || value == "live"
}

func isInvoicePaidStatus(value any) bool {
	return value == "paid" || value == "settling" || value == "settled"
}

func isDigits(value string) bool {
	for _, character := range value {
		if character < '0' || character > '9' {
			return false
		}
	}

	return value != ""
}

func isSignatureHex(value string) bool {
	if len(value) != 64 {
		return false
	}

	for _, character := range value {
		if !((character >= '0' && character <= '9') || (character >= 'a' && character <= 'f')) {
			return false
		}
	}

	return true
}

func absInt64(value int64) int64 {
	if value < 0 {
		return -value
	}

	return value
}
