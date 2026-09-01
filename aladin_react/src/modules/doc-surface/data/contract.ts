import bridge_responseSchema from "../../../../../shared/shard-v2/schemas/bridge-response.json";
import descriptorSchema from "../../../../../shared/shard-v2/schemas/descriptor.json";
import subscriptionSchema from "../../../../../shared/shard-v2/schemas/subscription.json";
import snapshotSchema from "../../../../../shared/shard-v2/schemas/snapshot.json";
import { safeJSON, validateData, validateSchema, scalarSchema, projectSchema, fieldSchema, object, own, requireThat, escapePointer, types } from "./schema-profile";
export { safeJSON, decodeJSON, validateData, validateSchema, projectSchema, fieldSchema } from "./schema-profile";
import { validateWithSchema } from "./schema-validation";
export { validateEvent } from "./event-validation";
import contractSchema from "../../../../../shared/shard-v2/schemas/contract.json";
import querySchema from "../../../../../shared/shard-v2/schemas/query.json";
import eventSchema from "../../../../../shared/shard-v2/schemas/event.json";
import recordSchema from "../../../../../shared/shard-v2/schemas/record.json";
import commandSchema from "../../../../../shared/shard-v2/schemas/command.json";
import bridgeSchema from "../../../../../shared/shard-v2/schemas/bridge-request.json";
import { LIMITS } from "./types";
import type { CompiledContract, Contract, Predicate, Query, Registry, Resource, Schema } from "./types";

