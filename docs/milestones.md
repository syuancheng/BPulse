# Milestones and acceptance criteria

1. **Bootstrap (Phase 0):** documented architecture/contracts; skeleton runtimes;
   `/healthz`; reversible initial migration; Docker MySQL; CI; `make verify` passes.
2. **Identity/profile:** trusted CloudBase identity, local-only debug identity,
   preferences, redacted logs, and cross-environment rejection tests.
3. **Plans/tasks:** idempotent local-day task generation, ownership, completion,
   skip flows, and timezone-boundary tests.
4. **Manual BP entry:** fresh/editable measurement time, shared draft, server-side
   averages, AES-256-GCM storage, bounded trends, idempotency, and ownership tests.
5. **Assisted entry:** deterministic voice/OCR candidates, explicit confirmation,
   blank ambiguity, guaranteed temporary-media cleanup, limits, and fake CI providers.
6. **Caregiver:** expiring invitations, active binding, permission-scoped reads,
   immediate revocation, and exhaustive cross-user tests.
7. **Reminders:** user-action subscription request, idempotent grants and jobs,
   atomic claims, bounded retry, fake notifier, and overlap tests.
8. **Privacy/release:** export/deletion, centralized reviewed copy, observability,
   runbooks, smoke tests, independent review, and human/test-environment approval.

No milestone advances with unresolved P0/P1 findings. Production deployment,
timers, secrets, and real providers require explicit human approval and setup.
