# ADR 0001: System architecture and bootstrap

- Status: Accepted
- Date: 2026-06-22

## Context

BP Companion needs a small, auditable architecture for a native WeChat Mini
Program. It handles sensitive health-management data but does not diagnose or
recommend treatment. Assisted voice/photo entry must create only an editable
draft and must never persist without an explicit user confirmation.

## Decision

Use a native Mini Program that calls a Go HTTP API in CloudBase Run through
`wx.cloud.callContainer`. Test and production identity is accepted only from
trusted CloudBase-injected headers; `X-Debug-OpenID` is reserved for local mode.

Use CloudBase managed MySQL for relational state, transactions, unique
constraints, and auditability. Blood-pressure payloads will be encrypted with
AES-256-GCM before persistence; stable lookup metadata remains separate. Store
timestamps in UTC and convert only at user-timezone boundaries.

Use two Node.js CloudBase functions: a short-lived media parser and a scheduled
reminder worker. Media providers are deterministic and fake by default in local
and CI. Raw media, transcripts, OCR text, file IDs, signed URLs, secrets, and
health values must not enter business logs or the business database. Cleanup is
required on every media path. Reminder work is database-idempotent.

## Boundaries

- Go owns business validation, authorization, averages, encryption, and final persistence.
- Media recognition returns untrusted candidates with `needsConfirmation: true`.
- Caregiver access is deny-by-default and requires active binding plus permission.
- Subscription prompts follow explicit user action and respect denial.
- No real cloud resource changes or credentials are needed for ordinary tests.

## Consequences

This split keeps secrets and media processing off the client and makes critical
authorization and persistence rules testable in Go. It adds multi-runtime CI and
requires shared contract fixtures in later phases. Phase 0 establishes only the
skeleton and `/healthz`; later business phases require separate review gates.
