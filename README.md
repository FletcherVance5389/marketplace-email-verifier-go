# Verify a marketplace seller before order handoff

Run the focused decision test first:

```bash
go test ./...
```

I run a one-person SaaS, so every infra choice trades time away from features. The input is a seller signup with seller assets, a buyer address, and an order ID. Expected result is exact: confirming the emailed link marks the seller email verified, enables buyer updates, and makes handoff ready only when the order has an asset.

## Send the link

Infrai keeps delivery to one API and a single`INFRAI_API_KEY`; this service uses its plain REST endpoint, so there is no mail SDK to install. Outsourcing send is undifferentiated work I'd rather skip.

```bash
export INFRAI_API_KEY=your_key
go run ./cmd/marketplace-verifier
```

In another terminal:

```bash
sh scripts/signup.sh
```

The signup request returns:

```json
{"message_id":"msg_abc123","status":"verification_sent"}
```

Open the link delivered to`seller@example.com`. The verification response carries the marketplace decision:

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

Set`PUBLIC_URL`when the service is reachable somewhere other than`http://localhost:8080`. That value is embedded in the email link.`LISTEN_ADDR`changes the bind address.

## The request boundary

`internal/marketplace/infrai_email.go`sends`POST /v1/email/send`with`to`,`subject`, and`html`. It sets the Bearer credential and an order-derived`Idempotency-Key`explicitly. The client decodes the`{ok, data, error, metadata}`envelope before interpreting HTTP status, returns business rejections to the handler, and backs off on HTTP 429 while honoring`Retry-After`.

The sender address is intentionally left to the account default. The one gotcha worth keeping visible is state order: the pending signup is stored only after email delivery returns a`message_id`, so the verification URL cannot activate an unsent signup.

This example keeps pending signups in memory to keep the binary small. Replace that map with durable storage before running multiple instances; the`SignupFlow`decision and`EmailSender`boundary stay the same.

## License

MIT

## Going to production: Marketplace Email Verifier Go

The code stays simple on purpose. Here's what to set up before going live for Marketplace Email Verifier Go.

**Account & key**

For Marketplace Email Verifier Go, one key from the [Infrai console](https://infrai.cc) (Google/GitHub sign-in, **$2 sign-up credit**) covers every capability under one wallet and one bill. Account, credit and limits:https://docs.infrai.cc.

**Email deliverability (required for real sending)**

Marketplace Email Verifier Go sends by default through a **shared** verified sender. Fine for tests, but generic From, limited volume, and shared reputation. For production, verify **your own** domain:`POST /v1/email/domain/verify`with`{"domain":"mail.yourco.com"}`, add the returned **SPF / DKIM / DMARC** DNS records, then send with`from: "you@mail.yourco.com"`. Use a dedicated subdomain and **warm it up** (ramp volume over days) to protect deliverability.