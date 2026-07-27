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

// InvoiceStatus is the accounting status returned with invoice API responses.
// Paid, settling, and settled all mean the buyer paid and differ only in how far
// the funds have moved; review_required is not a paid state.
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

// InvoicePaidStatus is a status that can carry an invoice.paid webhook.
type InvoicePaidStatus string

const (
	// InvoicePaidStatusPaid means the invoice is ready for automatic fulfillment.
	InvoicePaidStatusPaid InvoicePaidStatus = "paid"
	// InvoicePaidStatusSettling means the invoice is paid and settlement is in progress.
	InvoicePaidStatusSettling InvoicePaidStatus = "settling"
	// InvoicePaidStatusSettled means the invoice has settled.
	InvoicePaidStatusSettled InvoicePaidStatus = "settled"
)

// CheckoutStatus is the payer-facing state, derived on every response. It never
// authorizes fulfillment: use the invoice.paid webhook for that.
type CheckoutStatus string

const (
	// CheckoutStatusPaid means the invoice has been paid in full.
	CheckoutStatusPaid CheckoutStatus = "paid"
	// CheckoutStatusConfirming means payment evidence is on chain but not confirmed yet.
	CheckoutStatusConfirming CheckoutStatus = "confirming"
	// CheckoutStatusExpired means the payment window closed at monitoring_ends_at.
	CheckoutStatusExpired CheckoutStatus = "expired"
	// CheckoutStatusOpen means at least one payment option is ready to pay.
	CheckoutStatusOpen CheckoutStatus = "open"
	// CheckoutStatusUnavailable means no payment option can be paid right now.
	CheckoutStatusUnavailable CheckoutStatus = "unavailable"
)

// ChainNamespace is the namespace of a chain invoq supports.
type ChainNamespace string

const (
	// ChainNamespaceEIP155 is the EVM chain namespace.
	ChainNamespaceEIP155 ChainNamespace = "eip155"
	// ChainNamespaceSolana is the Solana chain namespace.
	ChainNamespaceSolana ChainNamespace = "solana"
	// ChainNamespaceTron is the TRON chain namespace.
	ChainNamespaceTron ChainNamespace = "tron"
)

// PaymentOptionCollectionMethod is how a payment option collects funds.
type PaymentOptionCollectionMethod string

const (
	// PaymentOptionCollectionMethodEVMDeposit collects any on-time transfer to a per-invoice deposit address.
	PaymentOptionCollectionMethodEVMDeposit PaymentOptionCollectionMethod = "evm_deposit"
	// PaymentOptionCollectionMethodDirectExact collects one exact-amount transfer to the merchant's own address.
	PaymentOptionCollectionMethodDirectExact PaymentOptionCollectionMethod = "direct_exact"
)

// PaymentOptionStatus is whether a payment option can be paid right now.
type PaymentOptionStatus string

const (
	// PaymentOptionStatusReady means the option carries payment instructions and can be paid.
	PaymentOptionStatusReady PaymentOptionStatus = "ready"
	// PaymentOptionStatusUnavailable means the option carries no payment instructions.
	PaymentOptionStatusUnavailable PaymentOptionStatus = "unavailable"
)

// WebhookEventType is a webhook event type this SDK models.
type WebhookEventType string

