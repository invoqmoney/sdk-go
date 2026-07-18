# invoq Go SDK

[English](../README.md) · [Bahasa Indonesia](./README.id.md) · [Español](./README.es-419.md) · [Français](./README.fr.md) · [Português](./README.pt-BR.md) · **Tiếng Việt** · [Türkçe](./README.tr.md) · [ไทย](./README.th.md) · [简体中文](./README.zh-Hans.md) · [繁體中文](./README.zh-Hant.md)

> Tài liệu này được dịch từ README tiếng Anh; nếu có chỗ khác nhau, [bản tiếng Anh](../README.md) là bản chuẩn.

SDK Go cho các API server của invoq và xác minh webhook. Tạo hóa đơn stablecoin, mô phỏng thanh toán thử nghiệm và xử lý đơn hàng từ webhook có chữ ký.

Chỉ dùng module này trên máy chủ của bạn. Nó nhận khóa bí mật và không được biên dịch vào các ứng dụng phía client.

## SDK server

Tạo hóa đơn và xác minh webhook từ backend của bạn bằng bất kỳ ngôn ngữ nào dưới đây — cùng REST API, cùng chữ ký webhook. Repo này là SDK Go.

| Ngôn ngữ | Repo |
| --- | --- |
| Node.js | [github.com/invoqmoney/sdk-js](https://github.com/invoqmoney/sdk-js) (`@invoq/server`) |
| Python | [github.com/invoqmoney/sdk-python](https://github.com/invoqmoney/sdk-python) |
| PHP | [github.com/invoqmoney/sdk-php](https://github.com/invoqmoney/sdk-php) |
| Go | **repo này** |
| Rust | [github.com/invoqmoney/sdk-rust](https://github.com/invoqmoney/sdk-rust) |
| Ruby | [github.com/invoqmoney/sdk-ruby](https://github.com/invoqmoney/sdk-ruby) |

Phía trình duyệt vẫn như nhau với mọi backend: **`@invoq/checkout`** (JavaScript, trong [github.com/invoqmoney/sdk-js](https://github.com/invoqmoney/sdk-js)) mở cửa sổ thanh toán nhúng trong trang cho mọi frontend.

## Cài đặt

```sh
go get github.com/invoqmoney/sdk-go
```

Yêu cầu Go 1.22 trở lên.

## Lấy khóa API

1. Đăng nhập [bảng điều khiển invoq](https://app.invoq.money) và tạo một dự án.
2. Ở trang **API keys**, tạo một khóa bí mật. Khóa thử nghiệm bắt đầu bằng `sk_test_`, khóa thật bằng `sk_live_`. Loại khóa quyết định hóa đơn tạo ra là thử nghiệm hay thật.
3. Trong phần cài đặt **webhooks** của dự án, lưu URL webhook của bạn. Mã bí mật của webhook (`whsec_...`) cho chế độ đó chỉ hiện đúng một lần, lúc bạn bật webhook lần đầu — hãy lưu lại ngay. URL webhook phải là URL HTTPS truy cập công khai được.

Thêm cả hai vào biến môi trường của máy chủ:

```sh
INVOQ_SECRET_KEY=sk_test_...
INVOQ_WEBHOOK_SECRET=whsec_...
```

Bắt đầu bằng khóa thử nghiệm. Khi chạy thật thì đổi sang khóa thật và mã bí mật webhook cho môi trường thật.

## Tạo client

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

Mặc định khi chạy thật:

- Origin API: `https://api.invoq.money`
- Thời gian chờ request: 10 giây
- User-Agent: `invoq-go/<phiên bản module>`

Ghi đè chúng khi phát triển local hoặc chạy thử bản xem trước:

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

`apiOrigin` phải là origin `http` hoặc `https` tuyệt đối, không có phần tên người dùng, mật khẩu, đường dẫn, query hay hash. SDK sẽ nối thêm các đường dẫn API `/v1/...`.

## Hóa đơn

Tạo hóa đơn từ máy chủ của bạn:

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
	// Xử lý lỗi.
}

_ = invoice.ID
```

Lưu ý:

- Số tiền phải do máy chủ quyết định. Đừng tin số tiền phía client gửi lên.
- `amount` là chuỗi thập phân USD từ `0.01` đến `1000000.00`, tối đa 2 chữ số lẻ, ví dụ `129` hoặc `129.99`.
- Dùng `reference_id` để nối webhook `invoice.paid` về đúng đơn hàng của bạn. Nó cũng giúp thao tác tạo an toàn khi thử lại: tạo lại với cùng `reference_id` và cùng nội dung hóa đơn sẽ trả về hóa đơn đã có thay vì tạo trùng; nếu nội dung khác nhau, API sẽ báo lỗi `409 reference_id_conflict`.
- Dùng `invoq.String(...)` cho các chuỗi tùy chọn trong request. Dùng `invoq.StringOrNull(...)` để đặt `return_url`, dùng `invoq.NullString()` để gửi giá trị `null` trong JSON, và bỏ trống trường đó để không gửi nó đi.

Lấy một hóa đơn công khai:

```go
invoice, err := client.Invoices.Get(ctx, "inv_123")
```

`Get` trả về `*invoq.PublicInvoice`. Việc đọc hóa đơn công khai không kèm theo `reference_id` riêng tư của bạn; hãy dùng webhook hoặc kho lưu đơn hàng của riêng bạn để ánh xạ cho việc xử lý đơn.

## Thanh toán thử nghiệm

Hóa đơn thử nghiệm không nhận được tiền thật. Hãy mô phỏng một khoản thanh toán từ máy chủ của bạn:

```go
paidInvoice, err := client.Invoices.CreateTestPayment(ctx, invoice.ID, invoq.CreateTestPaymentInput{
	Amount: invoice.Amount,
})
if err != nil {
	// Xử lý lỗi.
}

_ = paidInvoice.Status // invoq.InvoiceStatusPaid khi đã thanh toán đủ
```

`CreateTestPayment` chỉ dùng được với hóa đơn tạo bằng khóa `sk_test_`. Khi số tiền thanh toán đạt đủ giá trị hóa đơn, hóa đơn chuyển sang `paid` và invoq gửi một webhook `invoice.paid` có chữ ký thật đến URL webhook thử nghiệm của bạn. Có thể trả từng phần, hóa đơn sẽ thành `partially_paid`.

Để nhận webhook trên máy của mình, hãy mở máy chủ local ra ngoài bằng một tunnel HTTPS như ngrok hay cloudflared, rồi lưu URL tunnel làm URL webhook thử nghiệm trong bảng điều khiển. Bảng điều khiển cũng gửi được một `webhook.ping` có chữ ký để kiểm tra kết nối.

## Webhook

Hãy truyền nội dung request gốc vào `VerifyWebhook`. Đừng phân tích JSON rồi tuần tự hóa lại nó trước khi xác minh.

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

		// Xử lý đơn hàng cho *referenceID theo cách an toàn khi lặp lại.
	}

	response.WriteHeader(http.StatusOK)
}
```

Hãy dựa vào webhook `invoice.paid` để xử lý đơn hàng trên máy chủ. Khi `IsInvoicePaid(event)` là true, hóa đơn đã sẵn sàng để xử lý tự động; trạng thái của nó là `paid`, `settling` hoặc `settled`. Hóa đơn ở trạng thái `review_required` hiện chưa gửi webhook `invoice.paid`. Hãy đợi webhook `invoice.paid` sau khi được duyệt.

Lần gửi thất bại sẽ được gửi lại, nên hãy xử lý đơn theo cách an toàn khi lặp lại dựa trên `reference_id` hoặc `id` hóa đơn và bỏ qua những lần gửi lặp lại. Hãy trả về 2xx thật nhanh; mọi mã trạng thái khác đều bị tính là giao thất bại.

`VerifyWebhook` nhận `http.Header`. Dùng `VerifyWebhookWithSignature` khi bạn đã có sẵn giá trị của header `invoq-signature`.

Việc xác minh webhook thất bại sẽ trả về `*invoq.SignatureVerificationError`. SDK cho phép timestamp lệch tối đa 5 phút. Lần gửi thất bại được ký lại ở mỗi lần thử lại, nên những lần gửi lại thông thường vẫn xác minh được trong khoảng thời gian đó. Header chữ ký có dạng `invoq-signature: t=<giây unix>,v1=<HMAC-SHA256 dạng hex của "<t>.<nội dung request gốc>">`.

## Lỗi

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

Lỗi kết nối, hết thời gian chờ request, lỗi cấu hình và lỗi phân tích phản hồi sẽ trả về `*invoq.Error`. Phản hồi API không phải 2xx sẽ trả về `*invoq.APIError`. Mọi lỗi của SDK đều triển khai `invoq.SDKError`.

```go
var sdkErr invoq.SDKError
if errors.As(err, &sdkErr) {
	fmt.Println(sdkErr)
}
```

## Tham chiếu API

```go
client, err := invoq.New(apiKey,
	invoq.WithAPIOrigin("https://api.invoq.money"), // tùy chọn, ghi đè mặc định
	invoq.WithTimeout(invoq.DefaultTimeout),        // tùy chọn, thời gian chờ request
)
```

- `client.Invoices.Create(ctx, input)` tạo một hóa đơn. `input`: `Amount` (bắt buộc), `Currency` (`InvoiceCurrencyUSD`, mặc định), `Description`, `ReferenceID`, `ReturnURL`.
- `client.Invoices.Get(ctx, invoiceID)` lấy một hóa đơn công khai và trả về `*invoq.PublicInvoice`.
- `client.Invoices.CreateTestPayment(ctx, invoiceID, input)` mô phỏng thanh toán trên hóa đơn thử nghiệm và trả về `*invoq.TestPaymentInvoice`.
- `invoq.VerifyWebhook(rawBody, headers, webhookSecret)` xác minh một webhook và trả về `invoq.WebhookEvent`.
- `invoq.IsInvoicePaid(event)` và `invoq.AsInvoicePaidEvent(event)` nhận diện các sự kiện `invoice.paid` đã được định kiểu.
- SDK tự phát hiện phiên bản module Go của mình từ thông tin build để đặt `User-Agent`. Các tag đã phát hành như `v0.1.0` được gửi đi mà không có tiền tố `v`; còn các bản build từ mã nguồn local không có phiên bản module thì dùng `unknown`.
