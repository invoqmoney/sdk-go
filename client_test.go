package invoq

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestValidatesAPIKeysAPIOriginAndTimeout(t *testing.T) {
	if _, err := New(""); !isSDKError(err) {
		t.Fatalf("expected SDK error for empty API key, got %T", err)
	}

	invalidURLs := []string{
		"ftp://api.test",
		"https://api.test/api",
		"https://api.test/v1",
		"https://api.test?debug=1",
		"https://api.test/%2F",
		"https://api.test/%76%31",
		"https://user:pass@api.test",
		"http://:8787",
		"http://api.test:bad",
		"http://api.test:+80",
		"http://api.test:-1",
		"http://api.test:99999",
		"http://api.test:",
	}

	for _, invalidURL := range invalidURLs {
		if _, err := New("sk_test_123", WithAPIOrigin(invalidURL)); !isSDKError(err) {
			t.Fatalf("expected SDK error for %q, got %T", invalidURL, err)
		}
	}

	if _, err := New("sk_test_123", WithTimeout(0)); !isSDKError(err) {
		t.Fatalf("expected SDK error for zero timeout, got %T", err)
	}
	if _, err := New("sk_test_123", WithTimeout(maxTimeout+time.Nanosecond)); !isSDKError(err) {
		t.Fatalf("expected SDK error for too-large timeout, got %T", err)
	}
}

func TestNormalizesAPIOrigin(t *testing.T) {
	apiURL, err := normalizeAPIOrigin("https://api.test/")
	if err != nil {
		t.Fatal(err)
	}

	if apiURL.String() != "https://api.test/" {
		t.Fatalf("unexpected normalized URL: %s", apiURL.String())
	}
}

func TestDefaultHTTPClientFollowsRedirects(t *testing.T) {
	if defaultHTTPClient.Timeout != 0 {
		t.Fatalf(
			"expected SDK timeout to be enforced by request context, got http client timeout %s",
			defaultHTTPClient.Timeout,
		)
	}
	if defaultHTTPClient.CheckRedirect != nil {
		t.Fatal("expected default HTTP client to use the standard redirect behavior")
	}

	var followedRedirect atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/v1/redirect-target" {
			followedRedirect.Store(true)
			response.Header().Set("Content-Type", "application/json")
			_, _ = response.Write([]byte(`{"data":` + publicInvoiceJSON("unpaid") + `}`))
			return
		}

		if request.URL.Path != "/v1/invoices/inv_redirect" {
			t.Fatalf("unexpected request path: %s", request.URL.Path)
		}

		response.Header().Set("Location", "/v1/redirect-target")
		response.WriteHeader(http.StatusFound)
	}))
	defer server.Close()

	client, err := New("sk_test_123", WithAPIOrigin(server.URL))
	if err != nil {
		t.Fatal(err)
	}

	invoice, err := client.Invoices.Get(context.Background(), "inv_redirect")
	if err != nil {
		t.Fatal(err)
	}
	if invoice.ID != "inv_test_123" {
		t.Fatalf("unexpected invoice ID: %s", invoice.ID)
	}
	if !followedRedirect.Load() {
		t.Fatal("default HTTP client did not follow redirect")
	}
}

