package invoq

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
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
	if invoice.CheckoutStatus != CheckoutStatusUnavailable {
		t.Fatalf("unexpected checkout status: %s", invoice.CheckoutStatus)
	}
	if invoice.PaymentRevision != 0 {
		t.Fatalf("unexpected payment revision: %d", invoice.PaymentRevision)
	}
	if invoice.AmountDue != "149.000000000000000000" {
		t.Fatalf("unexpected amount due: %s", invoice.AmountDue)
	}
	if invoice.AmountOverpaid != "0.000000000000000000" {
		t.Fatalf("unexpected amount overpaid: %s", invoice.AmountOverpaid)
	}
	if invoice.MonitoringEndsAt != nil {
		t.Fatalf("expected nil monitoring end, got %#v", invoice.MonitoringEndsAt)
	}
	if invoice.PaymentOptions == nil || len(invoice.PaymentOptions) != 0 {
		t.Fatalf("expected empty payment options, got %#v", invoice.PaymentOptions)
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
		"description":  "Test order",
		"reference_id": "order_123",
		"return_url":   "https://merchant.test/thanks",
	}
	if !reflect.DeepEqual(payload, expected) {
		t.Fatalf("unexpected request body: %#v", payload)
	}
}

func TestCreateInvoiceBodyCarriesOnlyTheFourContractFields(t *testing.T) {
	// The create schema is strict: any other key, currency above all, fails the
	// whole request with 400 invalid_request and fields[].code "unknown_field".
	// A field on this struct is a field a caller can set, so the guard is that
	// the struct has no other field at all, not that the SDK skips it.
	jsonNames := make([]string, 0, 4)
	for _, field := range reflect.VisibleFields(reflect.TypeOf(CreateInvoiceInput{})) {
		name, _, _ := strings.Cut(field.Tag.Get("json"), ",")
		jsonNames = append(jsonNames, name)
	}

	if !reflect.DeepEqual(jsonNames, []string{"amount", "description", "reference_id", "return_url"}) {
		t.Fatalf("unexpected create invoice body fields: %#v", jsonNames)
	}

	body, err := json.Marshal(CreateInvoiceInput{
		Amount:      "149",
		Description: String("Test order"),
		ReferenceID: String("order_123"),
		ReturnURL:   StringOrNull("https://merchant.test/thanks"),
	})
	if err != nil {
		t.Fatal(err)
	}

	const expected = `{"amount":"149","description":"Test order","reference_id":"order_123","return_url":"https://merchant.test/thanks"}`
	if string(body) != expected {
		t.Fatalf("unexpected create invoice body: %s", string(body))
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
	if invoice.CheckoutStatus != CheckoutStatusUnavailable {
		t.Fatalf("unexpected checkout status: %s", invoice.CheckoutStatus)
	}
	if invoice.PaymentRevision != 0 {
		t.Fatalf("unexpected payment revision: %d", invoice.PaymentRevision)
	}
	if invoice.Project.Name == nil || *invoice.Project.Name != "Test project" {
		t.Fatalf("unexpected project: %#v", invoice.Project)
	}
	if invoice.AmountOverpaid != "0.000000000000000000" {
		t.Fatalf("unexpected amount overpaid: %s", invoice.AmountOverpaid)
	}
	if invoice.Transfers == nil || len(invoice.Transfers) != 0 {
		t.Fatalf("expected empty transfers, got %#v", invoice.Transfers)
	}
	if invoice.PaymentOptions == nil || len(invoice.PaymentOptions) != 0 {
		t.Fatalf("expected empty payment options, got %#v", invoice.PaymentOptions)
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

	if invoice.Status != InvoiceStatusPaid {
		t.Fatalf("unexpected invoice status: %s", invoice.Status)
	}
	if invoice.AmountPaid != "149.000000000000000000" {
		t.Fatalf("unexpected amount paid: %s", invoice.AmountPaid)
	}
	if invoice.AmountDue != "0.000000000000000000" {
		t.Fatalf("unexpected amount due: %s", invoice.AmountDue)
	}
	if invoice.AmountOverpaid != "0.000000000000000000" {
		t.Fatalf("unexpected amount overpaid: %s", invoice.AmountOverpaid)
	}
	if invoice.PaymentRevision != 1 {
		t.Fatalf("unexpected payment revision: %d", invoice.PaymentRevision)
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
		_, _ = response.Write([]byte(`{"code":"invalid_request","message":"Invalid request.","fields":[{"location":"body","field":"amount","code":"required","message":"Required."},{"location":"unexpected","field":"currency","code":"unknown_field","message":"Unknown field."},{"location":"body","field":"description","message":"No code."}],"meta":{"request_id":"req_test"}}`))
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
	if len(apiError.Fields) != 2 || apiError.Fields[0].Field != "amount" {
		t.Fatalf("unexpected fields: %#v", apiError.Fields)
	}
	if apiError.Fields[0].Location != APIErrorLocationBody {
		t.Fatalf("unexpected location: %#v", apiError.Fields[0])
	}
	if apiError.Fields[1].Location != APIErrorLocation("unexpected") ||
		apiError.Fields[1].Field != "currency" {
		t.Fatalf("unexpected fields: %#v", apiError.Fields)
	}
	if apiError.Meta["request_id"] != "req_test" {
		t.Fatalf("unexpected meta: %#v", apiError.Meta)
	}
}

func TestPreservesEmptyAPIErrorFields(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		response.Header().Set("Content-Type", "application/json")
		response.WriteHeader(http.StatusBadRequest)
		_, _ = response.Write([]byte(`{"code":"invalid_request","message":"Invalid request.","fields":[]}`))
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

	if apiError.Fields == nil || len(apiError.Fields) != 0 {
		t.Fatalf("expected an empty non-nil slice, got %#v", apiError.Fields)
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

func TestNonObjectDataEnvelopeReturnsSDKError(t *testing.T) {
	for _, body := range []string{`{"data":null}`, `{"data":5}`, `{"data":[]}`, `{"data":"x"}`} {
		server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
			_, _ = response.Write([]byte(body))
		}))

		client, err := New("sk_test_123", WithAPIOrigin(server.URL))
		if err != nil {
			server.Close()
			t.Fatal(err)
		}

		_, err = client.Invoices.Create(context.Background(), CreateInvoiceInput{Amount: "1"})
		server.Close()

		var sdkError *Error
		if !errors.As(err, &sdkError) {
			t.Fatalf("%s: expected SDK error, got %T", body, err)
		}
		if !strings.Contains(sdkError.Error(), "data envelope was not an object") {
			t.Fatalf("%s: unexpected error: %v", body, sdkError)
		}
		if sdkError.Payload == nil {
			t.Fatalf("%s: expected the raw payload to be attached", body)
		}
	}
}

// A control character reaches the transport differently in every runtime — some
// trim it and send, some send it raw. Rejected here so all six answer alike.
func TestRejectsAPIKeysWithControlCharacters(t *testing.T) {
	for _, key := range []string{"sk_test_x\r\nX-Injected: yes", "sk_test_x\n", "sk_test\x00x"} {
		if _, err := New(key); !isSDKError(err) {
			t.Fatalf("%q: expected SDK error, got %T", key, err)
		}
	}
}

// A log line that dumps the client must not carry the secret key.
func TestClientValuesDoNotPrintTheAPIKey(t *testing.T) {
	client, err := New("sk_live_SUPERSECRET")
	if err != nil {
		t.Fatal(err)
	}

	for _, printed := range []string{
		fmt.Sprintf("%v", client.Invoices),
		fmt.Sprintf("%+v", client.Invoices),
		fmt.Sprintf("%#v", client.Invoices),
		fmt.Sprintf("%v", *client.Invoices),
		fmt.Sprintf("%+v", *client.Invoices),
		fmt.Sprintf("%#v", *client.Invoices),
	} {
		if strings.Contains(printed, "SUPERSECRET") {
			t.Fatalf("secret key leaked: %s", printed)
		}
	}
}

func TestRejectsDotSegmentInvoiceIDs(t *testing.T) {
	client, err := New("sk_test_123")
	if err != nil {
		t.Fatal(err)
	}

	for _, id := range []string{".", ".."} {
		if _, err := client.Invoices.Get(context.Background(), id); err == nil {
			t.Fatalf("%q: expected an error", id)
		}
		if _, err := client.Invoices.CreateTestPayment(context.Background(), id, CreateTestPaymentInput{Amount: "1"}); err == nil {
			t.Fatalf("%q: expected an error", id)
		}
	}
}

func TestDecodesLiveInvoicePaymentOptionVariants(t *testing.T) {
	var invoice Invoice
	if err := json.Unmarshal([]byte(liveInvoiceJSON()), &invoice); err != nil {
		t.Fatal(err)
	}

	if invoice.CheckoutStatus != CheckoutStatusOpen {
		t.Fatalf("unexpected checkout status: %s", invoice.CheckoutStatus)
	}
	if invoice.MonitoringEndsAt == nil || *invoice.MonitoringEndsAt != "2026-06-16T00:00:00.000Z" {
		t.Fatalf("unexpected monitoring end: %#v", invoice.MonitoringEndsAt)
	}
	if len(invoice.PaymentOptions) != 3 {
		t.Fatalf("unexpected payment options length: %#v", invoice.PaymentOptions)
	}

	evmDeposit := invoice.PaymentOptions[0]
	if evmDeposit.CollectionMethod != PaymentOptionCollectionMethodEVMDeposit ||
		evmDeposit.ChainNamespace != ChainNamespaceEIP155 ||
		evmDeposit.Status != PaymentOptionStatusReady {
		t.Fatalf("unexpected evm deposit option: %#v", evmDeposit)
	}
	if evmDeposit.TokenDecimals != 6 {
		t.Fatalf("unexpected evm deposit token decimals: %d", evmDeposit.TokenDecimals)
	}
	if evmDeposit.DepositAddress == nil || *evmDeposit.DepositAddress != "0xdeposit" {
		t.Fatalf("unexpected deposit address: %#v", evmDeposit.DepositAddress)
	}
	if evmDeposit.SuggestedAmount == nil || *evmDeposit.SuggestedAmount != "149.000000" {
		t.Fatalf("unexpected suggested amount: %#v", evmDeposit.SuggestedAmount)
	}
	if evmDeposit.LogoURL != nil || evmDeposit.ChainLogoURL != nil {
		t.Fatalf("unexpected evm deposit logos: %#v", evmDeposit)
	}
	if evmDeposit.RecipientAddress != nil || evmDeposit.InvoiceAmount != nil ||
		evmDeposit.MatchingIncrement != nil || evmDeposit.ExactAmount != nil {
		t.Fatalf("unexpected direct_exact fields on an evm deposit option: %#v", evmDeposit)
	}

	directExact := invoice.PaymentOptions[1]
	if directExact.CollectionMethod != PaymentOptionCollectionMethodDirectExact ||
		directExact.ChainNamespace != ChainNamespaceSolana ||
		directExact.Status != PaymentOptionStatusReady {
		t.Fatalf("unexpected direct exact option: %#v", directExact)
	}
	if directExact.RecipientAddress == nil || *directExact.RecipientAddress != "SoLrecipient" {
		t.Fatalf("unexpected recipient address: %#v", directExact.RecipientAddress)
	}
	if directExact.InvoiceAmount == nil || *directExact.InvoiceAmount != "149.000000" {
		t.Fatalf("unexpected invoice amount: %#v", directExact.InvoiceAmount)
	}
	if directExact.MatchingIncrement == nil || *directExact.MatchingIncrement != "0.000123" {
		t.Fatalf("unexpected matching increment: %#v", directExact.MatchingIncrement)
	}
	if directExact.ExactAmount == nil || *directExact.ExactAmount != "149.000123" {
		t.Fatalf("unexpected exact amount: %#v", directExact.ExactAmount)
	}
	if directExact.DepositAddress != nil || directExact.SuggestedAmount != nil {
		t.Fatalf("unexpected evm_deposit fields on a direct exact option: %#v", directExact)
	}
	if directExact.LogoURL == nil || *directExact.LogoURL != "https://assets.test/usdc.svg" {
		t.Fatalf("unexpected direct exact logo URL: %#v", directExact.LogoURL)
	}
	if directExact.ChainLogoURL == nil || *directExact.ChainLogoURL != "https://assets.test/solana.svg" {
		t.Fatalf("unexpected direct exact chain logo URL: %#v", directExact.ChainLogoURL)
	}

	unavailable := invoice.PaymentOptions[2]
	if unavailable.ChainNamespace != ChainNamespaceTron ||
		unavailable.Status != PaymentOptionStatusUnavailable {
		t.Fatalf("unexpected unavailable option: %#v", unavailable)
	}
	if unavailable.DepositAddress != nil || unavailable.SuggestedAmount != nil ||
		unavailable.RecipientAddress != nil || unavailable.InvoiceAmount != nil ||
		unavailable.MatchingIncrement != nil || unavailable.ExactAmount != nil {
		t.Fatalf("unexpected instructions on an unavailable option: %#v", unavailable)
	}
	if unavailable.NetworkLabel != "TRON" || unavailable.DisplaySymbol != "USDT" {
		t.Fatalf("unexpected unavailable option display fields: %#v", unavailable)
	}
}

func TestDecodesPublicInvoiceTransfers(t *testing.T) {
	const raw = `{
		"id":"inv_live_123",
		"mode":"live",
		"amount":"149.0000",
		"currency":"USD",
		"description":null,
		"return_url":null,
		"project":{"id":"proj_live_123","name":"Live project","logo_url":null},
		"status":"paid",
		"checkout_status":"paid",
		"payment_revision":2,
		"amount_paid":"154.000000000000000000",
		"amount_due":"0.000000000000000000",
		"amount_overpaid":"5.000000000000000000",
		"transfers":[
			{
				"chain_namespace":"eip155",
				"chain_reference":"8453",
				"transaction_id":"0xhash1",
				"event_index":0,
				"amount":"149.000000000000000000",
				"explorer_transaction_url":"https://explorer.test/tx/0xhash1"
			},
			{
				"chain_namespace":"eip155",
				"chain_reference":"8453",
				"transaction_id":"0xhash1",
				"event_index":3,
				"amount":"5.000000000000000000",
				"explorer_transaction_url":null
			}
		],
		"monitoring_ends_at":null,
		"payment_options":[]
	}`

	var invoice PublicInvoice
	if err := json.Unmarshal([]byte(raw), &invoice); err != nil {
		t.Fatal(err)
	}

	if invoice.CheckoutStatus != CheckoutStatusPaid {
		t.Fatalf("unexpected checkout status: %s", invoice.CheckoutStatus)
	}
	if invoice.PaymentRevision != 2 {
		t.Fatalf("unexpected payment revision: %d", invoice.PaymentRevision)
	}
	if invoice.AmountOverpaid != "5.000000000000000000" {
		t.Fatalf("unexpected amount overpaid: %s", invoice.AmountOverpaid)
	}
	if len(invoice.Transfers) != 2 {
		t.Fatalf("unexpected transfers length: %#v", invoice.Transfers)
	}

	firstTransfer := invoice.Transfers[0]
	if firstTransfer.ChainNamespace != ChainNamespaceEIP155 || firstTransfer.ChainReference != "8453" {
		t.Fatalf("unexpected first transfer chain: %#v", firstTransfer)
	}
	if firstTransfer.TransactionID != "0xhash1" || firstTransfer.Amount != "149.000000000000000000" {
		t.Fatalf("unexpected first transfer: %#v", firstTransfer)
	}
	if firstTransfer.EventIndex != 0 {
		t.Fatalf("unexpected first transfer event index: %d", firstTransfer.EventIndex)
	}
	if firstTransfer.ExplorerTransactionURL == nil ||
		*firstTransfer.ExplorerTransactionURL != "https://explorer.test/tx/0xhash1" {
		t.Fatalf("unexpected first transfer explorer URL: %#v", firstTransfer.ExplorerTransactionURL)
	}

	// One transaction can carry several credits, separated by event_index only.
	secondTransfer := invoice.Transfers[1]
	if secondTransfer.TransactionID != firstTransfer.TransactionID || secondTransfer.EventIndex != 3 {
		t.Fatalf("unexpected second transfer: %#v", secondTransfer)
	}
	if secondTransfer.ExplorerTransactionURL != nil {
		t.Fatalf("expected nil explorer URL on second transfer, got %#v", secondTransfer.ExplorerTransactionURL)
	}
}

func isSDKError(err error) bool {
	var sdkError SDKError
	return errors.As(err, &sdkError)
}

// Test invoices carry no payment window and no payment options, and their
// checkout status is always unavailable.
func secretInvoiceJSON(status string) string {
	return `{
		"id":"inv_test_123",
		"mode":"test",
		"amount":"149.0000",
		"currency":"USD",
		"reference_id":"order_123",
		"description":"Test order",
		"return_url":"https://merchant.test/thanks",
		"status":"` + status + `",
		"checkout_status":"unavailable",
		"payment_revision":0,
		"amount_due":"149.000000000000000000",
		"amount_overpaid":"0.000000000000000000",
		"monitoring_ends_at":null,
		"payment_options":[]
	}`
}

func publicInvoiceJSON(status string) string {
	return `{
		"id":"inv_test_123",
		"mode":"test",
		"amount":"149.0000",
		"currency":"USD",
		"description":"Test order",
		"return_url":null,
		"project":{"id":"proj_test_123","name":"Test project","logo_url":null},
		"status":"` + status + `",
		"checkout_status":"unavailable",
		"payment_revision":0,
		"amount_paid":"0.000000000000000000",
		"amount_due":"149.000000000000000000",
		"amount_overpaid":"0.000000000000000000",
		"transfers":[],
		"monitoring_ends_at":null,
		"payment_options":[]
	}`
}

func testPaymentInvoiceJSON(status string) string {
	return `{
		"id":"inv_test_123",
		"mode":"test",
		"amount":"149.0000",
		"currency":"USD",
		"reference_id":"order_123",
		"description":"Test order",
		"return_url":"https://merchant.test/thanks",
		"status":"` + status + `",
		"checkout_status":"unavailable",
		"payment_revision":1,
		"amount_due":"0.000000000000000000",
		"amount_overpaid":"0.000000000000000000",
		"monitoring_ends_at":null,
		"payment_options":[],
		"amount_paid":"149.000000000000000000",
		"fully_paid_at":"2026-06-15T00:00:00.000Z"
	}`
}

// A live create response, with one payment option per issued variant.
func liveInvoiceJSON() string {
	return `{
		"id":"inv_live_123",
		"mode":"live",
		"amount":"149.0000",
		"currency":"USD",
		"reference_id":"order_123",
		"description":"Test order",
		"return_url":null,
		"status":"unpaid",
		"checkout_status":"open",
		"payment_revision":0,
		"amount_due":"149.000000000000000000",
		"amount_overpaid":"0.000000000000000000",
		"monitoring_ends_at":"2026-06-16T00:00:00.000Z",
		"payment_options":[
			{
				"collection_method":"evm_deposit",
				"chain_namespace":"eip155",
				"chain_reference":"8453",
				"currency":"USD",
				"token_address":"0xtoken",
				"token_decimals":6,
				"network_label":"Base",
				"display_symbol":"USDC",
				"logo_url":null,
				"chain_logo_url":null,
				"status":"ready",
				"deposit_address":"0xdeposit",
				"suggested_amount":"149.000000"
			},
			{
				"collection_method":"direct_exact",
				"chain_namespace":"solana",
				"chain_reference":"5eykt4UsFv8P8NJdTREpY1vzqKqZKvdp",
				"currency":"USD",
				"token_address":"SoLtoken",
				"token_decimals":6,
				"network_label":"Solana",
				"display_symbol":"USDC",
				"logo_url":"https://assets.test/usdc.svg",
				"chain_logo_url":"https://assets.test/solana.svg",
				"status":"ready",
				"recipient_address":"SoLrecipient",
				"invoice_amount":"149.000000",
				"matching_increment":"0.000123",
				"exact_amount":"149.000123"
			},
			{
				"collection_method":"direct_exact",
				"chain_namespace":"tron",
				"chain_reference":"0x2b6653dc",
				"currency":"USD",
				"token_address":"TRXtoken",
				"token_decimals":6,
				"network_label":"TRON",
				"display_symbol":"USDT",
				"logo_url":null,
				"chain_logo_url":null,
				"status":"unavailable"
			}
		]
	}`
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return fn(request)
}
