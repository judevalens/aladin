import type { ArtifactProperty } from "@/shared/api/models";
import type { PropertyDef } from "@/repos/artifacts/property-defs";

export type { PropertyDef };

/** The 5 generic presets that seed the type set. Trading-flavored ones come later. */
export const PRESET_PROPERTY_DEFS: PropertyDef[] = [
  { key: "Status", type: "select", options: ["Idea", "Researching", "Validated", "Live", "Archived"] },
  { key: "Tags", type: "tags" },
  { key: "Priority", type: "select", options: ["High", "Medium", "Low"] },
  { key: "Date", type: "date" },
  { key: "Link", type: "url" },
];

/** Instantiate an empty property value from a type definition. */
export function defToProperty(def: PropertyDef): ArtifactProperty {
  return {
    key: def.key,
    type: def.type,
    value: "",
    values: def.type === "tags" ? [] : undefined,
    options: def.options,
  };
}
