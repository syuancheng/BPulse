# BP Companion Implementation Guide

## 1. Purpose

Build a small, auditable WeChat Mini Program that reduces the daily management burden for a person with high blood pressure. The system provides reminders and records; it is not a medical diagnostic or treatment system.

Primary users:

- Patient: creates plans, completes tasks, records blood pressure, controls sharing.
- Caregiver: sees only information explicitly authorized by the patient.
- Operator: deploys, reviews, monitors, and handles data requests.

## 2. MVP scope

In scope:

- Daily blood-pressure, exercise, diet, and optional medication-reminder tasks.
- Low-friction blood-pressure entry with automatic current time, editable measurement time, manual input, voice input, photo OCR, optional second reading, and bounded trend display.
- Patient-controlled caregiver binding and permission scopes.
- One-time subscription-message reminders.
- Privacy notice, export, account deletion, and audit events.

Out of scope:

- Diagnosis, risk scoring, treatment recommendations, medication changes.
- Emergency dispatch.
- Bluetooth devices, AI diagnosis/chat, payments, and social feed.
- Hospital/clinic integration.

## 3. Architecture decision

```text
WeChat Mini Program
  │ wx.cloud.callContainer
  ▼
CloudBase Run: Go API
  │ standard SQL connection
  ▼
CloudBase managed MySQL

CloudBase Timer Trigger
  ▼
Node.js reminder-worker
  ├─ reads due reminders from MySQL
  └─ calls WeChat subscription-message API

Mini Program media capture
  ├─ wx.getRecorderManager for short voice input
  └─ wx.chooseMedia for camera/album image
        ▼
CloudBase Storage (short-lived object)
        ▼
Node.js bp-media-parser Cloud Function
  ├─ Tencent Cloud single-sentence ASR
  ├─ Tencent Cloud OCR
  ├─ deterministic field extraction
  └─ deletes temporary media after processing
```

Why:

- Go remains the primary business backend.
- CloudBase Run removes server maintenance.
- `wx.cloud.callContainer` avoids a public API domain for the first release.
- A timer function remains available even when the Go container scales down.
- MySQL provides transactions, unique constraints, and auditable relations.
- A separate media-parser function keeps OCR/ASR credentials off the client and allows short-lived processing without making the Go API accept large media uploads.
- Voice/photo recognition produces an editable draft only; the Go API remains authoritative for validation and final persistence.

## 4. Environment strategy

### Local

- Go API on localhost.
- Docker MySQL.
- Fake notifier and fake OCR/ASR providers.
- Synthetic voice transcripts and OCR fixtures; no real patient media.
- Synthetic identities via `X-Debug-OpenID`.
- No CloudBase credentials required for normal unit/integration tests.

### Test

- Separate CloudBase environment and MySQL.
- Separate CloudBase Run service.
- Worker timer disabled by default; manual trigger first.
- Media parser defaults to fake provider for normal E2E, with a separate manually triggered provider contract test against Tencent Cloud ASR/OCR.
- WeChat experience version points to test.
- Synthetic data only.

### Production

- Separate environment, database, encryption key, logs, service, and worker.
- Only production mini-program build points to production.
- Real reminders and real media providers enabled only after final smoke tests, cost limits, rate limits, and privacy declarations are verified.

Never share databases or encryption keys between test and production.

## 5. Suggested repository

```text
miniprogram/
  app.js
  app.json
  config/
  services/api.js
  services/media-input.js
  components/bp-draft-editor/
  pages/home/
  pages/bp-entry/
  pages/trends/
  pages/caregiver/
  pages/settings/

server/
  cmd/api/main.go
  internal/auth/
  internal/user/
  internal/careplan/
  internal/task/
  internal/bprecord/
  internal/bpinterpret/
  internal/caregiver/
  internal/subscription/
  internal/privacy/
  internal/platform/wechat/
  internal/platform/crypto/
  internal/platform/mysql/

media-parser/
  src/index.js
  src/providers/asr.js
  src/providers/ocr.js
  src/normalize.js
  src/delete-temp-file.js
  test/

reminder-worker/
  src/index.js
  src/claim.js
  src/notifier.js
  test/

migrations/
docs/
scripts/
```

## 6. Core data model

Recommended tables:

- `users`: internal ID, openid hash/reference, timezone, accessibility preferences.
- `care_plans`: owner, task type, local schedule, recurrence, enabled state.
- `task_instances`: one concrete task for one date/time.
- `bp_records`: owner, measured time, encrypted payload, nonce, key version, entry method, and idempotent client request ID.
- `caregiver_invitations`: hashed code, expiry, single-use state.
- `caregiver_bindings`: patient, caregiver, state.
- `caregiver_permissions`: binding and permission name.
- `subscription_grants`: estimated available count per user/template.
- `reminder_jobs`: due time, state, attempt count, idempotency key.
- `audit_events`: actor, action, target, redacted metadata, time. Never put raw transcript, OCR text, image/audio path, or pressure values in audit metadata.
- `deletion_requests`: deletion lifecycle and completion time.

