"use strict";

const assert = require("node:assert/strict");
const test = require("node:test");
const {main, parseRecognizedText, parseOcrBoxes, validateMedia, checkRateLimit, _private} = require("../src/index");

function storageWithMetadata(deleted, metadataByFileID = {}, ownerKey = "trusted-user") {
  return {
    deleteFile: async (fileID) => deleted.push(fileID),
    getMetadata: async (fileID) => metadataByFileID[fileID] || {contentType: "audio/mp3", sizeBytes: 10, durationSeconds: 2, ownerKey},
  };
}

test("voice phrases parse Chinese and Arabic numerals without saving", () => {
  const result = parseRecognizedText("高压是一百三十二，低压为84，心率约七十");
  assert.equal(result.candidate.systolic.value, 132);
  assert.equal(result.candidate.diastolic.value, 84);
  assert.equal(result.candidate.pulse.value, 70);
  assert.equal(result.needsConfirmation, true);
});

test("abbreviated spoken hundreds normalize to tens", () => {
  const result = parseRecognizedText("高压一百二，低压八十");
  assert.equal(result.candidate.systolic.value, 120);
  assert.equal(result.candidate.diastolic.value, 80);
});

test("ambiguous or unlabeled voice output remains blank", () => {
  const conflict = parseRecognizedText("收缩压130，上压135，舒张压80");
  assert.equal(conflict.candidate.systolic.value, null);
  assert.equal(conflict.candidate.systolic.reason, "conflict");
  const unlabeled = parseRecognizedText("132 84 70");
  assert.equal(unlabeled.candidate.systolic.value, null);
  assert.equal(unlabeled.candidate.diastolic.value, null);
});

test("OCR label and coordinate association ignores extra digits", () => {
  const result = parseOcrBoxes([
    {text: "2026", x: 10, y: 10},
    {text: "SYS", x: 10, y: 100},
    {text: "132", x: 130, y: 105},
    {text: "DIA", x: 10, y: 180},
    {text: "84", x: 130, y: 175},
    {text: "PUL", x: 10, y: 260},
    {text: "70", x: 130, y: 263},
  ]);
  assert.equal(result.candidate.systolic.value, 132);
  assert.equal(result.candidate.diastolic.value, 84);
  assert.equal(result.candidate.pulse.value, 70);
});

test("OCR conflicts and missing labels remain blank", () => {
  const result = parseOcrBoxes([
    {text: "SYS", x: 10, y: 100},
    {text: "132", x: 130, y: 105},
    {text: "收缩压", x: 10, y: 160},
    {text: "140", x: 130, y: 160},
    {text: "84", x: 130, y: 220},
  ]);
  assert.equal(result.candidate.systolic.value, null);
  assert.equal(result.candidate.systolic.reason, "conflict");
  assert.equal(result.candidate.diastolic.value, null);
});

test("low-confidence OCR fields remain blank", () => {
  const lowLabel = parseOcrBoxes([
    {text: "SYS", x: 10, y: 100, confidence: 0.6},
    {text: "132", x: 130, y: 105, confidence: 0.99},
  ]);
  assert.equal(lowLabel.candidate.systolic.value, null);
  assert.equal(lowLabel.candidate.systolic.reason, "low_confidence");

  const lowValue = parseOcrBoxes([
    {text: "DIA", x: 10, y: 100, confidence: 0.95},
    {text: "84", x: 130, y: 105, confidence: 0.5},
  ]);
  assert.equal(lowValue.candidate.diastolic.value, null);
  assert.equal(lowValue.candidate.diastolic.reason, "low_confidence");
});

test("media validation enforces type size duration", () => {
  assert.throws(() => validateMedia({purpose: "voice", file: {contentType: "audio/mpeg", durationSeconds: 2}}), /size/);
  assert.throws(() => validateMedia({purpose: "photo", file: {contentType: "image/jpeg", sizeBytes: 0}}), /size/);
  assert.throws(() => validateMedia({purpose: "voice", file: {contentType: "audio/mpeg", sizeBytes: 1}}), /duration/);
  assert.throws(() => validateMedia({purpose: "voice", file: {contentType: "audio/mpeg", sizeBytes: 1, durationSeconds: 16}}), /too long/);
  assert.throws(() => validateMedia({purpose: "photo", file: {contentType: "text/plain", sizeBytes: 1}}), /type/);
  assert.doesNotThrow(() => validateMedia({purpose: "photo", file: {contentType: "image/jpeg", sizeBytes: 1024}}));
});

