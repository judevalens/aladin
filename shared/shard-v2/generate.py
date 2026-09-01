#!/usr/bin/env python3
"""Generate protocol schemas and Go embeds. No network or third-party packages."""
import hashlib
import json
from pathlib import Path
import subprocess

HERE = Path(__file__).resolve().parent
ROOT = HERE.parent.parent
DRAFT = "https://json-schema.org/draft/2020-12/schema"
ID = {"type": "string", "pattern": "^[A-Za-z][A-Za-z0-9_-]{0,63}$"}
TOKEN = {"type": "string", "minLength": 1, "maxLength": 256}
POINTER = {"type": "string", "pattern": "^(?:/(?:[^~/]|~[01])*)+$", "maxLength": 1024}
OPS = ["snapshot", "query", "insert", "update", "delete"]
CAPS = OPS + ["observe"]


def obj(properties, required=(), **extra):
    return {"type": "object", "properties": properties, "required": list(required), "additionalProperties": False, **extra}


def arr(items, **extra):
    return {"type": "array", "items": items, **extra}


def enum(values):
    return {"enum": values}


def ref(name):
    return {"$ref": "#/$defs/" + name}


def document(body, definitions=None):
    return {"$schema": DRAFT, **body, **({"$defs": definitions} if definitions else {})}


