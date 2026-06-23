## Change

What changed and why?

## Scope

- [ ] Mini Program
- [ ] Go API
- [ ] Reminder worker
- [ ] Database migration
- [ ] Privacy/medical copy
- [ ] Deployment/configuration

## Verification

- [ ] `make verify` passed
- [ ] Unit tests added/updated
- [ ] Integration tests added/updated
- [ ] Cross-user authorization tested
- [ ] Reminder idempotency tested where relevant
- [ ] Experience-version manual test completed where relevant

## Privacy and safety

- [ ] No secret or real patient data added
- [ ] No health data/full openid logged
- [ ] Data collection and sharing remain consistent with privacy declaration
- [ ] No diagnosis or medication-change advice introduced

## Database and release

- [ ] Migration is backward-compatible
- [ ] Index/unique-constraint impact reviewed
- [ ] Rollback/disable path documented
- [ ] Environment variables documented without values

## Review

- [ ] Independent Codex `/review` completed
- [ ] No unresolved P0/P1 findings
- [ ] Human release checklist completed when production-facing


## Assisted entry (when applicable)

- [ ] Current measurement time defaults correctly and remains editable.
- [ ] Voice/photo output goes through the shared editable draft and never auto-saves.
- [ ] Permission denial/provider failure falls back to manual input.
- [ ] Temporary media is deleted on all paths; no media/transcript/OCR data is logged.
- [ ] Repeated save is idempotent.
