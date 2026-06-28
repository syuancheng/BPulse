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

Phase 4 adds `server/internal/bpentry`, `POST /api/v1/bp-entry/interpret`, and
the media-parser fake provider/parser. Caregiver and reminder business
implementation remain out of scope.