func TestCreatesInvoicesWithNativeJSONAndAuthorizationHeaders(t *testing.T) {
	received := make(chan *http.Request, 1)
	receivedBody := make(chan []byte, 1)
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		body, err := io.ReadAll(request.Body)
		if err != nil {
			t.Errorf("read request body: %v", err)
		}
		received <- request
		receivedBody <- body

		response.Header().Set("Content-Type", "application/json")
		response.WriteHeader(http.StatusCreated)
		_, _ = response.Write([]byte(`{"data":` + secretInvoiceJSON("unpaid") + `}`))
	}))
	defer server.Close()

	client, err := New("sk_test_123", WithAPIOrigin(server.URL))
	if err != nil {
		t.Fatal(err)
	}

	invoice, err := client.Invoices.Create(context.Background(), CreateInvoiceInput{
		Amount:      "149",
		Currency:    InvoiceCurrencyUSD,
		Description: String("Test order"),
		ReferenceID: String("order_123"),
		ReturnURL:   StringOrNull("https://merchant.test/thanks"),
	})
	if err != nil {
		t.Fatal(err)
	}

	request := <-received
	body := <-receivedBody

	if invoice.ID != "inv_test_123" {
		t.Fatalf("unexpected invoice ID: %s", invoice.ID)
	}
	if invoice.AmountDue != "149.000000000000000000" {
		t.Fatalf("unexpected amount due: %s", invoice.AmountDue)
	}
	if invoice.AmountOverpaid != "0.000000000000000000" {
		t.Fatalf("unexpected amount overpaid: %s", invoice.AmountOverpaid)
	}
	if invoice.MonitoringStatus != nil {
		t.Fatalf("expected nil monitoring status, got %#v", invoice.MonitoringStatus)
	}
	if invoice.ReturnURL == nil || *invoice.ReturnURL != "https://merchant.test/thanks" {
		t.Fatalf("unexpected return URL: %#v", invoice.ReturnURL)
	}
	if request.Method != http.MethodPost {
		t.Fatalf("unexpected method: %s", request.Method)
	}
	if request.URL.RequestURI() != "/v1/invoices" {
		t.Fatalf("unexpected request URI: %s", request.URL.RequestURI())
	}
	if got := request.Header.Get("Authorization"); got != "Bearer sk_test_123" {
		t.Fatalf("unexpected authorization header: %s", got)
	}
	if got := request.Header.Get("Accept"); got != "application/json" {
		t.Fatalf("unexpected accept header: %s", got)
	}
	if got := request.Header.Get("Content-Type"); got != "application/json" {
		t.Fatalf("unexpected content-type header: %s", got)
	}
	if got := request.Header.Get("User-Agent"); got != userAgent() {
		t.Fatalf("unexpected user-agent header: %s", got)
	}

	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatal(err)
	}

	expected := map[string]any{
		"amount":       "149",
		"currency":     "USD",
		"description":  "Test order",
		"reference_id": "order_123",
		"return_url":   "https://merchant.test/thanks",
	}
	if !reflect.DeepEqual(payload, expected) {
		t.Fatalf("unexpected request body: %#v", payload)
	}
}

func TestGetsInvoicesByID(t *testing.T) {
	received := make(chan *http.Request, 1)
	receivedBody := make(chan []byte, 1)
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		body, err := io.ReadAll(request.Body)
		if err != nil {
			t.Errorf("read request body: %v", err)
		}
		received <- request
		receivedBody <- body

		response.Header().Set("Content-Type", "application/json")
		_, _ = response.Write([]byte(`{"data":` + publicInvoiceJSON("unpaid") + `}`))
	}))
	defer server.Close()

	client, err := New("sk_test_123", WithAPIOrigin(server.URL))
	if err != nil {
		t.Fatal(err)
	}

	invoice, err := client.Invoices.Get(context.Background(), "inv/test 123")
	if err != nil {
		t.Fatal(err)
	}

	request := <-received
	body := <-receivedBody

	if invoice.ID != "inv_test_123" {
		t.Fatalf("unexpected invoice ID: %s", invoice.ID)
	}
	if invoice.PaymentStatus != InvoicePaymentStatusUnpaid {
		t.Fatalf("unexpected payment status: %s", invoice.PaymentStatus)
	}
	if invoice.Project.Name == nil || *invoice.Project.Name != "Test project" {
		t.Fatalf("unexpected project: %#v", invoice.Project)
	}
	if invoice.AmountOverpaid != "0.000000000000000000" {
		t.Fatalf("unexpected amount overpaid: %s", invoice.AmountOverpaid)
	}
	if invoice.MonitoringStatus != nil {
		t.Fatalf("expected nil monitoring status, got %#v", invoice.MonitoringStatus)
	}
	if invoice.Transfers == nil || len(invoice.Transfers) != 0 {
		t.Fatalf("expected empty transfers, got %#v", invoice.Transfers)
	}
	if request.Method != http.MethodGet {
		t.Fatalf("unexpected method: %s", request.Method)
	}
	if request.URL.RequestURI() != "/v1/invoices/inv%2Ftest%20123" {
		t.Fatalf("unexpected request URI: %s", request.URL.RequestURI())
	}
	if got := request.Header.Get("Content-Type"); got != "" {
		t.Fatalf("unexpected content-type header: %s", got)
	}
	if len(body) != 0 {
		t.Fatalf("expected empty body, got %q", string(body))
	}
}

