// Copyright 2026 Tutti contributors. SPDX-License-Identifier: MIT

package rdb

import "fmt"

// A false capacity means no watermark is configured. Used only before mutations
// inside the same Lua script, including administrative redrive of archived work.
const queueCapacityLua = `
local function queueCapacity(prefix)
    local high = tonumber(redis.call("GET", prefix .. "high_watermark") or "0")
    if not high or high < 0 then error("invalid queue high watermark") end
    if high == 0 then return false end
    local count = redis.call("LLEN", prefix .. "pending")
        + redis.call("LLEN", prefix .. "active")
        + redis.call("ZCARD", prefix .. "scheduled")
        + redis.call("ZCARD", prefix .. "retry")
    for _, group in ipairs(redis.call("SMEMBERS", prefix .. "groups")) do
        count = count + redis.call("ZCARD", prefix .. "g:" .. group)
    end
    return math.max(0, high - count)
end
`

// queueAdmissionLua runs in the SAME script as task creation. Existing IDs
// remain idempotent even when the queue has reached capacity.
func queueAdmissionLua(queueArg, taskKey, uniqueKey int) string {
	duplicate := ""
	if uniqueKey > 0 {
		duplicate = fmt.Sprintf(`if redis.call("EXISTS", KEYS[%d]) == 1 then return -1 end
`, uniqueKey)
	}
	return duplicate + queueCapacityLua + fmt.Sprintf(`
if redis.call("EXISTS", KEYS[%d]) == 1 then return 0 end
local capacity = queueCapacity("asynq:{" .. ARGV[%d] .. "}:")
if capacity and capacity < 1 then return -2 end
`, taskKey, queueArg)
}
