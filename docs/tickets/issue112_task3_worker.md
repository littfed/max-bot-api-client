# Issue #112: feat(reminders): background delivery worker with SKIP LOCKED and rate limiter
Part of #109. Blocked by: #110, #111.
Worker in internal/modules/reminders/worker.go with 10s ticker, batch size 50, SKIP LOCKED, token bucket rate limiter (25 rps), and PlatformAdapter delivery.