func TestCreatesTestPaymentsAndReturnsOnlyDataEnvelope(t *testing.T) {
	received := make(chan *http.Request, 1)
	receivedBody := make(chan []byte, 1)
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		body, err := io.ReadAll(request.Body)
		if err != nil {
			t.Errorf("read request body: %v", err)
		}
		received <- request
		receivedBody <- body

		response.Header().Set("Content-Type", "application/json")
		response.WriteHeader(http.StatusCreated)
		_, _ = response.Write([]byte(`{"data":` + testPaymentInvoiceJSON("paid") + `,"meta":{"result":"created"}}`))
	}))
	defer server.Close()

	client, err := New("sk_test_123", WithAPIOrigin(server.URL))
	if err != nil {
		t.Fatal(err)
	}

	invoice, err := client.Invoices.CreateTestPayment(context.Background(), "inv_test_123", CreateTestPaymentInput{
		Amount:      "149",
		ReferenceID: String("test_payment_001"),
	})
	if err != nil {
		t.Fatal(err)
	}

	request := <-received
	body := <-receivedBody

	if invoice.Status != "paid" {
		t.Fatalf("unexpected invoice status: %s", invoice.Status)
	}
	if invoice.AmountPaid != "149" {
		t.Fatalf("unexpected amount paid: %s", invoice.AmountPaid)
	}
	if invoice.AmountDue != "0.000000000000000000" {
		t.Fatalf("unexpected amount due: %s", invoice.AmountDue)
	}
	if invoice.AmountOverpaid != "0.000000000000000000" {
		t.Fatalf("unexpected amount overpaid: %s", invoice.AmountOverpaid)
	}
	if invoice.MonitoringStatus != nil {
		t.Fatalf("expected nil monitoring status, got %#v", invoice.MonitoringStatus)
	}
	if invoice.FullyPaidAt == nil {
		t.Fatal("expected fully paid timestamp")
	}
	if request.URL.RequestURI() != "/v1/invoices/inv_test_123/test-payments" {
		t.Fatalf("unexpected request URI: %s", request.URL.RequestURI())
	}

	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatal(err)
	}

	expected := map[string]any{
		"amount":       "149",
		"reference_id": "test_payment_001",
	}
	if !reflect.DeepEqual(payload, expected) {
		t.Fatalf("unexpected request body: %#v", payload)
	}
}

func TestCreateInputsOmitUnsetOptionalStringsAndSendSetStrings(t *testing.T) {
	omittedBytes, err := json.Marshal(CreateInvoiceInput{Amount: "149"})
	if err != nil {
		t.Fatal(err)
	}
	if string(omittedBytes) != `{"amount":"149"}` {
		t.Fatalf("unexpected omitted payload: %s", string(omittedBytes))
	}

	setBytes, err := json.Marshal(CreateInvoiceInput{
		Amount:      "149",
		Description: String(""),
		ReferenceID: String("order_123"),
		ReturnURL:   StringOrNull("https://merchant.test/thanks"),
	})
	if err != nil {
		t.Fatal(err)
	}

	var setPayload map[string]any
	if err := json.Unmarshal(setBytes, &setPayload); err != nil {
		t.Fatal(err)
	}

	expected := map[string]any{
		"amount":       "149",
		"description":  "",
		"reference_id": "order_123",
		"return_url":   "https://merchant.test/thanks",
	}
	if !reflect.DeepEqual(setPayload, expected) {
		t.Fatalf("unexpected set payload: %#v", setPayload)
	}

	nullReturnURLBytes, err := json.Marshal(CreateInvoiceInput{
		Amount:    "149",
		ReturnURL: NullString(),
	})
	if err != nil {
		t.Fatal(err)
	}

	var nullReturnURLPayload map[string]any
	if err := json.Unmarshal(nullReturnURLBytes, &nullReturnURLPayload); err != nil {
		t.Fatal(err)
	}

	expectedNullReturnURL := map[string]any{
		"amount":     "149",
		"return_url": nil,
	}
	if !reflect.DeepEqual(nullReturnURLPayload, expectedNullReturnURL) {
		t.Fatalf("unexpected null return URL payload: %#v", nullReturnURLPayload)
	}

	omittedTestPaymentBytes, err := json.Marshal(CreateTestPaymentInput{Amount: "149"})
	if err != nil {
		t.Fatal(err)
	}
	if string(omittedTestPaymentBytes) != `{"amount":"149"}` {
		t.Fatalf("unexpected omitted test payment payload: %s", string(omittedTestPaymentBytes))
	}

	testPaymentBytes, err := json.Marshal(CreateTestPaymentInput{
		Amount:      "149",
		ReferenceID: String("test_payment_001"),
	})
	if err != nil {
		t.Fatal(err)
	}

	var testPaymentPayload map[string]any
	if err := json.Unmarshal(testPaymentBytes, &testPaymentPayload); err != nil {
		t.Fatal(err)
	}

	expectedTestPayment := map[string]any{
		"amount":       "149",
		"reference_id": "test_payment_001",
	}
	if !reflect.DeepEqual(testPaymentPayload, expectedTestPayment) {
		t.Fatalf("unexpected test payment payload: %#v", testPaymentPayload)
	}
}

