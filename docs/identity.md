# Identity and user-profile boundary

Phase 1 trusts identity only at the HTTP middleware boundary. In test and
production, CloudBase Run injects `X-WX-OPENID`; the service is not intended for
public network access. In local mode only, synthetic `X-Debug-OpenID` is accepted.
A debug header in test or production makes the request unauthenticated, even if a
CloudBase header is also present.

The middleware validates the opaque header and immediately computes SHA-256.
Only the 32-byte reference enters request context or MySQL. Raw openid is not
available to handlers, repositories, responses, or application logging. Request
body, path, and query values are never identity evidence.

`GET /api/v1/me` idempotently creates a minimal profile. New profiles use `UTC`
until the client explicitly submits a valid IANA timezone. `PATCH
/api/v1/me/preferences` strictly accepts timezone plus large-text/high-contrast
preferences and updates only the current authenticated identity.

Current implemented owner-scoped resources include care plans, daily task
instances, and encrypted blood-pressure records. Caregiver and subscription
reminder flows remain future phases.
