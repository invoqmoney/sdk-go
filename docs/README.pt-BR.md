# SDK Go da invoq

[English](../README.md) · [Bahasa Indonesia](./README.id.md) · [Español](./README.es-419.md) · [Français](./README.fr.md) · **Português** · [Tiếng Việt](./README.vi.md) · [Türkçe](./README.tr.md) · [ไทย](./README.th.md) · [简体中文](./README.zh-Hans.md) · [繁體中文](./README.zh-Hant.md)

> Este documento é uma tradução do README em inglês; se algo divergir, vale a [versão em inglês](../README.md).

SDK Go para as APIs de servidor da invoq e verificação de webhooks. Crie faturas em stablecoin, simule pagamentos de teste e processe pedidos a partir de webhooks assinados.

Use este módulo apenas no seu servidor. Ele aceita chaves secretas e não deve ser compilado em aplicações do lado do cliente.

**Programa com IA? Cole isto.**

```
Adicione pagamentos em stablecoin ao meu projeto com invoq. Comece no modo de teste. Leia a documentação antes de escrever código: https://invoq.money/llms.txt
```

## SDKs de servidor

Crie faturas e verifique webhooks a partir do seu backend em qualquer uma destas linguagens — mesma REST API, mesma assinatura de webhook. Este repositório é o SDK de Go.

