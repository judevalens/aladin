import { useCallback, useEffect, useMemo, useState } from "react";
import { Plus, X } from "lucide-react";
import { useAppComposition } from "@/app/composition/app-composition";
import {
  Command,
  CommandGroup,
  CommandInput,
  CommandItem,
  CommandList,
} from "@/components/ui/command";
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover";
import { cn } from "@/lib/utils";
import type { Artifact, ArtifactProperty, ArtifactPropertyType } from "@/shared/api/models";
import {
  PRESET_PROPERTY_DEFS,
  defToProperty,
} from "@/modules/artifacts/ui/artifact-property-presets";
import { mergePropertyDefs, type PropertyDef } from "@/repos/artifacts/property-defs";

// Types you can create ad-hoc from the "New … as" row. `tags` is intentionally
// excluded — tag properties come only from a preset/template, not free creation.
const CREATABLE_TYPES: ArtifactPropertyType[] = ["text", "number", "date", "select", "url"];
const GHOST = "min-w-0 bg-transparent text-[13px] outline-none placeholder:text-ink-4";
const TILE = "min-w-0 rounded-md bg-field px-2 py-0.5 text-[13px] text-ink outline-none";

function SectionLabel({ children }: { children: React.ReactNode }) {
  return (
    <h3 className="mb-1.5 px-1.5 font-mono text-[11px] uppercase tracking-[0.14em] text-ink-4">
      {children}
    </h3>
  );
}

/** Multi-chip tag value with autocomplete over known values; Enter creates a new tag. */
function TagsField({
  values,
  known,
  onChange,
}: {
  values: string[];
  known: string[];
  onChange: (values: string[]) => void;
}) {
  const [draft, setDraft] = useState("");
  const add = (raw: string) => {
    const tag = raw.trim();
    if (tag && !values.includes(tag)) onChange([...values, tag]);
    setDraft("");
  };
  const suggestions = draft.trim()
    ? known
        .filter((k) => k.toLowerCase().includes(draft.toLowerCase()) && !values.includes(k))
        .slice(0, 6)
    : [];
  return (
    <div className="relative flex flex-1 flex-wrap items-center gap-1">
      {values.map((tag) => (
        <span
          key={tag}
          className="inline-flex items-center gap-1 rounded-chip border border-line bg-raise px-1.5 py-0.5 text-[11px] text-ink-2"
        >
          {tag}
          <button
            type="button"
            aria-label={`Remove ${tag}`}
            onClick={() => onChange(values.filter((t) => t !== tag))}
            className="text-ink-4 hover:text-against"
          >
            <X className="size-2.5" strokeWidth={2.5} />
          </button>
        </span>
      ))}
      <input
        className={cn(GHOST, "w-16 flex-1 text-ink")}
        placeholder={values.length ? "" : "Add tags"}
        value={draft}
        onChange={(event) => setDraft(event.target.value)}
        onBlur={() => add(draft)}
        onKeyDown={(event) => {
          if ((event.key === "Enter" || event.key === ",") && draft.trim()) {
            event.preventDefault();
            add(draft);
          } else if (event.key === "Backspace" && !draft && values.length) {
            onChange(values.slice(0, -1));
          }
        }}
      />
      {suggestions.length > 0 ? (
        <div className="absolute left-0 top-full z-20 mt-1 w-44 overflow-hidden rounded-md border border-line bg-panel py-1 shadow-panel">
          {suggestions.map((s) => (
            <button
              key={s}
              type="button"
              // onMouseDown (not onClick) so it fires before the input's onBlur.
              onMouseDown={(event) => {
                event.preventDefault();
                add(s);
              }}
              className="block w-full px-2.5 py-1 text-left text-[12px] text-ink-2 hover:bg-raise"
            >
              {s}
            </button>
          ))}
        </div>
      ) : null}
    </div>
  );
}

