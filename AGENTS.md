# Repository rules

Build BP Companion as health-management software, not diagnostic or treatment
software. Do not implement diagnosis, medication changes, treatment advice, or
unreviewed clinical claims. Never use real patient data in code, fixtures, logs,
screenshots, or prompts.

## Required workflow

Read the relevant design documents before editing. Plan multi-file work,
implement one coherent phase, add tests, run `make verify`, and perform an
independent review. Report changed files, exact test results, assumptions, risks,
and manual follow-up. Do not advance phases implicitly.

## Security and privacy

- Trust CloudBase-injected identity headers in test/production. Allow
  `X-Debug-OpenID` only when `APP_ENV=local`; never authorize from request data.
- Scope every query to the authenticated owner or an active, explicit caregiver permission.
- Encrypt blood-pressure payloads with AES-256-GCM before persistence and obtain keys from secrets.
- Never log request bodies for health endpoints, full openids, health values,
  notes, raw media, transcripts, OCR text, file IDs, signed URLs, or secrets.
- Delete temporary media on success and every failure path; retry and observe cleanup failures.
- Use fake ASR/OCR and notification providers in ordinary local and CI tests.
- Do not deploy, change cloud resources, enable timers, or rotate secrets without explicit approval.

## Blood-pressure entry constraints

Measurement time defaults to a fresh current local time when entry starts and is
manually editable. Manual, voice, and photo paths converge on one editable draft.
Voice/OCR results never auto-save; low-confidence or conflicting fields remain
blank. The Go API validates and computes averages. Final saves require an
idempotent `clientRequestId`. Store UTC and convert only at timezone boundaries.

## Engineering rules

Use thin Go handlers, services for business logic, repositories for persistence,
`context.Context` through I/O, explicit timeouts, strict DTO decoding, numbered
forward-compatible migrations, deterministic clocks/IDs in tests, and database
unique constraints for idempotency. Make reminder and daily-task work safe under
duplicate/concurrent invocation. Keep Mini Program CloudBase configuration in one
wrapper and implement loading, empty, offline, error, and retry states.

Required command surface: `make fmt`, `make lint`, `make unit`,
`make integration`, `make test`, `make verify`, `make run-api`,
`make migrate-up`, and `make migrate-down-one`.

No release may have unresolved P0/P1 findings. The detailed source constraints
remain in `bp-companion-codex-guide/AGENTS.md` and
`bp-companion-codex-guide/BP_ENTRY_DESIGN.md`.
