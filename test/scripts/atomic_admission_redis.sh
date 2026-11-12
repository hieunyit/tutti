#!/bin/bash
set -eu
cd "$(dirname "$0")/../.."
name="tutti-admission-test-$$"
trap 'docker rm -f "$name" >/dev/null 2>&1 || true' EXIT
container=$(docker run -d --rm --name "$name" --cpus=0.25 --memory=96m --memory-swap=96m -p 127.0.0.1::6379 redis:7-alpine redis-server --save '' --appendonly no)
addr=$(docker port "$container" 6379/tcp)
for i in $(seq 1 30); do
 if docker exec "$container" redis-cli ping >/dev/null 2>&1; then break; fi
 sleep 0.1
done
docker exec "$container" redis-server --version
TUTTI_TEST_REDIS_ADDR="$addr" EXT_REDIS=false GOMAXPROCS=2 GOPROXY=off go test -p 1 -race -count=1 -timeout=2m ./pkg/tasks -run '^TestAtomicAdmission_' -v