function ValueTile({
  property,
  onChange,
}: {
  property: ArtifactProperty;
  onChange: (value: string) => void;
}) {
  if (property.type === "select") {
    return (
      <select
        value={property.value}
        onChange={(event) => onChange(event.target.value)}
        className={cn(TILE, "w-fit")}
      >
        <option value="">—</option>
        {(property.options ?? []).map((opt) => (
          <option key={opt} value={opt}>
            {opt}
          </option>
        ))}
      </select>
    );
  }
  const textLike = property.type === "text" || property.type === "url";
  return (
    <input
      // `field-sizing:content` hugs content on Chromium; `size` covers WKWebView/Safari
      // for text/url. number/date keep their compact native width. Nothing stretches.
      className={cn(TILE, "w-auto max-w-full [field-sizing:content]")}
      type={property.type === "number" ? "number" : property.type === "date" ? "date" : "text"}
      inputMode={property.type === "number" ? "decimal" : undefined}
      placeholder={property.type === "url" ? "https://…" : "Empty"}
      size={textLike ? Math.max(property.value.length + 1, 5) : undefined}
      value={property.value}
      onChange={(event) => onChange(event.target.value)}
    />
  );
}

function PropertyRow({
  property,
  known,
  onChange,
  onRemove,
}: {
  property: ArtifactProperty;
  known: string[];
  onChange: (patch: Partial<ArtifactProperty>) => void;
  onRemove: () => void;
}) {
  return (
    <div className="group flex items-center gap-2 rounded-md px-1.5 py-1 transition-colors hover:bg-raise">
      {/* Free name, bold. The value's type is fixed at creation. */}
      <span className="w-[34%] shrink-0 truncate text-[13px] font-semibold text-ink">
        {property.key || <span className="font-normal text-ink-4">Untitled</span>}
      </span>
      {property.type === "tags" ? (
        <TagsField
          values={property.values ?? []}
          known={known}
          onChange={(values) => onChange({ values })}
        />
      ) : (
        <ValueTile property={property} onChange={(value) => onChange({ value })} />
      )}
      <button
        type="button"
        aria-label={`Remove ${property.key || "property"}`}
        onClick={onRemove}
        className="ml-auto grid size-5 shrink-0 place-items-center rounded text-ink-4 opacity-0 transition-opacity hover:text-against group-hover:opacity-100"
      >
        <X className="size-3.5" strokeWidth={2} />
      </button>
    </div>
  );
}

/**
 * Add-property popover. A property = a **free name** + one of the 6 unique data
 * **types**. You type a name; matching templates (presets + already-used) are
 * shortcuts; the "New '<name>' as" row creates a custom-named property of the type
 * you pick. Enter on a novel name → a `text` property.
 */
function AddProperty({
  defs,
  existingKeys,
  onOpen,
  onAdd,
}: {
  defs: PropertyDef[];
  existingKeys: string[];
  onOpen: () => void;
  onAdd: (property: ArtifactProperty) => void;
}) {
  const [open, setOpen] = useState(false);
  const [name, setName] = useState("");

  const reset = () => {
    setOpen(false);
    setName("");
  };

  const trimmed = name.trim();
  const available = defs.filter((d) => !existingKeys.includes(d.key.toLowerCase()));
  const filtered = trimmed
    ? available.filter((d) => d.key.toLowerCase().includes(trimmed.toLowerCase()))
    : available;
  const canCreate = trimmed.length > 0 && !existingKeys.includes(trimmed.toLowerCase());

  const addTemplate = (def: PropertyDef) => {
    onAdd(defToProperty(def));
    reset();
  };
  const addCustom = (type: ArtifactPropertyType) => {
    if (!trimmed) return;
    onAdd(defToProperty({ key: trimmed, type }));
    reset();
  };

  return (
    <Popover
      open={open}
      onOpenChange={(next) => {
        if (next) {
          onOpen();
          setOpen(true);
        } else {
          reset();
        }
      }}
    >
      <PopoverTrigger asChild>
        <button
          type="button"
          className="mt-1 flex w-full items-center gap-1.5 rounded-md px-1.5 py-1.5 text-[13px] text-ink-4 transition-colors hover:bg-raise hover:text-ink-2"
        >
          <Plus className="size-[15px]" strokeWidth={1.75} />
          Add property
        </button>
      </PopoverTrigger>
      <PopoverContent align="start" className="w-72 p-0">
        <Command shouldFilter={false}>
          <CommandInput
            value={name}
            onValueChange={setName}
            placeholder="Name a property…"
            onKeyDown={(event) => {
              // Enter on a novel name with no template matches → quick text property.
              if (event.key === "Enter" && canCreate && filtered.length === 0) {
                event.preventDefault();
                addCustom("text");
              }
            }}
          />
          {filtered.length > 0 ? (
            <CommandList>
              <CommandGroup heading="Templates">
                {filtered.map((def) => (
                  <CommandItem key={def.key} value={def.key} onSelect={() => addTemplate(def)}>
                    <span className="flex-1 truncate">{def.key}</span>
                    <span className="text-[11px] text-ink-4">{def.type}</span>
                  </CommandItem>
                ))}
              </CommandGroup>
            </CommandList>
          ) : null}
          {canCreate ? (
            <div className="border-t border-line p-2">
              <div className="mb-1.5 px-0.5 text-[11px] text-ink-4">
                New “{trimmed}” as
              </div>
              <div className="flex flex-wrap gap-1">
                {CREATABLE_TYPES.map((t) => (
                  <button
                    key={t}
                    type="button"
                    onClick={() => addCustom(t)}
                    className="rounded-md border border-line bg-field px-2 py-0.5 text-[12px] text-ink-2 transition-colors hover:bg-raise hover:text-ink"
                  >
                    {t}
                  </button>
                ))}
              </div>
            </div>
          ) : (
            <p className="px-3 py-3 text-center text-[12px] text-ink-4">
              {available.length ? "Type a name to add or create." : "All properties added."}
            </p>
          )}
        </Command>
      </PopoverContent>
    </Popover>
  );
}

