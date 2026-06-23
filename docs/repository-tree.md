# Repository tree

```text
.
├── miniprogram/           Native WeChat Mini Program skeleton
├── server/                Go CloudBase Run HTTP API and migration command
├── media-parser/          Node.js short-lived ASR/OCR function skeleton
├── reminder-worker/       Node.js scheduled reminder function skeleton
├── migrations/            Ordered MySQL migrations
├── docs/                  Architecture, contracts, environments, and plans
├── scripts/               Reproducible local and CI verification helpers
├── .github/workflows/     Pull-request CI
├── docker-compose.yml     Local MySQL only
└── Makefile               Stable developer and CI command surface
```

Phase 3 adds `server/internal/bprecord`, the manual Mini Program entry page at
`miniprogram/pages/bp-entry`, and migration `000005_bp_records`. Media
recognition, caregiver, and reminder business implementation remain out of
scope.
