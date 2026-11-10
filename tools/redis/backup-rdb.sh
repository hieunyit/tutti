#!/usr/bin/env bash
# Back up a running Docker Redis instance without stopping or flushing it.
set -euo pipefail

container=${1:-tutti-redis-1}
output=${2:-}
if [[ -z "$output" ]]; then
  echo "usage: $0 <redis-container> <output.rdb>" >&2
  exit 2
fi
if [[ -e "$output" ]]; then
  echo "output already exists: $output" >&2
  exit 2
fi
mkdir -p "$(dirname "$output")"
output=$(realpath -m "$output")

redis() { docker exec "$container" redis-cli --raw "$@" | tr -d '\r'; }
if [[ $(redis PING) != PONG ]]; then
  echo "Redis is unavailable in $container" >&2
  exit 1
fi
# A prior save must finish before requesting our own snapshot.
for _ in {1..120}; do
  [[ $(redis INFO persistence | sed -n 's/^rdb_bgsave_in_progress://p') == 0 ]] && break
  sleep 1
done
if [[ $(redis INFO persistence | sed -n 's/^rdb_bgsave_in_progress://p') != 0 ]]; then
  echo "timed out waiting for an existing Redis save" >&2
  exit 1
fi
before=$(redis LASTSAVE)
reply=$(redis BGSAVE)
if [[ $reply != 'Background saving started' ]]; then
  echo "BGSAVE failed: $reply" >&2
  exit 1
fi
running=1
status=unknown
for _ in {1..120}; do
  info=$(redis INFO persistence)
  running=$(sed -n 's/^rdb_bgsave_in_progress://p' <<< "$info")
  saved=$(redis LASTSAVE)
  if [[ $running == 0 && $saved -ge $before ]]; then
    status=$(sed -n 's/^rdb_last_bgsave_status://p' <<< "$info")
    [[ $status == ok ]] || { echo "Redis reported failed BGSAVE" >&2; exit 1; }
    break
  fi
  sleep 1
done
if [[ $running != 0 || $status != ok ]]; then
  echo "timed out waiting for Redis BGSAVE" >&2
  exit 1
fi
redis_dir=$(redis CONFIG GET dir | tail -n 1)
redis_file=$(redis CONFIG GET dbfilename | tail -n 1)
umask 077
tmp=$(mktemp "${output}.tmp.XXXXXXXX")
trap 'rm -f "$tmp"' EXIT
docker cp "${container}:${redis_dir%/}/${redis_file}" "$tmp"
chmod 600 "$tmp"
docker run --rm --network none -v "$tmp:/snapshot.rdb:ro" --entrypoint redis-check-rdb "$(docker inspect --format '{{.Image}}' "$container")" /snapshot.rdb >/dev/null
mv "$tmp" "$output"
trap - EXIT
printf 'Saved %s (%s bytes). Keep this snapshot and the encryption master key in separate protected backups.\n' "$output" "$(stat -c %s "$output")"
