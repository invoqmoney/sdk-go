# invoq Go SDK

[English](../README.md) · [Bahasa Indonesia](./README.id.md) · [Español](./README.es-419.md) · [Français](./README.fr.md) · [Português](./README.pt-BR.md) · [Tiếng Việt](./README.vi.md) · [Türkçe](./README.tr.md) · [ไทย](./README.th.md) · [简体中文](./README.zh-Hans.md) · **繁體中文**

> 本文是英文版 README 的繁體中文翻譯；若表述有出入，以[英文版](../README.md)為準。

這是 invoq 伺服器端 API 與 webhook 驗證的 Go SDK。可以建立穩定幣帳單、模擬測試付款，並用帶簽章的 webhook 處理訂單。

這個模組只能在你的伺服器端使用。它需要私密金鑰，絕不能被編譯進用戶端應用程式中。

**在用 AI 寫程式？把這段貼給它。**

```
用 invoq 幫我的專案串接穩定幣收款，從測試模式開始。寫程式前先讀文件 https://invoq.money/llms.txt
```

## 伺服器端 SDK

用下面任一種語言，都能從你的後端建立帳單、驗證 webhook——REST API 和 webhook 簽章完全一致。本倉庫是 Go SDK。

| 語言 | 倉庫 |
| --- | --- |
| Node.js | [github.com/invoqmoney/sdk-js](https://github.com/invoqmoney/sdk-js) (`@invoq/server`) |
| Python | [github.com/invoqmoney/sdk-python](https://github.com/invoqmoney/sdk-python) |
| PHP | [github.com/invoqmoney/sdk-php](https://github.com/invoqmoney/sdk-php) |
| Go | **本倉庫** |
| Rust | [github.com/invoqmoney/sdk-rust](https://github.com/invoqmoney/sdk-rust) |
| Ruby | [github.com/invoqmoney/sdk-ruby](https://github.com/invoqmoney/sdk-ruby) |

無論後端選哪種語言，瀏覽器這一側都一樣：**`@invoq/checkout`**（JavaScript，在 [github.com/invoqmoney/sdk-js](https://github.com/invoqmoney/sdk-js)）為任意前端打開嵌在頁面裡的結帳彈窗。

## 安裝

```sh
go get github.com/invoqmoney/sdk-go
```

需要 Go 1.22 以上。

## 取得金鑰

1. 登入 [invoq 商家後台](https://app.invoq.money)，建立一個專案。
2. 在 **API keys** 頁面建立一組私密金鑰（secret key）。測試金鑰以 `sk_test_` 開頭，正式金鑰以 `sk_live_` 開頭；用哪種金鑰，決定開出的帳單是測試單還是正式單。
3. 在專案的 **webhooks** 設定裡儲存你的 webhook URL。對應模式的 webhook 簽章金鑰（`whsec_...`）只在首次啟用 webhook 時顯示一次——記得馬上存好。webhook URL 必須是可公開存取的 HTTPS 網址。
4. 上線前先設定 **Receiving wallet**。測試帳單不需要它；沒有結算去向的正式帳單會以 `409 no_payment_options_available` 失敗。

把兩者都加進伺服器的環境變數：

```sh
INVOQ_SECRET_KEY=sk_test_...
INVOQ_WEBHOOK_SECRET=whsec_...
```

先用測試金鑰跑通，上線時再換成正式金鑰和正式 webhook 簽章金鑰。

## 建立用戶端

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

正式環境預設值：

- API 位址：`https://api.invoq.money`
- 請求逾時：10 秒
- User-Agent：`invoq-go/<模組版本>`

本機開發或預覽測試時可以覆寫：

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

`apiOrigin` 必須是完整的 `http` 或 `https` origin，且不能包含使用者名稱、密碼、路徑、查詢參數或 hash 部分。SDK 會在其後接上 `/v1/...` API 路徑。

## 帳單

在伺服器端建立帳單：

```go
ctx := context.Background()

invoice, err := client.Invoices.Create(ctx, invoq.CreateInvoiceInput{
	Amount:      "129",
	Description: invoq.String("SaaS boilerplate"),
	ReferenceID: invoq.String("order_1234"),
	ReturnURL:   invoq.StringOrNull("https://merchant.example/thanks"),
})
if err != nil {
	// 處理錯誤。
}

_ = invoice.ID
```

說明：

- 金額要由伺服器端決定，不要相信用戶端傳來的金額。
- `amount` 是 `0.01` 到 `1000000.00` 之間的十進位美元字串，最多兩位小數，例如 `129` 或 `129.99`。幣別恆為 USD，測試還是正式由金鑰決定——兩者都不是請求欄位。
- 用 `reference_id` 把 `invoice.paid` webhook 對應回你的訂單。它也讓建立動作可以放心重試：用相同的 `reference_id` 和相同的帳單條件再建立一次，回傳的是既有帳單而不是重複開單；條件不同則會回 `409 reference_id_conflict` API 錯誤。
- 可選的請求字串用 `invoq.String(...)`；用 `invoq.StringOrNull(...)` 設定 `return_url`，用 `invoq.NullString()` 送出 JSON `null`，不設定該欄位則會將其省略。

查詢公開帳單：

```go
invoice, err := client.Invoices.Get(ctx, "inv_123")
```

`Get` 會回傳 `*invoq.PublicInvoice`。讀取公開帳單不會包含你私有的 `reference_id`；做履約對應時，請使用 webhook 或你自己的訂單儲存。

## 測試付款

測試帳單收不了真錢，可以在伺服器端模擬一筆付款：

```go
paidInvoice, err := client.Invoices.CreateTestPayment(ctx, invoice.ID, invoq.CreateTestPaymentInput{
	Amount: invoice.Amount,
})
if err != nil {
	// 處理錯誤。
}

_ = paidInvoice.Status // 完全付清時為 invoq.InvoiceStatusPaid
```

`CreateTestPayment` 只對 `sk_test_` 金鑰建立的帳單有效。累計付款達到帳單金額時，帳單變為 `paid`，invoq 會向你的測試 webhook URL 送出一條真實簽章的 `invoice.paid` webhook。也可以只付部分金額，帳單會變成 `partially_paid`。

要在本機收 webhook，用 ngrok、cloudflared 之類的 HTTPS 隧道把本地伺服器公開出去，再把隧道網址存成商家後台裡的測試 webhook URL。

## 託管結帳頁

每張帳單都自帶一個託管結帳頁：

```text
https://pay.invoq.money/<帳單 id>
```

當頁內結帳彈窗不適合時，把連結分享出去或直接導向過去就行。

## Webhooks

把原始請求內容傳給 `VerifyWebhook`。不要在驗證前把 JSON 解析後再重新序列化。

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

		// 冪等地為 *referenceID 處理訂單。
	}

	response.WriteHeader(http.StatusOK)
}
```

訂單處理以伺服器端收到的 `invoice.paid` webhook 為準。`IsInvoicePaid(event)` 為 true 時，表示帳單可以自動履約；其狀態為 `paid`、`settling` 或 `settled`。`review_required` 帳單在審核通過前不會發出任何 `invoice.paid`。

帳單從已付款跌回不足額時，invoq 還會發 `invoice.payment_reversed`——例如鏈重組把一筆已確認的轉帳拿掉了。用 `invoq.IsInvoicePaymentReversed(event)` 接住它，用 `invoq.AsInvoicePaymentReversedEvent(event)` 解出內容，再依你自己的策略暫停或撤銷履約。

投遞失敗會重試（最多 5 次，間隔依序為 1 分鐘、5 分鐘、30 分鐘、2 小時），所以要按 `reference_id` 或帳單 `id` 冪等地處理訂單，重複送達直接略過即可。送達順序也不保證——請保留 `payment_revision` 最大的那份快照。請盡快回 2xx；任何其他狀態碼都算投遞失敗並會重試，重新導向和 `4xx` 也在其中。

`VerifyWebhook` 接受 `http.Header`。如果你已經拿到 `invoq-signature` 標頭的值，可以改用 `VerifyWebhookWithSignature`。

webhook 驗證失敗會回傳 `*invoq.SignatureVerificationError`。SDK 允許 5 分鐘的時間戳容差。投遞失敗後每次重試都會重新簽章，因此正常的重試投遞仍能在該時間窗內通過驗證。簽章標頭格式是 `invoq-signature: t=<unix 秒>,v1=<對 "<t>.<原始請求內容>" 計算的 HMAC-SHA256 十六進位值>`。

## 錯誤處理

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

連線失敗、請求逾時、設定錯誤和回應解析失敗會回傳 `*invoq.Error`。API 回應非 2xx 時會回傳 `*invoq.APIError`。所有 SDK 錯誤都實作了 `invoq.SDKError` 介面。

```go
var sdkErr invoq.SDKError
if errors.As(err, &sdkErr) {
	fmt.Println(sdkErr)
}
```

## API 參考

```go
client, err := invoq.New(apiKey,
	invoq.WithAPIOrigin("https://api.invoq.money"), // 可選，覆寫預設值
	invoq.WithTimeout(invoq.DefaultTimeout),        // 可選的請求逾時
)
```

- `client.Invoices.Create(ctx, input)` —— 建立帳單。`input`：`Amount`（必填）、`Description`、`ReferenceID`、`ReturnURL`。
- `client.Invoices.Get(ctx, invoiceID)` —— 查詢公開帳單，回傳 `*invoq.PublicInvoice`。
- `client.Invoices.CreateTestPayment(ctx, invoiceID, input)` —— 在測試帳單上模擬付款，回傳 `*invoq.TestPaymentInvoice`。
- `invoq.VerifyWebhook(rawBody, headers, webhookSecret)` —— 驗證 webhook，回傳 `invoq.WebhookEvent`。
- `invoq.IsInvoicePaid(event)` 和 `invoq.AsInvoicePaidEvent(event)` —— 辨識具型別的 `invoice.paid` 事件。`invoq.IsInvoicePaymentReversed(event)` 和 `invoq.AsInvoicePaymentReversedEvent(event)` 對 `invoice.payment_reversed` 做同樣的事。兩者都會拒絕結構不合法的事件；本版 SDK 尚未建模的事件型別同樣能通過驗證，並原樣回傳。
- SDK 會從建置資訊中讀取自己的 Go 模組版本，用於 `User-Agent`。像 `v0.1.0` 這樣的發布標籤會去掉 `v` 前綴後送出；本機原始碼建置若沒有模組版本，則使用 `unknown`。

`Invoices.Get` 回傳託管結帳頁使用的公開帳單結構：即建立回應的結構，加上 `AmountPaid`、`Project` 和 `Transfers`，去掉 `ReferenceID`。如果需要商家端的 `reference_id`，請使用建立帳單的回應或 `invoice.paid` webhook。

帳單有兩個狀態欄位。`Status` 是記帳狀態——`unpaid`、`partially_paid`、`paid`、`settling`、`settled`、`review_required`，其中三個等同已付款的取值只差在資金離你的錢包還有多遠。`CheckoutStatus` 是付款人看到的狀態——`open`、`confirming`、`expired`、`paid`、`unavailable`——它從不構成履約依據。`PaymentRevision` 每當已確認的付款集合改變就加一，你可以據此丟掉比手上更舊的快照。

回應裡的金額一律格式化為 4 位小數：用 `129` 建立，帳單會回傳 `Amount` `129.0000`。比較金額請按數值比，不要按字串比。`AmountDue` 依 `max(amount - amount_paid, 0)` 衍生，使用和 `AmountPaid` 相同的 18 位小數 scale；`AmountOverpaid` 與它互為鏡像，即 `max(amount_paid - amount, 0)`，所以你不必自己做減法。

`PaymentOptions` 裝的是付款指示，建立時即固定，測試模式下為 `[]`。每一項先看 `Status`，再看 `CollectionMethod`：只有 `ready` 可付，`evm_deposit` 帶 `DepositAddress` 和 `SuggestedAmount`，`direct_exact` 帶 `RecipientAddress` 以及買家必須一位不差轉出的 `ExactAmount`。這些指示欄位在其他項目上都是 `nil`；一個選項的身分是 `(ChainNamespace, ChainReference, TokenAddress)`，而不是它在切片中的位置。`Transfers` 是已確認的收款紀錄——`TransactionID`、`EventIndex`、`Amount`、`ExplorerTransactionURL`——在有付款確認前一直是 `[]`。完整欄位說明見 [REST API 文件](https://github.com/invoqmoney/api)。
