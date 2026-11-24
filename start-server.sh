#!/usr/bin/env bash
set -euo pipefail
project_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
temporal_source="${TEMPORAL_SOURCE:?Set TEMPORAL_SOURCE to your Temporal server checkout: TEMPORAL_SOURCE=/path/to/temporal ./start-server.sh}"
mkdir -p "$project_dir/.local"
(
    cd "$temporal_source"
    go build -tags disable_grpc_modules -o "$project_dir/.local/temporal-server" ./cmd/server
)
cd "$project_dir"
exec ./.local/temporal-server --config-file server.yaml --allow-no-auth start