/**
 * The property section of the "On the graph" pane: typed frontmatter. Reads off the
 * reactive `artifact` (artifactById stream); writes via the Rust-routed
 * updateArtifactProperties — which emits a frame, so every view updates with no reload.
 */
export function PropertiesSection({ artifact }: { artifact: Artifact }) {
  const { services } = useAppComposition();
  const [rows, setRows] = useState<ArtifactProperty[]>(artifact.properties ?? []);
  const [defs, setDefs] = useState<PropertyDef[]>(PRESET_PROPERTY_DEFS);

  // Re-seed rows when switching artifacts (local edits are authoritative within one).
  useEffect(() => {
    setRows(artifact.properties ?? []);
  }, [artifact.id]); // eslint-disable-line react-hooks/exhaustive-deps

  const loadDefs = useCallback(async () => {
    try {
      const learned = await services.workspace.listPropertyDefs();
      setDefs(mergePropertyDefs(PRESET_PROPERTY_DEFS, learned));
    } catch {
      setDefs(PRESET_PROPERTY_DEFS);
    }
  }, [services]);

  useEffect(() => {
    void loadDefs();
  }, [loadDefs]);

  function persist(next: ArtifactProperty[]) {
    setRows(next);
    void services.workspace.updateArtifactProperties(
      artifact.id,
      next.filter((p) => p.key.trim().length > 0),
    );
  }

  const knownByKey = useMemo(() => {
    const map = new Map<string, string[]>();
    for (const def of defs) map.set(def.key.toLowerCase(), def.options ?? []);
    return map;
  }, [defs]);

  const existingKeys = rows.map((r) => r.key.trim().toLowerCase()).filter(Boolean);

  // Default/known named properties not yet on this artifact — one-click quick-adds.
  // Presets sort first (mergePropertyDefs keeps that order).
  const suggestions = defs
    .filter((d) => !existingKeys.includes(d.key.toLowerCase()))
    .slice(0, 6);

  return (
    <section>
      <SectionLabel>Properties</SectionLabel>
      <div className="flex flex-col">
        {rows.map((property, index) => (
          <PropertyRow
            key={index}
            property={property}
            known={knownByKey.get(property.key.toLowerCase()) ?? []}
            onChange={(patch) => persist(rows.map((r, i) => (i === index ? { ...r, ...patch } : r)))}
            onRemove={() => persist(rows.filter((_, i) => i !== index))}
          />
        ))}
      </div>
      {suggestions.length > 0 ? (
        <div className="mt-1.5 flex flex-wrap gap-1 px-1.5">
          {suggestions.map((def) => (
            <button
              key={def.key}
              type="button"
              onClick={() => persist([...rows, defToProperty(def)])}
              className="inline-flex items-center gap-1 rounded-chip border border-dashed border-line px-2 py-0.5 text-[11px] text-ink-3 transition-colors hover:border-line-2 hover:text-ink-2"
            >
              <Plus className="size-2.5" strokeWidth={2.5} />
              {def.key}
            </button>
          ))}
        </div>
      ) : null}
      <AddProperty
        defs={defs}
        existingKeys={existingKeys}
        onOpen={() => void loadDefs()}
        onAdd={(property) => persist([...rows, property])}
      />
    </section>
  );
}
