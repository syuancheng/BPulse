# BP Companion (BPulse)

A WeChat Mini Program for health-task reminders, low-friction blood-pressure
recording, and patient-authorized caregiver visibility. It records and reminds;
it does not diagnose conditions or recommend treatment or medication changes.

This repository is currently at Phase 4. In addition to identity/profile, daily
care plans, today's task instances, and encrypted blood-pressure records, it
implements assisted voice/photo entry that only populates an editable draft.

## Local verification

Prerequisites: Go 1.19+, Node.js 20+, Docker with Compose, and Make.

```bash
cp .env.example .env.local
make verify
```

Run local infrastructure and the API:

```bash
docker compose --env-file .env.local up -d mysql
make migrate-up
make run-api
curl http://localhost:8080/healthz
curl -H 'X-Debug-OpenID: synthetic-local-user-001' \
  http://localhost:8080/api/v1/me
```

The default local/test configuration uses synthetic data and fake media and
notification providers. Never commit real WeChat, CloudBase, Tencent Cloud, or
patient credentials/data.

Phase 4 adds deterministic assisted entry. OCR/ASR candidates never save
automatically; final saves still require `clientRequestId` and
`DATA_ENCRYPTION_KEY_B64`. Local debug identity must use synthetic values and is
rejected in test/production.

See [docs/repository-tree.md](docs/repository-tree.md) and
[docs/architecture/0001-system-bootstrap.md](docs/architecture/0001-system-bootstrap.md).