Important constraints:

```text
UNIQUE(care_plan_id, scheduled_local_date, occurrence_key)
UNIQUE(reminder_idempotency_key)
UNIQUE(owner_id, client_request_id)
UNIQUE(patient_id, caregiver_id) for active binding semantics
```

## 7. API surface

Suggested MVP endpoints:

```text
GET    /healthz
GET    /readyz

GET    /api/v1/me
PATCH  /api/v1/me/preferences

POST   /api/v1/care-plans
GET    /api/v1/care-plans
PATCH  /api/v1/care-plans/{id}
DELETE /api/v1/care-plans/{id}

GET    /api/v1/tasks/today
POST   /api/v1/tasks/{id}/complete
POST   /api/v1/tasks/{id}/skip

POST   /api/v1/bp-entry/interpret
POST   /api/v1/bp-records
GET    /api/v1/bp-records?from=&to=&limit=
GET    /api/v1/bp-records/{id}
GET    /api/v1/bp-trends?days=7

POST   /api/v1/caregiver-invitations
POST   /api/v1/caregiver-bindings/accept
GET    /api/v1/caregiver-bindings
PATCH  /api/v1/caregiver-bindings/{id}/permissions
DELETE /api/v1/caregiver-bindings/{id}
GET    /api/v1/caregiver/patients/{patientId}/summary

POST   /api/v1/subscription-grants
GET    /api/v1/privacy/export
POST   /api/v1/privacy/deletion-requests
```

Use explicit error codes such as `INVALID_ARGUMENT`, `UNAUTHENTICATED`, `FORBIDDEN`, `NOT_FOUND`, `CONFLICT`, and `INTERNAL`.

## 8. Identity model

For mini-program-only CloudBase Run access:

- CloudBase injects the mini-program identity header.
- Go middleware maps the trusted openid to an internal user ID.
- The application never accepts openid in a request body as proof of identity.
- Public network access is disabled for the user-facing API.
- Local mode has a separate debug header guarded by environment.

Add tests proving the debug header is rejected in test/prod.

## 9. Health-data encryption

Use AES-256-GCM with a 32-byte environment key.

Plain payload example before encryption:

```json
{
  "readings": [
    {"sbp": 132, "dbp": 84, "pulse": 70},
    {"sbp": 129, "dbp": 82, "pulse": 69}
  ],
  "summary": {"avgSbp": 131, "avgDbp": 83, "avgPulse": 70},
  "entryMethod": "photo",
  "note": ""
}
```

Persist:

- owner ID;
- measured timestamp supplied by the confirmed draft;
- idempotent client request ID;
- entry method (`manual`, `voice`, or `photo`);
- ciphertext;
- nonce;
- key version;
- created timestamp.

Generate a fresh random nonce for every record. Authenticate stable record metadata as additional authenticated data. Never reuse a nonce with the same key.

For the small MVP, trend queries load a bounded record window and decrypt in Go. This avoids plaintext pressure values in database columns.

## 10. Low-friction blood-pressure entry

### 10.1 Product rule

The user should be able to create a correct draft in one action and save it after one explicit confirmation. Recognition is an input aid, not an authority.

```text
Open “记录血压”
  → timestamp defaults to current local time
  → choose manual / voice / photo
  → produce editable draft
  → user verifies values and measurement time
  → explicit “确认保存”
  → Go API validates, encrypts, and stores
```

Never auto-save OCR or ASR output. If recognition is ambiguous, leave the field empty instead of guessing.

### 10.2 Time behavior

- When the entry page is opened, show `刚刚 · HH:mm` using the user's configured timezone.
- Capture a fresh default timestamp when the first value is entered, recording starts, or a photo is selected; this avoids saving the stale page-open time.
- Tapping the timestamp opens date and time pickers.
- Show the selected absolute date when it is not today.
- The client submits RFC 3339 with offset plus `timezone` and `clientRequestId`.
- The server stores UTC and returns the confirmed local representation.
- Reject malformed time and require explicit confirmation for a future time or an unusually old record. These are data-quality checks, not clinical rules.

### 10.3 Manual input

The default editor has three large numeric fields:

```text
高压 / 收缩压    [   ]
低压 / 舒张压    [   ]
心率（可选）     [   ]
```

Requirements:

- numeric keyboard;
- auto-advance from systolic to diastolic to pulse;
- pulse is optional;
- one reading is sufficient to save;
- “再记录一次” adds a second reading and the server computes the average;
- repeated taps use the same `clientRequestId` and cannot create duplicates;
- engineering input bounds are centralized and described as format/data-quality limits, never diagnostic thresholds.

