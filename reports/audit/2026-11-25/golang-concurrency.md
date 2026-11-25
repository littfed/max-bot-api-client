# Go Concurrency Audit Report (2026-11-25)

Scope: All `.go` files in `C:\mimocode\max-bot-api-client` (Go `1.26.5`; `go vet ./...` passed with 0 warnings).

## Tickets Created

Tracking issue: [#75](https://github.com/littfed/max-bot-api-client/issues/75) (`audit(concurrency): Go concurrency audit findings (2026-11-25)`)

1. **[#78](https://github.com/littfed/max-bot-api-client/issues/78) (High):** `fix(bot): bound StartLongPolling goroutines, add graceful drain, and reuse timers`
2. **[#76](https://github.com/littfed/max-bot-api-client/issues/76) (High):** `fix(fsm): eliminate recursive RLock deadlock in Wizard and fix MemoryStorage Close`
3. **[#82](https://github.com/littfed/max-bot-api-client/issues/82) (High):** `fix(store,modules): fix data races and shallow-copy map/slice leaks in shop, chat, reception, and store`
4. **[#79](https://github.com/littfed/max-bot-api-client/issues/79) (High):** `fix(guard,fsm,store): avoid holding mutexes across network/DB I/O and remove dead stopCh`
5. **[#80](https://github.com/littfed/max-bot-api-client/issues/80) (High):** `fix(telemetry): synchronize PrometheusExporter.server and clone Collector.sinks`
6. **[#81](https://github.com/littfed/max-bot-api-client/issues/81) (High):** `fix(showcase-bot,livetest): synchronize handler maps and defer typing-indicator channel cleanup`
7. **[#77](https://github.com/littfed/max-bot-api-client/issues/77) (Medium):** `chore(test,docs): add -race / goleak checks and thread-safety GoDoc on concurrent types`

---

## High Severity Findings

1. **Unbounded goroutine spawning & no graceful drain in `StartLongPolling`**
   - `pkg/bot/bot.go:404-414`: Spawns an untracked `go func(item *maxbot.Update)` for every update with no concurrency limit (`errgroup.SetLimit` or worker pool) and no `sync.WaitGroup`. Returns immediately on `ctx.Done()` without waiting for in-flight handlers, and panics before `router.Handle` crash the process.
2. **Recursive `RLock` acquisition + callback under `RLock` (deadlock risk)**
   - `pkg/fsm/wizard.go:159-220`: `(*Wizard).Next` and `(*Wizard).Back` acquire `w.mu.RLock()` and call `w.CurrentStep(c)`, which acquires `w.mu.RLock()` again. `Next` also invokes `w.onComplete(c)` while holding `RLock`.
3. **Data race: mutating `*Order` under `RLock` and returning live pointers**
   - `internal/modules/shop/shop.go:142-259`, `internal/api/handlers_modules.go:62-125`: `GetUserOrders` mutates `*Order` fields under `m.mu.RLock()` and returns live `*Order` pointers read by HTTP handlers outside `m.mu`.
4. **Shallow-copied maps/slices returned by `Store.GetPoll` & `Store.GetGiveaway`**
   - `pkg/store/store.go:486-571`, `cmd/showcase-bot/scen_channels.go:412-418`: Shallow-copied `Poll.Votes` / `Giveaway.Participants` maps are read outside `s.mu` while `VotePoll` / `AddGiveawayParticipant` mutate them under `s.mu.Lock()`.
5. **Live `*Session` pointer returned without locking**
   - `internal/modules/chat/chat.go:170-212,454-468,528-536`, `internal/api/handlers_modules.go:540-550`: `GetSession` returns live `*Session` read without `m.mu`.
6. **Mutex held across network I/O**
   - `pkg/guard/captcha.go:76-120`: `HandleNewMember` holds `g.mu.Lock()` across `SendKeyboardMessage`.
   - `pkg/fsm/redis.go:296-548`: `respClient` holds `c.mu.Lock()` across `net.DialTimeout`, `Write`, and `readRESP`, and ignores `ctx` deadlines.
7. **Unsynchronized `pe.server` access & `Shutdown` under lock**
   - `pkg/telemetry/prometheus.go:158-186`: `StartServer` writes `pe.server` without `pe.mu`; `Close()` holds `pe.mu.Lock()` across `Shutdown`.
8. **Unprotected concurrent map/slice access in `showcase-bot` handlers & `Router`**
   - `cmd/showcase-bot/scen_ai.go:40-44` (`userMemories`, `userModels`), `cmd/showcase-bot/menu.go:29-43` (`chatLastBotMsg`), `pkg/bot/router.go:46-83` (`Router.routes`, `Router.middlewares`).

---

## Medium Severity Findings

- **Non-deferred `close(stopTyping)` & missing `<-ctx.Done()` in typing-indicator goroutines:** `cmd/showcase-bot/bot_wire.go:144-159,201-216`, `cmd/showcase-bot/menu.go:444-484`, `cmd/showcase-bot/scen_ai.go:326-408,508-557,1084-1160,1270-1347`.
- **Non-idempotent `close(ms.stopGC)` & unused `RWMutex` read lock:** `pkg/fsm/memory.go:17,102,126,160-167`.
- **`time.After` inside `for` loops:** `pkg/bot/bot.go:384,394,421`, `cmd/showcase-bot/scen_ai.go:2053`, `internal/livetest/matcher.go:79`.
- **Mutex held across DB/store I/O:** `pkg/fsm/sql.go:59-196`, `pkg/store/sql_store.go:60-195`, `internal/modules/chat/chat.go:171-283`, `pkg/store/store.go:255`.
- **Returning pointer into slice backing array after unlock:** `internal/modules/reception/reception.go:227-237`.
- **Legacy `sync/atomic` instead of typed atomics:** `cmd/showcase-bot/scen_webapp.go:39-45`, `pkg/telemetry/events.go:10-40`, `pkg/bot/bot_test.go:142-154`.
- **No `-race` or `goleak` in CI/tests.**

---

## Low Severity Findings

- Dead `stopCh` field on `PostScheduler` (`pkg/guard/scheduler.go:31,57`).
- `sync.Map` with unbounded growth and network I/O inside `Range` (`pkg/bot/bot.go:26,271-287`).
- Fire-and-forget startup/shutdown goroutines (`cmd/showcase-bot/main.go:156-168,344-349`, `pkg/fsm/memory.go:33`, examples/tests).
- One-shot `time.After` in hot paths/tests (`pkg/bot/bot.go:265`, `internal/livetest/cases_lifecycle.go:92,127`).
- Mutable package-level `globalAIDispatcher` (`cmd/showcase-bot/scen_ai.go:40,1495`, `cmd/showcase-bot/menu.go:413-415`).
- `Collector.Record` shares `c.sinks` backing array outside `RLock` (`pkg/telemetry/collector.go:56-62`).
- Missing thread-safety GoDoc comments on exported concurrent types.
