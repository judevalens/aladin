// Pure pieces of the board sync server — no sockets, no Postgres.
import { test } from "node:test";
import assert from "node:assert";
import {
  parseLegacyContent,
  roomSnapshotToArtifactContent,
  sanitizeRoomId,
} from "../src/services/board-sync.js";

test("room ids never traverse paths", () => {
  assert.equal(sanitizeRoomId("art_01ABC-def"), "art_01ABC-def");
  assert.equal(sanitizeRoomId("../../../etc/passwd"), "_________etc_passwd");
  assert.equal(sanitizeRoomId("a b/c"), "a_b_c");
});

test("legacy content parses to its document, and junk to null", () => {
  const doc = { store: { "shape:a": { id: "shape:a" } }, schema: { schemaVersion: 2 } };
  assert.deepEqual(parseLegacyContent(JSON.stringify({ document: doc, session: {} })), doc);
  assert.equal(parseLegacyContent(""), null);
  assert.equal(parseLegacyContent("   "), null);
  assert.equal(parseLegacyContent("{not json"), null);
  assert.equal(parseLegacyContent(JSON.stringify({ nothing: true })), null);
  // A schema-less document cannot seed a room (nor be migrated) — treated as no seed.
  assert.equal(
    parseLegacyContent(JSON.stringify({ document: { store: {} }, session: {} })),
    null,
  );
});

test("a room snapshot round-trips through artifact content", () => {
  const snapshot = {
    documents: [
      { state: { id: "shape:a", typeName: "shape" }, lastChangedClock: 4 },
      { state: { id: "page:page", typeName: "page" }, lastChangedClock: 1 },
    ],
    schema: { schemaVersion: 2 },
  };
  const content = roomSnapshotToArtifactContent(snapshot);
  const document = parseLegacyContent(content);
  assert.deepEqual(Object.keys(document.store).sort(), ["page:page", "shape:a"]);
  assert.deepEqual(document.schema, { schemaVersion: 2 });
});
