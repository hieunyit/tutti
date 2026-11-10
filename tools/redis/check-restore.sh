#!/usr/bin/env bash
# Verify an RDB by loading it in a disposable Redis container with no host network.
set -euo pipefail

snapshot=${1:-}
image=${2:-redis@sha256:1f88291b4c57bdf249f02eb085a8669889b2d2954e91d4de5118a5da6b78dd53}
if [[ ! -f "$snapshot" ]]; then
  echo "usage: $0 <snapshot.rdb> [redis-image]" >&2
  exit 2
fi
snapshot=$(realpath "$snapshot")
name="tutti-restore-check-$$"
cleanup() { docker rm -f "$name" >/dev/null 2>&1 || true; }
trap cleanup EXIT
# Read-only mount and disabled AOF/RDB writes prevent changes to the backup.
docker run -d --name "$name" --network none -v "$snapshot:/data/dump.rdb:ro" --entrypoint redis-server "$image" --appendonly no --save '' >/dev/null
for _ in {1..30}; do
  if docker exec "$name" redis-cli PING 2>/dev/null | tr -d '\r' | grep -qx PONG; then
    docker exec "$name" redis-cli INFO keyspace | tr -d '\r' | sed -n '/^# Keyspace/,$p'
    echo 'Disposable Redis loaded the snapshot successfully.'
    exit 0
  fi
  [[ $(docker inspect --format '{{.State.Running}}' "$name") == true ]] || break
  sleep 1
done
docker logs "$name" >&2
echo 'Disposable Redis could not load the snapshot' >&2
exit 1
