# AGENTS.md

## Mission

Build and maintain `bp-companion`, a WeChat Mini Program for health-task reminders, low-friction blood-pressure recording (manual, voice, and photo), exercise/diet check-ins, and caregiver-authorized visibility.

This is health-management software, not diagnostic or treatment software.

## Non-negotiable product boundaries

- Do not implement diagnosis, automatic medication changes, treatment recommendations, or claims of medical certainty.
- User-facing medical copy must remain generic or come from an explicitly approved content file.
- Do not hardcode clinical thresholds as universal truth. Personal target ranges are user-configured based on clinician advice. Engineering input bounds must be centralized and described only as data-quality limits.
- Never use real patient data in fixtures, screenshots, tests, logs, or prompts.

## Repository layout

- `miniprogram/`: native WeChat Mini Program client.
- `server/`: Go HTTP API deployed to CloudBase Run.
- `media-parser/`: Node.js CloudBase function for short-lived ASR/OCR processing.
- `reminder-worker/`: Node.js scheduled CloudBase function.
- `migrations/`: ordered MySQL migrations.
- `docs/`: architecture, API, privacy, runbooks, and decisions.
- `scripts/`: local verification and release helpers.

## Required workflow

1. Read relevant docs and existing code before editing.
2. For multi-file work, write or update a plan before implementation.
3. Implement the smallest coherent change.
4. Add or update tests with the code.
5. Run `make verify` before declaring completion.
6. Summarize changed files, tests, assumptions, risks, and follow-up work.
7. Run an independent review pass; implementation is not self-approval.

## Commands

Expected commands once the repository is bootstrapped:

```bash
make fmt
make lint
make unit
make integration
make test
make verify
make run-api
make migrate-up
make migrate-down-one
```

If a command is missing, add it rather than inventing ad-hoc alternatives in documentation.

## Go rules

- Use standard library first.
- Keep handlers thin; place business logic in services and persistence in repositories.
- Pass `context.Context` through all I/O paths.
- Set explicit HTTP and database timeouts.
- Wrap errors with operation context without leaking secrets or health data.
- Use typed request/response DTOs and strict JSON decoding.
- Use UTC in storage; convert to the user timezone only at boundaries.
- Make reminder and daily-task creation idempotent.
- Do not log request bodies for health-data endpoints.

## Mini Program rules

- Use one API wrapper around `wx.cloud.callContainer`.
- No environment IDs or service names scattered through page code.
- Measurement time defaults to a fresh current local time at the start of entry and remains manually editable.
- Manual, voice, and photo flows must converge on one editable draft component.
- Voice/photo output is never persisted without an explicit confirmation tap.
- Low-confidence or conflicting recognition must leave fields blank rather than guess.
- All loading, empty, offline, error, and retry states must be implemented.
- Large touch targets and readable text are required for older users.
- Subscription permission must be requested only after a clear user action.
- Do not repeatedly prompt after denial.

## Authentication and authorization

- Production/test identity comes from trusted CloudBase-injected headers.
- `X-Debug-OpenID` is allowed only when `APP_ENV=local`.
- Every query must be scoped by authenticated user or explicit caregiver binding.
- Caregiver access is deny-by-default and permission-scoped.
- Never use an openid supplied in JSON/query parameters as authorization.

## Health-data protection

- Blood-pressure payloads are encrypted with AES-256-GCM before database persistence.
- Raw audio, images, transcripts, OCR text, file IDs, and signed URLs are not stored in the business database or logs.
- Temporary media must be deleted in success and failure paths; cleanup failures are observable and retried.
- ASR/OCR secrets are server-side only and least-privileged.
- Encryption keys come from environment secrets; never commit keys.
- Store only metadata required for lookup outside encrypted payloads.
- Redact openid and health data in logs.
- Export and deletion operations require ownership checks and audit events.

## Database and migrations

- Every schema change uses a numbered migration.
- Migrations must be forward-compatible with the previous application revision.
- Do not combine destructive schema removal with the first deployment using a new schema.
- Add indexes for task lookup, patient/time lookup, binding lookup, and reminder idempotency.
- Unique constraints are preferred over application-only deduplication.

## Assisted media input

- Use `wx.getRecorderManager` for short voice capture and `wx.chooseMedia` for one compressed photo.
- Cap voice duration at 15 seconds and enforce media size/type limits.
- Use fake ASR/OCR providers in ordinary local and CI tests.
- Production parsing uses deterministic normalization and label/value association, not a generative model.
- Provider output is untrusted input. Validate shape and return an editable draft with reason codes.
- Final averages are computed by the Go API, never trusted from the client or provider.
- The final save endpoint requires `clientRequestId` idempotency.

## Reminder worker

- Scheduled invocations may repeat, overlap, or arrive late.
- Claim work atomically and persist a unique idempotency key.
- Retry transient failures with bounded exponential backoff.
- Permanent permission failures must not loop forever.
- A reminder is sent only when a recorded user grant is available.
- The worker must support fake mode and manual test invocation.

## Tests required

- Unit tests for domain rules, encryption, timezone boundaries, permission checks, idempotency, current-time capture, manual time override, numeral normalization, and OCR/ASR parsing.
- Integration tests against Docker MySQL.
- API tests for authentication, validation, ownership, and error contracts.
- Media-parser tests for permission denial, provider failure, ambiguous output, deletion in all paths, and no auto-save.
- Worker tests for duplicate invocation, retry, and permanent failure.
- Cross-user privacy tests are release blockers.
- Use deterministic clocks and deterministic IDs in tests.

## Review severity

- P0: health-data leak, cross-user access, committed secret, unsafe medical instruction, unrecoverable data loss.
- P1: broken authorization, duplicate reminders/records, missing idempotency, timezone bug, recognition auto-save, retained media, destructive migration, deletion/export failure.
- P2: maintainability, observability, accessibility, or non-blocking UX issue.

No release with unresolved P0 or P1 findings.

## Deployment safety

- Do not deploy, change CloudBase resources, enable timers, or rotate secrets without explicit user approval.
- Default Codex work is local code and test changes.
- Never use commands that bypass approvals or sandbox restrictions.
- Production changes require an explicit checklist and rollback plan.

## Definition of done

A task is done only when:

- implementation and tests are complete;
- `make verify` passes;
- documentation/API contract is updated;
- privacy and authorization effects are considered;
- the independent review has no unresolved P0/P1 findings;
- deployment or migration steps are documented when relevant.
