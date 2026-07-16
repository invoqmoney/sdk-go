package invoq

import "encoding/json"

// InvoiceMode is the invoice environment.
type InvoiceMode string

const (
	// InvoiceModeTest is the test invoice environment.
	InvoiceModeTest InvoiceMode = "test"
	// InvoiceModeLive is the live invoice environment.
	InvoiceModeLive InvoiceMode = "live"
)

// InvoiceCurrency is an invoice currency supported by invoq.
type InvoiceCurrency string

const (
	// InvoiceCurrencyUSD is the USD invoice currency.
	InvoiceCurrencyUSD InvoiceCurrency = "USD"
)

// InvoiceStatus is the lifecycle status returned with invoice API responses.
type InvoiceStatus string

const (
	// InvoiceStatusUnpaid means the invoice has not received funds.
	InvoiceStatusUnpaid InvoiceStatus = "unpaid"
	// InvoiceStatusPartiallyPaid means the invoice has received less than the requested amount.
	InvoiceStatusPartiallyPaid InvoiceStatus = "partially_paid"
	// InvoiceStatusPaid means the invoice has received enough funds for automatic fulfillment.
	InvoiceStatusPaid InvoiceStatus = "paid"
	// InvoiceStatusSettling means the invoice is paid and settlement is in progress.
	InvoiceStatusSettling InvoiceStatus = "settling"
	// InvoiceStatusSettled means the invoice has settled.
	InvoiceStatusSettled InvoiceStatus = "settled"
	// InvoiceStatusReviewRequired means the invoice needs manual review before fulfillment.
	InvoiceStatusReviewRequired InvoiceStatus = "review_required"
)

// InvoicePaymentStatus is the payment status returned by public invoice reads.
type InvoicePaymentStatus string

const (
	// InvoicePaymentStatusUnpaid means no payment has been detected.
	InvoicePaymentStatusUnpaid InvoicePaymentStatus = "unpaid"
	// InvoicePaymentStatusPartiallyPaid means less than the requested amount has been detected.
	InvoicePaymentStatusPartiallyPaid InvoicePaymentStatus = "partially_paid"
	// InvoicePaymentStatusConfirming means payment has been detected and is still confirming.
	InvoicePaymentStatusConfirming InvoicePaymentStatus = "confirming"
	// InvoicePaymentStatusPaid means enough funds have been received for automatic fulfillment.
	InvoicePaymentStatusPaid InvoicePaymentStatus = "paid"
	// InvoicePaymentStatusSettling means payment is complete and settlement is in progress.
	InvoicePaymentStatusSettling InvoicePaymentStatus = "settling"
	// InvoicePaymentStatusSettled means payment settlement is complete.
	InvoicePaymentStatusSettled InvoicePaymentStatus = "settled"
	// InvoicePaymentStatusReviewRequired means payment requires manual review.
	InvoicePaymentStatusReviewRequired InvoicePaymentStatus = "review_required"
)

// InvoicePaidStatus is a status that can emit an invoice.paid webhook.
type InvoicePaidStatus string

const (
	// InvoicePaidStatusPaid means the invoice is ready for automatic fulfillment.
	InvoicePaidStatusPaid InvoicePaidStatus = "paid"
	// InvoicePaidStatusSettling means the invoice is paid and settlement is in progress.
	InvoicePaidStatusSettling InvoicePaidStatus = "settling"
	// InvoicePaidStatusSettled means the invoice has settled.
	InvoicePaidStatusSettled InvoicePaidStatus = "settled"
)

// MonitoringStatus is the server-computed state of the invoice's deposit-address monitoring window.
type MonitoringStatus string

const (
	// MonitoringStatusActive means the invoice's deposit address is still being watched for payments.
	MonitoringStatusActive MonitoringStatus = "active"
	// MonitoringStatusEnded means the invoice's deposit address is no longer being watched.
	MonitoringStatusEnded MonitoringStatus = "ended"
)

// APIErrorLocation is the location of a field-level API error.
type APIErrorLocation string

const (
	// APIErrorLocationQuery indicates a query parameter error.
	APIErrorLocationQuery APIErrorLocation = "query"
	// APIErrorLocationPath indicates a path parameter error.
	APIErrorLocationPath APIErrorLocation = "path"
	// APIErrorLocationBody indicates a request body error.
	APIErrorLocationBody APIErrorLocation = "body"
	// APIErrorLocationHeader indicates a request header error.
	APIErrorLocationHeader APIErrorLocation = "header"
)

// APIErrorField is a field-level validation error returned by the invoq API.
type APIErrorField struct {
	Field    string           `json:"field"`
	Location APIErrorLocation `json:"location"`
	Code     string           `json:"code"`
	Message  string           `json:"message"`
}

// DirectOnchainRail is a direct-onchain payment rail returned with invoice payment instructions.
type DirectOnchainRail struct {
	ChainNamespace string  `json:"chain_namespace"`
	ChainReference string  `json:"chain_reference"`
	TokenAddress   string  `json:"token_address"`
	NetworkLabel   string  `json:"network_label"`
	DisplaySymbol  string  `json:"display_symbol"`
	LogoURL        *string `json:"logo_url"`
	ChainLogoURL   *string `json:"chain_logo_url"`
	NetworkFeeUSD  string  `json:"network_fee_usd"`
	ETASeconds     int64   `json:"eta_seconds"`
}

