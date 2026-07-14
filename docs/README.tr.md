# invoq Go SDK'sı

[English](../README.md) · [Bahasa Indonesia](./README.id.md) · [Español](./README.es-419.md) · [Français](./README.fr.md) · [Português](./README.pt-BR.md) · [Tiếng Việt](./README.vi.md) · **Türkçe** · [ไทย](./README.th.md) · [简体中文](./README.zh-Hans.md) · [繁體中文](./README.zh-Hant.md)

> Bu belge İngilizce README'nin çevirisidir; bir fark olursa [İngilizce sürüm](../README.md) esas alınır.

invoq sunucu API'leri ve webhook doğrulaması için Go SDK'sı. Stablecoin faturaları oluşturun, test ödemelerini simüle edin ve siparişleri imzalı webhook'larla işleyin.

Bu modülü yalnızca sunucunuzda kullanın. Gizli anahtarları kabul eder ve istemci tarafı uygulamalara derlenmemelidir.

## Sunucu SDK'ları

Bu dillerin herhangi biriyle arka ucunuzdan fatura oluşturun ve webhook'ları doğrulayın — aynı REST API, aynı webhook imzası. Bu repo, Go SDK'sıdır.

| Dil | Repo |
| --- | --- |
| Node.js | [github.com/invoqmoney/sdk-js](https://github.com/invoqmoney/sdk-js) (`@invoq/server`) |
| Python | [github.com/invoqmoney/sdk-python](https://github.com/invoqmoney/sdk-python) |
| PHP | [github.com/invoqmoney/sdk-php](https://github.com/invoqmoney/sdk-php) |
| Go | **bu repo** |
| Rust | [github.com/invoqmoney/sdk-rust](https://github.com/invoqmoney/sdk-rust) |
| Ruby | [github.com/invoqmoney/sdk-ruby](https://github.com/invoqmoney/sdk-ruby) |

Tarayıcı tarafı her arka uç için aynıdır: **`@invoq/checkout`** (JavaScript, [github.com/invoqmoney/sdk-js](https://github.com/invoqmoney/sdk-js) içinde) her ön uç için sayfa içi ödeme penceresini açar.

## Kurulum

```sh
go get github.com/invoqmoney/sdk-go
```

Go 1.22 veya üstünü ister.

## Anahtarlarınızı alın

1. [invoq paneline](https://app.invoq.money) giriş yapın ve bir proje oluşturun.
2. **API keys** sayfasında bir gizli anahtar oluşturun. Test anahtarları `sk_test_` ile, canlı anahtarlar `sk_live_` ile başlar. Anahtarın modu, faturaların test mi canlı mı olacağını belirler.
3. Projenizin **webhooks** ayarlarında webhook URL'nizi kaydedin. O modun webhook sırrı (`whsec_...`) yalnızca bir kez, webhook'u ilk etkinleştirdiğinizde gösterilir — hemen saklayın. Webhook URL'leri herkese açık HTTPS URL'leri olmalı.

İkisini de sunucu ortamınıza ekleyin:

```sh
INVOQ_SECRET_KEY=sk_test_...
INVOQ_WEBHOOK_SECRET=whsec_...
```

Test anahtarlarıyla başlayın. Canlı ortama geçerken canlı anahtara ve canlı webhook sırrına geçin.

## İstemci oluşturun

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

Canlı ortam varsayılanları:

- API origin'i: `https://api.invoq.money`
- İstek zaman aşımı: 10 saniye
- User-Agent: `invoq-go/<modül sürümü>`

Yerel geliştirmede veya önizleme testlerinde bunları değiştirin:

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

`apiOrigin`, kullanıcı adı, parola, yol, sorgu ya da hash parçaları içermeyen mutlak bir `http` veya `https` origin'i olmalı. SDK, `/v1/...` API yollarını ekler.

## Faturalar

Sunucunuzdan bir fatura oluşturun:

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
	// Hatayı işleyin.
}

_ = invoice.ID
```

Notlar:

- Tutarı sunucu tarafında belirleyin. İstemciden gelen tutarlara güvenmeyin.
- `amount`, `0.01` ile `999.99` arasında, en fazla 2 ondalık basamaklı, USD cinsinden ondalık bir dizedir — örneğin `129` veya `129.99`.
- `invoice.paid` webhook'larını siparişinize geri bağlamak için `reference_id` kullanın. Oluşturmayı yeniden denemeyi de güvenli kılar: aynı `reference_id` ve aynı fatura koşullarıyla tekrar oluşturursanız kopya yerine mevcut faturayı alırsınız; farklı koşullar ise `409 reference_id_conflict` API hatasıyla başarısız olur.
- İsteğe bağlı istek dizeleri için `invoq.String(...)` kullanın. `return_url` değerini ayarlamak için `invoq.StringOrNull(...)`, JSON `null` göndermek için `invoq.NullString()` kullanın; alanı atlamak için ise ayarsız bırakın.

Herkese açık bir fatura getirin:

```go
invoice, err := client.Invoices.Get(ctx, "inv_123")
```

`Get`, `*invoq.PublicInvoice` döndürür. Herkese açık fatura okumaları özel `reference_id`'nizi içermez; işleme eşlemesi için webhook'ları veya kendi sipariş deponuzu kullanın.

## Test ödemeleri

Test faturaları gerçek para alamaz. Sunucunuzdan bir ödeme simüle edin:

```go
paidInvoice, err := client.Invoices.CreateTestPayment(ctx, invoice.ID, invoq.CreateTestPaymentInput{
	Amount: invoice.Amount,
})
if err != nil {
	// Hatayı işleyin.
}

_ = paidInvoice.Status // tamamen ödendiğinde invoq.InvoiceStatusPaid
```

`CreateTestPayment` yalnızca `sk_test_` anahtarıyla oluşturulmuş faturalarda çalışır. Ödemeler fatura tutarına ulaştığında fatura `paid` olur ve invoq, test webhook URL'nize gerçekten imzalanmış bir `invoice.paid` webhook'u gönderir. Kısmi tutarlara izin verilir; sonuç `partially_paid` olur.

Webhook'ları kendi makinenizde almak için yerel sunucunuzu ngrok veya cloudflared gibi bir HTTPS tüneliyle dışa açın ve tünel URL'sini panelde test webhook URL'niz olarak kaydedin. Panel, bağlantıyı denetlemek için imzalı bir `webhook.ping` de gönderebilir.

## Webhook'lar

Ham istek gövdesini `VerifyWebhook`'a geçirin. Doğrulamadan önce JSON'u ayrıştırıp yeniden serileştirmeyin.

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

		// *referenceID için siparişi idempotent şekilde işleyin.
	}

	response.WriteHeader(http.StatusOK)
}
```

Siparişleri sunucunuzda `invoice.paid` webhook'larıyla işleyin. `IsInvoicePaid(event)` true olduğunda fatura otomatik olarak işlenmeye hazırdır; durumu `paid`, `settling` ya da `settled` olur. `review_required` durumundaki fatura henüz `invoice.paid` webhook'u göndermez. İnceleme onaylandıktan sonra gelecek `invoice.paid` webhook'unu bekleyin.

Başarısız teslimatlar yeniden denenir; bu yüzden `reference_id` veya fatura `id`'siyle idempotent şekilde işleyin ve tekrar gelen teslimatları yok sayın. Hızla 2xx dönün; diğer her durum kodu başarısız teslimat sayılır.

`VerifyWebhook`, `http.Header` kabul eder. `invoq-signature` başlık değerine zaten sahipseniz `VerifyWebhookWithSignature` kullanın.

Webhook doğrulama hataları `*invoq.SignatureVerificationError` döndürür. SDK, 5 dakikalık bir zaman damgası toleransı tanır. Başarısız teslimatlar her yeniden denemede yeniden imzalanır; bu yüzden normal şekilde yeniden denenen teslimatlar bu pencere içinde yine de doğrulanır. İmza başlığı `invoq-signature: t=<unix saniye>,v1=<"<t>.<ham gövde>" değerinin onaltılık HMAC-SHA256'sı>` biçimindedir.

## Hatalar

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

Bağlantı hataları, istek zaman aşımları, yapılandırma hataları ve yanıt ayrıştırma hataları `*invoq.Error` döndürür. 2xx olmayan API yanıtları `*invoq.APIError` döndürür. Tüm SDK hataları `invoq.SDKError` arayüzünü uygular.

```go
var sdkErr invoq.SDKError
if errors.As(err, &sdkErr) {
	fmt.Println(sdkErr)
}
```

## API referansı

```go
client, err := invoq.New(apiKey,
	invoq.WithAPIOrigin("https://api.invoq.money"), // isteğe bağlı, varsayılanı değiştirir
	invoq.WithTimeout(invoq.DefaultTimeout),        // isteğe bağlı istek zaman aşımı
)
```

- `client.Invoices.Create(ctx, input)` bir fatura oluşturur. `input`: `Amount` (zorunlu), `Currency` (`InvoiceCurrencyUSD`, varsayılan), `Description`, `ReferenceID`, `ReturnURL`.
- `client.Invoices.Get(ctx, invoiceID)` herkese açık bir faturayı getirir ve `*invoq.PublicInvoice` döndürür.
- `client.Invoices.CreateTestPayment(ctx, invoiceID, input)` test faturasında ödeme simüle eder ve `*invoq.TestPaymentInvoice` döndürür.
- `invoq.VerifyWebhook(rawBody, headers, webhookSecret)` bir webhook'u doğrular ve `invoq.WebhookEvent` döndürür.
- `invoq.IsInvoicePaid(event)` ve `invoq.AsInvoicePaidEvent(event)`, tipli `invoice.paid` olaylarını ayırt eder.
- SDK, `User-Agent` için Go modül sürümünü derleme bilgisinden (build info) algılar. `v0.1.0` gibi yayımlanan etiketler `v` öneki olmadan gönderilir; modül sürümü olmayan yerel kaynak derlemeleri `unknown` kullanır.
