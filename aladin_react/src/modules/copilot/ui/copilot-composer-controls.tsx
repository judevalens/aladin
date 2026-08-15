import { Check, ChevronDown, Plus } from "lucide-react";
import { Icon } from "@/components/ui/icon";
import type { CopilotEffortOption, CopilotModelOption } from "@/repos/copilot/copilot-repo";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover";
import type { ScopeSummary } from "@/modules/copilot/ui/copilot-surface";

/**
 * The controls that sit on the composer: which model, how much effort, and what the turn is
 * scoped to. Presentational — each takes its options and a callback, and none of them know
 * where the selection is stored.
 */

export function EffortSwitcher({
  efforts,
  activeEffort,
  defaultEffort,
  onSelect,
}: {
  efforts: CopilotEffortOption[];
  activeEffort: string | null;
  defaultEffort: string | null;
  onSelect: (effort: string) => void;
}) {
  const active = efforts.find((effort) => effort.id === activeEffort);
  const label = active?.label ?? activeEffort ?? "Effort";
  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <button
          type="button"
          className="flex max-w-[76px] items-center gap-1 rounded-chip px-1.5 py-0.5 font-mono text-meta text-ink-3 hover:bg-raise hover:text-ink"
          aria-label="Copilot effort"
          title={active?.description ?? active?.id ?? "Copilot effort"}
        >
          <span className="min-w-0 truncate">{label}</span>
          <Icon as={ChevronDown} size="inline" mark />
        </button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="start" className="w-72">
        <DropdownMenuLabel>Effort</DropdownMenuLabel>
        {efforts.length === 0 ? (
          <DropdownMenuItem disabled>Backend default</DropdownMenuItem>
        ) : (
          efforts.map((effort) => (
            <DropdownMenuItem key={effort.id} onSelect={() => onSelect(effort.id)} className="items-start gap-2">
              <span className="mt-0.5 grid size-4 shrink-0 place-items-center text-amber">
                {effort.id === activeEffort ? <Icon as={Check} size="inline" mark /> : null}
              </span>
              <span className="min-w-0 flex-1">
                <span className="flex items-center gap-1.5 text-small text-ink">
                  {effort.label}
                  {effort.id === defaultEffort ? (
                    <span className="font-mono text-meta text-ink-4">default</span>
                  ) : null}
                </span>
                {effort.description ? (
                  <span className="mt-0.5 block text-meta leading-snug text-ink-4">{effort.description}</span>
                ) : null}
              </span>
            </DropdownMenuItem>
          ))
        )}
      </DropdownMenuContent>
    </DropdownMenu>
  );
}

export function ModelSwitcher({
  models,
  activeModel,
  defaultModel,
  onSelect,
}: {
  models: CopilotModelOption[];
  activeModel: string | null;
  defaultModel: string | null;
  onSelect: (model: string) => void;
}) {
  const active = models.find((model) => model.id === activeModel);
  const label = active?.label ?? activeModel ?? "Model";
  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <button
          type="button"
          className="flex max-w-[92px] items-center gap-1 rounded-chip px-1.5 py-0.5 font-mono text-meta text-ink-3 hover:bg-raise hover:text-ink"
          aria-label="Copilot model"
          title={active?.description ?? active?.id ?? "Copilot model"}
        >
          <span className="min-w-0 truncate">{label}</span>
          <Icon as={ChevronDown} size="inline" mark />
        </button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="start" className="w-72">
        <DropdownMenuLabel>Model</DropdownMenuLabel>
        {models.length === 0 ? (
          <DropdownMenuItem disabled>Backend default</DropdownMenuItem>
        ) : (
          models.map((model) => (
            <DropdownMenuItem
              key={model.id}
              onSelect={() => onSelect(model.id)}
              className="items-start gap-2"
            >
              <span className="mt-0.5 grid size-4 shrink-0 place-items-center text-amber">
                {model.id === activeModel ? <Icon as={Check} size="inline" mark /> : null}
              </span>
              <span className="min-w-0 flex-1">
                <span className="flex items-center gap-1.5 text-small text-ink">
                  {model.label}
                  {model.id === defaultModel ? (
                    <span className="font-mono text-meta text-ink-4">default</span>
                  ) : null}
                </span>
                {model.description ? (
                  <span className="mt-0.5 block text-meta leading-snug text-ink-4">{model.description}</span>
                ) : null}
              </span>
            </DropdownMenuItem>
          ))
        )}
      </DropdownMenuContent>
    </DropdownMenu>
  );
}

export function ScopeChip({
  scope,
  onNewThread,
}: {
  scope: ScopeSummary;
  onNewThread: () => void;
}) {
  return (
    <div className="border-b border-line/60 px-2.5 py-1.5">
      <Popover>
        <PopoverTrigger asChild>
          <button
            type="button"
            className="flex max-w-full items-center gap-1.5 rounded-chip px-1.5 py-0.5 text-left transition-colors hover:bg-raise"
            aria-label={`Current scope: ${scope.title}`}
          >
            <Icon as={scope.icon} size="inline" mark className="shrink-0 text-amber" />
            <span className="shrink-0 font-mono text-meta text-ink-4">asking about</span>
            <span className="min-w-0 truncate font-mono text-meta text-ink-2">{scope.title}</span>
            <Icon as={ChevronDown} size="inline" mark className="shrink-0 text-ink-4" />
          </button>
        </PopoverTrigger>
        <PopoverContent className="w-72 p-3">
          <div className="flex items-start gap-2">
            <Icon as={scope.icon} mark className="mt-0.5 shrink-0 text-amber" />
            <div className="min-w-0 flex-1">
              <p className="truncate text-small font-semibold text-ink">{scope.title}</p>
              <p className="font-mono text-meta text-ink-4">{scope.kind}</p>
            </div>
          </div>
          {scope.rows.length > 0 ? (
            <dl className="mt-2 space-y-1 rounded-card border border-line bg-field p-2">
              {scope.rows.map((row) => (
                <div key={row.label} className="grid grid-cols-[64px_minmax(0,1fr)] gap-2 font-mono text-meta">
                  <dt className="text-ink-4">{row.label}</dt>
                  <dd className="truncate text-ink-2" title={row.value}>
                    {row.value}
                  </dd>
                </div>
              ))}
            </dl>
          ) : null}
          <button
            type="button"
            onClick={onNewThread}
            className="mt-2 inline-flex items-center gap-1.5 rounded-chip border border-line px-2.5 py-1 text-small text-ink-2 transition-colors hover:border-amber-line hover:text-ink"
          >
            <Icon as={Plus} size="inline" mark />
            New chat
          </button>
        </PopoverContent>
      </Popover>
    </div>
  );
}
