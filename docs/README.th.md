# invoq Go SDK

[English](../README.md) · [Bahasa Indonesia](./README.id.md) · [Español](./README.es-419.md) · [Français](./README.fr.md) · [Português](./README.pt-BR.md) · [Tiếng Việt](./README.vi.md) · [Türkçe](./README.tr.md) · **ไทย** · [简体中文](./README.zh-Hans.md) · [繁體中文](./README.zh-Hant.md)

> เอกสารนี้แปลจาก README ภาษาอังกฤษ หากมีข้อความไม่ตรงกัน ให้ยึด[ฉบับภาษาอังกฤษ](../README.md)เป็นหลัก

Go SDK สำหรับ API ฝั่งเซิร์ฟเวอร์ของ invoq และการตรวจสอบ webhook สร้างใบแจ้งหนี้ stablecoin จำลองการจ่ายทดสอบ และจัดการคำสั่งซื้อจาก webhook ที่มีลายเซ็นกำกับ

ใช้โมดูลนี้บนเซิร์ฟเวอร์ของคุณเท่านั้น โมดูลนี้รับคีย์ลับและต้องไม่ถูกคอมไพล์รวมเข้าไปในแอปพลิเคชันฝั่งไคลเอนต์

## SDK ฝั่งเซิร์ฟเวอร์

สร้างใบแจ้งหนี้และตรวจสอบ webhook จากแบ็กเอนด์ของคุณด้วยภาษาใดก็ได้เหล่านี้ — REST API และลายเซ็น webhook เหมือนกันทุกภาษา repo นี้คือ SDK สำหรับ Go

