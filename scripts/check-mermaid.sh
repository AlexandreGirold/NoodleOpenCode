#!/usr/bin/env bash
# Compare generated Mermaid to reviewed fixtures without modifying the source tree.
set -euo pipefail
cd "$(dirname "$0")/.."
tmp=$(mktemp -d "${TMPDIR:-/tmp}/noodle-mermaid.XXXXXX")
trap 'rm -rf "$tmp"' EXIT
system=examples/project-architecture
for view in overview stack communication deployment; do
  go run ./cmd/noodle render "$system" --view "$view" --format mermaid -o "$tmp/$view.mmd"
  cmp "$system/diagrams/$view.mmd" "$tmp/$view.mmd"
done