func TestRejectsInvalidRequestStringsBeforeSending(t *testing.T) {
	var requestCount atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		requestCount.Add(1)
		response.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	client, err := New("sk_test_123", WithAPIOrigin(server.URL))
	if err != nil {
		t.Fatal(err)
	}

	_, err = client.Invoices.Create(context.Background(), CreateInvoiceInput{Amount: ""})
	if !isSDKError(err) {
		t.Fatalf("expected SDK error for empty amount, got %T", err)
	}

	_, err = client.Invoices.Create(context.Background(), CreateInvoiceInput{Amount: "  "})
	if !isSDKError(err) {
		t.Fatalf("expected SDK error for blank amount, got %T", err)
	}

	_, err = client.Invoices.Get(context.Background(), "")
	if !isSDKError(err) {
		t.Fatalf("expected SDK error for empty invoice ID, got %T", err)
	}

	_, err = client.Invoices.CreateTestPayment(context.Background(), "", CreateTestPaymentInput{Amount: "1"})
	if !isSDKError(err) {
		t.Fatalf("expected SDK error for empty test payment invoice ID, got %T", err)
	}

	_, err = client.Invoices.CreateTestPayment(context.Background(), "inv_test_123", CreateTestPaymentInput{Amount: ""})
	if !isSDKError(err) {
		t.Fatalf("expected SDK error for empty test payment amount, got %T", err)
	}

	if requestCount.Load() != 0 {
		t.Fatalf("expected no requests, got %d", requestCount.Load())
	}
}

func TestMapsAPIErrorEnvelopesToAPIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		response.Header().Set("Content-Type", "application/json")
		response.WriteHeader(http.StatusBadRequest)
		_, _ = response.Write([]byte(`{"code":"invalid_request","message":"Invalid request.","fields":[{"location":"body","field":"amount","code":"required","message":"Required."}],"meta":{"request_id":"req_test"}}`))
	}))
	defer server.Close()

	client, err := New("sk_test_123", WithAPIOrigin(server.URL))
	if err != nil {
		t.Fatal(err)
	}

	_, err = client.Invoices.Create(context.Background(), CreateInvoiceInput{Amount: "0.001"})
	var apiError *APIError
	if !errors.As(err, &apiError) {
		t.Fatalf("expected APIError, got %T", err)
	}

	if apiError.Status != http.StatusBadRequest {
		t.Fatalf("unexpected status: %d", apiError.Status)
	}
	if apiError.Code != "invalid_request" {
		t.Fatalf("unexpected code: %s", apiError.Code)
	}
	if len(apiError.Fields) != 1 || apiError.Fields[0].Field != "amount" {
		t.Fatalf("unexpected fields: %#v", apiError.Fields)
	}
	if apiError.Meta["request_id"] != "req_test" {
		t.Fatalf("unexpected meta: %#v", apiError.Meta)
	}
}

