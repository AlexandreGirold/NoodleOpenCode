#!/usr/bin/env bash
# Session smoke test. Run it first in every session and fix anything red before new work.
set -euo pipefail
cd "$(dirname "$0")/.."

echo "== go vet / test"
go vet ./...
go test ./...

echo "== lint reference example"
go run ./cmd/noodle render examples/cnp-runtime --view runtime
go run ./cmd/noodle render examples/cnp-runtime --view oidc-login
go run ./cmd/noodle render examples/sequence-basics
go run ./cmd/noodle render examples/landscape --view stack
go run ./cmd/noodle render examples/catalog --view catalog
go run ./cmd/noodle render examples/cnp-access --view matrix
for d in platform-overview app-runtime-view repo-map; do go run ./cmd/noodle render examples/platform-regression/$d -icons examples/platform-regression/icons; done
go run ./cmd/noodle render docs/diagrams/contract-pipeline -icons docs/diagrams/icons

echo "== generated Mermaid architecture"
bash ./scripts/check-mermaid.sh

echo "== features.json history"
./scripts/check-features.sh

echo "== plugin manifest"
if command -v claude >/dev/null; then
  claude plugin validate . | tail -1
else
  echo "claude CLI absent: skipped"
fi

echo "== next features (docs/features.json)"
jq -r '.phases[] | .features[] | select(.passes == false) | "  \(.id)  \(.description)"' docs/features.json | head -5
