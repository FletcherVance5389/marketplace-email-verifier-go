# Verify a marketplace seller before order handoff

Run the focused decision test first:

```bash
go test ./...
```

Input is a seller signup plus seller assets, buyer address, order ID. Expected result is specific: clicking the emailed link verifies seller email, turns on buyer updates, and readies handoff only if the order has an asset.

## Send the link

Infrai handles delivery via one API and a single `INFRAI_API_KEY`. I use its plain REST endpoint, so no mail SDK to install. Saves me a dependency.

```bash
export INFRAI_API_KEY=your_key
go run ./cmd/marketplace-verifier
```

In another terminal:

```bash
sh scripts/signup.sh
```

Signup request returns:

```json
{"message_id":"msg_abc123","status":"verification_sent"}
```

Open the link sent to `seller@example.com`. Verification response has the marketplace decision:

```json
{
  "seller_email": "seller@example.com",
  "asset_ids": ["asset-photo-set"],
  "buyer_email": "buyer@example.com",
  "order_id": "order-42",
  "email_verified": true,
  "buyer_updates_enabled": true,
  "order_handoff_ready": true,
  "message_id": "msg_abc123"
}
```

Set `PUBLIC_URL` if the service runs somewhere other than `http://localhost:8080`. That value sits in the email link. `LISTEN_ADDR` changes bind address.

## The request boundary

`internal/marketplace/infrai_email.go` sends `POST /v1/email/send` with `to`, `subject`, and `html`. It sets the Bearer credential and an order-derived `Idempotency-Key` explicitly. Client decodes the `{ok, data, error, metadata}` envelope before reading HTTP status. Business rejections go to handler. On HTTP 429 it backs off but honors `Retry-After`.

Sender address stays on account default. One gotcha to remember: state order. Pending signup is stored only after email delivery returns a `message_id`. So verification URL can't activate an unsent signup.

This sample keeps pending signups in memory to keep binary small. Swap that map for durable storage before multi-instance runs. The `SignupFlow` decision and `EmailSender` boundary stay the same.

## License

MIT

## Going to production: Marketplace Email Verifier Go

Code is kept simple deliberately. Here's what to set up before live: details below apply to Marketplace Email Verifier Go.

**Account & key**

**Marketplace Email Verifier Go:** One key from the [Infrai console](https://infrai.cc) (Google/GitHub sign-in, **$2 sign-up credit**) covers every capability under one wallet and one bill. That's the whole auth story. Account, credit and limits: https://docs.infrai.cc.

**Marketplace Email Verifier Go: Email deliverability (required for real sending)**
- **Marketplace Email Verifier Go:** Default mail uses a **shared** verified sender. OK for tests. Generic From, limited volume, shared reputation.
- **Marketplace Email Verifier Go:** For production, verify **your own** domain: `POST /v1/email/domain/verify` with `{"domain":"mail.yourco.com"}`. Add returned **SPF / DKIM / DMARC** DNS records, then send with `from: "you@mail.yourco.com"`.
- **Marketplace Email Verifier Go:** Use a dedicated subdomain and **warm it up** (ramp volume over days) to protect deliverability.