func TestMapsNonJSONHTTPErrorsToAPIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		response.Header().Set("Content-Type", "text/html")
		response.WriteHeader(http.StatusBadGateway)
		_, _ = response.Write([]byte("<html>bad gateway</html>"))
	}))
	defer server.Close()

	client, err := New("sk_test_123", WithAPIOrigin(server.URL))
	if err != nil {
		t.Fatal(err)
	}

	_, err = client.Invoices.Create(context.Background(), CreateInvoiceInput{Amount: "1"})
	var apiError *APIError
	if !errors.As(err, &apiError) {
		t.Fatalf("expected APIError, got %T", err)
	}

	if apiError.Status != http.StatusBadGateway {
		t.Fatalf("unexpected status: %d", apiError.Status)
	}
	if apiError.Payload != "<html>bad gateway</html>" {
		t.Fatalf("unexpected payload: %#v", apiError.Payload)
	}
}

func TestMapsNetworkAndParseFailuresToSDKError(t *testing.T) {
	client, err := New("sk_test_123", WithHTTPClient(&http.Client{
		Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			return nil, errors.New("boom")
		}),
	}))
	if err != nil {
		t.Fatal(err)
	}

	_, err = client.Invoices.Create(context.Background(), CreateInvoiceInput{Amount: "1"})
	if !isSDKError(err) {
		t.Fatalf("expected SDK error, got %T", err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		_, _ = response.Write([]byte("not json"))
	}))
	defer server.Close()

	client, err = New("sk_test_123", WithAPIOrigin(server.URL))
	if err != nil {
		t.Fatal(err)
	}

	_, err = client.Invoices.Create(context.Background(), CreateInvoiceInput{Amount: "1"})
	if !isSDKError(err) {
		t.Fatalf("expected SDK error, got %T", err)
	}
}

func TestMapsRequestTimeoutsToSDKError(t *testing.T) {
	client, err := New("sk_test_123", WithTimeout(time.Nanosecond), WithHTTPClient(&http.Client{
		Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			<-request.Context().Done()
			return nil, request.Context().Err()
		}),
	}))
	if err != nil {
		t.Fatal(err)
	}

	_, err = client.Invoices.Create(context.Background(), CreateInvoiceInput{Amount: "1"})
	var sdkError *Error
	if !errors.As(err, &sdkError) {
		t.Fatalf("expected SDK error, got %T", err)
	}
	if sdkError.Message != "invoq API request timed out." {
		t.Fatalf("unexpected timeout error: %v", sdkError)
	}
}

func TestMissingDataEnvelopeReturnsSDKError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		_, _ = response.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	client, err := New("sk_test_123", WithAPIOrigin(server.URL))
	if err != nil {
		t.Fatal(err)
	}

	_, err = client.Invoices.Create(context.Background(), CreateInvoiceInput{Amount: "1"})
	var sdkError *Error
	if !errors.As(err, &sdkError) {
		t.Fatalf("expected SDK error, got %T", err)
	}
	if !strings.Contains(sdkError.Error(), "data envelope") {
		t.Fatalf("unexpected error: %v", sdkError)
	}
}

func TestNullDataEnvelopeReturnsSDKError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		_, _ = response.Write([]byte(`{"data":null}`))
	}))
	defer server.Close()

	client, err := New("sk_test_123", WithAPIOrigin(server.URL))
	if err != nil {
		t.Fatal(err)
	}

	_, err = client.Invoices.Create(context.Background(), CreateInvoiceInput{Amount: "1"})
	var sdkError *Error
	if !errors.As(err, &sdkError) {
		t.Fatalf("expected SDK error, got %T", err)
	}
	if !strings.Contains(sdkError.Error(), "data envelope was null") {
		t.Fatalf("unexpected error: %v", sdkError)
	}
}

