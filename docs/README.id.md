# invoq Go SDK

[English](../README.md) · **Bahasa Indonesia** · [Español](./README.es-419.md) · [Français](./README.fr.md) · [Português](./README.pt-BR.md) · [Tiếng Việt](./README.vi.md) · [Türkçe](./README.tr.md) · [ไทย](./README.th.md) · [简体中文](./README.zh-Hans.md) · [繁體中文](./README.zh-Hant.md)

> Dokumen ini terjemahan dari README bahasa Inggris; kalau ada perbedaan, [versi bahasa Inggris](../README.md) yang berlaku.

SDK Go untuk API server invoq dan verifikasi webhook. Buat invoice stablecoin, simulasikan pembayaran uji coba, dan proses pesanan dari webhook bertanda tangan.

Gunakan modul ini hanya di server Anda. Modul ini menerima kunci rahasia dan tidak boleh dikompilasi ke dalam aplikasi sisi klien.

## SDK server

Buat invoice dan verifikasi webhook dari backend Anda dalam bahasa mana pun berikut — REST API dan tanda tangan webhook-nya sama persis. Repo ini adalah SDK Go.

| Bahasa | Repositori |
| --- | --- |
| Node.js | [github.com/invoqmoney/sdk-js](https://github.com/invoqmoney/sdk-js) (`@invoq/server`) |
| Python | [github.com/invoqmoney/sdk-python](https://github.com/invoqmoney/sdk-python) |
| PHP | [github.com/invoqmoney/sdk-php](https://github.com/invoqmoney/sdk-php) |
| Go | **repo ini** |
| Rust | [github.com/invoqmoney/sdk-rust](https://github.com/invoqmoney/sdk-rust) |
| Ruby | [github.com/invoqmoney/sdk-ruby](https://github.com/invoqmoney/sdk-ruby) |

Sisi browser-nya sama untuk setiap backend: **`@invoq/checkout`** (JavaScript, di [github.com/invoqmoney/sdk-js](https://github.com/invoqmoney/sdk-js)) membuka jendela checkout yang tertanam di halaman untuk frontend apa pun.

## Instalasi

```sh
go get github.com/invoqmoney/sdk-go
```

Membutuhkan Go 1.22 atau lebih baru.

## Siapkan kunci Anda

1. Masuk ke [dashboard invoq](https://app.invoq.money) dan buat sebuah proyek.
2. Di halaman **API keys**, buat kunci rahasia (secret key). Kunci uji coba diawali `sk_test_`, kunci produksi diawali `sk_live_`. Mode kuncinya menentukan apakah invoice yang dibuat itu uji coba atau produksi.
3. Di pengaturan **webhooks** proyek Anda, simpan URL webhook Anda. Kunci rahasia webhook (`whsec_...`) untuk mode itu hanya ditampilkan sekali, saat webhook pertama kali diaktifkan — langsung simpan. URL webhook harus berupa URL HTTPS yang bisa diakses publik.
4. Siapkan **Receiving wallet** Anda sebelum go live. Invoice uji coba tidak membutuhkannya; invoice live tanpa tujuan penyelesaian gagal dengan `409 no_payment_options_available`.

Tambahkan keduanya ke lingkungan server Anda:

```sh
INVOQ_SECRET_KEY=sk_test_...
INVOQ_WEBHOOK_SECRET=whsec_...
```

Mulailah dengan kunci uji coba. Ganti ke kunci produksi dan kunci rahasia webhook produksi saat masuk produksi.

## Buat klien

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

Bawaan produksi:

- Origin API: `https://api.invoq.money`
- Timeout request: 10 detik
- User-Agent: `invoq-go/<versi modul>`

Timpa nilai-nilai ini saat pengembangan lokal atau pengujian pratinjau:

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

`apiOrigin` harus berupa origin `http` atau `https` absolut tanpa bagian username, password, path, query, atau hash. SDK menambahkan path API `/v1/...`.

## Invoice

Buat invoice dari server Anda:

```go
ctx := context.Background()

invoice, err := client.Invoices.Create(ctx, invoq.CreateInvoiceInput{
	Amount:      "129",
	Description: invoq.String("SaaS boilerplate"),
	ReferenceID: invoq.String("order_1234"),
	ReturnURL:   invoq.StringOrNull("https://merchant.example/thanks"),
})
if err != nil {
	// Tangani error-nya.
}

_ = invoice.ID
```

Catatan:

- Tentukan jumlahnya di sisi server. Jangan percaya jumlah yang dikirim klien.
- `amount` adalah string desimal USD dari `0.01` sampai `1000000.00` dengan maksimal 2 angka di belakang koma, misalnya `129` atau `129.99`. Mata uangnya selalu USD, dan mode uji coba atau live ditentukan oleh kuncinya — keduanya bukan field permintaan.
- Pakai `reference_id` untuk memetakan webhook `invoice.paid` kembali ke pesanan Anda. Ini juga membuat pembuatan invoice aman diulang: membuat lagi dengan `reference_id` yang sama dan ketentuan invoice yang sama mengembalikan invoice yang sudah ada, bukan duplikat, sementara ketentuan yang berbeda gagal dengan error API `409 reference_id_conflict`.
- Pakai `invoq.String(...)` untuk string request yang opsional. Pakai `invoq.StringOrNull(...)` untuk mengisi `return_url`, `invoq.NullString()` untuk mengirim JSON `null`, dan biarkan field-nya tidak diisi untuk menghilangkannya.

Ambil invoice publik:

```go
invoice, err := client.Invoices.Get(ctx, "inv_123")
```

`Get` mengembalikan `*invoq.PublicInvoice`. Pembacaan invoice publik tidak menyertakan `reference_id` privat Anda; gunakan webhook atau penyimpanan pesanan Anda sendiri untuk pemetaan pemrosesan pesanan.

## Pembayaran uji coba

Invoice uji coba tidak bisa menerima dana sungguhan. Simulasikan pembayaran dari server Anda:

```go
paidInvoice, err := client.Invoices.CreateTestPayment(ctx, invoice.ID, invoq.CreateTestPaymentInput{
	Amount: invoice.Amount,
})
if err != nil {
	// Tangani error-nya.
}

_ = paidInvoice.Status // invoq.InvoiceStatusPaid saat dibayar penuh
```

`CreateTestPayment` hanya bekerja pada invoice yang dibuat dengan kunci `sk_test_`. Begitu pembayaran mencapai jumlah invoice, invoice menjadi `paid` dan invoq mengirim webhook `invoice.paid` bertanda tangan sungguhan ke URL webhook uji coba Anda. Jumlah parsial diperbolehkan dan menghasilkan `partially_paid`.

Untuk menerima webhook di mesin Anda sendiri, buka server lokal lewat tunnel HTTPS seperti ngrok atau cloudflared, lalu simpan URL tunnel-nya sebagai URL webhook uji coba di dashboard.

## Webhook

Teruskan isi request mentah ke `VerifyWebhook`. Jangan mem-parse JSON lalu men-serialisasi ulang sebelum verifikasi.

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

		// Proses pesanan untuk *referenceID secara idempoten.
	}

	response.WriteHeader(http.StatusOK)
}
```

Gunakan webhook `invoice.paid` untuk memproses pesanan di server Anda. Saat `IsInvoicePaid(event)` bernilai true, invoice siap diproses otomatis; statusnya `paid`, `settling`, atau `settled`. Invoice dengan status `review_required` tidak mengirim `invoice.paid` sampai peninjauannya selesai.

invoq juga mengirim `invoice.payment_reversed` ketika invoice yang tadinya lunas turun lagi di bawah jumlahnya — misalnya karena reorg rantai membatalkan transfer yang sudah terkonfirmasi. Tangkap event itu dengan `invoq.IsInvoicePaymentReversed(event)`, dekode dengan `invoq.AsInvoicePaymentReversedEvent(event)`, lalu tahan atau batalkan pemrosesan sesuai kebijakan Anda sendiri.

Pengiriman yang gagal akan diulang (sampai 5 kali, dengan jeda 1 menit, 5 menit, 30 menit, lalu 2 jam), jadi proses pesanan secara idempoten berdasarkan `reference_id` atau `id` invoice dan abaikan kiriman ulang. Urutan kedatangannya juga tidak dijamin — simpan snapshot dengan `payment_revision` tertinggi. Balas 2xx secepatnya; status lain dihitung sebagai pengiriman gagal dan akan diulang, termasuk redirect dan `4xx`.

`VerifyWebhook` menerima `http.Header`. Pakai `VerifyWebhookWithSignature` kalau Anda sudah punya nilai header `invoq-signature`.

Kegagalan verifikasi webhook mengembalikan `*invoq.SignatureVerificationError`. SDK memberi toleransi timestamp 5 menit. Pengiriman yang gagal akan ditandatangani ulang setiap kali dicoba lagi, jadi pengiriman ulang yang normal tetap lolos verifikasi di dalam jendela waktu tersebut. Header tanda tangannya `invoq-signature: t=<detik unix>,v1=<HMAC-SHA256 heks dari "<t>.<isi request mentah>">`.

## Error

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

Kegagalan koneksi, timeout request, error konfigurasi, dan kegagalan mem-parse respons mengembalikan `*invoq.Error`. Respons API non-2xx mengembalikan `*invoq.APIError`. Semua error SDK mengimplementasikan `invoq.SDKError`.

```go
var sdkErr invoq.SDKError
if errors.As(err, &sdkErr) {
	fmt.Println(sdkErr)
}
```

## Referensi API

```go
client, err := invoq.New(apiKey,
	invoq.WithAPIOrigin("https://api.invoq.money"), // opsional, menimpa bawaan
	invoq.WithTimeout(invoq.DefaultTimeout),        // opsional, timeout request
)
```

- `client.Invoices.Create(ctx, input)` membuat invoice. `input`: `Amount` (wajib), `Description`, `ReferenceID`, `ReturnURL`.
- `client.Invoices.Get(ctx, invoiceID)` mengambil invoice publik dan mengembalikan `*invoq.PublicInvoice`.
- `client.Invoices.CreateTestPayment(ctx, invoiceID, input)` menyimulasikan pembayaran pada invoice uji coba dan mengembalikan `*invoq.TestPaymentInvoice`.
- `invoq.VerifyWebhook(rawBody, headers, webhookSecret)` memverifikasi webhook dan mengembalikan `invoq.WebhookEvent`.
- `invoq.IsInvoicePaid(event)` dan `invoq.AsInvoicePaidEvent(event)` mengidentifikasi event `invoice.paid` yang bertipe. `invoq.IsInvoicePaymentReversed(event)` dan `invoq.AsInvoicePaymentReversedEvent(event)` melakukan hal yang sama untuk `invoice.payment_reversed`. Keduanya menolak event yang bentuknya tidak sesuai; tipe event yang belum dikenal versi SDK ini tetap lolos verifikasi dan dikembalikan apa adanya.
- SDK mendeteksi versi modul Go-nya dari informasi build untuk `User-Agent`. Tag rilis seperti `v0.1.0` dikirim tanpa awalan `v`; build dari sumber lokal tanpa versi modul memakai `unknown`.

`Invoices.Get` mengembalikan bentuk invoice publik yang dipakai halaman checkout ter-hosting: bentuk respons pembuatan ditambah `AmountPaid`, `Project`, dan `Transfers`, dikurangi `ReferenceID`. Gunakan respons pembuatan atau webhook `invoice.paid` saat Anda membutuhkan `reference_id` merchant.

Dua field status. `Status` adalah status pembukuan — `unpaid`, `partially_paid`, `paid`, `settling`, `settled`, `review_required` — dan tiga nilai yang berarti sudah dibayar hanya berbeda pada seberapa jauh dananya bergerak ke dompet Anda. `CheckoutStatus` adalah status yang dilihat pembayar — `open`, `confirming`, `expired`, `paid`, `unavailable` — dan tidak pernah menjadi izin memproses pesanan. `PaymentRevision` naik setiap kali kumpulan pembayaran terkonfirmasi berubah, jadi Anda bisa membuang snapshot yang lebih lama dari yang sudah Anda pegang.

Jumlah di respons dinormalkan ke 4 angka desimal: buat dengan `129` dan invoice mengembalikan `Amount` `129.0000`. Bandingkan jumlah secara numerik, bukan sebagai string. `AmountDue` diturunkan sebagai `max(amount - amount_paid, 0)` dan memakai skala 18 desimal yang sama dengan `AmountPaid`; `AmountOverpaid` adalah kebalikannya, `max(amount_paid - amount, 0)`, jadi Anda tidak perlu mengurangkannya sendiri.

`PaymentOptions` berisi instruksi pembayarannya, ditetapkan saat pembuatan dan `[]` di mode uji coba. Tiap entri dibedakan oleh `Status`, lalu `CollectionMethod`: hanya `ready` yang bisa dibayar, `evm_deposit` membawa `DepositAddress` dan `SuggestedAmount`, `direct_exact` membawa `RecipientAddress` dan `ExactAmount` yang harus dikirim pembeli persis sampai digit terakhir. Field instruksi itu bernilai `nil` pada entri lainnya, dan identitas sebuah opsi adalah `(ChainNamespace, ChainReference, TokenAddress)`, bukan posisinya di dalam slice. `Transfers` adalah jejak penerimaan terkonfirmasi — `TransactionID`, `EventIndex`, `Amount`, `ExplorerTransactionURL` — dan tetap `[]` sampai ada pembayaran yang terkonfirmasi. Referensi field lengkap: [dokumen REST API](https://github.com/invoqmoney/api).
