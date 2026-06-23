# Test strategy

## Phase 0 gates

- Formatting: `gofmt` and repository text checks.
- Lint: `go vet`, JavaScript syntax checks, shell syntax checks, Compose config.
- Unit: Go router/config tests and Node function skeleton tests.
- Integration: apply migration up, down-one, and up against Docker MySQL; verify schema.
- Build: compile Go commands and validate Mini Program/Node entry files.
- Security: scan tracked source for common committed-secret patterns.

`make verify` runs all gates. Integration tests use only synthetic local data and
safe local credentials. CI starts a MySQL 8.4 service and never calls WeChat,
CloudBase, Tencent ASR, or Tencent OCR.

## Phase 1 identity and profile coverage

- local/test/production identity-header matrix;
- non-local debug-header rejection, including when both headers are present;
- strict preference JSON and IANA timezone validation;
- body/query openid cannot influence authenticated identity;
- repeated bootstrap returns one user;
- two synthetic users retain separate preferences;
- database stores a fixed 32-byte identity hash and API errors/responses omit openid.

## Phase 2 care-plan and task coverage

- care-plan validation for allowed task types, local HH:MM time, and safe titles;
- CRUD scoped by authenticated internal user ID;
- local-day task generation uses the user's configured timezone;
- repeated `GET /api/v1/tasks/today` is idempotent through a database unique key;
- complete/skip actions are owner-scoped and cross-user attempts return not found;
- optional medication reminders are represented only as reminder tasks, with no dose advice.

## Phase 3 blood-pressure record coverage

- manual quick-entry draft defaults to fresh current local time and allows date/time edits;
- final saves require `clientRequestId` and repeated save taps return the same record;
- Go service validates data-quality bounds and computes averages server-side;
- measured times are submitted with offset/timezone and stored/queryable as UTC;
- AES-256-GCM encryption round trip, nonce uniqueness, wrong-key failure, and corrupted ciphertext paths;
- list, get, and trend APIs are owner-scoped; cross-user attempts return empty/not found;
- integration tests assert ciphertext does not contain synthetic plaintext health payload text.

## Later required coverage

Later phases add parser ambiguity, media cleanup, caregiver permission, privacy,
and overlapping worker tests. Release requires real-device testing with two
synthetic accounts. Recognition output must be proven unable to auto-save.

## Migration execution policy

MySQL DDL is not treated as transactional. The runner holds a database advisory
lock and records `applying`, `applied`, or `reverting` state around each DDL
operation. Interrupted/failed work remains visibly dirty and blocks later runs
until an operator inspects the schema. Each numbered migration file contains
exactly one SQL statement; multi-step changes use consecutive files.
Each statement has its own configurable timeout (`MIGRATION_STATEMENT_TIMEOUT`,
default `5m`); connection checks and migration locking use separate bounds.
