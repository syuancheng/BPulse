"use strict";

const assert = require("node:assert/strict");
const test = require("node:test");
const {main} = require("../src/index");

test("Phase 0 worker skeleton performs no send", async () => {
  const result = await main();
  assert.deepEqual(result, {
    code: "NOT_IMPLEMENTED",
    mode: "fake",
  });
});