def schemas():
    scalar = {"type": ["string", "number", "boolean", "null"]}
    predicate = {"oneOf": [
        obj({"and": arr(ref("predicate"), minItems=1, maxItems=32)}, ["and"]),
        obj({"or": arr(ref("predicate"), minItems=1, maxItems=32)}, ["or"]),
        obj({"field": POINTER, "op": enum(["eq", "gt", "gte", "lt", "lte"]), "value": scalar}, ["field", "op", "value"]),
        obj({"field": POINTER, "op": {"const": "in"}, "value": arr(scalar, minItems=1, maxItems=100)}, ["field", "op", "value"]),
        obj({"field": POINTER, "op": {"const": "exists"}, "value": {"type": "boolean"}}, ["field", "op", "value"]),
    ]}
    query = obj({
        "where": ref("predicate"),
        "orderBy": arr(obj({"field": POINTER, "direction": enum(["asc", "desc"])}, ["field", "direction"]), maxItems=32),
        "limit": {"type": "integer", "minimum": 1, "maximum": 500},
        "cursor": {"type": ["string", "null"], "minLength": 1, "maxLength": 4096},
    })
    capability_list = arr(enum(CAPS), uniqueItems=True, maxItems=len(CAPS))
    parameter = {"oneOf": [
        obj({"input": ID}, ["input"]),
        obj({"binding": ID, "pointer": {"type": "string", "pattern": "^/data(?:/(?:[^~/]|~[01])*)*$"}}, ["binding", "pointer"]),
        obj({"literal": {}}, ["literal"]),
        {"type": ["string", "number", "boolean", "null", "array"]},
        {"type": "object", "not": {"anyOf": [{"required": [k]} for k in ["input", "binding", "literal"]]}},
    ]}
    resource = obj({
        "uri": {"type": "string", "pattern": "^shard://self/resources/[A-Za-z][A-Za-z0-9_-]{0,63}$"},
        "kind": enum(["collection", "singleton"]), "meaning": {"type": "string", "minLength": 1},
        "schemaVersion": {"type": "integer", "minimum": 1, "maximum": 9007199254740991},
        "schema": {"type": "object"},
        "source": obj({"provider": TOKEN, "version": {"const": 1}, "dataset": ID, "params": {"type": "object"}}, ["provider"]),
        "operations": arr(enum(OPS), uniqueItems=True, minItems=1, maxItems=5),
        "observe": obj({"mode": {"const": "changes"}, "protocol": {"const": "shard-data/1"}}, ["mode", "protocol"]),
        "exposure": obj({"app": capability_list, "agent": capability_list}),
        "query": obj({"filterFields": arr(POINTER, uniqueItems=True, maxItems=64), "sortFields": arr(POINTER, uniqueItems=True, maxItems=64), "maxLimit": {"type": "integer", "minimum": 1, "maximum": 500}}),
    }, ["uri", "kind", "meaning", "schemaVersion", "schema", "source", "operations"])
    binding = obj({"resource": ID, "params": {"type": "object", "additionalProperties": parameter}, "inputsSchema": {"type": "object"}, "query": query, "select": arr(POINTER, minItems=1, uniqueItems=True, maxItems=64)}, ["resource"])
    contract = document(obj({"version": {"const": 2}, "intent": {"type": "string", "minLength": 1}, "resources": {"type": "object", "propertyNames": ID, "additionalProperties": resource, "minProperties": 1, "maxProperties": 32}, "bindings": {"type": "object", "propertyNames": ID, "additionalProperties": binding, "maxProperties": 128}}, ["version", "intent", "resources", "bindings"]), {"predicate": predicate})
    record = obj({"id": TOKEN, "revision": TOKEN, "schemaVersion": {"type": "integer", "minimum": 1, "maximum": 9007199254740991}, "data": {"type": "object"}}, ["id", "revision", "schemaVersion", "data"])
    routing = {"protocol": {"const": "shard-data/1"}, "subscriptionId": TOKEN, "resource": {"type": "string", "pattern": "^shard://[A-Za-z0-9_-]+/resources/[A-Za-z][A-Za-z0-9_-]{0,63}\\?view=[A-Za-z0-9_-]+$", "maxLength": 1024}, "epoch": TOKEN, "seq": {"type": "string", "pattern": "^(0|[1-9][0-9]*)$", "maxLength": 128}}
    event = document({"oneOf": [
        obj({**routing, "sourceUpdatedAt": TOKEN, "op": {"const": "snapshot"}, "records": arr(record, maxItems=500), "complete": {"const": True}, "nextCursor": TOKEN}, [*routing, "op", "records", "complete"]),
        obj({**routing, "sourceUpdatedAt": TOKEN, "op": enum(["insert", "update"]), "record": record}, [*routing, "op", "record"]),
        obj({**routing, "sourceUpdatedAt": TOKEN, "op": {"const": "delete"}, "id": TOKEN, "revision": TOKEN, "reason": enum(["deleted", "left-view"])}, [*routing, "op", "id", "reason"]),
    ]})
    command_base = {"resource": ID, "requestId": TOKEN, "contractHash": TOKEN}
    commands = []
    for op in ["insert", "update", "delete"]:
        props = {**command_base, "op": {"const": op}, "id": TOKEN}
        required = [*command_base, "op"]
        if op != "delete":
            props["data"] = {"type": "object"}
            required += ["data"]
        if op != "insert":
            props["baseRevision"] = TOKEN
            required += ["id", "baseRevision"]
        commands.append(obj(props, required))
    command = document({"oneOf": commands})
    requests = []
    binding_params = {"binding": ID, "inputs": {"type": "object"}}
    for method in ["hello", "theme.get", "resource.describe", "resource.read", "resource.query", "resource.subscribe", "resource.unsubscribe", "resource.insert", "resource.update", "resource.delete"]:
        params, required = {}, []
        if method == "resource.unsubscribe":
            params, required = {"subscriptionId": TOKEN}, ["subscriptionId"]
        elif method.startswith("resource."):
            params, required = dict(binding_params), ["binding"]
            if method == "resource.query":
                params["query"] = query
                required += ["query"]
            if method == "resource.read":
                params["id"] = TOKEN
            op = method.removeprefix("resource.")
            if op in ["insert", "update", "delete"]:
                params.update({"id": TOKEN, "requestId": TOKEN})
                required += ["requestId"]
                if op != "delete":
                    params["data"] = {"type": "object"}
                    required += ["data"]
                if op != "insert":
                    params["baseRevision"] = TOKEN
                    required += ["id", "baseRevision"]
        requests.append(obj({"aladin": {"const": "bridge/2"}, "type": {"const": "request"}, "id": {"type": "integer", "minimum": 0, "maximum": 9007199254740991}, "method": {"const": method}, "params": obj(params, required)}, ["aladin", "type", "id", "method", "params"]))
    schema_node = obj({
        "$schema": {"const": DRAFT}, "$ref": {"type": "string", "pattern": "^#/"},
        "$defs": {"type": "object", "additionalProperties": ref("node")},
        "type": {"oneOf": [enum(["object", "array", "string", "number", "integer", "boolean", "null"]), arr(enum(["object", "array", "string", "number", "integer", "boolean", "null"]), minItems=1, uniqueItems=True)]},
        "properties": {"type": "object", "additionalProperties": ref("node")},
        "required": arr({"type": "string"}, uniqueItems=True),
        "additionalProperties": {"oneOf": [{"type": "boolean"}, ref("node")]},
        "items": ref("node"), "enum": arr({}, minItems=1), "const": {},
        **{key: {"type": "number"} for key in ["minimum", "maximum", "exclusiveMinimum", "exclusiveMaximum"]},
        **{key: {"type": "integer", "minimum": 0} for key in ["minLength", "maxLength", "minItems", "maxItems"]},
        **{key: {"type": "string"} for key in ["title", "description", "format"]},
    })
    descriptor = obj({
        "kind": enum(["collection", "singleton"]), "schemaVersion": {"type": "integer", "minimum": 1, "maximum": 9007199254740991},
        "schema": {"type": "object"}, "capabilities": capability_list,
        "observation": enum(["ordered-changes", "refresh-snapshots"]),
        "delivery": enum(["deltas", "snapshots"]),
        "limit": {"type": "integer", "minimum": 1, "maximum": 500},
    }, ["kind", "schemaVersion", "schema", "capabilities", "delivery", "limit"])
    identity = obj({key: routing[key] for key in ["subscriptionId", "resource", "epoch"]}, ["subscriptionId", "resource", "epoch"])
    snapshot = obj({"resource": routing["resource"], "records": arr(record, maxItems=500), "complete": {"const": True}, "nextCursor": TOKEN, "sourceUpdatedAt": TOKEN}, ["resource", "records", "complete"])
    response = {"oneOf": [
        obj({"aladin": {"const": "bridge/2"}, "type": {"const": "response"}, "id": {"type": "integer", "minimum": 0, "maximum": 9007199254740991}, "ok": {"const": True}, "data": {}}, ["aladin", "type", "id", "ok", "data"]),
        obj({"aladin": {"const": "bridge/2"}, "type": {"const": "response"}, "id": {"type": "integer", "minimum": 0, "maximum": 9007199254740991}, "ok": {"const": False}, "code": TOKEN, "error": {"type": "string"}, "data": {}}, ["aladin", "type", "id", "ok", "code", "error"]),
    ]}
    return {"contract": contract, "query": document(query, {"predicate": predicate}), "record": document(record), "event": event, "command": command, "bridge-request": document({"oneOf": requests}, {"predicate": predicate}), "bridge-response": document(response), "descriptor": document(descriptor), "subscription": document(identity), "snapshot": document(snapshot), "data-schema": document(ref("node"), {"node": schema_node})}


