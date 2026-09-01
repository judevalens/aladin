import Ajv2020 from "ajv/dist/2020";
import type { ValidateFunction } from "ajv";
import type { Schema } from "./types";

// The v2 sandbox explicitly permits AJV runtime compilation. Do not mutate,
// coerce, default, or remotely load user data/schemas.
const ajv = new Ajv2020({ strict: false, validateFormats: false, allErrors: false, ownProperties: true });
const compiled = new WeakMap<Schema, ValidateFunction>();

export function validateWithSchema(schema: Schema, value: unknown): void {
  let validate = compiled.get(schema);
  if (!validate) {
    validate = ajv.compile(schema);
    compiled.set(schema, validate);
    // AJV's own cache is strong. Resource schemas must be collectable when
    // their final consumer detaches; the WeakMap owns reuse instead.
    ajv.removeSchema(schema);
  }
  if (!validate(value)) throw new Error(ajv.errorsText(validate.errors));
}
