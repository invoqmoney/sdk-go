# SDK Go invoq

[English](../README.md) · [Bahasa Indonesia](./README.id.md) · [Español](./README.es-419.md) · **Français** · [Português](./README.pt-BR.md) · [Tiếng Việt](./README.vi.md) · [Türkçe](./README.tr.md) · [ไทย](./README.th.md) · [简体中文](./README.zh-Hans.md) · [繁體中文](./README.zh-Hant.md)

> Ce document est une traduction du README anglais ; en cas de divergence, la [version anglaise](../README.md) fait foi.

SDK Go pour les API serveur d’invoq et la vérification des webhooks. Créez des factures en stablecoins, simulez des paiements de test et traitez vos commandes à partir de webhooks signés.

Utilisez ce module uniquement sur votre serveur. Il accepte des clés secrètes et ne doit pas être compilé dans des applications côté client.

## SDK serveur

Créez des factures et vérifiez les webhooks depuis votre backend dans l’un de ces langages — même REST API, même signature de webhook. Ce dépôt est le SDK Go.

| Langage | Dépôt |
| --- | --- |
| Node.js | [github.com/invoqmoney/sdk-js](https://github.com/invoqmoney/sdk-js) (`@invoq/server`) |
| Python | [github.com/invoqmoney/sdk-python](https://github.com/invoqmoney/sdk-python) |
| PHP | [github.com/invoqmoney/sdk-php](https://github.com/invoqmoney/sdk-php) |
| Go | **ce dépôt** |
| Rust | [github.com/invoqmoney/sdk-rust](https://github.com/invoqmoney/sdk-rust) |
| Ruby | [github.com/invoqmoney/sdk-ruby](https://github.com/invoqmoney/sdk-ruby) |

Le côté navigateur est le même pour tous les backends : **`@invoq/checkout`** (JavaScript, dans [github.com/invoqmoney/sdk-js](https://github.com/invoqmoney/sdk-js)) ouvre la fenêtre de paiement intégrée à la page pour n’importe quel frontend.

## Installation

```sh
go get github.com/invoqmoney/sdk-go
```

Nécessite Go 1.22 ou plus récent.

## Récupérez vos clés

1. Connectez-vous au [tableau de bord invoq](https://app.invoq.money) et créez un projet.
2. Sur la page **API keys**, créez une clé secrète. Les clés de test commencent par `sk_test_`, les clés de production par `sk_live_`. Le mode de la clé détermine si les factures sont de test ou de production.
3. Dans les réglages **webhooks** de votre projet, enregistrez votre URL de webhook. Le secret du webhook (`whsec_...`) pour ce mode ne s’affiche qu’une seule fois, à la première activation du webhook — notez-le tout de suite. L’URL du webhook doit être une URL HTTPS publique.

Ajoutez les deux à l’environnement de votre serveur :

```sh
INVOQ_SECRET_KEY=sk_test_...
INVOQ_WEBHOOK_SECRET=whsec_...
```

Commencez avec les clés de test. Passez à la clé de production et au secret de webhook de production au moment de la mise en production.

## Créer un client

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

Valeurs par défaut en production :

- Origin de l’API : `https://api.invoq.money`
- Délai des requêtes : 10 secondes
- User-Agent : `invoq-go/<version du module>`

Surchargez-les en développement local ou pour tester une préversion :

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

`apiOrigin` doit être une origine `http` ou `https` absolue, sans nom d’utilisateur, mot de passe, chemin, requête ni fragment. Le SDK ajoute les chemins d’API `/v1/...`.

## Factures

Créez une facture depuis votre serveur :

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
	// Gérez l’erreur.
}

_ = invoice.ID
```

Notes :

- Définissez le montant côté serveur. Ne faites pas confiance aux montants envoyés par le client.
- `amount` est une chaîne décimale en USD de `0.01` à `999.99`, avec au plus 2 décimales, comme `129` ou `129.99`.
- Utilisez `reference_id` pour relier les webhooks `invoice.paid` à votre commande. Il permet aussi de relancer la création sans risque : si vous recréez avec le même `reference_id` et les mêmes conditions, vous récupérez la facture existante au lieu d’un doublon ; avec des conditions différentes, l’appel échoue avec une erreur d’API `409 reference_id_conflict`.
- Utilisez `invoq.String(...)` pour les chaînes optionnelles de la requête. Utilisez `invoq.StringOrNull(...)` pour définir `return_url`, `invoq.NullString()` pour envoyer un `null` JSON, et laissez le champ non défini pour l’omettre.

Récupérez une facture publique :

```go
invoice, err := client.Invoices.Get(ctx, "inv_123")
```

`Get` renvoie `*invoq.PublicInvoice`. La lecture d’une facture publique n’inclut pas votre `reference_id` privé ; utilisez les webhooks ou votre propre base de commandes pour établir la correspondance nécessaire au traitement.

## Paiements de test

Les factures de test ne peuvent pas recevoir de vrais fonds. Simulez un paiement depuis votre serveur :

```go
paidInvoice, err := client.Invoices.CreateTestPayment(ctx, invoice.ID, invoq.CreateTestPaymentInput{
	Amount: invoice.Amount,
})
if err != nil {
	// Gérez l’erreur.
}

_ = paidInvoice.Status // invoq.InvoiceStatusPaid une fois entièrement payée
```

`CreateTestPayment` ne fonctionne que sur les factures créées avec une clé `sk_test_`. Quand les paiements atteignent le montant de la facture, celle-ci passe à `paid` et invoq envoie un vrai webhook `invoice.paid` signé à votre URL de webhook de test. Les montants partiels sont autorisés et produisent `partially_paid`.

Pour recevoir des webhooks sur votre machine, exposez votre serveur local via un tunnel HTTPS comme ngrok ou cloudflared, et enregistrez l’URL du tunnel comme URL de webhook de test dans le tableau de bord. Le tableau de bord peut aussi envoyer un `webhook.ping` signé pour vérifier la connectivité.

## Webhooks

Passez le corps brut de la requête à `VerifyWebhook`. N’analysez pas le JSON pour le re-sérialiser avant la vérification.

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

		// Traitez la commande pour *referenceID de façon idempotente.
	}

	response.WriteHeader(http.StatusOK)
}
```

Traitez les commandes à partir des webhooks `invoice.paid` reçus côté serveur. Quand `IsInvoicePaid(event)` est vrai, la facture peut être traitée automatiquement ; son statut est `paid`, `settling` ou `settled`. Une facture `review_required` n’émet pas encore de webhook `invoice.paid`. Attendez un webhook `invoice.paid` ultérieur après validation.

Les livraisons échouées sont retentées ; traitez donc les commandes de façon idempotente par `reference_id` ou par `id` de facture, et ignorez les livraisons répétées. Répondez vite avec un 2xx ; tout autre statut compte comme une livraison échouée.

`VerifyWebhook` accepte un `http.Header`. Utilisez `VerifyWebhookWithSignature` lorsque vous disposez déjà de la valeur de l’en-tête `invoq-signature`.

Les échecs de vérification de webhook renvoient `*invoq.SignatureVerificationError`. Le SDK autorise une tolérance de 5 minutes sur l’horodatage. Les livraisons échouées sont signées à nouveau à chaque nouvelle tentative, si bien que les livraisons retentées normales restent vérifiables dans cette fenêtre. L’en-tête de signature est `invoq-signature: t=<secondes unix>,v1=<HMAC-SHA256 hexadécimal de "<t>.<corps brut>">`.

## Erreurs

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

Les échecs de connexion, les délais de requête dépassés, les erreurs de configuration et les échecs d’analyse des réponses renvoient `*invoq.Error`. Les réponses d’API non 2xx renvoient `*invoq.APIError`. Toutes les erreurs du SDK implémentent `invoq.SDKError`.

```go
var sdkErr invoq.SDKError
if errors.As(err, &sdkErr) {
	fmt.Println(sdkErr)
}
```

## Référence de l’API

```go
client, err := invoq.New(apiKey,
	invoq.WithAPIOrigin("https://api.invoq.money"), // optionnel, remplace le défaut
	invoq.WithTimeout(invoq.DefaultTimeout),        // optionnel, délai des requêtes
)
```

- `client.Invoices.Create(ctx, input)` crée une facture. `input` : `Amount` (requis), `Currency` (`InvoiceCurrencyUSD`, défaut), `Description`, `ReferenceID`, `ReturnURL`.
- `client.Invoices.Get(ctx, invoiceID)` récupère une facture publique et renvoie `*invoq.PublicInvoice`.
- `client.Invoices.CreateTestPayment(ctx, invoiceID, input)` simule un paiement sur une facture de test et renvoie `*invoq.TestPaymentInvoice`.
- `invoq.VerifyWebhook(rawBody, headers, webhookSecret)` vérifie un webhook et renvoie `invoq.WebhookEvent`.
- `invoq.IsInvoicePaid(event)` et `invoq.AsInvoicePaidEvent(event)` identifient les événements typés `invoice.paid`.
- Le SDK détecte la version de son module Go à partir des informations de build pour le `User-Agent`. Les tags publiés comme `v0.1.0` sont envoyés sans le préfixe `v` ; les builds locaux depuis les sources, sans version de module, utilisent `unknown`.