### 10.4 Voice input

Interaction:

1. User taps a large microphone button once to start.
2. The page shows an example: `高压一百三十二，低压八十四，心率七十`.
3. User taps again to stop; stop automatically after 15 seconds.
4. Upload the short audio file to a random, short-lived CloudBase Storage path.
5. Call `bp-media-parser` with `{kind: "voice", fileID, requestId}`.
6. The function calls single-sentence ASR, normalizes Chinese/Arabic numerals and synonyms, and returns candidates.
7. Display transcript and editable fields; require confirmation.
8. Delete the temporary object in a `finally` path whether recognition succeeds or fails.

Recognized labels include:

- systolic: `高压`, `收缩压`, `上压`;
- diastolic: `低压`, `舒张压`, `下压`;
- pulse: `心率`, `脉搏`.

Do not use a generative model for the first-release parser. Use deterministic token normalization and label/value association with table-driven tests.

### 10.5 Photo input

Interaction:

1. Use `wx.chooseMedia` with one compressed image from camera or album.
2. Show framing guidance: fill the frame with the blood-pressure monitor display, avoid glare, keep digits upright.
3. Upload to a random, short-lived CloudBase Storage path.
4. Call `bp-media-parser` with `{kind: "photo", fileID, requestId}`.
5. OCR returns text, boxes, and provider confidence when available.
6. Associate values with `SYS`, `DIA`, `PUL/PULSE`, `mmHg`, `bpm`, or Chinese labels using coordinates and deterministic heuristics.
7. Return per-field candidates and confidence/reason codes.
8. The client always opens the same draft editor; low-confidence or conflicting fields remain blank.
9. Delete the temporary image after processing.

Do not retain the original monitor photo in the health record. A future explicit “keep photo” feature would require separate consent, storage, encryption, retention, export, and deletion design.

### 10.6 Media-parser contract

Request:

```json
{
  "kind": "voice",
  "fileID": "cloud://...",
  "requestId": "uuid"
}
```

Response:

```json
{
  "requestId": "uuid",
  "recognizedText": "高压132低压84心率70",
  "candidates": {
    "sbp": {"value": 132, "confidence": 0.98, "reason": "label_match"},
    "dbp": {"value": 84, "confidence": 0.97, "reason": "label_match"},
    "pulse": {"value": 70, "confidence": 0.93, "reason": "label_match"}
  },
  "needsConfirmation": true,
  "warnings": []
}
```

`recognizedText` is returned to the current client session for transparency but is not written to the business database or application logs.

### 10.7 Final record contract

```json
{
  "clientRequestId": "uuid",
  "measuredAt": "2026-06-22T23:20:00+08:00",
  "timezone": "Asia/Singapore",
  "entryMethod": "voice",
  "readings": [
    {"sbp": 132, "dbp": 84, "pulse": 70}
  ],
  "note": ""
}
```

The server recomputes averages, validates ownership and shape, encrypts the payload, and returns the stored record. The server never trusts a client-supplied average or recognition confidence.

### 10.8 Privacy, permissions, and cost controls

- Explain the purpose immediately before requesting microphone/camera access.
- Declare microphone, camera/album, temporary cloud storage, ASR, and OCR usage in the Mini Program privacy materials.
- ASR/OCR credentials exist only in CloudBase secrets or a least-privileged workload identity; never in Mini Program code.
- Enforce file type, duration, pixel dimensions, and size limits before provider calls.
- Rate-limit media parsing per user and per environment.
- Set Tencent Cloud budget alerts and disable accidental post-free-tier spending until explicitly approved.
- Do not log file IDs, signed URLs, transcripts, OCR text, candidate values, or provider request bodies.
- Record only redacted operational metrics such as provider, success/failure reason, duration bucket, and deletion success.
- Treat temporary-file deletion failures as alerts and retry with bounded cleanup jobs.

## 11. Reminder design

A one-time subscription permission is a scarce user-granted capability. Treat the local count as an estimate, not an entitlement.

Flow:

```text
User taps “提醒我”
  → Mini Program requests subscription
  → accepted result is recorded
  → task produces a reminder job
  → worker atomically claims due job
  → worker sends message
  → success marks sent and decrements estimate
```

Worker states:

```text
pending → claimed → sent
                ↘ retry_wait
                ↘ permanent_failed
```

Idempotency:

```text
{task_instance_id}:{reminder_level}:{scheduled_at}
```

The database unique constraint is authoritative. Do not rely only on Redis or in-memory locks.

## 12. Testing strategy

### Unit tests

- care-plan recurrence;
- local-day generation around midnight;
- encryption/decryption and bad-key behavior;
- caregiver permission matrix;
- reminder retry classification;
- request validation and average calculation;
- current-time capture, manual time override, and timezone conversion;
- Chinese/Arabic numeral normalization and voice label parsing;
- OCR box-to-label association, conflict handling, and low-confidence blanking;
- recognition never auto-saves.

