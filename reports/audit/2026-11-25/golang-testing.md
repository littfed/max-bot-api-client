# Go Test Suite Audit Report (2026-11-25)

Scope: All `*_test.go` files in `C:\mimocode\max-bot-api-client` (Go `1.26.5`).

## Tickets Created

Tracking issue: [#83](https://github.com/littfed/max-bot-api-client/issues/83) (`audit(testing): Go test suite audit findings (2026-11-25)`)

1. **[#84](https://github.com/littfed/max-bot-api-client/issues/84) (High):** `test(showcase-bot,certs): mock Pollinations/TTS calls and gate live TLS handshake behind integration tag`
2. **[#85](https://github.com/littfed/max-bot-api-client/issues/85) (High):** `test(modules,platform): add unit tests for reception, shop, support, telegram adapter, and bot-tester`
3. **[#86](https://github.com/littfed/max-bot-api-client/issues/86) (High):** `test(bot,maxbot): add router_test.go, WebhookHandler/On* matcher tests, and errors_test.go`
4. **[#87](https://github.com/littfed/max-bot-api-client/issues/87) (High):** `test(api): fix resp.Body leaks/nil-deref, remove shared validMAXInitData globals, and add goleak`
5. **[#88](https://github.com/littfed/max-bot-api-client/issues/88) (High):** `test(goleak,store): add goleak to showcase-bot/livetest and speed up TestStoreConcurrency`
6. **[#89](https://github.com/littfed/max-bot-api-client/issues/89) (Medium):** `test(quality): convert table tests to t.Run, replace time.Sleep with synctest/fake clock, and enable t.Parallel()`
7. **[#90](https://github.com/littfed/max-bot-api-client/issues/90) (Medium):** `test(structure): align test filenames with source files and gate integration suites`

---

## High Severity Findings

1. **Unguarded live HTTPS calls in unit tests (`cmd/showcase-bot`, `certs`)**
   - `cmd/showcase-bot/scen_ai_test.go:817-890, 1025-1075`: `mockAIServer` returns 404 for `/images/generations`, triggering `deliverImageGen` -> `fetchFluxImage` (`scen_ai.go:2051-2093`), which makes live HTTPS requests to `pollinations.ai` and sleeps 8–15s on 402/429 rate limits (11s–30s+ timeout).
   - `certs/certs_test.go:33-57` (`TestLiveTLSHandshake`): Makes a live HTTPS `GET https://platform-api2.max.ru/me` without `//go:build integration` or `testing.Short()`.
2. **Zero unit tests in core business modules & Telegram adapter**
   - `internal/modules/reception/reception.go` (0.0%), `internal/modules/shop/shop.go` (0.0%), `internal/modules/support/support.go` (0.0%), `internal/platform/telegram/adapter.go` (0.0%), `cmd/bot-tester/main.go` (0.0%).
   - Low coverage (< 50%): `internal/livetest` (19.4%), `pkg/bot` (38.1% — no `router_test.go`, `WebhookHandler`/`On*` untested), `cmd/ai-bot` (42.7%), `pkg/maxbot` (46.0% — `errors.go` untested).
3. **Unclosed `resp.Body` & nil-deref risk in `internal/api` tests**
   - `internal/api/chat_sync_test.go:141-326`, `internal/api/api_test.go:155-190`: `resp.Body.Close()` is called inline after `if err != nil || resp.StatusCode != http.StatusOK`.
4. **Shared mutable package-level globals & missing `t.Parallel()`**
   - `internal/api/api_test.go:33-138`, `internal/api/chat_sync_test.go:121`: `validMAXInitData` and `validAdminInitData` are package-level globals mutated in setup helpers.
   - `t.Parallel()` is used in only 2 tests across the entire repo.
5. **Missing `goleak` coverage in goroutine-spawning packages**
   - `cmd/showcase-bot`, `internal/api`, and `internal/livetest` lack `goleak.VerifyTestMain(m)`. Typing-indicator goroutines in `bot_wire.go` and `handlers_modules.go` do not wait via `WaitGroup`.
6. **Heavy synchronous disk I/O storm in `TestStoreConcurrency`**
   - `pkg/store/store_test.go:139-174`: 20 goroutines × 50 iterations trigger 2,000 synchronous JSON marshals + disk writes (~7.2s standalone, ~24s under parallel load).

---

## Medium & Low Severity Findings

- Table-driven tests missing `name` + `t.Run`: `pkg/bot/bot_test.go:15-43,564`, `pkg/logger/logger_test.go:12-54`, `pkg/fsm/redis_test.go:214`, `internal/platform/adapter_test.go:135-165`, `cmd/showcase-bot/scen_ai_test.go:749-771`.
- Flaky wall-clock `time.Sleep` assertions (5–100 ms): `internal/core/core_test.go:411`, `pkg/fsm/fsm_test.go:80`, `pkg/fsm/wizard_test.go:178`, `pkg/gma/session_test.go:73`, `pkg/telemetry/middleware_test.go:36-57`.
- Test file naming & organization mismatches: `cmd/showcase-bot/poll_bug_test.go`, `internal/api/diagnosing_bugs_test.go`, misplaced `Context`/`Middleware` tests in `pkg/bot/bot_test.go`, monolithic package-named test files.
- Live gateway / subprocess tests without build tags: `internal/livetest/ai_test.go:276-292`, `cmd/showcase-bot/scen_ai_test.go` (`ffmpeg` / TTS), `cmd/showcase-bot/integration_test.go`, `pkg/store/sql_store_test.go`.
- Global mock SQL driver state: `pkg/fsm/sql_test.go:27-33`.
- Process-wide `os.Setenv` instead of `t.Setenv`: `pkg/logger/logger_test.go:134-138`.
- `t.Cleanup` / `defer db.Close()` registered after `NewSQLStore`/`NewSQLStorage`: `pkg/store/sql_store_test.go:221-231`, `pkg/fsm/sql_test.go:165-174`.
- Numbered subtests in `cmd/ai-bot/selftest_test.go:40-123`.
