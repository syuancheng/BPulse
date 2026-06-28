"use strict";

const crypto = require("node:crypto");

const MAX_TEXT_CHARS = 500;
const MAX_IMAGE_BYTES = 5 * 1024 * 1024;
const MAX_AUDIO_BYTES = 3 * 1024 * 1024;
const MAX_AUDIO_SECONDS = 15;
const MIN_OCR_CONFIDENCE = 0.75;
const RATE_LIMIT_WINDOW_MS = 60 * 1000;
const RATE_LIMIT_MAX = 10;

const rateBuckets = new Map();

const FIELD_DEFINITIONS = Object.freeze({
  systolic: {
    labels: ["高压", "收缩压", "上压", "SYS"],
    min: 40,
    max: 260,
  },
  diastolic: {
    labels: ["低压", "舒张压", "下压", "DIA"],
    min: 30,
    max: 180,
  },
  pulse: {
    labels: ["心率", "脉搏", "PUL", "bpm"],
    min: 30,
    max: 220,
  },
});

function emptyField(reason = "missing_label") {
  return {value: null, confidence: 0, reason};
}

function valueField(value, confidence = 0.92, reason = "matched_label") {
  return {value, confidence, reason};
}

function emptyCandidate() {
  return {
    systolic: emptyField(),
    diastolic: emptyField(),
    pulse: emptyField(),
  };
}

function parseRecognizedText(text) {
  if (typeof text !== "string" || text.trim() === "" || [...text].length > MAX_TEXT_CHARS) {
    throw new Error("recognized text is invalid");
  }
  const candidate = emptyCandidate();
  for (const field of Object.keys(FIELD_DEFINITIONS)) {
    candidate[field] = parseField(text, field);
  }
  return {candidate, needsConfirmation: true};
}

function parseField(text, field) {
  const definition = FIELD_DEFINITIONS[field];
  const values = new Set();
  for (const label of definition.labels) {
    const escaped = label.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
    const pattern = new RegExp(`${escaped}[：:\\s]*(?:是|为|约|大概)?[：:\\s]*([一二三四五六七八九十百零〇两\\d]{2,5})`, "gi");
    for (const match of text.matchAll(pattern)) {
      const value = parseNumber(match[1]);
      if (value !== null) values.add(value);
    }
  }
  if (values.size === 0) return emptyField("missing_label");
  if (values.size > 1) return emptyField("conflict");
  const value = [...values][0];
  if (value < definition.min || value > definition.max) return emptyField("out_of_bounds");
  return valueField(value);
}

function parseNumber(raw) {
  if (/^\d+$/.test(raw)) return Number(raw);
  if (raw.includes("百")) {
    const [hundredsRaw, restRaw = ""] = raw.split("百");
    const hundreds = parseSmallChineseNumber(hundredsRaw || "一");
    if (hundreds === null) return null;
    if (restRaw === "") return hundreds * 100;
    if (/^[一二两三四五六七八九]$/.test(restRaw)) {
      return hundreds * 100 + parseSmallChineseNumber(restRaw) * 10;
    }
    const rest = parseSmallChineseNumber(restRaw);
    return rest === null ? null : hundreds * 100 + rest;
  }
  return parseSmallChineseNumber(raw);
}

function parseSmallChineseNumber(raw) {
  const digits = new Map([
    ["零", 0],
    ["〇", 0],
    ["一", 1],
    ["二", 2],
    ["两", 2],
    ["三", 3],
    ["四", 4],
    ["五", 5],
    ["六", 6],
    ["七", 7],
    ["八", 8],
    ["九", 9],
  ]);
  let total = 0;
  let current = 0;
  for (const char of raw) {
    if (char === "百") {
      total += (current || 1) * 100;
      current = 0;
    } else if (char === "十") {
      total += (current || 1) * 10;
      current = 0;
    } else if (digits.has(char)) {
      current = digits.get(char);
    } else {
      return null;
    }
  }
  const value = total + current;
  return value === 0 ? null : value;
}

function parseOcrBoxes(boxes) {
  if (!Array.isArray(boxes)) throw new Error("ocr boxes are invalid");
  const candidate = emptyCandidate();
  for (const field of Object.keys(FIELD_DEFINITIONS)) {
    candidate[field] = parseOcrField(boxes, field);
  }
  return {candidate, needsConfirmation: true};
}

function parseOcrField(boxes, field) {
  const definition = FIELD_DEFINITIONS[field];
  const values = new Set();
  let lowConfidence = false;
  for (const box of boxes) {
    const text = String(box.text || "");
    if (!definition.labels.some((label) => text.toUpperCase().includes(label.toUpperCase()))) continue;
    if (isLowConfidence(box)) {
      lowConfidence = true;
      continue;
    }
    const nearby = boxes
      .filter((candidate) => candidate !== box)
      .filter((candidate) => Math.abs(Number(candidate.y) - Number(box.y)) <= 30)
      .filter((candidate) => Number(candidate.x) >= Number(box.x))
      .sort((a, b) => Number(a.x) - Number(b.x));
    for (const candidate of nearby) {
      if (isLowConfidence(candidate)) {
        lowConfidence = true;
        continue;
      }
      const value = parseNumber(String(candidate.text || "").replace(/[^\d一二三四五六七八九十百零〇两]/g, ""));
      if (value !== null && value >= definition.min && value <= definition.max) {
        values.add(value);
        break;
      }
    }
  }
  if (lowConfidence) return emptyField("low_confidence");
  if (values.size === 0) return emptyField("missing_or_low_confidence");
  if (values.size > 1) return emptyField("conflict");
  return valueField([...values][0], 0.86, "matched_ocr_label");
}