### Integration tests

Run against clean Docker MySQL:

- migrations up/down-one/up;
- repository transactions;
- unique constraints under concurrency;
- API ownership checks;
- deletion/export flow;
- worker claim and duplicate prevention;
- final record idempotency with repeated taps;
- media-parser fake-provider contracts and guaranteed temp-file cleanup.

### Contract tests

Use shared JSON fixtures so the Mini Program client and Go API agree on:

- field names;
- error shape;
- date format;
- pagination;
- optional fields.

### Experience-version E2E

Test on real devices and two accounts:

- patient onboarding;
- task creation and completion;
- encrypted recording and trends;
- caregiver invite/approve/revoke;
- subscription accept/reject;
- worker send;
- offline retry;
- account deletion;
- manual entry, timestamp edit, voice entry, and photo entry on real devices;
- microphone/camera denial, cancellation, interruption, timeout, and retry;
- photo glare/rotation/extra-number cases and ambiguous-recognition correction.

### Non-functional tests

- repeated taps;
- concurrent worker invocations;
- expired invitation;
- DB outage;
- WeChat API timeout/rate failure;
- corrupted ciphertext;
- cold start;
- large but bounded history;
- media size/duration limits, parser rate limits, provider outage, and orphan cleanup.

## 13. CI and quality gates

PR CI should run:

```text
Go format check
go vet
static analysis/lint
govulncheck
Go unit tests with race detector where practical
MySQL integration tests
Node lint and tests
Mini Program lint/type checks
secret scan
migration validation
```

Suggested release thresholds:

- Core domain packages: at least 80% statement coverage.
- Whole backend: at least 70% statement coverage.
- No unresolved P0/P1 review findings.
- All privacy and cross-user tests mandatory regardless of coverage.

Coverage is a signal, not a substitute for authorization tests.

## 14. Reviewer model

### Implementer

Codex session that changes code and tests.

### Independent code reviewer

A fresh Codex session or `/review` pass. It does not edit the working tree. It reports P0/P1/P2 findings.

### Human release owner

Checks product behavior, medical wording, privacy declarations, CloudBase configuration, migration, backup, and the actual WeChat experience build.

### Medical-content reviewer

Any new clinical-sounding user copy should be reviewed by a qualified person before broad public release. Until then, keep copy generic and non-diagnostic.

### Assisted-entry UX reviewer

A human reviewer tests voice and photo capture on at least two real devices, verifies every recognized value is editable, and confirms no recognition path can persist without the explicit save action.

## 15. Git workflow

- `main` is always releasable.
- Work on `feat/...`, `fix/...`, or `chore/...` branches.
- One coherent feature per PR.
- Squash merge after CI, independent review, and human checklist.
- Tag production releases, for example `v0.1.0`.
- Store release notes and migration notes with each tag.

A solo developer must not merge directly to `main` for production changes.

## 16. Observability

Track:

- API request count, latency, and error rate;
- DB pool saturation and query errors;
- reminder attempted/sent/duplicate/retry/permanent failure;
- task generation duplicates prevented;
- encryption/decryption errors;
- account deletion/export failures.

Logs contain request ID, operation, duration, status, and redacted internal user ID. They do not contain raw health records, notes, secrets, full openid, or message payloads.

## 17. Release process

1. Freeze feature scope.
2. `make verify` on clean checkout.
3. Independent Codex review.
4. Human review checklist.
5. Backup production database.
6. Apply additive migration.
7. Deploy API revision.
8. Smoke test API and DB.
9. Deploy worker disabled.
10. Upload production-configured experience version.
11. Real-device regression.
12. Enable worker.
13. Submit WeChat review.
14. Publish after approval.
15. Post-release smoke test and observation.

## 18. Rollback

Prepare before release:

- previous CloudBase Run revision/image;
- previous Mini Program production version or feature flag path;
- database backup;
- migration rollback notes;
- ability to disable worker immediately.

Prefer forward fixes and feature flags. Do not attempt destructive rollback after new writes unless explicitly tested.

## 19. Launch risks

Highest risks:

1. Cross-patient data exposure.
2. Health data in logs or plaintext database fields.
3. Misleading medical advice.
4. Duplicate reminders caused by retries/concurrency.
5. Test environment accidentally connected to production.
6. Privacy declaration not matching implementation.
7. CloudBase/WeChat console configuration performed manually but not documented.
8. OCR/ASR guesses saved without user verification.
9. Temporary audio/images, transcripts, or signed URLs retained or logged.
10. Incorrect measurement time caused by stale page time or timezone conversion.

Treat the first three and any assisted-entry privacy or auto-save defect as release blockers.