| Linguagem | Repositório |
| --- | --- |
| Node.js | [github.com/invoqmoney/sdk-js](https://github.com/invoqmoney/sdk-js) (`@invoq/server`) |
| Python | [github.com/invoqmoney/sdk-python](https://github.com/invoqmoney/sdk-python) |
| PHP | [github.com/invoqmoney/sdk-php](https://github.com/invoqmoney/sdk-php) |
| Go | **este repositório** |
| Rust | [github.com/invoqmoney/sdk-rust](https://github.com/invoqmoney/sdk-rust) |
| Ruby | [github.com/invoqmoney/sdk-ruby](https://github.com/invoqmoney/sdk-ruby) |

O lado do navegador é o mesmo para qualquer backend: **`@invoq/checkout`** (JavaScript, em [github.com/invoqmoney/sdk-js](https://github.com/invoqmoney/sdk-js)) abre a janela de checkout dentro da página para qualquer frontend.

## Instalação

```sh
go get github.com/invoqmoney/sdk-go
```

Requer Go 1.22 ou mais novo.

## Pegue suas chaves

1. Entre no painel da invoq e crie um projeto.
2. Na página **API keys**, crie uma chave secreta. Chaves de teste começam com `sk_test_`, chaves de produção com `sk_live_`. O modo da chave define se as faturas são de teste ou de produção.
3. Nas configurações de **webhooks** do projeto, salve a URL do seu webhook. O segredo do webhook (`whsec_...`) daquele modo aparece uma única vez, quando você ativa o webhook pela primeira vez — guarde na hora. A URL do webhook precisa ser HTTPS e pública.
4. Configure a sua **Receiving wallet** antes de ir para produção. Faturas de teste não precisam dela; uma fatura real sem destino de liquidação falha com `409 no_payment_options_available`.

Adicione os dois ao ambiente do seu servidor:

```sh
INVOQ_SECRET_KEY=sk_test_...
INVOQ_WEBHOOK_SECRET=whsec_...
```

Comece com as chaves de teste. Troque para a chave de produção e o segredo de webhook de produção quando for para produção.

## Crie um cliente

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

Padrões de produção:

- Origin da API: `https://api.invoq.money`
- Timeout da requisição: 10 segundos
- User-Agent: `invoq-go/<versão do módulo>`

Sobrescreva-os no desenvolvimento local ou em testes de pré-visualização:

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

`apiOrigin` precisa ser um origin `http` ou `https` absoluto, sem usuário, senha, caminho, query ou hash. O SDK anexa os caminhos de API `/v1/...`.

## Faturas

Crie uma fatura a partir do seu servidor:

```go
ctx := context.Background()

invoice, err := client.Invoices.Create(ctx, invoq.CreateInvoiceInput{
	Amount:      "129",
	Description: invoq.String("SaaS boilerplate"),
	ReferenceID: invoq.String("order_1234"),
	ReturnURL:   invoq.StringOrNull("https://merchant.example/thanks"),
})
if err != nil {
	// Trate o erro.
}

_ = invoice.ID
```

Notas:

- Defina o valor no servidor. Não confie em valores vindos do cliente.
- `amount` é uma string decimal em USD de `0.01` a `1000000.00`, com até 2 casas decimais, como `129` ou `129.99`. A moeda é sempre USD, e teste ou live vem da chave — nenhum dos dois é campo da requisição.
- Use o `reference_id` para ligar os webhooks `invoice.paid` ao seu pedido. Ele também deixa a criação segura para repetir: se você criar de novo com o mesmo `reference_id` e os mesmos termos, recebe a fatura existente em vez de uma duplicata; com termos diferentes, a chamada falha com o erro de API `409 reference_id_conflict`.
- Use `invoq.String(...)` para strings opcionais da requisição. Use `invoq.StringOrNull(...)` para definir `return_url`, `invoq.NullString()` para enviar `null` em JSON e deixe o campo sem definir para omiti-lo.

Busque uma fatura pública:

```go
invoice, err := client.Invoices.Get(ctx, "inv_123")
```

`Get` retorna `*invoq.PublicInvoice`. As leituras de fatura pública não incluem o seu `reference_id` privado; use webhooks ou o seu próprio armazenamento de pedidos para mapear o processamento.

## Pagamentos de teste

Faturas de teste não recebem dinheiro de verdade. Simule um pagamento a partir do seu servidor:

```go
paidInvoice, err := client.Invoices.CreateTestPayment(ctx, invoice.ID, invoq.CreateTestPaymentInput{
	Amount: invoice.Amount,
})
if err != nil {
	// Trate o erro.
}

_ = paidInvoice.Status // invoq.InvoiceStatusPaid quando totalmente pago
```

`CreateTestPayment` só funciona em faturas criadas com chave `sk_test_`. Quando os pagamentos atingem o valor da fatura, ela vira `paid` e a invoq envia um webhook `invoice.paid` assinado de verdade para a sua URL de webhook de teste. Valores parciais são permitidos e produzem `partially_paid`.

Para receber webhooks na sua máquina, exponha o servidor local com um túnel HTTPS como ngrok ou cloudflared e salve a URL do túnel como URL de webhook de teste no painel.

## Página de checkout hospedada

Toda fatura também tem uma página de checkout hospedada em:

```text
https://pay.invoq.money/<id da fatura>
```

Compartilhe o link ou redirecione para lá quando uma janela de checkout dentro da página não encaixar.

## Webhooks

Passe o corpo bruto da requisição para `VerifyWebhook`. Não interprete nem serialize novamente o JSON antes da verificação.

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

		// Processe o pedido de *referenceID de forma idempotente.
	}

	response.WriteHeader(http.StatusOK)
}
```

Use os webhooks `invoice.paid` para processar os pedidos no seu servidor. Quando `IsInvoicePaid(event)` for true, a fatura está pronta para processamento automático; o status dela é `paid`, `settling` ou `settled`. Uma fatura `review_required` não envia nenhum `invoice.paid` até a revisão ser aprovada.

A invoq também envia `invoice.payment_reversed` quando uma fatura já paga volta a ficar abaixo do valor dela — por exemplo, quando uma reorganização da chain derruba uma transferência confirmada. Capture com `invoq.IsInvoicePaymentReversed(event)`, decodifique com `invoq.AsInvoicePaymentReversedEvent(event)` e segure ou reverta o processamento conforme a sua própria política.

Entregas que falham são reenviadas (até 5 tentativas, com intervalos de 1 minuto, 5 minutos, 30 minutos e depois 2 horas), então processe de forma idempotente por `reference_id` ou pelo `id` da fatura e trate entregas repetidas como operações sem efeito. Elas também podem chegar fora de ordem: fique com o snapshot de maior `payment_revision`. Responda com 2xx rápido; qualquer outro status conta como entrega falhada e é reenviado, inclusive redirecionamentos e `4xx`.

`VerifyWebhook` aceita `http.Header`. Use `VerifyWebhookWithSignature` quando você já tiver o valor do cabeçalho `invoq-signature`.

Falhas de verificação de webhook retornam `*invoq.SignatureVerificationError`. O SDK permite uma tolerância de 5 minutos no timestamp. Entregas que falham são assinadas de novo a cada nova tentativa, então entregas reenviadas normais ainda passam na verificação dentro dessa janela de tempo. O cabeçalho de assinatura é `invoq-signature: t=<segundos unix>,v1=<HMAC-SHA256 em hex de "<t>.<corpo bruto>">`.

## Erros

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

Falhas de conexão, tempos de espera esgotados, erros de configuração e falhas ao interpretar a resposta retornam `*invoq.Error`. Respostas de API não 2xx retornam `*invoq.APIError`. Todos os erros do SDK implementam `invoq.SDKError`.

```go
var sdkErr invoq.SDKError
if errors.As(err, &sdkErr) {
	fmt.Println(sdkErr)
}
```

## Referência da API

```go
client, err := invoq.New(apiKey,
	invoq.WithAPIOrigin("https://api.invoq.money"), // opcional, sobrescreve o padrão
	invoq.WithTimeout(invoq.DefaultTimeout),        // opcional, timeout da requisição
)
```

- `client.Invoices.Create(ctx, input)` cria uma fatura. `input`: `Amount` (obrigatório), `Description`, `ReferenceID`, `ReturnURL`.
- `client.Invoices.Get(ctx, invoiceID)` busca uma fatura pública e retorna `*invoq.PublicInvoice`.
- `client.Invoices.CreateTestPayment(ctx, invoiceID, input)` simula um pagamento numa fatura de teste e retorna `*invoq.TestPaymentInvoice`.
- `invoq.VerifyWebhook(rawBody, headers, webhookSecret)` verifica um webhook e retorna `invoq.WebhookEvent`.
- `invoq.IsInvoicePaid(event)` e `invoq.AsInvoicePaidEvent(event)` identificam eventos `invoice.paid` tipados. `invoq.IsInvoicePaymentReversed(event)` e `invoq.AsInvoicePaymentReversedEvent(event)` fazem o mesmo para `invoice.payment_reversed`. Os dois rejeitam um evento malformado; um tipo de evento que esta versão do SDK ainda não modela continua sendo verificado e devolvido como veio.
- O SDK detecta a versão do seu módulo Go a partir das informações de build para o `User-Agent`. Tags lançadas como `v0.1.0` são enviadas sem o prefixo `v`; builds locais a partir do código-fonte, sem uma versão de módulo, usam `unknown`.

`Invoices.Get` retorna o formato de fatura pública usado pela página de checkout hospedada: o formato da resposta de criação mais `AmountPaid`, `Project` e `Transfers`, sem `ReferenceID`. Use a resposta de criação ou o webhook `invoice.paid` quando precisar do seu `reference_id` de comerciante.

Dois campos de status. `Status` é o contábil — `unpaid`, `partially_paid`, `paid`, `settling`, `settled`, `review_required` — e os três valores equivalentes a pago diferem apenas em quanto os fundos já andaram até a sua carteira. `CheckoutStatus` é o que o pagador vê — `open`, `confirming`, `expired`, `paid`, `unavailable` — e nunca autoriza processar o pedido. `PaymentRevision` sobe sempre que o conjunto de pagamentos confirmados muda, então você descarta um snapshot mais antigo do que o que já tem.

Os valores nas respostas são normalizados para 4 casas decimais: crie com `129` e a fatura devolve `Amount` `129.0000`. Compare valores numericamente, não como texto. `AmountDue` é derivado como `max(amount - amount_paid, 0)` e usa a mesma escala de 18 casas decimais de `AmountPaid`; `AmountOverpaid` é o espelho dele, `max(amount_paid - amount, 0)`, então você nunca precisa subtrair dinheiro por conta própria.

`PaymentOptions` guarda as instruções de pagamento, fixadas na criação e `[]` no modo de teste. As entradas são discriminadas por `Status` e depois por `CollectionMethod`: só `ready` é pagável, `evm_deposit` traz `DepositAddress` e `SuggestedAmount`, `direct_exact` traz `RecipientAddress` e um `ExactAmount` que o comprador precisa enviar até o último dígito. Esses campos de instrução são `nil` em qualquer outra entrada, e a identidade de uma opção é `(ChainNamespace, ChainReference, TokenAddress)`, nunca a posição dela no slice. `Transfers` é o registro confirmado de recebimentos — `TransactionID`, `EventIndex`, `Amount`, `ExplorerTransactionURL` — e fica `[]` até um pagamento confirmar. Referência completa: [documentação da API REST](https://github.com/invoqmoney/api).
