# SDK de invoq para Go

[English](../README.md) · [Bahasa Indonesia](./README.id.md) · **Español** · [Français](./README.fr.md) · [Português](./README.pt-BR.md) · [Tiếng Việt](./README.vi.md) · [Türkçe](./README.tr.md) · [ไทย](./README.th.md) · [简体中文](./README.zh-Hans.md) · [繁體中文](./README.zh-Hant.md)

> Este documento es una traducción del README en inglés; si algo difiere, vale la [versión en inglés](../README.md).

SDK de Go para las APIs de servidor de invoq y la verificación de webhooks. Crea facturas en stablecoins, simula pagos de prueba y procesa pedidos a partir de webhooks firmados.

Usa este módulo solo en tu servidor. Acepta claves secretas y no debe compilarse en aplicaciones del lado del cliente.

## SDKs de servidor

Crea facturas y verifica webhooks desde tu backend en cualquiera de estos lenguajes — la misma REST API y la misma firma de webhook. Este repositorio es el SDK de Go.

| Lenguaje | Repositorio |
| --- | --- |
| Node.js | [github.com/invoqmoney/sdk-js](https://github.com/invoqmoney/sdk-js) (`@invoq/server`) |
| Python | [github.com/invoqmoney/sdk-python](https://github.com/invoqmoney/sdk-python) |
| PHP | [github.com/invoqmoney/sdk-php](https://github.com/invoqmoney/sdk-php) |
| Go | **este repositorio** |
| Rust | [github.com/invoqmoney/sdk-rust](https://github.com/invoqmoney/sdk-rust) |
| Ruby | [github.com/invoqmoney/sdk-ruby](https://github.com/invoqmoney/sdk-ruby) |

El lado del navegador es el mismo para cualquier backend: **`@invoq/checkout`** (JavaScript, en [github.com/invoqmoney/sdk-js](https://github.com/invoqmoney/sdk-js)) abre la ventana de pago integrada en la página para cualquier frontend.

## Instalación

```sh
go get github.com/invoqmoney/sdk-go
```

Requiere Go 1.22 o más nuevo.

## Consigue tus claves

1. Inicia sesión en el [panel de invoq](https://app.invoq.money) y crea un proyecto.
2. En la página **API keys**, crea una clave secreta. Las claves de prueba empiezan con `sk_test_`, las claves de producción con `sk_live_`. El modo de la clave determina si las facturas son de prueba o de producción.
3. En la configuración de **webhooks** de tu proyecto, guarda tu URL de webhook. El secreto del webhook (`whsec_...`) de ese modo se muestra una sola vez, cuando activas el webhook por primera vez — guárdalo de inmediato. La URL del webhook debe ser HTTPS y pública.

Agrega ambos al entorno de tu servidor:

```sh
INVOQ_SECRET_KEY=sk_test_...
INVOQ_WEBHOOK_SECRET=whsec_...
```

Empieza con las claves de prueba. Cambia a la clave de producción y al secreto de webhook de producción cuando pases a producción.

## Crea un cliente

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

Valores predeterminados de producción:

- Origin de la API: `https://api.invoq.money`
- Tiempo de espera de la solicitud: 10 segundos
- User-Agent: `invoq-go/<versión del módulo>`

Sobrescríbelos durante el desarrollo local o las pruebas de previsualización:

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

`apiOrigin` debe ser un origen `http` o `https` absoluto, sin usuario, contraseña, ruta, consulta ni hash. El SDK le agrega las rutas de API `/v1/...`.

## Facturas

Crea una factura desde tu servidor:

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
	// Maneja el error.
}

_ = invoice.ID
```

Notas:

- Define el monto en el servidor. No confíes en montos que manda el cliente.
- `amount` es una cadena decimal en USD de `0.01` a `1000000.00` con hasta 2 decimales, como `129` o `129.99`.
- Usa `reference_id` para vincular los webhooks `invoice.paid` con tu pedido. También hace que puedas reintentar la creación sin riesgo: si creas otra factura con el mismo `reference_id` y los mismos términos, recibes la factura existente en lugar de un duplicado; si los términos son distintos, falla con un error de API `409 reference_id_conflict`.
- Usa `invoq.String(...)` para las cadenas opcionales de la solicitud. Usa `invoq.StringOrNull(...)` para definir `return_url`, `invoq.NullString()` para enviar `null` en JSON, y deja el campo sin definir para omitirlo.

Obtén una factura pública:

```go
invoice, err := client.Invoices.Get(ctx, "inv_123")
```

`Get` devuelve `*invoq.PublicInvoice`. Las lecturas de facturas públicas no incluyen tu `reference_id` privado; usa los webhooks o tu propio almacén de pedidos para vincular el procesamiento.

## Pagos de prueba

Las facturas de prueba no pueden recibir fondos reales. Simula un pago desde tu servidor:

```go
paidInvoice, err := client.Invoices.CreateTestPayment(ctx, invoice.ID, invoq.CreateTestPaymentInput{
	Amount: invoice.Amount,
})
if err != nil {
	// Maneja el error.
}

_ = paidInvoice.Status // invoq.InvoiceStatusPaid cuando está totalmente pagada
```

`CreateTestPayment` solo funciona con facturas creadas con una clave `sk_test_`. Cuando los pagos alcanzan el monto de la factura, la factura pasa a `paid` e invoq envía un webhook `invoice.paid` firmado de verdad a tu URL de webhook de prueba. Se permiten montos parciales, que producen `partially_paid`.

Para recibir webhooks en tu máquina, expón tu servidor local con un túnel HTTPS como ngrok o cloudflared y guarda la URL del túnel como tu URL de webhook de prueba en el panel. El panel también puede enviar un `webhook.ping` firmado para revisar la conectividad.

## Webhooks

Pasa el cuerpo sin procesar de la solicitud a `VerifyWebhook`. No proceses el JSON ni lo vuelvas a serializar antes de verificarlo.

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

		// Procesa el pedido de *referenceID de forma idempotente.
	}

	response.WriteHeader(http.StatusOK)
}
```

Usa los webhooks `invoice.paid` para procesar los pedidos en tu servidor. Cuando `IsInvoicePaid(event)` es true, la factura está lista para procesarse automáticamente; su estado es `paid`, `settling` o `settled`. Una factura `review_required` aún no emite un webhook `invoice.paid`. Espera un webhook `invoice.paid` posterior después de que se apruebe la revisión.

Las entregas fallidas se reintentan, así que procesa de forma idempotente por `reference_id` o por `id` de factura y trata las entregas repetidas como operaciones sin efecto. Responde con un 2xx rápido; cualquier otro estado cuenta como entrega fallida.

`VerifyWebhook` acepta `http.Header`. Usa `VerifyWebhookWithSignature` cuando ya tengas el valor del encabezado `invoq-signature`.

Las fallas de verificación de webhook devuelven `*invoq.SignatureVerificationError`. El SDK permite una tolerancia de 5 minutos en el timestamp. Las entregas fallidas se firman de nuevo en cada reintento, así que las entregas reintentadas normales siguen verificándose dentro de esa ventana. El encabezado de firma es `invoq-signature: t=<segundos unix>,v1=<HMAC-SHA256 en hex de "<t>.<cuerpo sin procesar>">`.

## Errores

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

Las fallas de conexión, los tiempos de espera de solicitud agotados, los errores de configuración y las fallas al procesar la respuesta devuelven `*invoq.Error`. Las respuestas de API no 2xx devuelven `*invoq.APIError`. Todos los errores del SDK implementan `invoq.SDKError`.

```go
var sdkErr invoq.SDKError
if errors.As(err, &sdkErr) {
	fmt.Println(sdkErr)
}
```

## Referencia de la API

```go
client, err := invoq.New(apiKey,
	invoq.WithAPIOrigin("https://api.invoq.money"), // opcional, sobrescribe el valor predeterminado
	invoq.WithTimeout(invoq.DefaultTimeout),        // opcional, tiempo de espera de la solicitud
)
```

- `client.Invoices.Create(ctx, input)` crea una factura. `input`: `Amount` (requerido), `Currency` (`InvoiceCurrencyUSD`, predeterminado), `Description`, `ReferenceID`, `ReturnURL`.
- `client.Invoices.Get(ctx, invoiceID)` trae una factura pública y devuelve `*invoq.PublicInvoice`.
- `client.Invoices.CreateTestPayment(ctx, invoiceID, input)` simula un pago en una factura de prueba y devuelve `*invoq.TestPaymentInvoice`.
- `invoq.VerifyWebhook(rawBody, headers, webhookSecret)` verifica un webhook y devuelve `invoq.WebhookEvent`.
- `invoq.IsInvoicePaid(event)` y `invoq.AsInvoicePaidEvent(event)` identifican eventos `invoice.paid` tipados.
- El SDK detecta la versión de su módulo de Go a partir de la información de compilación para el `User-Agent`. Las etiquetas publicadas como `v0.1.0` se envían sin el prefijo `v`; las compilaciones locales desde el código fuente sin versión de módulo usan `unknown`.