| ภาษา | Repo |
| --- | --- |
| Node.js | [github.com/invoqmoney/sdk-js](https://github.com/invoqmoney/sdk-js) (`@invoq/server`) |
| Python | [github.com/invoqmoney/sdk-python](https://github.com/invoqmoney/sdk-python) |
| PHP | [github.com/invoqmoney/sdk-php](https://github.com/invoqmoney/sdk-php) |
| Go | **repo นี้** |
| Rust | [github.com/invoqmoney/sdk-rust](https://github.com/invoqmoney/sdk-rust) |
| Ruby | [github.com/invoqmoney/sdk-ruby](https://github.com/invoqmoney/sdk-ruby) |

ฝั่งเบราว์เซอร์เหมือนกันหมดสำหรับทุกแบ็กเอนด์: **`@invoq/checkout`** (JavaScript อยู่ใน [github.com/invoqmoney/sdk-js](https://github.com/invoqmoney/sdk-js)) เปิดหน้าชำระเงินแบบฝังในหน้าเว็บให้ฟรอนต์เอนด์ใดก็ได้

## ติดตั้ง

```sh
go get github.com/invoqmoney/sdk-go
```

ต้องใช้ Go 1.22 ขึ้นไป

## รับคีย์ของคุณ

1. เข้าสู่ระบบ[แดชบอร์ด invoq](https://app.invoq.money) แล้วสร้างโปรเจกต์
2. ที่หน้า **API keys** สร้างคีย์ลับ (secret key) ขึ้นมา คีย์ทดสอบขึ้นต้นด้วย `sk_test_` คีย์จริงขึ้นต้นด้วย `sk_live_` โหมดของคีย์เป็นตัวกำหนดว่าใบแจ้งหนี้ที่สร้างจะเป็นแบบทดสอบหรือของจริง
3. ในการตั้งค่า **webhooks** ของโปรเจกต์ บันทึก URL ของ webhook ที่จะใช้ ซีเคร็ตของ webhook (`whsec_...`) สำหรับโหมดนั้นจะแสดงแค่ครั้งเดียวตอนเปิดใช้ webhook ครั้งแรก — รีบเก็บไว้ทันที URL ของ webhook ต้องเป็น HTTPS ที่เข้าถึงได้แบบสาธารณะ

เพิ่มทั้งสองค่าเข้าเป็นตัวแปรสภาพแวดล้อมของเซิร์ฟเวอร์:

```sh
INVOQ_SECRET_KEY=sk_test_...
INVOQ_WEBHOOK_SECRET=whsec_...
```

เริ่มจากคีย์ทดสอบก่อน แล้วค่อยสลับเป็นคีย์จริงกับซีเคร็ต webhook ของจริงตอนใช้งานจริง

## สร้างไคลเอนต์

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

ค่าเริ่มต้นในสภาพแวดล้อมจริง:

- origin ของ API: `https://api.invoq.money`
- timeout ของ request: 10 วินาที
- User-Agent: `invoq-go/<เวอร์ชันโมดูล>`

ตอนพัฒนาบนเครื่องหรือทดสอบพรีวิว ให้ปรับทับค่าเริ่มต้นได้:

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

`apiOrigin` ต้องเป็น origin แบบ `http` หรือ `https` เต็มรูปแบบ โดยไม่มีส่วน username, password, path, query หรือ hash SDK จะต่อท้ายด้วยพาธ API `/v1/...`

## ใบแจ้งหนี้

สร้างใบแจ้งหนี้จากเซิร์ฟเวอร์ของคุณ:

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
	// จัดการข้อผิดพลาด
}

_ = invoice.ID
```

หมายเหตุ:

- กำหนดยอดเงินที่ฝั่งเซิร์ฟเวอร์เท่านั้น อย่าเชื่อยอดเงินที่ส่งมาจากฝั่งไคลเอนต์
- `amount` เป็นสตริงเลขทศนิยมสกุล USD ตั้งแต่ `0.01` ถึง `1000000.00` ทศนิยมไม่เกิน 2 ตำแหน่ง เช่น `129` หรือ `129.99`
- ใช้ `reference_id` เพื่อโยง webhook `invoice.paid` กลับไปหาคำสั่งซื้อของคุณ และยังทำให้การสร้างใบแจ้งหนี้ลองใหม่ได้อย่างปลอดภัย: ถ้าสร้างซ้ำด้วย `reference_id` เดิมและเงื่อนไขเดิม จะได้ใบแจ้งหนี้ใบเดิมกลับมาแทนที่จะเกิดใบซ้ำ ส่วนเงื่อนไขที่ต่างกันจะล้มเหลวด้วยข้อผิดพลาด API `409 reference_id_conflict`
- ใช้ `invoq.String(...)` สำหรับสตริงใน request ที่ไม่บังคับ ใช้ `invoq.StringOrNull(...)` เพื่อกำหนด `return_url`, ใช้ `invoq.NullString()` เพื่อส่งค่า JSON `null` และปล่อยฟิลด์ไว้โดยไม่กำหนดค่าเพื่อละเว้นฟิลด์นั้น

ดึงข้อมูลใบแจ้งหนี้สาธารณะ:

```go
invoice, err := client.Invoices.Get(ctx, "inv_123")
```

`Get` คืนค่าเป็น `*invoq.PublicInvoice` การอ่านข้อมูลใบแจ้งหนี้สาธารณะจะไม่มี `reference_id` ส่วนตัวของคุณ ให้ใช้ webhook หรือที่เก็บคำสั่งซื้อของคุณเองในการโยงเพื่อจัดการคำสั่งซื้อ

## การจ่ายเงินทดสอบ

ใบแจ้งหนี้ทดสอบรับเงินจริงไม่ได้ ให้จำลองการจ่ายจากเซิร์ฟเวอร์ของคุณ:

```go
paidInvoice, err := client.Invoices.CreateTestPayment(ctx, invoice.ID, invoq.CreateTestPaymentInput{
	Amount: invoice.Amount,
})
if err != nil {
	// จัดการข้อผิดพลาด
}

_ = paidInvoice.Status // invoq.InvoiceStatusPaid เมื่อจ่ายครบเต็มจำนวน
```

`CreateTestPayment` ใช้ได้เฉพาะกับใบแจ้งหนี้ที่สร้างด้วยคีย์ `sk_test_` เมื่อยอดจ่ายครบตามจำนวนของใบแจ้งหนี้ ใบแจ้งหนี้จะกลายเป็น `paid` แล้ว invoq จะส่ง webhook `invoice.paid` ที่ลงลายเซ็นจริงไปยัง URL webhook ทดสอบของคุณ จ่ายบางส่วนก็ได้ ผลจะเป็น `partially_paid`

ถ้าอยากรับ webhook บนเครื่องตัวเอง ให้เปิดเซิร์ฟเวอร์ในเครื่องออกสู่ภายนอกผ่าน HTTPS tunnel อย่าง ngrok หรือ cloudflared แล้วบันทึก URL ของ tunnel เป็น URL webhook ทดสอบในแดชบอร์ด แดชบอร์ดยังส่ง `webhook.ping` แบบมีลายเซ็นมาให้เช็กการเชื่อมต่อได้ด้วย

## Webhooks

ส่งเนื้อหา request ดิบไปยัง `VerifyWebhook` อย่าแปลง JSON แล้วแปลงกลับเป็นข้อความก่อนการตรวจสอบ

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

		// จัดการคำสั่งซื้อสำหรับ *referenceID อย่างปลอดภัยเมื่อรับซ้ำ
	}

	response.WriteHeader(http.StatusOK)
}
```

ให้ยึด webhook `invoice.paid` เป็นหลักในการจัดการคำสั่งซื้อบนเซิร์ฟเวอร์ เมื่อ `IsInvoicePaid(event)` เป็น true แปลว่าใบแจ้งหนี้พร้อมให้จัดการอัตโนมัติแล้ว โดยสถานะของใบแจ้งหนี้จะเป็น `paid`, `settling` หรือ `settled` ใบแจ้งหนี้สถานะ `review_required` จะยังไม่ส่ง webhook `invoice.paid` ให้รอ webhook `invoice.paid` ที่จะส่งตามมาหลังการตรวจสอบผ่าน

การส่งที่ล้มเหลวจะถูกส่งซ้ำ ดังนั้นให้จัดการคำสั่งซื้ออย่างปลอดภัยเมื่อรับซ้ำโดยอิง `reference_id` หรือ `id` ของใบแจ้งหนี้ และทำให้การส่งซ้ำไม่เกิดผลอะไร ตอบกลับด้วย 2xx ให้เร็ว สถานะอื่นใดถือว่าส่งไม่สำเร็จ

`VerifyWebhook` รับ `http.Header` ใช้ `VerifyWebhookWithSignature` เมื่อคุณมีค่าเฮดเดอร์ `invoq-signature` อยู่แล้ว

การตรวจสอบ webhook ที่ล้มเหลวจะคืนค่า `*invoq.SignatureVerificationError` SDK อนุญาตให้ timestamp คลาดเคลื่อนได้ไม่เกิน 5 นาที การส่งที่ล้มเหลวจะถูกลงลายเซ็นใหม่ในการส่งซ้ำแต่ละครั้ง การส่งซ้ำตามปกติจึงยังตรวจสอบผ่านภายในกรอบเวลานั้น ส่วนเฮดเดอร์ลายเซ็นมีรูปแบบ `invoq-signature: t=<วินาที unix>,v1=<HMAC-SHA256 ฐานสิบหกของ "<t>.<เนื้อหา request ดิบ>">`

## ข้อผิดพลาด

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

การเชื่อมต่อล้มเหลว, request หมดเวลารอ, ข้อผิดพลาดในการตั้งค่า และการแปลง response ล้มเหลว จะคืนค่าเป็น `*invoq.Error` การตอบกลับ API ที่ไม่ใช่ 2xx จะคืนค่าเป็น `*invoq.APIError` ข้อผิดพลาดทั้งหมดของ SDK จะ implement `invoq.SDKError`

```go
var sdkErr invoq.SDKError
if errors.As(err, &sdkErr) {
	fmt.Println(sdkErr)
}
```

## ข้อมูลอ้างอิง API

```go
client, err := invoq.New(apiKey,
	invoq.WithAPIOrigin("https://api.invoq.money"), // ระบุได้ถ้าต้องการทับค่าเริ่มต้น
	invoq.WithTimeout(invoq.DefaultTimeout),        // timeout ของ request ระบุได้
)
```

- `client.Invoices.Create(ctx, input)` สร้างใบแจ้งหนี้ โดย `input` ประกอบด้วย `Amount` (จำเป็น), `Currency` (`InvoiceCurrencyUSD` เป็นค่าเริ่มต้น), `Description`, `ReferenceID`, `ReturnURL`
- `client.Invoices.Get(ctx, invoiceID)` ดึงข้อมูลใบแจ้งหนี้สาธารณะและคืนค่า `*invoq.PublicInvoice`
- `client.Invoices.CreateTestPayment(ctx, invoiceID, input)` จำลองการจ่ายบนใบแจ้งหนี้ทดสอบและคืนค่า `*invoq.TestPaymentInvoice`
- `invoq.VerifyWebhook(rawBody, headers, webhookSecret)` ตรวจสอบ webhook และคืนค่า `invoq.WebhookEvent`
- `invoq.IsInvoicePaid(event)` และ `invoq.AsInvoicePaidEvent(event)` ใช้ระบุเหตุการณ์ `invoice.paid` แบบมีชนิดข้อมูล
- SDK จะตรวจหาเวอร์ชันโมดูล Go ของตัวเองจากข้อมูล build เพื่อใช้กับ `User-Agent` แท็กที่รีลีสแล้วอย่าง `v0.1.0` จะถูกส่งโดยตัดคำนำหน้า `v` ออก ส่วนการ build จากซอร์สในเครื่องที่ไม่มีเวอร์ชันโมดูลจะใช้ค่า `unknown`
