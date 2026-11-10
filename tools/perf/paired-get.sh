#!/usr/bin/env bash
# Short, paired direct-vs-proxy GET benchmark. Seeds and removes its own bucket.
set -euo pipefail

: "${DIRECT_ENDPOINT:?set DIRECT_ENDPOINT, e.g. http://127.0.0.1:19001}"
: "${PROXY_ENDPOINT:?set PROXY_ENDPOINT, e.g. http://127.0.0.1:29879}"
: "${S3_ACCESS_KEY:?set S3_ACCESS_KEY}"
: "${S3_SECRET_KEY:?set S3_SECRET_KEY}"

warp_image=${WARP_IMAGE:-minio/warp@sha256:72ae1b02216b51bd102b73a6923b597c092a5acc7a655c936c2f31bee897a7c5}
results=${1:-"perf-get-$(date -u +%Y%m%dT%H%M%SZ)"}
if [[ -e "$results" ]]; then
  echo "results path already exists: $results" >&2
  exit 2
fi
mkdir -p "$results"
results=$(realpath "$results")
seed=$(mktemp -d)
export MC_CONFIG_DIR="$seed/mc-config"
bucket="tutti-perf-$(date -u +%Y%m%dt%H%M%Sz)-$$"
created=0
cleanup() {
  if (( created )); then
    mc rm --recursive --force "bench-direct/$bucket" >/dev/null 2>&1 || true
    mc rb "bench-direct/$bucket" >/dev/null 2>&1 || true
  fi
  rm -rf "$seed"
}
trap cleanup EXIT

mc alias set bench-direct "$DIRECT_ENDPOINT" "$S3_ACCESS_KEY" "$S3_SECRET_KEY" >/dev/null
mc alias set bench-proxy "$PROXY_ENDPOINT" "$S3_ACCESS_KEY" "$S3_SECRET_KEY" >/dev/null
mc mb "bench-direct/$bucket" >/dev/null
created=1
mkdir -p "$seed/objects"
# 256 distinct 4 KiB objects keep preparation short and the read workload repeatable.
python3 - "$seed/objects" <<'PY'
import os,sys
for i in range(256):
    with open(os.path.join(sys.argv[1], f'object-{i:04d}'), 'wb') as f:
        f.write((i.to_bytes(4,'big') * 1024))
PY
mc cp --recursive "$seed/objects/" "bench-direct/$bucket/" >/dev/null
mc stat "bench-proxy/$bucket/object-0000" >/dev/null
printf 'label\trepeat\tobjects_per_second\tmegabytes_per_second\terrors\n' > "$results/summary.tsv"
printf 'commit=%s\nbucket=%s\ndirect=%s\nproxy=%s\nimage=%s\nconcurrency=16\nduration=10s\nobjects=256\nobject_size=4KiB\n' \
  "$(git rev-parse HEAD 2>/dev/null || echo unknown)" "$bucket" "$DIRECT_ENDPOINT" "$PROXY_ENDPOINT" "$warp_image" > "$results/environment.txt"
git status --porcelain=v1 > "$results/git-status.txt" 2>/dev/null || true
printf "dirty_files=%s\n" "$(wc -l < "$results/git-status.txt")" >> "$results/environment.txt"

measure() {
  local label=$1 repeat=$2 endpoint=$3 outfile="$results/$1-$2.json"
  local -a args warp_args
  local host=${endpoint#http://}
  host=${host#https://}
  args=(--rm --network host -e WARP_ACCESS_KEY -e WARP_SECRET_KEY)
  warp_args=()
  if [[ $endpoint == https://* ]]; then warp_args+=(--tls); fi
  WARP_ACCESS_KEY=$S3_ACCESS_KEY WARP_SECRET_KEY=$S3_SECRET_KEY \
    docker run "${args[@]}" "$warp_image" get "${warp_args[@]}" \
      --host "$host" --bucket "$bucket" --list-existing --noclear \
      --obj.size 4KiB --concurrent 16 --duration 10s --json > "$outfile"
  jq -e '.by_op_type.GET.throughput.measure_duration_millis > 0' "$outfile" >/dev/null
  jq -r --arg label "$label" --arg repeat "$repeat" \
    '[$label, $repeat, ((.by_op_type.GET.throughput.objects * 1000 / .by_op_type.GET.throughput.measure_duration_millis) | tostring), ((.by_op_type.GET.throughput.bytes * 1000 / .by_op_type.GET.throughput.measure_duration_millis / 1048576) | tostring), ((.by_op_type.GET.total_errors // 0) | tostring)] | @tsv' \
    "$outfile" >> "$results/summary.tsv"
}
for repeat in 1 2 3; do
  if (( repeat % 2 )); then
    measure direct "$repeat" "$DIRECT_ENDPOINT"
    measure proxy "$repeat" "$PROXY_ENDPOINT"
  else
    measure proxy "$repeat" "$PROXY_ENDPOINT"
    measure direct "$repeat" "$DIRECT_ENDPOINT"
  fi
done
cat "$results/summary.tsv"
printf 'Raw Warp JSON and environment: %s\n' "$results"