// PublicInvoiceProject is payer-visible project branding returned by public invoice reads.
type PublicInvoiceProject struct {
	ID      string  `json:"id"`
	Name    *string `json:"name"`
	LogoURL *string `json:"logo_url"`
}

// PublicInvoiceTransfer is one confirmed inbound transfer credited to the invoice, part of the payer-facing receipt trail.
type PublicInvoiceTransfer struct {
	TxHash        string  `json:"tx_hash"`
	Amount        string  `json:"amount"`
	ExplorerTxURL *string `json:"explorer_tx_url"`
}

// Invoice is returned when creating an invoice.
type Invoice struct {
	ID                 string              `json:"id"`
	Mode               InvoiceMode         `json:"mode"`
	Amount             string              `json:"amount"`
	Currency           InvoiceCurrency     `json:"currency"`
	ReferenceID        *string             `json:"reference_id"`
	Description        *string             `json:"description"`
	ReturnURL          *string             `json:"return_url"`
	DepositAddress     *string             `json:"deposit_address"`
	Status             InvoiceStatus       `json:"status"`
	AmountDue          string              `json:"amount_due"`
	AmountOverpaid     string              `json:"amount_overpaid"`
	MonitoringEndsAt   *string             `json:"monitoring_ends_at"`
	MonitoringStatus   *MonitoringStatus   `json:"monitoring_status"`
	DirectOnchainRails []DirectOnchainRail `json:"direct_onchain_rails"`
}

// TestPaymentInvoice is returned after simulating payment on a test invoice.
type TestPaymentInvoice struct {
	Invoice
	AmountPaid  string  `json:"amount_paid"`
	FullyPaidAt *string `json:"fully_paid_at"`
}

// PublicInvoice is returned when fetching an invoice by ID.
type PublicInvoice struct {
	ID                 string                  `json:"id"`
	Mode               InvoiceMode             `json:"mode"`
	Amount             string                  `json:"amount"`
	Currency           InvoiceCurrency         `json:"currency"`
	Description        *string                 `json:"description"`
	ReturnURL          *string                 `json:"return_url"`
	DepositAddress     *string                 `json:"deposit_address"`
	Status             InvoiceStatus           `json:"status"`
	AmountDue          string                  `json:"amount_due"`
	AmountOverpaid     string                  `json:"amount_overpaid"`
	MonitoringEndsAt   *string                 `json:"monitoring_ends_at"`
	MonitoringStatus   *MonitoringStatus       `json:"monitoring_status"`
	DirectOnchainRails []DirectOnchainRail     `json:"direct_onchain_rails"`
	AmountPaid         string                  `json:"amount_paid"`
	PaymentStatus      InvoicePaymentStatus    `json:"payment_status"`
	Project            PublicInvoiceProject    `json:"project"`
	Transfers          []PublicInvoiceTransfer `json:"transfers"`
}

// CreateInvoiceInput is the input for creating an invoice.
type CreateInvoiceInput struct {
	Amount      string          `json:"amount"`
	Currency    InvoiceCurrency `json:"currency,omitempty"`
	Description *string         `json:"description,omitempty"`
	ReferenceID *string         `json:"reference_id,omitempty"`
	ReturnURL   *NullableString `json:"return_url,omitempty"`
}

// CreateTestPaymentInput is the input for creating a test payment.
type CreateTestPaymentInput struct {
	Amount      string  `json:"amount"`
	ReferenceID *string `json:"reference_id,omitempty"`
}

// InvoicePaidEventInvoice is the invoice payload for an invoice.paid event.
type InvoicePaidEventInvoice struct {
	ID          string            `json:"id"`
	Mode        InvoiceMode       `json:"mode"`
	Status      InvoicePaidStatus `json:"status"`
	Amount      string            `json:"amount"`
	Currency    InvoiceCurrency   `json:"currency"`
	AmountPaid  string            `json:"amount_paid"`
	ReferenceID *string           `json:"reference_id"`
	FullyPaidAt *string           `json:"fully_paid_at"`
}

// InvoicePaidEventData is the data payload for an invoice.paid event.
type InvoicePaidEventData struct {
	Invoice InvoicePaidEventInvoice `json:"invoice"`
}

// InvoicePaidEvent is the known invoice.paid webhook event.
type InvoicePaidEvent struct {
	ID        string               `json:"id"`
	Type      string               `json:"type"`
	Mode      InvoiceMode          `json:"mode"`
	CreatedAt string               `json:"created_at"`
	Data      InvoicePaidEventData `json:"data"`
}

// WebhookEvent is a verified webhook event payload. Unknown future event types are preserved.
type WebhookEvent map[string]any

// NullableString represents a nullable optional string request field.
type NullableString struct {
	value *string
}

// MarshalJSON encodes a nullable string request field.
func (value *NullableString) MarshalJSON() ([]byte, error) {
	if value == nil || value.value == nil {
		return []byte("null"), nil
	}

	return json.Marshal(*value.value)
}

// String returns a string pointer for optional request fields.
func String(value string) *string {
	return &value
}

// StringOrNull returns a nullable string containing value.
func StringOrNull(value string) *NullableString {
	return &NullableString{value: &value}
}

// NullString returns a nullable string containing JSON null.
func NullString() *NullableString {
	return &NullableString{}
}