test("main deletes temporary media on provider success and failure", async () => {
  const deleted = [];
  const storage = storageWithMetadata(deleted, {
    "cloud://env/bp-entry/tmp-a": {contentType: "audio/mp3", sizeBytes: 10, durationSeconds: 2, ownerKey: "trusted-cleanup-success"},
    "cloud://env/bp-entry/tmp-b": {contentType: "audio/mp3", sizeBytes: 10, durationSeconds: 2, ownerKey: "trusted-cleanup-failure"},
  });
  const success = await main(
    {purpose: "voice", userKey: "cleanup-success", file: {fileID: "cloud://env/bp-entry/tmp-a", contentType: "audio/mp3", sizeBytes: 10, durationSeconds: 2}, fakeRecognizedText: "高压120低压80"},
    {storage, identity: "trusted-cleanup-success"},
  );
  assert.equal(success.candidate.systolic.value, 120);
  assert.deepEqual(deleted, ["cloud://env/bp-entry/tmp-a"]);
  await assert.rejects(
    () =>
      main(
        {purpose: "voice", userKey: "cleanup-failure", file: {fileID: "cloud://env/bp-entry/tmp-b", contentType: "audio/mp3", sizeBytes: 10, durationSeconds: 2}},
        {storage, identity: "trusted-cleanup-failure", provider: {transcribe: async () => { throw new Error("timeout"); }}},
      ),
    /timeout/,
  );
  assert.deepEqual(deleted, ["cloud://env/bp-entry/tmp-a", "cloud://env/bp-entry/tmp-b"]);
});

test("main requires trusted identity for rate limiting", async () => {
  await assert.rejects(
    () => main({purpose: "text", recognizedText: "高压120低压80"}),
    /trusted identity/,
  );
  const result = await main({purpose: "text", recognizedText: "高压120低压80"}, {identity: "trusted-user"});
  assert.equal(result.candidate.systolic.value, 120);
});

test("real provider mode fails closed when Tencent adapter is not configured", async () => {
  await assert.rejects(
    () => main({purpose: "text", recognizedText: "高压120低压80"}, {identity: "trusted-user", providerMode: "tencent", env: {}}),
    /Tencent provider is not configured/,
  );
});

test("main deletes temporary media when provider configuration fails", async () => {
  const deleted = [];
  const storage = storageWithMetadata(deleted, {}, "trusted-config-failure");
  await assert.rejects(
    () =>
      main(
        {purpose: "voice", file: {fileID: "cloud://env/bp-entry/tmp-config-failure", contentType: "audio/mp3", sizeBytes: 10, durationSeconds: 2}},
        {storage, identity: "trusted-config-failure", providerMode: "real", env: {}},
      ),
    /Tencent provider is not configured/,
  );
  assert.deepEqual(deleted, ["cloud://env/bp-entry/tmp-config-failure"]);
});

test("real provider mode calls configured server-side adapter", async () => {
  const calls = [];
  const deleted = [];
  const httpClient = async (url, options) => {
    calls.push({url, options});
    return {
      ok: true,
      async json() {
        return {recognizedText: "高压一百二十低压八十"};
      },
    };
  };
  const result = await main(
    {purpose: "voice", file: {fileID: "cloud://env/bp-entry/tmp-real", contentType: "audio/mp3", sizeBytes: 10, durationSeconds: 2}},
    {
      storage: storageWithMetadata(deleted, {}, "trusted-real-user"),
      identity: "trusted-real-user",
      providerMode: "tencent",
      env: {
        TENCENT_ASR_ENDPOINT: "https://asr.invalid",
        TENCENT_OCR_ENDPOINT: "https://ocr.invalid",
        TENCENT_PROVIDER_AUTH_TOKEN: "synthetic-token",
        __httpClient: httpClient,
      },
    },
  );
  assert.equal(result.candidate.systolic.value, 120);
  assert.equal(result.candidate.diastolic.value, 80);
  assert.equal(calls.length, 1);
  assert.equal(calls[0].url, "https://asr.invalid");
});

