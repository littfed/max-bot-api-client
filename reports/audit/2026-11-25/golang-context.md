# Go `context.Context` Audit Report (2026-11-25)

Scope: All `.go` files in `C:\mimocode\max-bot-api-client` (Go `1.26.5`).

## Tickets Created

Tracking issue: [#91](https://github.com/littfed/max-bot-api-client/issues/91) (`audit(context): Go context.Context audit findings (2026-11-25)`)

1. **[#92](https://github.com/littfed/max-bot-api-client/issues/92) (High):** `fix(showcase-bot,api): propagate request/bot ctx instead of context.Background() in handlers`
2. **[#93](https://github.com/littfed/max-bot-api-client/issues/93) (High):** `fix(fsm): abort blocking Redis net.Conn I/O on ctx cancellation via context.AfterFunc`
3. **[#94](https://github.com/littfed/max-bot-api-client/issues/94) (Medium):** `refactor(store): add ctx context.Context and *Context DB calls to legacy SQLStore methods`
4. **[#95](https://github.com/littfed/max-bot-api-client/issues/95) (Medium):** `fix(showcase-bot,livetest,examples): respect ctx.Done() in retry sleeps and use WithoutCancel in defer cleanups`

---

## High Severity Findings

1. **`context.Background()` in HTTP handlers, WebBridge actions, and bot handlers**
   - `cmd/showcase-bot/scen_webapp.go:187,313,444,503,515,541,552,650,731,747,755`
   - `cmd/showcase-bot/scen_channels.go:151,200,215,221,285,330,429,482,552`
   - `cmd/showcase-bot/scen_guard.go:161`
   - `internal/api/handlers_modules.go:636`
2. **Blocking `net.Conn` I/O in `respClient` does not abort on `ctx` cancellation**
   - `pkg/fsm/redis.go:362-560` (`Get`, `Set`, `SetNX`, `Incr`, `Del`, `Ping`)

## Medium & Low Severity Findings

- `pkg/store/sql_store.go:118-610`: 25 DB calls across 19 legacy/showcase methods use `Exec`/`Query`/`QueryRow` without `ctx`.
- `cmd/showcase-bot/scen_ai.go:1610-1619, 2220-2227`: `time.Sleep` inside retry loops without checking `ctx.Done()`.
- `cmd/showcase-bot/main.go:153-182`: manual `pingCancel()` after `if/else` instead of helper with `defer pingCancel()`.
- `examples/02_feature_tour/main.go:196-413`: `context.Background()` in handlers/goroutines.
- `internal/livetest/cases_interactive.go:66`, `cases_lifecycle.go:43`: `defer` cleanup should use `context.WithoutCancel(ctx)`.
- `internal/livetest/matcher.go:62-75`: `pollCtx` inside loop.
- `pkg/fsm/sql.go:52`: `db.Exec` in `NewSQLStorage`.
- `pkg/fsm/redis.go:175`: inline `interface{ Ping(context.Context) error }` omits param name.