function isLowConfidence(box) {
  if (box.confidence === undefined || box.confidence === null) return false;
  const confidence = Number(box.confidence);
  return !Number.isFinite(confidence) || confidence < MIN_OCR_CONFIDENCE;
}

function validateMedia(event) {
  const purpose = event && event.purpose;
  if (purpose !== "voice" && purpose !== "photo" && purpose !== "text" && purpose !== "ocr-fixture") {
    throw new Error("purpose is invalid");
  }
  if (purpose === "voice") {
    if (!event.file || !String(event.file.contentType || "").startsWith("audio/")) throw new Error("audio type is invalid");
    const sizeBytes = Number(event.file.sizeBytes);
    if (!Number.isFinite(sizeBytes) || sizeBytes <= 0) throw new Error("audio size is invalid");
    if (sizeBytes > MAX_AUDIO_BYTES) throw new Error("audio is too large");
    const durationSeconds = Number(event.file.durationSeconds);
    if (!Number.isFinite(durationSeconds) || durationSeconds <= 0) throw new Error("audio duration is invalid");
    if (durationSeconds > MAX_AUDIO_SECONDS) throw new Error("audio is too long");
  }
  if (purpose === "photo") {
    if (!event.file || !String(event.file.contentType || "").startsWith("image/")) throw new Error("image type is invalid");
    const sizeBytes = Number(event.file.sizeBytes);
    if (!Number.isFinite(sizeBytes) || sizeBytes <= 0) throw new Error("image size is invalid");
    if (sizeBytes > MAX_IMAGE_BYTES) throw new Error("image is too large");
  }
}

async function withTrustedMediaMetadata(event, storage, userKey) {
  const purpose = event && event.purpose;
  if (purpose !== "voice" && purpose !== "photo") return event;
  const fileID = event.file && event.file.fileID;
  if (!fileID) throw new Error("fileID is required");
  if (!String(fileID).includes("/bp-entry/")) throw new Error("media namespace is invalid");
  if (!storage || typeof storage.getMetadata !== "function") throw new Error("media metadata is required");
  const metadata = await storage.getMetadata(fileID);
  if (!metadata) throw new Error("media metadata is required");
  if (!metadataBelongsToUser(metadata, userKey)) throw new Error("media owner is invalid");
  return {
    ...event,
    file: {
      ...event.file,
      contentType: metadata.contentType,
      sizeBytes: metadata.sizeBytes,
      durationSeconds: Number.isFinite(Number(metadata.durationSeconds)) && Number(metadata.durationSeconds) > 0 ? metadata.durationSeconds : event.file.durationSeconds,
    },
  };
}

function metadataBelongsToUser(metadata, userKey) {
  if (!userKey) return false;
  if (metadata.ownerKey && metadata.ownerKey === userKey) return true;
  if (metadata.ownerHash && metadata.ownerHash === hashUserKey(userKey)) return true;
  return false;
}

function hashUserKey(userKey) {
  return crypto.createHash("sha256").update(String(userKey)).digest("hex");
}

function checkRateLimit(userKey, now = Date.now()) {
  if (!userKey) throw new Error("user is required");
  const bucket = rateBuckets.get(userKey) || [];
  const recent = bucket.filter((timestamp) => now - timestamp < RATE_LIMIT_WINDOW_MS);
  if (recent.length >= RATE_LIMIT_MAX) throw new Error("rate limit exceeded");
  recent.push(now);
  rateBuckets.set(userKey, recent);
}

function createFakeProvider() {
  return {
    async transcribe(event) {
      return event.fakeRecognizedText || "";
    },
    async ocr(event) {
      return event.fakeOcrBoxes || [];
    },
  };
}

function createTencentProviderFromEnv(env = process.env) {
  const asrEndpoint = env.TENCENT_ASR_ENDPOINT;
  const ocrEndpoint = env.TENCENT_OCR_ENDPOINT;
  const authToken = env.TENCENT_PROVIDER_AUTH_TOKEN;
  const httpClient = env.__httpClient || fetch;
  if (!asrEndpoint || !ocrEndpoint || !authToken || typeof httpClient !== "function") {
    throw new Error("Tencent provider is not configured");
  }
  async function postJSON(endpoint, event) {
    const response = await httpClient(endpoint, {
      method: "POST",
      headers: {
        authorization: `Bearer ${authToken}`,
        "content-type": "application/json",
      },
      body: JSON.stringify({
        fileID: event.file && event.file.fileID,
        contentType: event.file && event.file.contentType,
        sizeBytes: event.file && event.file.sizeBytes,
        durationSeconds: event.file && event.file.durationSeconds,
      }),
    });
    if (!response || !response.ok) throw new Error("Tencent provider request failed");
    return response.json();
  }
  return {
    async transcribe(event) {
      const result = await postJSON(asrEndpoint, event);
      return result.recognizedText || "";
    },
    async ocr(event) {
      const result = await postJSON(ocrEndpoint, event);
      return result.ocrBoxes || [];
    },
  };
}

