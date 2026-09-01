import profileSchema from "../../../../../shared/shard-v2/schemas/data-schema.json";
import { validateWithSchema } from "./schema-validation";
import { LIMITS } from "./types";
import type { Schema } from "./types";

export const object = (v: unknown): v is Record<string, unknown> => v !== null && typeof v === "object" && !Array.isArray(v);
export const own = (o: object, key: string) => Object.hasOwn(o, key);
export function requireThat(ok: unknown, message: string): asserts ok { if (!ok) throw new Error(message); }

export function safeJSON(value: unknown, depth = 0): void {
  requireThat(depth <= LIMITS.jsonDepth, "JSON depth exceeded");
  if (value === null || typeof value === "boolean" || typeof value === "string") return;
  if (typeof value === "number") {
    requireThat(Number.isFinite(value) && (!Number.isInteger(value) || Number.isSafeInteger(value)), "unsafe JSON number");
    return;
  }
  requireThat(Array.isArray(value) || object(value), "not a JSON value");
  requireThat(Array.isArray(value) || Object.getPrototypeOf(value) === Object.prototype || Object.getPrototypeOf(value) === null, "not a plain JSON object");
  if (Array.isArray(value)) requireThat(Object.keys(value).length === value.length, "sparse or extended JSON array");
  for (const child of Object.values(value)) safeJSON(child, depth + 1);
}
export function jsonKey(value: unknown): string {
  safeJSON(value);
  const encode = (v: unknown): string => {
    if (Array.isArray(v)) return "[" + v.map(encode).join(",") + "]";
    if (object(v)) return "{" + Object.keys(v).sort().map(key => JSON.stringify(key) + ":" + encode(v[key])).join(",") + "}";
    return JSON.stringify(v);
  };
  return encode(value);
}
export function decodeJSON(source: string): unknown {
  requireThat(new TextEncoder().encode(source).byteLength <= LIMITS.jsonBytes, "JSON byte limit exceeded");
  const value: unknown = JSON.parse(source);
  safeJSON(value);
  return value;
}
const keywords = new Set("$schema $defs $ref type properties required additionalProperties items enum const minimum maximum exclusiveMinimum exclusiveMaximum minLength maxLength minItems maxItems title description format".split(" "));
export const escapePointer = (key: string) => key.replaceAll("~", "~0").replaceAll("/", "~1");
export function pointerParts(pointer: string): string[] {
  if (pointer === "") return [];
  requireThat(pointer.startsWith("/") && !/~(?:[^01]|$)/u.test(pointer), "invalid JSON Pointer");
  return pointer.slice(1).split("/").map(part => part.replaceAll("~1", "/").replaceAll("~0", "~"));
}
function deref(root: Schema, schema: Schema): Schema {
  while (typeof schema.$ref === "string") {
    let value: unknown = root;
    for (const part of pointerParts(schema.$ref.slice(1))) {
      requireThat(object(value) && own(value, part), "unresolved reference");
      value = value[part];
    }
    requireThat(object(value), "reference is not a schema");
    schema = value;
  }
  return schema;
}
export const types = (schema: Schema): string[] => typeof schema.type === "string" ? [schema.type] : Array.isArray(schema.type) ? schema.type as string[] : [];
export function validateSchema(root: Schema): void {
  safeJSON(root);
  const nodes = new Map<string, Schema>();
  function collect(schema: Schema, path: string): void {
    requireThat(nodes.size < 1024, "schema exceeds 1024 nodes");
    nodes.set(path, schema);
    for (const key of Object.keys(schema)) requireThat(keywords.has(key), path + ": unsupported schema keyword " + key);
    if (own(schema, "$schema")) requireThat(schema.$schema === "https://json-schema.org/draft/2020-12/schema", "unsupported schema draft");
    if (own(schema, "$ref")) {
      for (const key of Object.keys(schema)) requireThat(["$ref", "$defs", "$schema", "title", "description", "format"].includes(key), "$ref supports annotation siblings only");
    }
    for (const key of ["properties", "$defs"]) {
      if (!own(schema, key)) continue;
      const children = schema[key];
      requireThat(object(children), key + " must be an object");
      for (const [name, child] of Object.entries(children)) {
        requireThat(object(child), "expected schema object");
        collect(child, path + "/" + key + "/" + escapePointer(name));
      }
    }
    for (const key of ["items", "additionalProperties"]) {
      if (!own(schema, key)) continue;
      const child = schema[key];
      if (object(child)) collect(child, path + "/" + key);
      else requireThat(key === "additionalProperties" && typeof child === "boolean", "expected schema object");
    }
  }
  collect(root, "#");
  const active = new Set<string>(), done = new Set<string>();
  function visit(path: string): void {
    requireThat(!active.has(path), "recursive schema");
    if (done.has(path)) return;
    const schema = nodes.get(path);
    requireThat(schema, "unresolved local schema reference");
    active.add(path);
    if (own(schema, "$ref")) {
      requireThat(typeof schema.$ref === "string" && schema.$ref.startsWith("#/"), "only local JSON Pointer references supported");
      visit(schema.$ref);
    }
    for (const child of nodes.keys()) if (child !== path && child.startsWith(path + "/")) visit(child);
    active.delete(path); done.add(path);
  }
  visit("#");
  requireThat(deref(root, root).type === "object", "schema root must declare object type");
  validateWithSchema(profileSchema, root);
}
export function fieldSchema(root: Schema, pointer: string): Schema {
  let schema = root;
  for (const part of pointerParts(pointer)) {
    schema = deref(root, schema);
    requireThat(types(schema).includes("object") && object(schema.properties) && own(schema.properties, part), "field is not explicitly declared: " + pointer);
    const child = schema.properties[part];
    requireThat(object(child), "expected field schema");
    schema = child;
  }
  return deref(root, schema);
}
export function scalarSchema(root: Schema, pointer: string): Schema {
  const schema = fieldSchema(root, pointer), fieldTypes = types(schema);
  requireThat(fieldTypes.length && fieldTypes.every(t => ["string", "number", "integer", "boolean", "null"].includes(t)), "field must declare scalar types");
  return schema;
}
export function projectSchema(root: Schema, selection: string[] = []): Schema {
  if (!selection.length) return root;
  const result: Schema = { type: "object", properties: Object.create(null), additionalProperties: false };
  if (root.$defs) result.$defs = root.$defs;
  for (const [index, pointer] of selection.entries()) {
    requireThat(!selection.some((other, j) => j !== index && (other === pointer || pointer.startsWith(other + "/"))), "overlapping projection fields");
    fieldSchema(root, pointer);
    const parts = pointerParts(pointer);
    let src = deref(root, root), dst = result;
    for (const [i, part] of parts.entries()) {
      const child = deref(root, (src.properties as Record<string, Schema>)[part]);
      const properties = dst.properties as Record<string, Schema>;
      if (Array.isArray(src.required) && src.required.includes(part)) {
        const required = (dst.required ?? []) as string[];
        if (!required.includes(part)) required.push(part);
        dst.required = required;
      }
      if (i === parts.length - 1) { properties[part] = child; break; }
      if (!own(properties, part)) properties[part] = { type: child.type, properties: Object.create(null), additionalProperties: false };
      src = child; dst = properties[part];
    }
  }
  return result;
}

export function validateData(schema: Schema, value: unknown): void {
  safeJSON(value);
  validateWithSchema(schema, value);
}
