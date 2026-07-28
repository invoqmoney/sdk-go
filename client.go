package invoq

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// DefaultAPIOrigin is the production invoq API origin.
const DefaultAPIOrigin = "https://api.invoq.money"

// DefaultTimeout is the default request timeout for invoq API calls.
// Invoice calls sit in a buyer's checkout path, so hung requests should fail fast by default.
const DefaultTimeout = 10 * time.Second

const maxTimeout = time.Duration(4_294_967_295) * time.Millisecond

var defaultHTTPClient = &http.Client{}

// Client is a client for invoq server APIs.
type Client struct {
	Invoices *Invoices
}

// Option configures a client.
type Option interface {
	apply(*clientOptions)
}

type optionFunc func(*clientOptions)

func (fn optionFunc) apply(options *clientOptions) {
	fn(options)
}

type clientOptions struct {
	apiOrigin  string
	httpClient *http.Client
	timeout    time.Duration
}

// WithAPIOrigin uses a custom API origin.
func WithAPIOrigin(apiOrigin string) Option {
	return optionFunc(func(options *clientOptions) {
		options.apiOrigin = apiOrigin
	})
}

// WithHTTPClient uses a custom HTTP client.
func WithHTTPClient(httpClient *http.Client) Option {
	return optionFunc(func(options *clientOptions) {
		options.httpClient = httpClient
	})
}

// WithTimeout uses a custom request timeout.
func WithTimeout(timeout time.Duration) Option {
	return optionFunc(func(options *clientOptions) {
		options.timeout = timeout
	})
}

// New creates a client using the production invoq API by default.
func New(apiKey string, options ...Option) (*Client, error) {
	if strings.TrimSpace(apiKey) == "" {
		return nil, configurationError("invoq API key must be a non-empty string.")
	}

	// The transport refuses this as a connection error, which sends the caller to
	// debug their network. Name it here, the same way every SDK does.
	if strings.ContainsFunc(apiKey, func(r rune) bool { return r < 0x20 || r == 0x7F }) {
		return nil, configurationError("invoq API key must not contain control characters.")
	}

	resolvedOptions := clientOptions{
		apiOrigin:  DefaultAPIOrigin,
		httpClient: defaultHTTPClient,
		timeout:    DefaultTimeout,
	}

	for _, option := range options {
		if option != nil {
			option.apply(&resolvedOptions)
		}
	}

	apiOrigin, err := normalizeAPIOrigin(resolvedOptions.apiOrigin)
	if err != nil {
		return nil, err
	}

	if resolvedOptions.timeout <= 0 || resolvedOptions.timeout > maxTimeout {
		return nil, configurationError("timeout must be a positive duration of at most 4294967295 milliseconds.")
	}

	if resolvedOptions.httpClient == nil {
		return nil, configurationError("http client must not be nil.")
	}

	requestOptions := requestClientOptions{
		apiKey:     apiKey,
		apiOrigin:  apiOrigin,
		httpClient: resolvedOptions.httpClient,
		timeout:    resolvedOptions.timeout,
	}

	return &Client{
		Invoices: &Invoices{clientOptions: requestOptions},
	}, nil
}

// Invoices contains invoice API operations.
type Invoices struct {
	clientOptions requestClientOptions
}

// Value receivers, so a dereferenced copy is covered too. fmt prints <nil> for a
// nil pointer without calling these.
func (invoices Invoices) String() string {
	return "invoq.Invoices{apiOrigin: " + invoices.clientOptions.apiOrigin.String() + "}"
}

func (invoices Invoices) GoString() string {
	return invoices.String()
}

// Create creates an invoice.
func (invoices *Invoices) Create(ctx context.Context, input CreateInvoiceInput) (*Invoice, error) {
	amount, err := requiredRequestString(input.Amount, "amount")
	if err != nil {
		return nil, err
	}

	input.Amount = amount

	return requestJSON[Invoice](ctx, invoices.clientOptions, http.MethodPost, []string{"v1", "invoices"}, input)
}

