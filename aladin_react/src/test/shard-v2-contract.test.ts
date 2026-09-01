import { describe, expect, it } from "vitest";
import corpus from "../../../shared/shard-v2/fixtures/validation.json";
import { compileContract, decodeJSON, projectSchema, validateData, validateEvent, validateProtocol, validateQuery, validateSchema } from "@/modules/doc-surface/data/contract";
import type { Query, Registry, Resource, Schema } from "@/modules/doc-surface/data/types";

describe("shared Shard v2 validation fixtures", () => {
  for (const fixture of corpus.cases) {
    it(fixture.kind + ": " + fixture.name, () => {
      const run = () => {
        const value = decodeJSON(JSON.stringify(fixture.value));
        const resource = ("resource" in fixture ? fixture.resource : null) as Resource;
        switch (fixture.kind) {
          case "contract": return compileContract(value, corpus.providers as Registry);
          case "schema": return validateSchema(value as Schema);
          case "data": return validateData(("schema" in fixture ? fixture.schema : {}) as Schema, value);
          case "query": return validateQuery(resource, value as Query);
          case "event": return validateEvent(value, resource, resource.schema);
          default: return validateProtocol(fixture.kind as "command" | "bridge-request", value);
        }
      };
      if (fixture.valid) expect(run).not.toThrow();
      else expect(run).toThrow();
    });
  }
});

it("projection preserves nested required fields without exposing siblings", () => {
  const schema = {
    type: "object",
    properties: {
      profile: { type: "object", properties: { name: { type: "string" }, secret: { type: "string" } }, required: ["name", "secret"] },
    },
    required: ["profile"],
  };
  const output = projectSchema(schema, ["/profile/name"]);
  expect(() => validateData(output, { profile: { name: "Ada" } })).not.toThrow();
  expect(() => validateData(output, { profile: { name: "Ada", secret: "hidden" } })).toThrow();
  expect(() => validateData(output, { profile: {} })).toThrow();
  expect(() => validateData(output, {})).toThrow();
});
