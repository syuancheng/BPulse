# Codex CLI Prompt Pack

Use one phase at a time. Start each phase in plan mode. Do not ask Codex to implement all phases in a single session.

## Phase 0 — Architecture and repository bootstrap

```text
Read AGENTS.md and the implementation guide. Work in plan mode first.

Design and bootstrap the bp-companion repository with:
- native WeChat Mini Program client;
- Go HTTP API for CloudBase Run;
- MySQL migrations and repositories;
- Node.js CloudBase function for short-lived voice/photo parsing;
- Node.js CloudBase timer function for reminders;
- local Docker MySQL;
- Makefile and GitHub Actions CI.

Before writing application features, produce:
1. repository tree;
2. architecture decision record;
3. API contract for MVP endpoints;
4. database ERD and indexes;
5. environment variable matrix;
6. test strategy;
7. milestone plan with acceptance criteria.

Constraints:
- no medical diagnosis or medication advice;
- CloudBase mini-program-only access through wx.cloud.callContainer;
- CloudBase-injected identity headers in test/prod;
- X-Debug-OpenID only in local mode;
- encrypted blood-pressure payloads;
- no secrets in Git;
- deterministic tests.

After the plan, implement only the repository skeleton, health endpoint, local Docker MySQL, migration runner, Makefile, CI, and tests. Run make verify and report exact results.
```

## Phase 1 — Identity and user profile

```text
Implement authenticated user bootstrap.

Requirements:
- middleware reads trusted X-WX-OPENID in test/prod;
- local mode accepts X-Debug-OpenID;
- non-local mode must reject X-Debug-OpenID;
- never authorize using an openid supplied in body/path/query;
- upsert a minimal user record;
- endpoints: GET /api/v1/me and PATCH /api/v1/me/preferences;
- store timezone and accessibility preferences;
- unit, integration, and cross-user tests;
- redact identity in logs.

Update API docs and run make verify. Do not implement tasks yet.
```

## Phase 2 — Care plans and daily tasks

```text
Implement care plans and today's task instances.

Task types: blood_pressure, exercise, diet, medication_reminder_optional.
Medication tasks are reminders only and must not contain dose-change advice.

Requirements:
- CRUD care plans;
- generate daily task instances idempotently;
- GET /api/v1/tasks/today;
- POST /api/v1/tasks/{id}/complete;
- POST /api/v1/tasks/{id}/skip;
- UTC storage and user-timezone boundaries;
- unique constraints prevent duplicates;
- injectable clock for tests;
- validation and ownership tests.

Update docs, migrations, and tests. Run make verify.
```

## Phase 3 — Manual quick entry and encrypted blood-pressure records

```text
Implement the shared blood-pressure draft editor and final recording API.

Requirements:
- default measurement time to a fresh current local time when entry begins;
- allow manual date/time editing;
- use one large editable draft component shared by all future entry methods;
- accept one required reading, optional pulse/note, and an optional second reading;
- compute averages server-side;
- accept `entryMethod` and an idempotent `clientRequestId`;
- repeated save taps must return the same record;
- encrypt the health payload using AES-256-GCM;
- store lookup metadata separately from ciphertext;
- endpoints to create, list recent records, and retrieve a record;
- trend endpoint decrypts only the bounded requested window;
- strict validation without declaring a diagnosis;
- never log health payloads;
- tests for current-time capture, manual time override, timezone conversion, repeated taps, encryption round trip, nonce uniqueness, wrong key failure, ownership, validation, and corrupted ciphertext.

Add key-version support for future rotation. Run make verify.
```

## Phase 4 — Voice and photo assisted entry

```text
Implement assisted blood-pressure entry without changing the final save authority.

Mini Program requirements:
- voice capture through wx.getRecorderManager with tap-to-start/tap-to-stop and 15-second cap;
- photo selection through wx.chooseMedia, one compressed image from camera or album;
- upload to a random short-lived CloudBase Storage path;
- purpose explanation and graceful permission denial;
- voice/photo results populate the Phase 3 editable draft;
- no OCR/ASR result can auto-save.

Media-parser Cloud Function requirements:
- provider interfaces with fake and Tencent implementations;
- Tencent single-sentence ASR and OCR credentials only in server-side secrets;
- deterministic Chinese/Arabic numeral normalization;
- label synonyms for systolic, diastolic, and pulse;
- OCR label/box association using SYS/DIA/PUL/mmHg/bpm/Chinese labels;
- per-field confidence and reason codes; ambiguous fields remain empty;
- strict file type/size/duration limits and per-user rate limiting;
- raw media, transcript, OCR text, file IDs, and values never logged or persisted;
- temporary media deleted in a finally path, with bounded cleanup retry and metrics.

Go API requirements:
- POST /api/v1/bp-entry/interpret accepts recognized text and returns deterministic candidates;
- provider/client output is untrusted;
- final POST /api/v1/bp-records from Phase 3 remains the only persistence path.

Tests:
- table-driven voice phrases with Chinese and Arabic numerals;
- OCR fixtures with labels, coordinate variations, rotation, glare, extra digits, missing labels, and conflicts;
- permission denial, cancellation, provider timeout/failure, retry, deletion on every path, no auto-save, and repeated save idempotency;
- manual real-provider contract tests disabled by default in CI.

Update privacy/data-flow docs and run make verify.
```