const (
	// WebhookEventTypeInvoicePaid is the invoice.paid event type.
	WebhookEventTypeInvoicePaid WebhookEventType = "invoice.paid"
	// WebhookEventTypeInvoicePaymentReversed is the invoice.payment_reversed event type.
	WebhookEventTypeInvoicePaymentReversed WebhookEventType = "invoice.payment_reversed"
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

// PaymentOption is one issued way to pay an invoice, fixed when the invoice is
// created: a receiving address or rail configured later never rewrites it. Only
// Status is re-evaluated per response, and only PaymentOptionStatusReady can be
// paid.
//
// The instruction fields below are carried by one (Status, CollectionMethod)
// combination each and are nil on every other one, so a nil pointer means the
// field was absent from the response rather than empty. Identify an option by
// the (ChainNamespace, ChainReference, TokenAddress) triple, never by its
// position in the slice.
type PaymentOption struct {
	CollectionMethod PaymentOptionCollectionMethod `json:"collection_method"`
	ChainNamespace   ChainNamespace                `json:"chain_namespace"`
	ChainReference   string                        `json:"chain_reference"`
	Currency         InvoiceCurrency               `json:"currency"`
	TokenAddress     string                        `json:"token_address"`
	TokenDecimals    int64                         `json:"token_decimals"`
	NetworkLabel     string                        `json:"network_label"`
	DisplaySymbol    string                        `json:"display_symbol"`
	LogoURL          *string                       `json:"logo_url"`
	ChainLogoURL     *string                       `json:"chain_logo_url"`
	Status           PaymentOptionStatus           `json:"status"`

	// DepositAddress is set on ready evm_deposit options only. The address
	// belongs to this invoice alone, and any on-time transfer to it is credited.
	DepositAddress *string `json:"deposit_address,omitempty"`
	// SuggestedAmount is set on ready evm_deposit options only. It is guidance,
	// not a matching requirement, and can exceed AmountDue by one token unit.
	SuggestedAmount *string `json:"suggested_amount,omitempty"`

	// RecipientAddress is set on ready direct_exact options only. It is the
	// merchant's own address.
	RecipientAddress *string `json:"recipient_address,omitempty"`
	// InvoiceAmount is set on ready direct_exact options only.
	InvoiceAmount *string `json:"invoice_amount,omitempty"`
	// MatchingIncrement is set on ready direct_exact options only. It attributes
	// the payment to this invoice and is never credited as invoice payment.
	MatchingIncrement *string `json:"matching_increment,omitempty"`
	// ExactAmount is set on ready direct_exact options only. The buyer must send
	// exactly this amount, InvoiceAmount plus MatchingIncrement, in one transfer.
	// All three carry exactly TokenDecimals fractional digits.
	ExactAmount *string `json:"exact_amount,omitempty"`
}

// PublicInvoiceProject is payer-visible project branding returned by public invoice reads.
type PublicInvoiceProject struct {
	ID      string  `json:"id"`
	Name    *string `json:"name"`
	LogoURL *string `json:"logo_url"`
}

// PublicInvoiceTransfer is one confirmed inbound transfer credited to the
// invoice, part of the payer-facing receipt trail. Amount is in invoice currency
// at the scale of AmountPaid and excludes a direct_exact matching increment.
// TransactionID is not unique on its own: one transaction can carry several
// credits, which EventIndex separates.
type PublicInvoiceTransfer struct {
	ChainNamespace         ChainNamespace `json:"chain_namespace"`
	ChainReference         string         `json:"chain_reference"`
	TransactionID          string         `json:"transaction_id"`
	EventIndex             int64          `json:"event_index"`
	Amount                 string         `json:"amount"`
	ExplorerTransactionURL *string        `json:"explorer_transaction_url"`
}

// Invoice is returned when creating an invoice.
type Invoice struct {
	ID          string          `json:"id"`
	Mode        InvoiceMode     `json:"mode"`
	Amount      string          `json:"amount"`
	Currency    InvoiceCurrency `json:"currency"`
	ReferenceID *string         `json:"reference_id"`
	Description *string         `json:"description"`
	ReturnURL   *string         `json:"return_url"`
	Status      InvoiceStatus   `json:"status"`
	// CheckoutStatus is payer-facing and never authorizes fulfillment.
	CheckoutStatus CheckoutStatus `json:"checkout_status"`
	// PaymentRevision increments whenever the confirmed payment set changes;
	// settlement alone does not move it. Use it to discard a snapshot older than
	// one you already hold.
	PaymentRevision int64 `json:"payment_revision"`
	// AmountDue and AmountOverpaid are max(amount - amount_paid, 0) and
	// max(amount_paid - amount, 0), both at the 18-decimal scale of amount_paid.
	// Read these instead of subtracting money yourself.
	AmountDue      string `json:"amount_due"`
	AmountOverpaid string `json:"amount_overpaid"`
	// MonitoringEndsAt is one day after creation and is the only payment window.
	// It is nil in test mode.
	MonitoringEndsAt *string `json:"monitoring_ends_at"`
	// PaymentOptions is the only place payment instructions live. It is empty in
	// test mode.
	PaymentOptions []PaymentOption `json:"payment_options"`
}

// TestPaymentInvoice is returned after simulating payment on a test invoice.
type TestPaymentInvoice struct {
	Invoice
	AmountPaid  string  `json:"amount_paid"`
	FullyPaidAt *string `json:"fully_paid_at"`
}

// PublicInvoice is returned when fetching an invoice by ID. It is the create
// shape plus Project, AmountPaid, and Transfers, minus ReferenceID.
type PublicInvoice struct {
	ID              string               `json:"id"`
	Mode            InvoiceMode          `json:"mode"`
	Amount          string               `json:"amount"`
	Currency        InvoiceCurrency      `json:"currency"`
	Description     *string              `json:"description"`
	ReturnURL       *string              `json:"return_url"`
	Project         PublicInvoiceProject `json:"project"`
	Status          InvoiceStatus        `json:"status"`
	CheckoutStatus  CheckoutStatus       `json:"checkout_status"`
	PaymentRevision int64                `json:"payment_revision"`
	AmountPaid      string               `json:"amount_paid"`
	AmountDue       string               `json:"amount_due"`
	AmountOverpaid  string               `json:"amount_overpaid"`
	// Transfers holds the confirmed receipts, at most the 20 largest, largest
	// first. It is empty in test mode.
	Transfers        []PublicInvoiceTransfer `json:"transfers"`
	MonitoringEndsAt *string                 `json:"monitoring_ends_at"`
	PaymentOptions   []PaymentOption         `json:"payment_options"`
}

// CreateInvoiceInput is the input for creating an invoice. These four fields are
// the whole request body: currency is fixed to USD, mode comes from the API key,
// and the API rejects unknown body keys.
type CreateInvoiceInput struct {
	Amount      string          `json:"amount"`
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
// Payment instructions and return_url are absent by design: reconcile by invoice
// ID plus ReferenceID.
type InvoicePaidEventInvoice struct {
	ID              string            `json:"id"`
	Mode            InvoiceMode       `json:"mode"`
	Status          InvoicePaidStatus `json:"status"`
	Amount          string            `json:"amount"`
	Currency        InvoiceCurrency   `json:"currency"`
	AmountPaid      string            `json:"amount_paid"`
	ReferenceID     *string           `json:"reference_id"`
	PaymentRevision int64             `json:"payment_revision"`
	FullyPaidAt     *string           `json:"fully_paid_at"`
}

// InvoicePaidEventData is the data payload for an invoice.paid event.
type InvoicePaidEventData struct {
	Invoice InvoicePaidEventInvoice `json:"invoice"`
}

// InvoicePaidEvent is the known invoice.paid webhook event.
type InvoicePaidEvent struct {
	ID        string               `json:"id"`
	Type      WebhookEventType     `json:"type"`
	Mode      InvoiceMode          `json:"mode"`
	CreatedAt string               `json:"created_at"`
	Data      InvoicePaidEventData `json:"data"`
}

// InvoicePaymentReversedEventInvoice is the invoice payload for an
// invoice.payment_reversed event. It carries the invoice's current status, which
// is any InvoiceStatus, a higher PaymentRevision than the payment it reverses,
// and a nil FullyPaidAt.
type InvoicePaymentReversedEventInvoice struct {
	ID              string          `json:"id"`
	Mode            InvoiceMode     `json:"mode"`
	Status          InvoiceStatus   `json:"status"`
	Amount          string          `json:"amount"`
	Currency        InvoiceCurrency `json:"currency"`
	AmountPaid      string          `json:"amount_paid"`
	ReferenceID     *string         `json:"reference_id"`
	PaymentRevision int64           `json:"payment_revision"`
	FullyPaidAt     *string         `json:"fully_paid_at"`
}

// InvoicePaymentReversedEventData is the data payload for an invoice.payment_reversed event.
type InvoicePaymentReversedEventData struct {
	Invoice InvoicePaymentReversedEventInvoice `json:"invoice"`
}

// InvoicePaymentReversedEvent is the known invoice.payment_reversed webhook
// event, sent when a previously paid invoice drops back below its amount, a
// chain reorg removing a credited transfer for example.
type InvoicePaymentReversedEvent struct {
	ID        string                          `json:"id"`
	Type      WebhookEventType                `json:"type"`
	Mode      InvoiceMode                     `json:"mode"`
	CreatedAt string                          `json:"created_at"`
	Data      InvoicePaymentReversedEventData `json:"data"`
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
