# invoq Go SDK

[English](../README.md) · [Bahasa Indonesia](./README.id.md) · [Español](./README.es-419.md) · [Français](./README.fr.md) · [Português](./README.pt-BR.md) · [Tiếng Việt](./README.vi.md) · [Türkçe](./README.tr.md) · [ไทย](./README.th.md) · **简体中文** · [繁體中文](./README.zh-Hant.md)

> 本文是英文版 README 的简体中文翻译；若表述有出入，以[英文版](../README.md)为准。

面向 invoq 服务端 API 与 webhook 验签的 Go SDK。可用它创建稳定币账单、模拟测试付款，并根据带签名的 webhook 处理订单。

本模块只能在你的服务端使用。它需要密钥，绝不能被编译进客户端应用中。

**在用 AI 写代码？把这段贴给它。**

```
用 invoq 给我的项目接入稳定币收款，从测试模式开始。写代码前先读文档 https://invoq.money/llms.txt
```

## 服务端 SDK

用下面任意一种语言，都能从你的后端创建账单、验证 webhook——REST API 和 webhook 签名完全一致。本仓库是 Go SDK。

| 语言 | 仓库 |
| --- | --- |
| Node.js | [github.com/invoqmoney/sdk-js](https://github.com/invoqmoney/sdk-js) (`@invoq/server`) |
| Python | [github.com/invoqmoney/sdk-python](https://github.com/invoqmoney/sdk-python) |
| PHP | [github.com/invoqmoney/sdk-php](https://github.com/invoqmoney/sdk-php) |
| Go | **本仓库** |
| Rust | [github.com/invoqmoney/sdk-rust](https://github.com/invoqmoney/sdk-rust) |
| Ruby | [github.com/invoqmoney/sdk-ruby](https://github.com/invoqmoney/sdk-ruby) |

无论后端选哪种语言，浏览器这一侧都一样：**`@invoq/checkout`**（JavaScript，在 [github.com/invoqmoney/sdk-js](https://github.com/invoqmoney/sdk-js)）为任意前端打开嵌在页面里的收银台弹窗。

## 安装

```sh
go get github.com/invoqmoney/sdk-go
```

要求 Go 1.22 及以上版本。

## 获取密钥

1. 登录 invoq 商户后台，创建一个项目。
2. 在 **API keys** 页面创建一把密钥（secret key）。测试密钥以 `sk_test_` 开头，正式密钥以 `sk_live_` 开头；用哪种密钥，决定开出的账单是测试单还是正式单。
3. 在项目的 **webhooks** 设置里保存你的 webhook URL。对应模式的 webhook 签名密钥（`whsec_...`）只在首次启用 webhook 时展示一次——记得马上存好。webhook URL 必须是公网可访问的 HTTPS 地址。
4. 上线前先设置 **Receiving wallet**。测试账单不需要它；没有结算去向的正式账单会以 `409 no_payment_options_available` 失败。

把两者都加进服务端环境变量：

```sh
INVOQ_SECRET_KEY=sk_test_...
INVOQ_WEBHOOK_SECRET=whsec_...
```

先用测试密钥跑通，上线时再换成正式密钥和正式 webhook 签名密钥。

## 创建客户端

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

生产环境默认值：

- API 地址：`https://api.invoq.money`
- 请求超时：10 秒
- User-Agent：`invoq-go/<模块版本>`

本地开发或预览测试时可以覆盖：

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

`apiOrigin` 必须是完整的 `http` 或 `https` origin，且不能包含用户名、密码、路径、查询参数或 hash 部分。SDK 会在其后拼接 `/v1/...` API 路径。

## 账单

在服务端创建账单：

```go
ctx := context.Background()

invoice, err := client.Invoices.Create(ctx, invoq.CreateInvoiceInput{
	Amount:      "129",
	Description: invoq.String("SaaS boilerplate"),
	ReferenceID: invoq.String("order_1234"),
	ReturnURL:   invoq.StringOrNull("https://merchant.example/thanks"),
})
if err != nil {
	// 处理错误。
}

_ = invoice.ID
```

说明：

- 金额要由服务端决定，不要相信客户端传来的金额。
- `amount` 是 `0.01` 到 `1000000.00` 之间的十进制美元字符串，最多两位小数，比如 `129` 或 `129.99`。币种恒为 USD，测试还是正式由密钥决定——两者都不是请求字段。
- 用 `reference_id` 把 `invoice.paid` webhook 对应回你的订单。它还让创建操作可以放心重试：用相同的 `reference_id` 和相同的账单条款再次创建，返回的是已有账单而不是重复开单；条款不同则会报 `409 reference_id_conflict` API 错误。
- 可选的请求字符串用 `invoq.String(...)`；用 `invoq.StringOrNull(...)` 设置 `return_url`，用 `invoq.NullString()` 发送 JSON `null`，不设置该字段则会将其省略。

查询公开账单：

```go
invoice, err := client.Invoices.Get(ctx, "inv_123")
```

`Get` 返回 `*invoq.PublicInvoice`。读取公开账单不会包含你私有的 `reference_id`；做履约映射时，请使用 webhook 或你自己的订单存储。

## 测试付款

测试账单收不了真钱，可以在服务端模拟一笔付款：

```go
paidInvoice, err := client.Invoices.CreateTestPayment(ctx, invoice.ID, invoq.CreateTestPaymentInput{
	Amount: invoice.Amount,
})
if err != nil {
	// 处理错误。
}

_ = paidInvoice.Status // 完全付清时为 invoq.InvoiceStatusPaid
```

`CreateTestPayment` 只对 `sk_test_` 密钥创建的账单有效。累计付款达到账单金额时，账单变为 `paid`，invoq 会向你的测试 webhook URL 发送一条真实签名的 `invoice.paid` webhook。也可以只付部分金额，账单会变成 `partially_paid`。

要在本机收 webhook，用 ngrok、cloudflared 之类的 HTTPS 隧道把本地服务暴露出去，再把隧道地址保存为商户后台里的测试 webhook URL。

## 托管收银页

每张账单都自带一个托管收银页：

```text
https://pay.invoq.money/<账单 id>
```

当页内收银台弹窗不合适时，把链接发出去或直接跳转过去就行。

## Webhooks

把原始请求体传给 `VerifyWebhook`。不要在验签前把 JSON 解析后再重新序列化。

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

		// 幂等地为 *referenceID 处理订单。
	}

	response.WriteHeader(http.StatusOK)
}
```

订单处理以服务端收到的 `invoice.paid` webhook 为准。`IsInvoicePaid(event)` 为 true 时，表示账单可以自动履约；其状态为 `paid`、`settling` 或 `settled`。`review_required` 账单在审核通过前不会发出任何 `invoice.paid`。

账单从已付款跌回不足额时，invoq 还会发 `invoice.payment_reversed`——比如链重组把一笔已确认的转账拿掉了。用 `invoq.IsInvoicePaymentReversed(event)` 接住它，用 `invoq.AsInvoicePaymentReversedEvent(event)` 解出内容，再按你自己的策略暂停或撤销履约。

投递失败会重试（最多 5 次，间隔依次为 1 分钟、5 分钟、30 分钟、2 小时），所以要按 `reference_id` 或账单 `id` 幂等地处理订单，重复送达直接忽略即可。送达顺序也不保证——请保留 `payment_revision` 最大的那份快照。请尽快返回 2xx；任何其他状态码都算投递失败并会重试，重定向和 `4xx` 也在其中。

`VerifyWebhook` 接受 `http.Header`。如果你已经拿到了 `invoq-signature` 头的值，可以改用 `VerifyWebhookWithSignature`。

webhook 验签失败会返回 `*invoq.SignatureVerificationError`。SDK 允许 5 分钟的时间戳容差。投递失败后每次重试都会重新签名，因此正常的重试投递仍能在该时间窗口内通过验签。签名头格式是 `invoq-signature: t=<unix 秒>,v1=<对 "<t>.<原始请求体>" 计算的 HMAC-SHA256 十六进制值>`。

## 错误处理

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

连接失败、请求超时、配置错误和响应解析失败会返回 `*invoq.Error`。API 返回非 2xx 时会返回 `*invoq.APIError`。所有 SDK 错误都实现了 `invoq.SDKError` 接口。

```go
var sdkErr invoq.SDKError
if errors.As(err, &sdkErr) {
	fmt.Println(sdkErr)
}
```

## API 参考

```go
client, err := invoq.New(apiKey,
	invoq.WithAPIOrigin("https://api.invoq.money"), // 可选，覆盖默认值
	invoq.WithTimeout(invoq.DefaultTimeout),        // 可选的请求超时
)
```

- `client.Invoices.Create(ctx, input)` —— 创建账单。`input`：`Amount`（必填）、`Description`、`ReferenceID`、`ReturnURL`。
- `client.Invoices.Get(ctx, invoiceID)` —— 查询公开账单，返回 `*invoq.PublicInvoice`。
- `client.Invoices.CreateTestPayment(ctx, invoiceID, input)` —— 在测试账单上模拟付款，返回 `*invoq.TestPaymentInvoice`。
- `invoq.VerifyWebhook(rawBody, headers, webhookSecret)` —— 对 webhook 验签，返回 `invoq.WebhookEvent`。
- `invoq.IsInvoicePaid(event)` 和 `invoq.AsInvoicePaidEvent(event)` —— 识别带类型的 `invoice.paid` 事件。`invoq.IsInvoicePaymentReversed(event)` 和 `invoq.AsInvoicePaymentReversedEvent(event)` 对 `invoice.payment_reversed` 做同样的事。两者都会拒绝结构不合法的事件；本版 SDK 尚未建模的事件类型同样能验签通过，并原样返回。
- SDK 会从构建信息中读取自己的 Go 模块版本，用于 `User-Agent`。像 `v0.1.0` 这样的发布标签会去掉 `v` 前缀后发送；本地源码构建若没有模块版本，则使用 `unknown`。

`Invoices.Get` 返回托管收银页使用的公开账单结构：即创建响应的结构，加上 `AmountPaid`、`Project` 和 `Transfers`，去掉 `ReferenceID`。如果需要商户侧的 `reference_id`，请使用创建账单的响应或 `invoice.paid` webhook。

账单有两个状态字段。`Status` 是记账状态——`unpaid`、`partially_paid`、`paid`、`settling`、`settled`、`review_required`，其中三个等同于已付款的取值只差在资金离你的钱包还有多远。`CheckoutStatus` 是付款人看到的状态——`open`、`confirming`、`expired`、`paid`、`unavailable`——它从不构成履约依据。`PaymentRevision` 每当已确认的付款集合变化就加一，你可以据此丢掉比手上更旧的快照。

响应里的金额统一格式化为 4 位小数：用 `129` 创建，账单返回 `Amount` `129.0000`。比较金额请按数值比，不要按字符串比。`AmountDue` 按 `max(amount - amount_paid, 0)` 派生，使用和 `AmountPaid` 相同的 18 位小数 scale；`AmountOverpaid` 与它互为镜像，即 `max(amount_paid - amount, 0)`，所以你不必自己做减法。

`PaymentOptions` 装的是付款指令，创建时即固定，测试模式下为 `[]`。每一项先按 `Status` 分辨，再按 `CollectionMethod` 分辨：只有 `ready` 可付，`evm_deposit` 带 `DepositAddress` 和 `SuggestedAmount`，`direct_exact` 带 `RecipientAddress` 以及买家必须一位不差转出的 `ExactAmount`。这些指令字段在其他条目上都是 `nil`；一个选项的身份是 `(ChainNamespace, ChainReference, TokenAddress)`，而不是它在切片中的位置。`Transfers` 是已确认的收款记录——`TransactionID`、`EventIndex`、`Amount`、`ExplorerTransactionURL`——在有付款确认前一直是 `[]`。完整字段说明见 [REST API 文档](https://github.com/invoqmoney/api)。
