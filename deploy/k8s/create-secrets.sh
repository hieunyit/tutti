#!/bin/sh
# Creates a fresh secret only. Existing credentials must never be rotated implicitly.
set -eu

command -v python3 >/dev/null
command -v kubectl >/dev/null

python3 - <<'PY' | kubectl create -f -
import json
import secrets

passwords = {key: secrets.token_urlsafe(48) for key in (
    "POSTGRES_PASSWORD", "REDIS_PASSWORD", "MASTER_PASSWORD", "JWT_SECRET"
)}
passwords["POSTGRES_USER"] = "tutti"
passwords["POSTGRES_DB"] = "tutti"
passwords["POSTGRES_DSN"] = (
    "postgres://tutti:" + passwords["POSTGRES_PASSWORD"]
    + "@postgres:5432/tutti?sslmode=disable"
)
print(json.dumps({
    "apiVersion": "v1",
    "kind": "Secret",
    "metadata": {"name": "tutti-secrets", "namespace": "tutti"},
    "type": "Opaque",
    "stringData": passwords,
}))
PY