// Get gets an invoice by ID.
func (invoices *Invoices) Get(ctx context.Context, invoiceID string) (*PublicInvoice, error) {
	id, err := requiredPathSegment(invoiceID, "invoiceId")
	if err != nil {
		return nil, err
	}

	return requestJSON[PublicInvoice](ctx, invoices.clientOptions, http.MethodGet, []string{"v1", "invoices", id}, nil)
}

// CreateTestPayment creates a test payment for a test invoice.
func (invoices *Invoices) CreateTestPayment(ctx context.Context, invoiceID string, input CreateTestPaymentInput) (*TestPaymentInvoice, error) {
	id, err := requiredPathSegment(invoiceID, "invoiceId")
	if err != nil {
		return nil, err
	}

	amount, err := requiredRequestString(input.Amount, "amount")
	if err != nil {
		return nil, err
	}

	input.Amount = amount

	return requestJSON[TestPaymentInvoice](ctx, invoices.clientOptions, http.MethodPost, []string{"v1", "invoices", id, "test-payments"}, input)
}

func normalizeAPIOrigin(value string) (*url.URL, error) {
	apiURL, err := url.Parse(value)
	if err != nil {
		return nil, configurationError("apiOrigin must be an absolute http or https origin.")
	}

	if !apiURL.IsAbs() || apiURL.Host == "" || apiURL.Hostname() == "" || (apiURL.Scheme != "http" && apiURL.Scheme != "https") {
		return nil, configurationError("apiOrigin must be an absolute http or https origin.")
	}

	if !isValidAPIOriginHost(apiURL.Host) {
		return nil, configurationError("apiOrigin must be an absolute http or https origin.")
	}

	if apiURL.RawQuery != "" || apiURL.ForceQuery || apiURL.Fragment != "" {
		return nil, configurationError("apiOrigin must not include query or hash parts.")
	}

	if apiURL.User != nil {
		return nil, configurationError("apiOrigin must be an absolute http or https origin.")
	}

	pathname := strings.TrimRight(apiURL.EscapedPath(), "/")
	if pathname == "" {
		pathname = "/"
	}

	if pathname != "/" {
		return nil, configurationError("apiOrigin must not include a path.")
	}

	apiURL.Path = "/"
	apiURL.RawPath = ""
	apiURL.RawQuery = ""
	apiURL.Fragment = ""
	apiURL.ForceQuery = false

	return apiURL, nil
}

func isValidAPIOriginHost(host string) bool {
	if host == "" {
		return false
	}

	if strings.HasPrefix(host, "[") {
		bracketIndex := strings.LastIndex(host, "]")
		if bracketIndex == -1 {
			return false
		}

		portPart := host[bracketIndex+1:]
		if portPart == "" {
			return true
		}

		return strings.HasPrefix(portPart, ":") && isValidAPIOriginPort(portPart[1:])
	}

	colonIndex := strings.LastIndex(host, ":")
	if colonIndex == -1 {
		return true
	}

	if strings.Contains(host[:colonIndex], ":") {
		return false
	}

	return isValidAPIOriginPort(host[colonIndex+1:])
}

func isValidAPIOriginPort(value string) bool {
	if value == "" {
		return false
	}

	for _, character := range value {
		if character < '0' || character > '9' {
			return false
		}
	}

	_, err := strconv.ParseUint(value, 10, 16)
	return err == nil
}

// A URL resolver pops "." and "..", so an id of either would call a different
// endpoint instead of 404ing. Percent-encoding is no help: normalization is first.
func requiredPathSegment(value string, fieldName string) (string, error) {
	segment, err := requiredRequestString(value, fieldName)
	if err != nil {
		return "", err
	}

	if segment == "." || segment == ".." {
		return "", configurationError(fieldName + " must not be a path segment that resolves ('.' or '..').")
	}

	return segment, nil
}

func requiredRequestString(value string, fieldName string) (string, error) {
	if strings.TrimSpace(value) == "" {
		return "", configurationError(fieldName + " must be a non-empty string.")
	}

	return value, nil
}
