import type { ArtifactPropertyType } from "@/shared/api/models";

/**
 * A property TYPE definition — the reusable half of a property (name + data type +
 * config). The set of these is the artifact "schema" vocabulary; an artifact carries
 * property *values* drawn from it. `options` holds select choices, or the known tag
 * values (for autocomplete) of a `tags` type. Data-layer so the repo can derive it.
 */
export interface PropertyDef {
  key: string;
  type: ArtifactPropertyType;
  options?: string[];
}

/** Merge lists of definitions, deduped by lowercased key; options/known-values are unioned. */
export function mergePropertyDefs(...lists: PropertyDef[][]): PropertyDef[] {
  const byKey = new Map<string, PropertyDef>();
  for (const list of lists) {
    for (const def of list) {
      const id = def.key.trim().toLowerCase();
      if (!id) continue;
      const prev = byKey.get(id);
      byKey.set(id, {
        key: prev?.key ?? def.key,
        type: def.type,
        options: dedupe([...(prev?.options ?? []), ...(def.options ?? [])]),
      });
    }
  }
  return [...byKey.values()].map((d) => ({
    ...d,
    options: d.options && d.options.length > 0 ? d.options : undefined,
  }));
}

function dedupe(values: string[]): string[] {
  return [...new Set(values.map((v) => v.trim()).filter(Boolean))];
}