function createProvider(dependencies = {}) {
  if (dependencies.provider) return dependencies.provider;
  const mode = dependencies.providerMode || process.env.MEDIA_PROVIDER_MODE || "fake";
  if (mode === "fake") return createFakeProvider();
  if (mode === "tencent" || mode === "real") return createTencentProviderFromEnv(dependencies.env || process.env);
  throw new Error("media provider mode is invalid");
}

function getTrustedUserKey(event, dependencies = {}) {
  if (dependencies.identity) return dependencies.identity;
  if (dependencies.getWXContext) {
    const context = dependencies.getWXContext();
    if (context && (context.OPENID || context.FROM_OPENID)) return context.OPENID || context.FROM_OPENID;
  }
  const runtimeContext = getCloudBaseContext();
  if (runtimeContext && (runtimeContext.OPENID || runtimeContext.FROM_OPENID)) {
    return runtimeContext.OPENID || runtimeContext.FROM_OPENID;
  }
  if ((dependencies.appEnv || process.env.APP_ENV) === "local" && event && event.syntheticLocalUserKey) {
    return event.syntheticLocalUserKey;
  }
  throw new Error("trusted identity is required");
}

function getCloudBaseContext() {
  try {
    const cloud = require("wx-server-sdk");
    if (cloud && cloud.init) cloud.init();
    if (cloud && cloud.getWXContext) return cloud.getWXContext();
  } catch (error) {
    return null;
  }
  return null;
}

async function deleteWithRetry(storage, fileID, metrics) {
  if (!storage || !fileID) return;
  for (let attempt = 1; attempt <= 2; attempt += 1) {
    try {
      await storage.deleteFile(fileID);
      metrics.cleanupDeleted += 1;
      return;
    } catch (error) {
      metrics.cleanupFailed += 1;
    }
  }
}

function createStorage(dependencies = {}) {
  if (dependencies.storage) return dependencies.storage;
  try {
    const cloud = require("wx-server-sdk");
    if (cloud && cloud.init) cloud.init();
    if (cloud && cloud.deleteFile && cloud.getTempFileURL && typeof fetch === "function") {
      return {
        async deleteFile(fileID) {
          await cloud.deleteFile({fileList: [fileID]});
        },
        async getMetadata(fileID) {
          const result = await cloud.getTempFileURL({fileList: [fileID]});
          const file = result && result.fileList && result.fileList[0];
          if (!file || file.status !== 0 || !file.tempFileURL) throw new Error("media metadata is unavailable");
          const response = await fetch(file.tempFileURL, {method: "HEAD"});
          if (!response || !response.ok) throw new Error("media metadata is unavailable");
          return {
            contentType: response.headers.get("content-type") || "",
            sizeBytes: Number(response.headers.get("content-length")),
            durationSeconds: Number(file.durationSeconds),
            ownerKey: file.openid || file.ownerOpenID || file.ownerKey,
            ownerHash: file.ownerHash,
          };
        },
      };
    }
  } catch (error) {
    return null;
  }
  return null;
}

async function main(event = {}, dependencies = {}) {
  const metrics = {attempted: 0, cleanupDeleted: 0, cleanupFailed: 0};
  metrics.attempted += 1;
  const storage = createStorage(dependencies);
  let cleanupFileID = "";
  try {
    const userKey = getTrustedUserKey(event, dependencies);
    const trustedEvent = await withTrustedMediaMetadata(event, storage, userKey);
    cleanupFileID = trustedEvent.file && trustedEvent.file.fileID ? trustedEvent.file.fileID : "";
    validateMedia(trustedEvent);
    checkRateLimit(userKey, dependencies.now ? dependencies.now() : Date.now());
    const provider = createProvider(dependencies);
    if (trustedEvent.purpose === "text") {
      return {...parseRecognizedText(trustedEvent.recognizedText), metrics};
    }
    if (trustedEvent.purpose === "ocr-fixture") {
      return {...parseOcrBoxes(trustedEvent.ocrBoxes), metrics};
    }
    if (trustedEvent.purpose === "voice") {
      const text = await provider.transcribe(trustedEvent);
      return {...parseRecognizedText(text), metrics};
    }
    const boxes = await provider.ocr(trustedEvent);
    return {...parseOcrBoxes(boxes), metrics};
  } finally {
    await deleteWithRetry(storage, cleanupFileID, metrics);
  }
}

module.exports = {
  main,
  parseRecognizedText,
  parseOcrBoxes,
  validateMedia,
  checkRateLimit,
  createFakeProvider,
  createProvider,
  getTrustedUserKey,
  createTencentProviderFromEnv,
  createStorage,
  _private: {parseNumber, rateBuckets, isLowConfidence, withTrustedMediaMetadata, hashUserKey},
};
