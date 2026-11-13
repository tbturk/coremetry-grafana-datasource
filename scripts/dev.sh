#!/usr/bin/env bash
set -euo pipefail

repo_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
action="${1:-}"

if [[ -z "$action" ]]; then
  echo "Usage: $0 {install|build|watch|test|lint|format|format:check|typecheck|metadata|check}" >&2
  exit 2
fi
shift

case "$action" in
  install)
    set -- ci "$@"
    ;;
  build)
    set -- run build "$@"
    ;;
  watch)
    set -- run dev "$@"
    ;;
  test)
    set -- run test:ci "$@"
    ;;
  lint|format|format:check|typecheck|metadata|check)
    set -- run "$action" "$@"
    ;;
  *)
    echo "Unknown action: $action" >&2
    exit 2
    ;;
esac
docker_args=(
--rm --init
  --user "$(id -u):$(id -g)" \
  -e HOME=/tmp \
  -e WATCHPACK_POLLING=true \
  -v "$repo_dir:/workspace" \
  -w /workspace \
)

docker run "${docker_args[@]}" node:24.5.0-bookworm-slim npm "$@"
