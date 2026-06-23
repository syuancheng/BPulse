# Review Checklist

Reviewer must be independent from the implementation pass. For a solo project, use a fresh Codex review session plus a deliberate human review.

## 1. Product and medical boundary

- [ ] No diagnosis is presented as fact.
- [ ] No medication dose/start/stop recommendation exists.
- [ ] No universal clinical threshold is silently hardcoded.
- [ ] Advice copy is centralized and reviewed.
- [ ] Abnormal-result copy asks the user to remeasure/contact a clinician rather than prescribing action.

## 2. Authentication and authorization

- [ ] Test/prod identity comes only from trusted CloudBase headers.
- [ ] Local debug identity is impossible outside local mode.
- [ ] Every patient query is scoped by authenticated identity.
- [ ] Caregiver reads require active binding plus explicit permission.
- [ ] Revocation takes effect immediately.
- [ ] Cross-user negative tests exist.

## 3. Privacy and sensitive data

- [ ] No health data or full openid appears in logs.
- [ ] No real patient data is present in tests or screenshots.
- [ ] Blood-pressure payload is encrypted before persistence.
- [ ] Keys and AppSecret are environment secrets.
- [ ] Export and deletion require ownership checks and auditing.
- [ ] Privacy declaration matches actual data collection and sharing.

## 4. Data integrity

- [ ] Daily-task generation has a database unique constraint.
- [ ] Reminder send has a database unique idempotency key.
- [ ] Concurrent workers cannot double-send.
- [ ] Transactions cover multi-record state changes.
- [ ] Timestamps are stored in UTC.
- [ ] User-day calculations use the configured timezone.

## 5. API quality

- [ ] Strict request validation.
- [ ] Stable error codes without sensitive details.
- [ ] Request size limits.
- [ ] Explicit HTTP and outbound timeouts.
- [ ] Context cancellation reaches database and external calls.
- [ ] Retries are bounded and only for transient errors.

## 6. Database migrations

- [ ] Migration is numbered and reproducible.
- [ ] New code works during rolling deployment with old schema/revision.
- [ ] No same-release destructive column removal.
- [ ] Required indexes and unique constraints exist.
- [ ] Backup and rollback steps are documented.

## 7. Blood-pressure entry and assisted recognition

- [ ] Page displays a fresh current local time by default.
- [ ] Date and time can be changed manually before saving.
- [ ] Time is submitted with offset/timezone and stored as UTC.
- [ ] Manual input remains fully usable when media permission/provider/network fails.
- [ ] Manual, voice, and photo use the same editable confirmation draft.
- [ ] OCR/ASR output is never auto-saved.
- [ ] Low-confidence/conflicting fields are blank rather than guessed.
- [ ] Repeated save taps cannot create duplicate records.
- [ ] Raw audio/images/transcripts/OCR text/file IDs/signed URLs are absent from DB and logs.
- [ ] Temporary objects are deleted on success, provider error, timeout, and client cancellation.
- [ ] Microphone/camera purpose is explained before permission request.
- [ ] Media type, duration, size, rate, and provider cost limits are enforced.

## 8. Mini Program UX

- [ ] Loading, empty, offline, error, and retry states exist.
- [ ] Buttons and text are suitable for older users.
- [ ] Subscription prompt follows a direct user action.
- [ ] Denial does not cause repeated harassment.
- [ ] Double-tap cannot create duplicate submissions.
- [ ] Test/debug pages are absent from production build.

## 9. Testing

- [ ] `make verify` passes.
- [ ] Domain unit tests cover edge cases.
- [ ] MySQL integration tests pass from a clean database.
- [ ] Privacy and authorization negative tests pass.
- [ ] Reminder overlap and duplicate tests pass.
- [ ] Production smoke-test procedure is current.
- [ ] Voice parser covers Chinese and Arabic numerals plus label synonyms.
- [ ] OCR parser covers labels, coordinates, rotation/glare, extra digits, and ambiguous output.
- [ ] Real-device tests cover permission denial, interruption, cancellation, timeout, correction, and manual time editing.

## 10. Release gate

- [ ] No unresolved P0.
- [ ] No unresolved P1.
- [ ] Test environment E2E completed.
- [ ] Production secrets verified outside Git.
- [ ] Database backup completed.
- [ ] Rollback owner and procedure identified.
- [ ] Privacy/service-category/subscribe-template configuration completed.
- [ ] ASR/OCR services, least-privilege credentials, quotas, budget alerts, and privacy declarations verified.
- [ ] Orphan-media cleanup and alert path tested.
