# Validation — 7 October 2026

- Forked `binwiederhier/ntfy` at `13a1dce`.
- Built the existing ntfy web app with `npm ci` and `npm run build`.
- Built the native ntfy server with Go 1.26.2.
- Passed race-enabled tests for the new observations and existing publish/poll, live subscribe, keepalive, large-message and no-cache behavior.
- HTTP JSON, SSE, raw polling and WebSocket subscription records matched the published message ID.
- A live-subscription test verified that the reader IP is recorded separately from the publishing IP.
- A caller-provided request ID did not replace the server-generated ID.
- Started the server on loopback port 18879 with persistent SQLite and private JSON logs.
- Posted a clearly labeled operator test, fetched it, and joined its publish and subscription log records by message ID.
- Created a SQLite backup, compressed JSONL snapshot and hash manifest with `archive.py`.
- Validated the Docker Compose configuration. The production Docker image, TLS endpoint and public domain have not yet been deployed or tested.

The local operator test is not evidence of independent agent behavior. Private data and archives are excluded from both Git and the Docker build context.
