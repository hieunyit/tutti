# Chorctl

CLI for Tutti management API.

![tuttictl.png](../../docs/media/tuttictl.png)

## Install

From source (requires [Go language](https://go.dev/doc/install)):
```shell
cd ./tools/tuttictl && go build .
```

Or install globally:
```shell
cd ./tools/tuttictl && go install .
```

Or download binary from [the latest release](https://github.com/clyso/chorus/releases).

Or with homebrew (macOS and Linux only):
```shell
brew install clyso/tap/tuttictl
```

## Usage

`tuttictl` sends requests to the REST API hosted by [Tutti worker](../../service/worker).
Deploy worker and provide REST API URL (including `http://` or `https://` scheme) to `tuttictl`
with `--address` flag or `TUTTI_ADDRESS` envar (default: `http://localhost:9671`):
```shell
export TUTTI_ADDRESS=http://127.0.0.1:9671
```

For `https://` addresses, TLS certificate verification can be disabled with `--insecure` (`-k`) flag.

If the [Web UI](../../ui) is deployed, it proxies the same API under `/api`. E.g. for UI on
`http://example.com:9090` use:
```shell
tuttictl --address http://example.com:9090/api
```

Run `tuttictl --help` for available commands. Key commands:

### Dashboard
```shell
tuttictl dash
```

### Replications
```shell
# List replications
tuttictl repl

# User-level replication (all buckets for user)
tuttictl repl add -u <user> -f <from storage> -t <to storage>

# Storage-level replication (no user; uses each storage connection account)
tuttictl repl add -f <from storage> -t <to storage>

# Bucket-level replication
tuttictl repl add -u <user> -b <bucket> -f <from storage> -t <to storage>

# See all repl subcommands
tuttictl repl --help
```

### Switch (change main storage after replication)
```shell
tuttictl repl switch --help
```

### Routing
```shell
tuttictl route --help
```

### Diff
```shell
tuttictl diff --help
```

### Dynamic Credentials
```shell
tuttictl set-user --help
```

For full command reference, run `tuttictl <command> --help`.
