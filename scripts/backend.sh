#!/usr/bin/env bash
set -euo pipefail

repo_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
action="${1:-}"

if [[ -z "$action" ]]; then
  echo "Usage: $0 {test|build|check}" >&2
  exit 2
fi

case "$action" in
  test)
    go_command="go test ./pkg/..."
    ;;
  build)
    go_command="go run github.com/magefile/mage -v"
    ;;
  check)
    go_command="go test ./pkg/... && go run github.com/magefile/mage -v"
    ;;
  *)
    echo "Unknown action: $action" >&2
    exit 2
    ;;
esac

repo_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
docker run --rm --init \
  --user "$(id -u):$(id -g)" \
  -e HOME=/tmp \
  -e GOPATH=/tmp/go \
  -e GOCACHE=/tmp/go-build \
  -v "$repo_dir:/workspace" \
  -w /workspace \
  golang:1.26.3-bookworm \
  sh -c "$go_command"
