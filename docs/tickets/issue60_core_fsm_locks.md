# Issue #60: feat(core): composite fsm with redis distributed locks and maintenance guard
Part of #58. Blocked by: #59.
internal/core/: FSM (user_id, mode_id), Redis SET NX EX 5 lock, maintenance guard.