const schemas = { "bridge-response": bridge_responseSchema, "descriptor": descriptorSchema, "subscription": subscriptionSchema, "snapshot": snapshotSchema, contract: contractSchema, query: querySchema, event: eventSchema, record: recordSchema, command: commandSchema, "bridge-request": bridgeSchema };
export function validateProtocol(name: keyof typeof schemas, value: unknown): void {
  safeJSON(value);
  validateWithSchema(schemas[name], value);
}
export function validateQuery(resource: Resource, query: Query): void {
  validateProtocol("query", query);
  requireThat((query.limit ?? LIMITS.defaultLimit) <= (resource.query?.maxLimit ?? LIMITS.maxLimit), "query exceeds resource limit");
  if (query.where || query.orderBy?.length) requireThat(resource.operations.includes("query"), "unsupported query capability");
  const seen = new Set<string>();
  for (const order of query.orderBy ?? []) {
    requireThat(!seen.has(order.field) && resource.query?.sortFields?.includes(order.field), "invalid sort field");
    seen.add(order.field); scalarSchema(resource.schema, order.field);
  }
  let count = 0;
  function walk(p: Predicate, depth: number): void {
    requireThat(depth <= 8, "query depth exceeds 8");
    if ("and" in p || "or" in p) { for (const child of "and" in p ? p.and : p.or) walk(child, depth + 1); return; }
    requireThat(++count <= 32 && resource.query?.filterFields?.includes(p.field), "invalid filter or predicate limit exceeded");
    const schema = scalarSchema(resource.schema, p.field);
    if (p.op === "exists") return;
    for (const value of p.op === "in" ? p.value : [p.value]) {
      if (["gt", "gte", "lt", "lte"].includes(p.op)) requireThat(typeof value === "number" && types(schema).every(t => ["number", "integer", "null"].includes(t)), "range field and value must be numeric");
      validateData(schema, value);
    }
  }
  if (query.where) walk(query.where, 1);
}
function validateParams(schema: Schema, params: Record<string, unknown>, dynamic: Set<string>, complete: boolean): void {
  const partial = structuredClone(schema);
  const properties = object(partial.properties) ? partial.properties : {};
  for (const field of dynamic) requireThat(own(properties, field), "unknown provider parameter " + field);
  partial.required = complete && Array.isArray(partial.required) ? partial.required.filter(name => !dynamic.has(name as string)) : [];
  validateData(partial, params);
}
export function compileContract(value: unknown, providers: Registry): CompiledContract {
  validateProtocol("contract", value);
  const contract = value as Contract;
  const datasets = new Set<string>();
  for (const [id, resource] of Object.entries(contract.resources)) {
    requireThat(resource.uri === "shard://self/resources/" + id, "URI must match resource ID");
    validateSchema(resource.schema);
    requireThat(own(providers, resource.source.provider), "unknown provider");
    const provider = providers[resource.source.provider];
    requireThat(!resource.source.version || resource.source.version === provider.version, "unknown provider version");
    requireThat(resource.operations.includes("snapshot") && resource.operations.every(op => provider.operations.includes(op)), "unsupported capability");
    requireThat(!resource.observe || provider.observation, "provider cannot observe");
    if (provider.owned) {
      requireThat(resource.source.dataset && !datasets.has(resource.source.dataset), "owned dataset missing or already declared");
      datasets.add(resource.source.dataset);
    } else requireThat(!resource.source.dataset, "external provider cannot select dataset");
    const declared: string[] = [...resource.operations, ...(resource.observe ? ["observe"] : [])];
    for (const cap of [...resource.exposure?.app ?? [], ...resource.exposure?.agent ?? []]) requireThat(declared.includes(cap), "exposure exceeds declared capabilities");
    const fields = [...resource.query?.filterFields ?? [], ...resource.query?.sortFields ?? []];
    for (const pointer of fields) scalarSchema(resource.schema, pointer);
    requireThat(!fields.length || resource.operations.includes("query"), "query fields require query capability");
    validateParams(provider.paramsSchema, resource.source.params ?? {}, new Set(), false);
  }
  const outputSchemas: Record<string, Schema> = Object.create(null);
  const dependencies = new Map<string, string[]>();
  for (const [id, binding] of Object.entries(contract.bindings)) {
    requireThat(own(contract.resources, binding.resource), "unknown resource");
    const resource = contract.resources[binding.resource];
    if (binding.inputsSchema) validateSchema(binding.inputsSchema);
    if (binding.query) validateQuery(resource, binding.query);
    outputSchemas[id] = projectSchema(resource.schema, binding.select);
    const params: Record<string, unknown> = Object.assign(Object.create(null), resource.source.params);
    const dynamic = new Set<string>(), deps: string[] = [];
    for (const [name, value] of Object.entries(binding.params ?? {})) {
      if (object(value)) {
        if (own(value, "literal")) { params[name] = value.literal; continue; }
        if (typeof value.input === "string") {
          requireThat(binding.inputsSchema, "missing inputsSchema");
          fieldSchema(binding.inputsSchema, "/" + escapePointer(value.input));
          dynamic.add(name); delete params[name]; continue;
        }
        if (typeof value.binding === "string") {
          requireThat(own(contract.bindings, value.binding), "unknown dependency");
          const dep = contract.bindings[value.binding], depResource = contract.resources[dep.resource];
          requireThat(depResource?.kind === "singleton", "binding dependency must be singleton");
          fieldSchema(projectSchema(depResource.schema, dep.select), String(value.pointer).slice(5));
          deps.push(value.binding); dynamic.add(name); delete params[name]; continue;
        }
      }
      params[name] = value;
    }
    validateParams(providers[resource.source.provider].paramsSchema, params, dynamic, true);
    dependencies.set(id, deps);
  }
  const bindingOrder: string[] = [], active = new Set<string>(), done = new Set<string>();
  function visit(id: string): void {
    requireThat(!active.has(id), "binding dependency cycle");
    if (done.has(id)) return;
    active.add(id);
    for (const dependency of (dependencies.get(id) ?? []).sort()) visit(dependency);
    active.delete(id); done.add(id); bindingOrder.push(id);
  }
  for (const id of Object.keys(contract.bindings).sort()) visit(id);
  return { contract, bindingOrder, outputSchemas };
}
