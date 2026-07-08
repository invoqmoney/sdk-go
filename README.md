# invoq Go SDK

Go SDK for invoq server APIs and webhook verification. Create stablecoin
invoices, simulate test payments, and fulfill orders from signed webhooks.

Use this module only on your server. It accepts secret keys and must not be
compiled into client-side applications.

## Installation

```sh
go get github.com/invoqmoney/sdk-go
```

Requires Go 1.22 or newer.

## Get your keys

1. Sign in to the [invoq dashboard](https://app.invoq.money) and create a
   project.
2. On the **API keys** page, create a secret key. Test keys start with
   `sk_test_`, live keys with `sk_live_`. The key mode determines whether
   invoices are test or live.
3. In your project's **webhooks** settings, save your webhook URL. The webhook
   secret (`whsec_...`) for that mode is shown once when you first enable the
   webhook, so store it right away. Webhook URLs must be public HTTPS URLs.

Add both to your server environment:

```sh
INVOQ_SECRET_KEY=sk_test_...
INVOQ_WEBHOOK_SECRET=whsec_...
```

Start with test keys. Switch to the live key and live webhook secret when you
go to production.

## Create a client

```go
package main

import (
	"os"

	invoq "github.com/invoqmoney/sdk-go"
)

func main() {
	client, err := invoq.New(os.Getenv("INVOQ_SECRET_KEY"))
	if err != nil {
		panic(err)
	}

	_ = client
}
```

Production defaults:

- API origin: `https://api.invoq.money`
- Request timeout: 10 seconds
- User-Agent: `invoq-go/<module version>`

Override them during local development or preview testing:

```go
package main

import (
	"os"
	"time"

	invoq "github.com/invoqmoney/sdk-go"
)

func main() {
	client, err := invoq.New(
		os.Getenv("INVOQ_SECRET_KEY"),
		invoq.WithAPIOrigin("http://localhost:8787"),
		invoq.WithTimeout(15 * time.Second),
	)
	if err != nil {
		panic(err)
	}

	_ = client
}
```

`apiOrigin` must be an absolute `http` or `https` origin without username,
password, path, query, or hash parts. The SDK appends `/v1/...` API paths.

## Invoices

Create an invoice from your server:

```go
ctx := context.Background()

invoice, err := client.Invoices.Create(ctx, invoq.CreateInvoiceInput{
	Amount:      "129",
	Currency:    invoq.InvoiceCurrencyUSD,
	Description: invoq.String("SaaS boilerplate"),
	ReferenceID: invoq.String("order_1234"),
	ReturnURL:   invoq.StringOrNull("https://merchant.example/thanks"),
})
if err != nil {
	// Handle the error.
}

_ = invoice.ID
```

Notes:

- Use a server-side amount. Do not trust client-supplied amounts.
- `amount` is a decimal USD string from `0.01` to `999.99` with up to 2 decimal
  places, such as `129` or `129.99`.
- Use `reference_id` to map `invoice.paid` webhooks back to your order. It also
  makes creation retry-safe: creating again with the same `reference_id` and the
  same invoice terms returns the existing invoice instead of a duplicate, while
  different terms fail with a `409 reference_id_conflict` API error.
- Use `invoq.String(...)` for optional request strings. Use
  `invoq.StringOrNull(...)` to set `return_url`, `invoq.NullString()` to send
  JSON `null`, and leave the field unset to omit it.

Get a public invoice:

```go
invoice, err := client.Invoices.Get(ctx, "inv_123")
```

`Get` returns `*invoq.PublicInvoice`. Public invoice reads do not include your
private `reference_id`; use webhooks or your own order store for fulfillment
mapping.

## Test payments

Test invoices cannot receive real funds. Simulate a payment from your server:

```go
paidInvoice, err := client.Invoices.CreateTestPayment(ctx, invoice.ID, invoq.CreateTestPaymentInput{
	Amount: invoice.Amount,
})
if err != nil {
	// Handle the error.
}

_ = paidInvoice.Status // invoq.InvoiceStatusPaid when fully paid
```

`CreateTestPayment` only works on invoices created with a `sk_test_` key. When
payments reach the invoice amount, the invoice becomes `paid` and invoq sends a
real signed `invoice.paid` webhook to your test webhook URL. Partial amounts are
allowed and produce `partially_paid`.

To receive webhooks on your machine, expose your local server with an HTTPS
tunnel such as ngrok or cloudflared and save the tunnel URL as your test webhook
URL in the dashboard. The dashboard can also send a signed `webhook.ping` to
check connectivity.

## Webhooks

Pass the raw request body to `VerifyWebhook`. Do not parse JSON and re-serialize
it before verification.

```go
func handleWebhook(response http.ResponseWriter, request *http.Request) {
	body, err := io.ReadAll(request.Body)
	if err != nil {
		http.Error(response, "Invalid request body.", http.StatusBadRequest)
		return
	}

	event, err := invoq.VerifyWebhook(
		body,
		request.Header,
		os.Getenv("INVOQ_WEBHOOK_SECRET"),
	)
	if err != nil {
		http.Error(response, "Invalid webhook signature.", http.StatusBadRequest)
		return
	}

	if invoq.IsInvoicePaid(event) {
		invoicePaid, _ := invoq.AsInvoicePaidEvent(event)
		referenceID := invoicePaid.Data.Invoice.ReferenceID
		if referenceID == nil {
			http.Error(response, "Missing invoice reference_id.", http.StatusBadRequest)
			return
		}

		// Fulfill the order for *referenceID idempotently.
	}

	response.WriteHeader(http.StatusOK)
}
```

Use `invoice.paid` webhooks to fulfill orders on your server. When
`IsInvoicePaid(event)` is true, the invoice is ready for automatic fulfillment;
its status is `paid`, `settling`, or `settled`. A `review_required` invoice does
not emit an `invoice.paid` webhook yet. Wait for a later `invoice.paid` webhook
after review is approved.

Failed deliveries are retried, so fulfill idempotently by `reference_id` or
invoice `id` and make repeat deliveries a no-op. Respond with a 2xx quickly; any
other status counts as a failed delivery.

`VerifyWebhook` accepts `http.Header`. Use `VerifyWebhookWithSignature` when you
already have the `invoq-signature` header value.

Webhook verification failures return `*invoq.SignatureVerificationError`. The
SDK allows a 5-minute timestamp tolerance. Failed deliveries are signed again
on each retry, so normal retried deliveries still verify inside that window.
The signature header is `invoq-signature: t=<unix seconds>,v1=<hex HMAC-SHA256
of "<t>.<raw body>">`.

## Errors

```go
invoice, err := client.Invoices.Create(ctx, invoq.CreateInvoiceInput{Amount: "0.001"})
if err != nil {
	var apiErr *invoq.APIError
	if errors.As(err, &apiErr) {
		fmt.Println(apiErr.Status)
		fmt.Println(apiErr.Code)
		fmt.Println(apiErr.Fields)
		fmt.Println(apiErr.Meta)
	}

	_ = invoice
}
```

Connection failures, request timeouts, configuration errors, and response parse
failures return `*invoq.Error`. API non-2xx responses return `*invoq.APIError`.
All SDK errors implement `invoq.SDKError`.

```go
var sdkErr invoq.SDKError
if errors.As(err, &sdkErr) {
	fmt.Println(sdkErr)
}
```

## API reference

```go
client, err := invoq.New(apiKey,
	invoq.WithAPIOrigin("https://api.invoq.money"), // optional override
	invoq.WithTimeout(invoq.DefaultTimeout),        // optional request timeout
)
```

- `client.Invoices.Create(ctx, input)` creates an invoice. `input`: `Amount`
  (required), `Currency` (`InvoiceCurrencyUSD`, default), `Description`,
  `ReferenceID`, `ReturnURL`.
- `client.Invoices.Get(ctx, invoiceID)` fetches a public invoice and returns
  `*invoq.PublicInvoice`.
- `client.Invoices.CreateTestPayment(ctx, invoiceID, input)` simulates payment
  on a test invoice and returns `*invoq.TestPaymentInvoice`.
- `invoq.VerifyWebhook(rawBody, headers, webhookSecret)` verifies a webhook and
  returns `invoq.WebhookEvent`.
- `invoq.IsInvoicePaid(event)` and `invoq.AsInvoicePaidEvent(event)` identify
  typed `invoice.paid` events.
- The SDK detects its Go module version from build info for the `User-Agent`.
  Released tags such as `v0.1.0` are sent without the `v` prefix; local source
  builds without a module version use `unknown`.
