# Agent Instructions

Project: MicroVault (Go backend + JS frontend)

## Key Behaviors
- Config uses YAML by default; prefer `.localconfig.yaml`/`config.yaml` (JSON still parsed for legacy).
- Credits: `credits.scale` derives the internal unit as `10^scale`; `credits.units_per_credit` is the price in minor currency units per credit.
- Monetization crediting divides by `units_per_credit` and multiplies by `10^scale` (no separate `credits.unit`).
- Frontend auto-starts uploads on selection, queues additional files, highlights the active upload, and exposes a single cancel control. Balance is polled from `/user` every 10s.
- Upload flow: allocate via `/files/upload-url` (charges), upload via proxy `/files/{uploadId}/upload`, complete via `/files/{uploadId}/complete`.

## Constraints
- Do not revert user-made changes. Avoid destructive git commands (`reset --hard`, `checkout --` without request).
- Default to ASCII for edits; add comments only when non-obvious.
- Use YAML-first in docs and examples; keep configs aligned with `credits.units_per_credit` and `credits.scale`.

## Build & Test
- Backend: `go test ./...` and `go build -o microvault .`
- Frontend: `cd sample-client && pnpm install --frozen-lockfile && pnpm run build`

## Deployment Notes
- `deploy.sh` validates `deploy/config.yaml` via PyYAML before upload.
- Default config paths: `.localconfig.yaml/.yml` and `/etc/microvault/config.yaml` (YAML preferred).

## Editing Guidance
- Use `apply_patch` for single-file edits when practical; avoid unnecessary whitespace churn.
- Keep upload queue UX: visible queue, active item highlighted, single cancel control, auto-start on selection.
- Balance display should keep polling unless explicitly disabled.
