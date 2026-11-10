// Copyright 2025 Clyso GmbH
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package policy

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	// SnapshotVersionKey is a counter in the config Redis. Every change of data that the proxy snapshot caches
	// increments it, in the same MULTI/EXEC as the change whenever the change is a transaction.
	SnapshotVersionKey = "tutti:cfg:version"
	// SnapshotChannel is the pub/sub channel on which the new version is published after each bump.
	SnapshotChannel = "tutti:cfg:changed"
)

// snapshotKeys are the Redis keys (or key families "<name>:<id>") the proxy snapshot is built from.
// Writes to any other key (replication status, version vectors, history, ...) do not invalidate it.
var snapshotKeys = []string{
	"p:route_user", "p:route_block_user", "p:route_bucket", "p:route_block_bucket",
	"p:repl_user_from", "p:repl_user_to", "p:repl_bucket_from", "p:repl_bucket_to",
	"p:repl_user_gen", "p:repl_bucket_gen",
	"p:repl_user_switch_zd_active", "p:repl_bucket_switch_zd_active",
	"p:repl_switch_user", "p:repl_switch_bucket",
}

// writeCommands are the Redis commands that modify keys (scripts are handled separately).
var writeCommands = map[string]bool{
	"set": true, "setnx": true, "getdel": true, "del": true, "unlink": true, "rename": true, "renamenx": true,
	"hset": true, "hsetnx": true, "hmset": true, "hdel": true, "hincrby": true, "hincrbyfloat": true,
	"sadd": true, "srem": true, "spop": true, "smove": true,
	"zadd": true, "zrem": true, "zincrby": true, "incr": true, "incrby": true, "decr": true, "decrby": true,
	"lpush": true, "rpush": true, "lrem": true, "ltrim": true, "expire": true, "pexpire": true, "persist": true,
}

const bumpScript = `local v = redis.call('incr', KEYS[1]); redis.call('publish', ARGV[1], tostring(v)); return v`

var changeNotifiers sync.Map // redis.UniversalClient -> struct{}

// installChangeNotifier registers a hook on the client (once) that bumps SnapshotVersionKey and publishes on
// SnapshotChannel after every write to a key the snapshot depends on. All mutation paths (policy service,
// failover controller, worker API, switch handlers) go through stores built on this client, so this is the one
// place that keeps writers and proxy snapshots in sync.
func installChangeNotifier(client redis.UniversalClient) {
	if client == nil {
		return
	}
	if _, loaded := changeNotifiers.LoadOrStore(client, struct{}{}); loaded {
		return
	}
	client.AddHook(changeHook{})
}

type changeHook struct{}

func (changeHook) DialHook(next redis.DialHook) redis.DialHook { return next }

func (changeHook) ProcessHook(next redis.ProcessHook) redis.ProcessHook {
	return func(ctx context.Context, cmd redis.Cmder) error {
		err := next(ctx, cmd)
		if touchesSnapshotKeys(cmd) {
			// bump even if the write returned an error: a timed out write may still have been applied
			bumpCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Second)
			defer cancel()
			_ = next(bumpCtx, newBumpCmd(bumpCtx))
		}
		return err
	}
}

func (changeHook) ProcessPipelineHook(next redis.ProcessPipelineHook) redis.ProcessPipelineHook {
	return func(ctx context.Context, cmds []redis.Cmder) error {
		touched := false
		for _, c := range cmds {
			if touchesSnapshotKeys(c) {
				touched = true
				break
			}
		}
		if !touched {
			return next(ctx, cmds)
		}
		if n := len(cmds); n >= 2 && cmds[0].Name() == "multi" && cmds[n-1].Name() == "exec" {
			// transaction: the bump becomes part of the same MULTI/EXEC, so readers never see the change
			// without a new version
			withBump := make([]redis.Cmder, 0, n+1)
			withBump = append(withBump, cmds[:n-1]...)
			withBump = append(withBump, newBumpCmd(ctx), cmds[n-1])
			return next(ctx, withBump)
		}
		err := next(ctx, cmds)
		bumpCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Second)
		defer cancel()
		_ = next(bumpCtx, []redis.Cmder{newBumpCmd(bumpCtx)})
		return err
	}
}

func newBumpCmd(ctx context.Context) redis.Cmder {
	return redis.NewCmd(ctx, "eval", bumpScript, 1, SnapshotVersionKey, SnapshotChannel)
}

func touchesSnapshotKeys(cmd redis.Cmder) bool {
	args := cmd.Args()
	if len(args) < 2 {
		return false
	}
	name, _ := args[0].(string)
	name = strings.ToLower(name)
	switch name {
	case "eval", "evalsha", "fcall":
		// EVAL script numkeys key... arg...
		if len(args) < 3 {
			return false
		}
		n, ok := toInt(args[2])
		if !ok {
			return false
		}
		for i := 3; i < 3+n && i < len(args); i++ {
			if s, ok := args[i].(string); ok && snapshotKey(s) {
				return true
			}
		}
		return false
	case "del", "unlink":
		for _, a := range args[1:] {
			if s, ok := a.(string); ok && snapshotKey(s) {
				return true
			}
		}
		return false
	}
	if !writeCommands[name] {
		return false
	}
	k, _ := args[1].(string)
	return snapshotKey(k)
}

func toInt(a any) (int, bool) {
	switch v := a.(type) {
	case int:
		return v, true
	case int64:
		return int(v), true
	}
	return 0, false
}

// snapshotKey reports whether the key (or a SCAN-style key pattern) belongs to the snapshot data.
func snapshotKey(k string) bool {
	for _, w := range snapshotKeys {
		if k == w || strings.HasPrefix(k, w+":") {
			return true
		}
		if strings.HasPrefix(k, w) && strings.ContainsAny(k, "*?[") {
			return true
		}
	}
	return false
}
