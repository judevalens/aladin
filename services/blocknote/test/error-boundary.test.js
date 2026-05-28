// node --test. Verifies the error boundary catches both synchronous throws
// and async rejections (routing them to next() instead of crashing the
// process) and that the error handler maps status codes correctly.

import { test } from "node:test";
import assert from "node:assert";
import { wrap, errorHandler } from "../src/middleware/error-boundary.js";
import { BadRequest } from "../src/errors.js";

function mockRes() {
  return {
    statusCode: 200,
    body: undefined,
    headersSent: false,
    status(code) {
      this.statusCode = code;
      return this;
    },
    json(payload) {
      this.body = payload;
      this.headersSent = true;
      return this;
    },
  };
}

const flush = () => new Promise((resolve) => setImmediate(resolve));

test("wrap forwards synchronous throws to next (no crash)", async () => {
  let forwarded;
  const handler = wrap(() => {
    throw new Error("sync boom");
  });
  handler({}, mockRes(), (err) => {
    forwarded = err;
  });
  await flush();
  assert.equal(forwarded?.message, "sync boom");
});

test("wrap forwards async rejections to next (no crash)", async () => {
  let forwarded;
  const handler = wrap(async () => {
    throw new Error("async boom");
  });
  handler({}, mockRes(), (err) => {
    forwarded = err;
  });
  await flush();
  assert.equal(forwarded?.message, "async boom");
});

test("wrap leaves successful handlers alone", async () => {
  let nextCalled = false;
  const res = mockRes();
  const handler = wrap(async (_req, r) => {
    r.json({ ok: true });
  });
  handler({}, res, () => {
    nextCalled = true;
  });
  await flush();
  assert.equal(nextCalled, false);
  assert.deepEqual(res.body, { ok: true });
});

test("errorHandler maps BadRequest to 400", () => {
  const res = mockRes();
  errorHandler(new BadRequest("bad input"), { method: "POST", path: "/x" }, res);
  assert.equal(res.statusCode, 400);
  assert.equal(res.body.error, "bad input");
});

test("errorHandler maps generic errors to 500", () => {
  const res = mockRes();
  errorHandler(new Error("kaboom"), { method: "POST", path: "/x" }, res);
  assert.equal(res.statusCode, 500);
  assert.equal(res.body.error, "kaboom");
});

test("errorHandler is a no-op when headers already sent", () => {
  const res = mockRes();
  res.headersSent = true;
  res.statusCode = 200;
  errorHandler(new Error("late"), { method: "POST", path: "/x" }, res);
  assert.equal(res.statusCode, 200); // unchanged
});