test("documented real mode aliases Tencent provider adapter", async () => {
  const deleted = [];
  const httpClient = async () => ({
    ok: true,
    async json() {
      return {recognizedText: "高压一百二低压八十"};
    },
  });
  const result = await main(
    {purpose: "voice", file: {fileID: "cloud://env/bp-entry/tmp-real-mode", contentType: "audio/mp3", sizeBytes: 10, durationSeconds: 2}},
    {
      storage: storageWithMetadata(deleted, {}, "trusted-real-user-2"),
      identity: "trusted-real-user-2",
      providerMode: "real",
      env: {
        TENCENT_ASR_ENDPOINT: "https://asr.invalid",
        TENCENT_OCR_ENDPOINT: "https://ocr.invalid",
        TENCENT_PROVIDER_AUTH_TOKEN: "synthetic-token",
        __httpClient: httpClient,
      },
    },
  );
  assert.equal(result.candidate.systolic.value, 120);
});

test("main rejects forged client media metadata and deletes temporary media", async () => {
  const deleted = [];
  let providerCalled = false;
  await assert.rejects(
    () =>
      main(
        {purpose: "photo", file: {fileID: "cloud://env/bp-entry/tmp-forged", contentType: "image/jpeg", sizeBytes: 1}},
        {
          storage: storageWithMetadata(deleted, {"cloud://env/bp-entry/tmp-forged": {contentType: "image/jpeg", sizeBytes: 6 * 1024 * 1024, ownerKey: "trusted-forged-user"}}),
          identity: "trusted-forged-user",
          provider: {
            ocr: async () => {
              providerCalled = true;
              return [];
            },
          },
        },
      ),
    /too large/,
  );
  assert.equal(providerCalled, false);
  assert.deepEqual(deleted, ["cloud://env/bp-entry/tmp-forged"]);
});

test("main rejects media metadata owned by a different user and deletes temporary media", async () => {
  const deleted = [];
  let providerCalled = false;
  await assert.rejects(
    () =>
      main(
        {purpose: "photo", file: {fileID: "cloud://env/bp-entry/tmp-other-owner", contentType: "image/jpeg", sizeBytes: 1}},
        {
          storage: storageWithMetadata(deleted, {"cloud://env/bp-entry/tmp-other-owner": {contentType: "image/jpeg", sizeBytes: 10, ownerKey: "other-user"}}),
          identity: "trusted-owner-user",
          provider: {
            ocr: async () => {
              providerCalled = true;
              return [];
            },
          },
        },
      ),
    /owner/,
  );
  assert.equal(providerCalled, false);
  assert.deepEqual(deleted, []);
});

test("main preserves recorder duration when storage metadata cannot provide it", async () => {
  const deleted = [];
  const result = await main(
    {purpose: "voice", file: {fileID: "cloud://env/bp-entry/tmp-no-duration", contentType: "audio/mp3", sizeBytes: 1, durationSeconds: 2}, fakeRecognizedText: "高压120低压80"},
    {
      storage: storageWithMetadata(deleted, {"cloud://env/bp-entry/tmp-no-duration": {contentType: "audio/mp3", sizeBytes: 10, ownerKey: "trusted-duration-user"}}),
      identity: "trusted-duration-user",
    },
  );
  assert.equal(result.candidate.systolic.value, 120);
  assert.deepEqual(deleted, ["cloud://env/bp-entry/tmp-no-duration"]);
});

test("trusted identity can come from CloudBase context hook", async () => {
  const result = await main(
    {purpose: "text", recognizedText: "高压120低压80"},
    {getWXContext: () => ({OPENID: "trusted-cloudbase-user"})},
  );
  assert.equal(result.candidate.systolic.value, 120);
});

test("per-user rate limiting is deterministic", () => {
  _private.rateBuckets.clear();
  for (let i = 0; i < 10; i += 1) checkRateLimit("rate-user", 1000 + i);
  assert.throws(() => checkRateLimit("rate-user", 2000), /rate limit/);
  assert.doesNotThrow(() => checkRateLimit("rate-user", 70_000));
});
