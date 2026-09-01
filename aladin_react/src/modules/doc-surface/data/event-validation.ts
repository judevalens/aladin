import eventSchema from "../../../../../shared/shard-v2/schemas/event.json";
import { validateWithSchema } from "./schema-validation";
import { safeJSON, requireThat, validateData } from "./schema-profile";
import { LIMITS } from "./types";
import type { Resource, ResourceEvent, Schema } from "./types";

export function validateEvent(value: unknown, resource: Pick<Resource, "kind" | "schemaVersion">, output: Schema): ResourceEvent {
  safeJSON(value);
  validateWithSchema(eventSchema, value);
  requireThat(new TextEncoder().encode(JSON.stringify(value)).byteLength <= LIMITS.jsonBytes, "event exceeds byte limit");
  const event = value as ResourceEvent;
  const records = event.op === "snapshot" ? event.records : "record" in event ? [event.record] : [];
  requireThat(resource.kind !== "singleton" || records.length <= 1, "singleton has multiple records");
  const seen = new Set<string>();
  for (const record of records) {
    requireThat(!seen.has(record.id), "duplicate snapshot record"); seen.add(record.id);
    requireThat(resource.kind !== "singleton" || record.id === "value", "invalid singleton ID");
    requireThat(record.schemaVersion === resource.schemaVersion, "schema version mismatch");
    requireThat(new TextEncoder().encode(JSON.stringify(record.data)).byteLength <= LIMITS.recordBytes, "record exceeds byte limit");
    validateData(output, record.data);
  }
  requireThat(resource.kind !== "singleton" || event.op !== "delete" || event.id === "value", "invalid singleton ID");
  return event;
}
