# Go Security Audit Report (2026-11-25)

Scope: All non-test `.go` files, `.env`/Docker configs, and `go.mod` dependencies in `C:\mimocode\max-bot-api-client` (Go `1.26.5`; `go_vulncheck`: 0 third-party CVEs, 23 stdlib advisories).

## Tickets Created

Tracking issue: [#101](https://github.com/littfed/max-bot-api-client/issues/101) (`audit(security): Go security audit findings (2026-11-25)`)

1. **[#102](https://github.com/littfed/max-bot-api-client/issues/102) (Critical):** `security(api,auth): remove hardcoded JWT secret fallback, restrict CORS, and gate AllowDevHeaders`
2. **[#107](https://github.com/littfed/max-bot-api-client/issues/107) (Critical):** `security(payments,bot): fix T-Bank signature oracle, authenticate YooKassa/bot webhooks, and remove hardcoded creds`
3. **[#103](https://github.com/littfed/max-bot-api-client/issues/103) (Critical):** `security(webapp,shop): authenticate /api/webapp/* routes, remove lastActiveUserID/ownerUserID fallbacks, and fix shop simulate IDOR`
4. **[#104](https://github.com/littfed/max-bot-api-client/issues/104) (High):** `security(ssrf,dos): block SSRF in web summarizer/image fetcher and cap PDF/DOCX decompression`
5. **[#105](https://github.com/littfed/max-bot-api-client/issues/105) (High):** `security(fsm,channels): fix RESP parser panic/makeslice OOM and bound poll option index`
6. **[#106](https://github.com/littfed/max-bot-api-client/issues/106) (High):** `security(logging,errors): redact REDIS_URL/errors in logs and return generic HTTP errors`
7. **[#108](https://github.com/littfed/max-bot-api-client/issues/108) (Medium):** `security(http,deps): add MaxBytesReader, Server timeouts, security headers, rate limiting, and upgrade Go 1.26.x`

---

## Summary of Findings

- **Critical:** Hardcoded JWT fallback (`internal/api/router.go:80-113`), T-Bank signature oracle & hardcoded creds (`pkg/payments/gateway.go:190-192`, `cmd/showcase-bot/main.go:75`), unauthenticated YooKassa webhook (`pkg/payments/gateway.go:57-113`), unauthenticated `/api/webapp/*` & `SetAllowUnauthenticated(true)` (`cmd/showcase-bot/scen_webapp.go`, `pkg/webapp/bridge.go`).
- **High:** SSRF in web summarizer & AI image fetcher (`cmd/showcase-bot/scen_ai.go:2428-2445`, `scen_webapp.go:515-516`), PDF/DOCX decompression bombs (`scen_ai.go:2586, 2679`), shop simulate IDOR (`internal/api/handlers_modules.go:73-101`), empty `MAX_WEBHOOK_SECRET` bypass (`pkg/bot/bot.go:498-506`), permissive CORS with credentials (`internal/api/router.go:226-256`), `readRESP` panic/OOM (`pkg/fsm/redis.go:531-565`) & unbounded `optIdx` (`scen_channels.go:369-405`), plaintext `REDIS_URL` & raw HTTP errors, 23 Go `1.26.5` stdlib advisories.
- **Medium & Low:** Unbounded body/download reads, missing `http.Server` timeouts, missing security headers, unauthenticated `/metrics`, missing HTTP rate limiting, unconditional `X-Forwarded-For` trust, `WithInsecureTLS` / debug PII logging, `ffmpeg`/`pdftotext` missing timeout, `math/rand` giveaways, `0755`/`0644` store file perms, hardcoded admin ID `"396393759"`.
