Local source snapshot of github.com/clyso/asynq v0.0.0-20251202163730-20cb0d89aa76 (upstream module github.com/hibiken/asynq). MIT license retained.

Tutti patch: enforce asynq:{queue}:high_watermark in the same Lua transaction as task creation for pending, scheduled and grouped tasks (including Unique variants). Duplicate TaskID/Unique remains a no-op before admission; quota rejection does not create task/unique state. ErrQueueFull is exposed for retryable backpressure. Queues without a watermark retain upstream behavior.

Only production Go sources and module/license files are included; original upstream tests/docs are not copied. Root pkg/tasks tests exercise the patched broker through real Asynq clients.

Archived/completed redrive through Inspector.RunTask is quota-checked atomically. RunAllArchivedTasks moves at most the remaining capacity and returns the moved count; excess archived tasks remain for a later retry. Scheduled/retry/aggregating-to-pending transfers keep existing outstanding work and do not require a new slot.

Inspector.RedisClient exposes the existing queue connection so Tutti admission/readiness code does not use reflection or unsafe to read private fields. The accessor does not transfer ownership.
