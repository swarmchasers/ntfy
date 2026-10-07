# ntfy with request and subscription records

This fork keeps ntfy's topics, publishing, subscriptions, web app, rate limits and SQLite cache. A small patch adds a server-generated request ID and records which message IDs were written to each subscriber. It does not change the experiments in the ai-collusion repository.

Run locally, from the repository root:

```sh
make web-deps web-build cli-deps-static-sites
go build -o dist/ntfy-observer .
./observer/run-local.sh
curl --data-binary 'a test message' http://127.0.0.1:18879/demo
curl 'http://127.0.0.1:18879/demo/json?poll=1&since=all'
```

The SQLite database and JSON request log are in `observer/private-data/`, excluded from Git. The API retains messages for 30 days. Logs are append-only during normal operation and have no automatic expiry. They are ordinary files, not an independently witnessed record.

## What the records establish

- `http_request_id` is assigned by the server. Client-supplied request IDs are not used. `X-Request-ID` is returned where the transport permits.
- `http_peer_addr` is the socket peer. `visitor_ip` is ntfy's interpreted client IP. In the production configuration only Caddy can reach ntfy, and Caddy overwrites `X-Observer-Client-IP` from the connection address.
- At trace level, ntfy records method, target, headers and up to 128 KiB of the raw request body. Longer bodies are explicitly labeled truncated. Bodies, including GET publishing parameters, remain private. HTTP parsing normalizes header casing and does not preserve the original wire representation.
- Parsed message contents and message IDs appear in ntfy's publish logs. `observation_event=message_published` means the normal publish path completed and its response was written.
- `observation_event=subscription_message_written` links a message ID to a subscriber request, reader IP and transport. JSON, SSE, raw and WebSocket subscriptions are covered. HTTP output chunks also have a SHA-256 digest. It records the subscriber, not the publisher passed to live callbacks.
- A successful write does not prove client receipt, reading or reuse. Failed authentication may be logged without a body. Keepalive/open frames are omitted from the message-delivery observations. Logging is not transactional with message delivery: a crash can leave a gap, and a full filesystem can lose logs. Monitor the host and preserve gaps rather than claiming exhaustive capture.

A post, a later subscription response containing its ID, and a subsequent use of distinctive content would support information transfer. Separate IPs, user-agents or request IDs alone do not establish separate agents. Operator tests must use clearly named test topics and stay separate from unsolicited activity. Publishing this service is an intervention, not evidence about earlier activity on other servers.

## Archive

```sh
python3 observer/archive.py
```

This makes a consistent SQLite backup, a gzip copy of complete log records up to a fixed byte boundary, and a hash manifest. The two snapshots are taken at separate times. Keep archives off-host. Publicly timestamping manifests through an independent service would make later alterations detectable relative to that timestamp; a locally generated hash by itself does not establish authenticity. No off-host replication or timestamp publication is configured yet.

## Deploy on a host with a public address

Buy a domain, point its A record (and AAAA only if IPv6 works) at the host, and allow TCP 80/443. Do not put a JavaScript challenge in front of the API. The example assumes Caddy connects directly to clients; do not add another proxy without revisiting client-IP trust.

```sh
cd observer
printf 'PUBLIC_DOMAIN=YOUR-DOMAIN\n' > .env
mkdir -p private-data
chmod 700 private-data
docker compose --env-file .env -f compose.yml up -d --build
```

Caddy serves the plain HTML introduction and logging disclosure; ntfy serves the web app at `/app` and its normal topic APIs. The landing page documents 30-day API retention and research logging. Attachments and account signup are off. No experimental identity is claimed for visitors. This service must not be presented as an official ntfy instance.

The production Compose deployment has been prepared and its configuration validated; the image build and public TLS deployment have not been tested yet. It needs the chosen domain and host. `site-link.html` is a short link for the public research website; replace its placeholder after deployment. Do not publish the private data directory.

## Discovery

Link the useful service and its documentation from the public website and the fork README. Submit the landing page through Search Console once ownership is verified. The robots file permits the documentation and discourages topic crawling; it is not access control. Google does not guarantee indexing or ranking. There is no fabricated activity, claimed model-provider endorsement, paid promotion or automatic outreach.

Sources: [ntfy configuration](https://docs.ntfy.sh/config/), [publishing](https://docs.ntfy.sh/publish/), [subscription API](https://docs.ntfy.sh/subscribe/api/), [Google indexing guidance](https://developers.google.com/search/docs/crawling-indexing/ask-google-to-recrawl).
