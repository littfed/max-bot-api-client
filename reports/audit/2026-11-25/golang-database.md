# Go Database Audit Report (2026-11-25)

Scope: All non-test `.go` files and `.sql` migrations in `C:\mimocode\max-bot-api-client` (Go `1.26.5`).

## Tickets Created

Tracking issue: [#96](https://github.com/littfed/max-bot-api-client/issues/96) (`audit(database): Go database audit findings (2026-11-25)`)

1. **[#97](https://github.com/littfed/max-bot-api-client/issues/97) (High):** `fix(store): wrap read-modify-write ops in SQL transactions and handle Scan/rows.Err/RowsAffected errors`
2. **[#98](https://github.com/littfed/max-bot-api-client/issues/98) (High):** `fix(fsm): validate TableName, wrap SQLStorage writes in transactions, and handle sql.ErrNoRows with errors.Is`
3. **[#99](https://github.com/littfed/max-bot-api-client/issues/99) (High):** `fix(db,store): configure SQLite/Postgres connection pools properly in OpenDatabase, NewFromEnv, and showcase-bot`
4. **[#100](https://github.com/littfed/max-bot-api-client/issues/100) (Medium):** `fix(store,api,db): make support ticket+message creation atomic and add RunMigrationsContext`

---

## High Severity Findings

1. **Swallowed DB error before `ON CONFLICT DO UPDATE` (`pkg/store/sql_store.go:325-363`)**
   - `ToggleReactionContext` ignores non-`sql.ErrNoRows` errors in `row.Scan` and overwrites existing counts/votes.
2. **Multi-step read-modify-write operations without SQL transactions**
   - `pkg/fsm/sql.go:156-194` (`UpdateData`)
   - `pkg/store/sql_store.go:313-370` (`ToggleReactionContext`), `446-492` (`VotePollContext`), `550-574` (`AddGiveawayParticipantContext`), `580-620` (`DrawGiveawayWinnersContext`)
3. **In-memory SQLite `:memory:` fallback without pool limits (`cmd/showcase-bot/main.go:228`)**
   - Missing `SetMaxOpenConns(1)`, `SetMaxIdleConns(1)`, `SetConnMaxLifetime(0)`, `SetConnMaxIdleTime(0)`, and `err != nil` check.

## Medium & Low Severity Findings

- `pkg/fsm/sql.go:33-201`: unvalidated `TableName` interpolation, swallowed error in `UpdateData`, non-transactional `SetState`/`SetData`, `err == sql.ErrNoRows` instead of `errors.Is`.
- `pkg/store/sql_store.go:154-285, 668-704`: missing `rows.Err()` and swallowed `QueryContext`/`Scan` errors in `ListChannelsContext`, `ListOrdersByUserContext`, `ListAllOrdersContext`, `ListMediaArtifactsContext`.
- `pkg/store/sql_store.go:300-303, 784-787, 942-944`: swallowed `RowsAffected()`, `ExecContext` errors.
- `pkg/store/store.go:73-107`: unconfigured pool in `OpenDatabase` and missing `SetConnMaxIdleTime` in `NewFromEnv`.
- `internal/api/handlers_modules.go:436-447`: non-transactional support ticket + message creation.
- `pkg/db/migrations.go:23, 51`: `RunMigrations` uses `goose.Up` instead of `goose.UpContext`.