## Phase 5 — Caregiver binding and permissions

```text
Implement caregiver invitations and permission-scoped access.

Requirements:
- short-lived single-use invitation code;
- patient approves the binding;
- permissions: task_completion, exact_bp_values, diet_records;
- deny by default;
- patient can revoke binding immediately;
- caregiver endpoints must join through active binding and explicit permission;
- audit grant, permission change, and revoke events;
- exhaustive patient-A/patient-B/caregiver tests.

Do not expose exact blood pressure when only task_completion is granted. Run make verify.
```

## Phase 6 — Subscription grants and reminder worker

```text
Implement one-time subscription-message reminder flow.

Mini Program:
- request subscription only after the user taps “提醒我”; 
- report accept/reject result to the API;
- do not repeatedly prompt after rejection.

Backend:
- record an estimated available grant count per user/template;
- expose idempotent grant-recording endpoint;
- no health values in message content.

Worker:
- Node.js scheduled function using the same MySQL database;
- query due pending reminders;
- atomically claim work;
- unique idempotency key per task/reminder level;
- fake notifier mode;
- bounded retry for transient failures;
- permanent permission failure stops retries and reconciles grant count;
- manual invocation for test environment;
- metrics for attempted, sent, duplicate, transient failure, permanent failure.

Add tests for overlapping worker runs and duplicate invocations. Run make verify.
```

## Phase 7 — Privacy, export, deletion, and medical copy

```text
Implement privacy-facing functions.

Requirements:
- privacy and terms pages in the Mini Program;
- clear statement that the app records and reminds, not diagnoses or adjusts medication;
- list data categories, caregiver-sharing behavior, microphone/camera usage, short-lived media storage, and external ASR/OCR processing;
- export endpoint with ownership check and audit event;
- account deletion endpoint with confirmation and deterministic deletion/anonymization plan;
- revoke caregiver bindings during deletion;
- centralize all health advice copy in content/medical-copy.zh-CN.json;
- add tests proving code behavior matches declared data use.

Do not invent legal promises. Mark text that requires operator/legal review. Run make verify.
```

## Phase 8 — Production readiness

```text
Harden the repository for a limited production release.

Add:
- request IDs and structured redacted logs;
- HTTP, database, and outbound API timeouts;
- rate limiting for write endpoints;
- healthz and readiness endpoints;
- graceful shutdown;
- database connection-pool limits;
- metrics and alert recommendations;
- migration runbook;
- test/prod environment matrix;
- smoke-test script;
- backup and rollback runbook;
- release checklist.

Run make verify, then run a dedicated /review focused on authorization, privacy, encryption, migrations, reminder idempotency, and unsafe medical copy.
```

## Independent reviewer prompt

Run in a fresh Codex session or through `/review`:

```text
Review this change as an adversarial production reviewer. Do not modify files.

Prioritize:
1. cross-user or caregiver authorization bypass;
2. leakage of openid, health data, secrets, or request bodies;
3. encryption misuse, nonce reuse, or key-handling mistakes;
4. duplicate daily tasks or reminders under concurrency;
5. timezone and DST/date-boundary bugs;
6. destructive or non-backward-compatible migrations;
7. account deletion/export gaps;
8. user-facing diagnosis, medication adjustment, or misleading certainty;
9. recognition output persisted without explicit confirmation;
10. retained/logged media, transcripts, OCR text, file IDs, or signed URLs;
11. stale/default measurement time or timezone conversion error;
12. missing failure handling and observability.

Report findings as P0/P1/P2 with file, line, impact, and a specific fix. If there are no blocking findings, state which checks you performed and remaining test gaps.
```
