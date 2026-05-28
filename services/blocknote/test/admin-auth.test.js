// Hermetic test for the /admin/* shared-secret guard. No DB / server needed.

import { test } from "node:test";
import assert from "node:assert";
import { requireAdminSecret } from "../src/middleware/admin-auth.js";

function fakeReq(secret) {
  return {
    get: (h) => (h.toLowerCase() === "x-admin-secret" ? secret : undefined),
  };
}

function fakeRes() {
  const res = { statusCode: 0, body: null };
  res.status = (code) => {
    res.statusCode = code;
    return res;
  };
  res.json = (body) => {
    res.body = body;
    return res;
  };
  return res;
}

test("no configured secret → 503, next not called (fail closed)", () => {
  const res = fakeRes();
  let nexted = false;
  requireAdminSecret("")(fakeReq("anything"), res, () => {
    nexted = true;
  });
  assert.equal(res.statusCode, 503);
  assert.equal(nexted, false);
});

test("wrong secret → 401, next not called", () => {
  const res = fakeRes();
  let nexted = false;
  requireAdminSecret("right")(fakeReq("wrong"), res, () => {
    nexted = true;
  });
  assert.equal(res.statusCode, 401);
  assert.equal(nexted, false);
});

test("missing header → 401", () => {
  const res = fakeRes();
  let nexted = false;
  requireAdminSecret("right")(fakeReq(undefined), res, () => {
    nexted = true;
  });
  assert.equal(res.statusCode, 401);
  assert.equal(nexted, false);
});

test("matching secret → next() called, no status written", () => {
  const res = fakeRes();
  let nexted = false;
  requireAdminSecret("right")(fakeReq("right"), res, () => {
    nexted = true;
  });
  assert.equal(nexted, true);
  assert.equal(res.statusCode, 0);
});
