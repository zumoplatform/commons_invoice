# commons_invoice

Shared Invoice model + repo (status FSM, auto-recalculation of totals)

## Versioning

Tags follow a "cap at 9, carry over" scheme: `v1.0.0 → v1.0.9 → v1.1.0 →
… → v1.9.9 → v2.0.0`. Auto-tagged on every push to `main` by
`.github/workflows/auto-tag.yml`. Consumers should pin to an exact tag
(`go get github.com/zumoplatform/commons_invoice@v1.0.0`).
