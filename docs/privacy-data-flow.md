# Privacy and assisted-entry data flow

Phase 4 assisted entry is a draft helper only. Voice and photo recognition
outputs populate editable fields, and the user must tap confirm before the Go API
stores an encrypted blood-pressure record.

Data-flow rules:

- Mini Program explains microphone/camera purpose before capture.
- Audio and images upload to random `bp-entry/...` CloudBase Storage paths for a
  single recognition attempt.
- The media-parser function validates type, size, duration, and per-user rate.
- Fake providers are used in local and CI tests. Real Tencent ASR/OCR adapter
  endpoints and credentials must stay in server-side configuration/secrets and
  are not configured in Git. Disabled-by-default contract tests must verify those
  endpoints before production release.
- Raw media, file IDs, transcripts, OCR text, signed URLs, and recognized values
  are not written to MySQL or application logs.
- Temporary media deletion runs in a finally path; failures are counted for
  observation and must be retried/alerted in production operations.
- External ASR/OCR processing and microphone/camera use require operator/legal
  review before production release.

This document is operational guidance, not a legal privacy notice.