def main():
    directory = HERE / "schemas"
    directory.mkdir(parents=True, exist_ok=True)
    all_schemas = schemas()
    # Host transport context wraps, but never changes, the iframe request.
    bridge = all_schemas["bridge-request"]
    all_schemas["host-request"] = document(obj({
        "target": obj({"shardId": TOKEN, "environment": enum(["draft", "published"]), "contractHash": TOKEN}, ["shardId", "environment", "contractHash"]),
        "request": {"oneOf": bridge["oneOf"]},
    }, ["target", "request"]), bridge["$defs"])
    for name, schema in all_schemas.items():
        (directory / f"{name}.json").write_text(json.dumps(schema, indent=2, ensure_ascii=False) + "\n")
    output = ROOT / "backend_v2/internal/shardv2/protocol_schemas_generated.go"
    output.parent.mkdir(parents=True, exist_ok=True)
    go = '// Code generated by shared/shard-v2/generate.py; DO NOT EDIT.\npackage shardv2\n\nvar protocolSchemas = map[string]string{\n'
    for name in sorted(all_schemas):
        go += f'"{name}": ' + chr(96) + json.dumps(all_schemas[name], separators=(",", ":")) + chr(96) + ',\n'
    output.write_text(go + '}\n')
    subprocess.run(["gofmt", "-w", str(output)], check=True)
    paths = sorted([*directory.glob("*.json"), *(HERE / "fixtures").glob("*.json")])
    manifest = {"version": "1.0.0", "files": {str(p.relative_to(HERE)): hashlib.sha256(p.read_bytes()).hexdigest() for p in paths}}
    (HERE / "manifest.json").write_text(json.dumps(manifest, indent=2) + "\n")


if __name__ == "__main__":
    main()