func TestDecodesPublicInvoiceMonitoringStatusAndTransfers(t *testing.T) {
	const raw = `{
		"id":"inv_live_123",
		"mode":"live",
		"amount":"149",
		"currency":"USD",
		"description":null,
		"return_url":null,
		"project":{"id":"proj_live_123","name":"Live project","logo_url":null},
		"deposit_address":"0xdeposit",
		"status":"paid",
		"amount_due":"0.000000000000000000",
		"amount_overpaid":"5.000000000000000000",
		"monitoring_ends_at":null,
		"monitoring_status":"ended",
		"amount_paid":"154.000000000000000000",
		"payment_status":"paid",
		"transfers":[
			{"tx_hash":"0xhash1","amount":"149.000000000000000000","explorer_tx_url":"https://explorer.test/tx/0xhash1"},
			{"tx_hash":"0xhash2","amount":"5.000000000000000000","explorer_tx_url":null}
		],
		"direct_onchain_rails":[]
	}`

	var invoice PublicInvoice
	if err := json.Unmarshal([]byte(raw), &invoice); err != nil {
		t.Fatal(err)
	}

	if invoice.AmountOverpaid != "5.000000000000000000" {
		t.Fatalf("unexpected amount overpaid: %s", invoice.AmountOverpaid)
	}
	if invoice.MonitoringStatus == nil || *invoice.MonitoringStatus != MonitoringStatusEnded {
		t.Fatalf("unexpected monitoring status: %#v", invoice.MonitoringStatus)
	}
	if len(invoice.Transfers) != 2 {
		t.Fatalf("unexpected transfers length: %#v", invoice.Transfers)
	}
	if invoice.Transfers[0].TxHash != "0xhash1" || invoice.Transfers[0].Amount != "149.000000000000000000" {
		t.Fatalf("unexpected first transfer: %#v", invoice.Transfers[0])
	}
	if invoice.Transfers[0].ExplorerTxURL == nil || *invoice.Transfers[0].ExplorerTxURL != "https://explorer.test/tx/0xhash1" {
		t.Fatalf("unexpected first transfer explorer URL: %#v", invoice.Transfers[0].ExplorerTxURL)
	}
	if invoice.Transfers[1].ExplorerTxURL != nil {
		t.Fatalf("expected nil explorer URL on second transfer, got %#v", invoice.Transfers[1].ExplorerTxURL)
	}
}

func isSDKError(err error) bool {
	var sdkError SDKError
	return errors.As(err, &sdkError)
}

func secretInvoiceJSON(status string) string {
	return `{
		"id":"inv_test_123",
		"mode":"test",
		"amount":"149",
		"currency":"USD",
		"reference_id":"order_123",
		"description":"Test order",
		"return_url":"https://merchant.test/thanks",
		"deposit_address":null,
		"status":"` + status + `",
		"amount_due":"149.000000000000000000",
		"amount_overpaid":"0.000000000000000000",
		"monitoring_ends_at":null,
		"monitoring_status":null,
		"direct_onchain_rails":[]
	}`
}

func publicInvoiceJSON(status string) string {
	return `{
		"id":"inv_test_123",
		"mode":"test",
		"amount":"149",
		"currency":"USD",
		"description":"Test order",
		"return_url":null,
		"project":{"id":"proj_test_123","name":"Test project","logo_url":null},
		"deposit_address":null,
		"status":"` + status + `",
		"amount_due":"149.000000000000000000",
		"amount_overpaid":"0.000000000000000000",
		"monitoring_ends_at":null,
		"monitoring_status":null,
		"amount_paid":"0",
		"payment_status":"unpaid",
		"transfers":[],
		"direct_onchain_rails":[]
	}`
}

func testPaymentInvoiceJSON(status string) string {
	return `{
		"id":"inv_test_123",
		"mode":"test",
		"amount":"149",
		"currency":"USD",
		"reference_id":"order_123",
		"description":"Test order",
		"return_url":"https://merchant.test/thanks",
		"deposit_address":null,
		"status":"` + status + `",
		"amount_due":"0.000000000000000000",
		"amount_overpaid":"0.000000000000000000",
		"monitoring_ends_at":null,
		"monitoring_status":null,
		"amount_paid":"149",
		"fully_paid_at":"2026-06-15T00:00:00.000Z",
		"direct_onchain_rails":[]
	}`
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return fn(request)
}
