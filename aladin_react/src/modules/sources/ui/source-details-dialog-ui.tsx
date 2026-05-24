import { Trash2 } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogBody,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Textarea } from "@/components/ui/textarea";
import { FactCard, Pill } from "@/modules/sources/ui/sources-parts-ui";
import type {
  SourceDetailsDialogProps,
  SourcesFormatters,
} from "@/modules/sources/ui/sources-ui-types";

export function SourceDetailsDialog({
  selectedSource,
  removeSourcePending,
  onSelectSource,
  onRemoveSelectedSource,
  formatters,
}: SourceDetailsDialogProps & { formatters: SourcesFormatters }) {
  return (
    <Dialog
      open={Boolean(selectedSource)}
      onOpenChange={(open) => !open && onSelectSource(null)}
    >
      <DialogContent className="w-[min(92vw,760px)]">
        {selectedSource ? (
          <>
            <DialogHeader>
              <DialogTitle>{selectedSource.name}</DialogTitle>
              <DialogDescription>
                {formatters.descriptionLine(selectedSource)}
              </DialogDescription>
            </DialogHeader>
            <DialogBody className="space-y-5 pb-4">
              <div className="flex flex-wrap gap-2">
                <Badge>{formatters.healthLabel(selectedSource)}</Badge>
                <Pill>{formatters.healthLabel(selectedSource)}</Pill>
                <Pill>{formatters.lastRefreshSummary(selectedSource)}</Pill>
              </div>
              <div className="grid gap-4 md:grid-cols-2">
                {formatters.formatSourceFacts(selectedSource).map((fact) => (
                  <FactCard key={fact.label} label={fact.label} value={fact.value} />
                ))}
              </div>
              <div className="border-t border-[#e7e5e4] pt-4">
                <div className="eyebrow">Config snapshot</div>
                <Textarea
                  className="mt-3 min-h-[180px] font-mono text-xs"
                  readOnly
                  value={JSON.stringify(selectedSource.config, null, 2)}
                />
              </div>
            </DialogBody>
            <DialogFooter>
              <Button
                variant="destructive"
                onClick={onRemoveSelectedSource}
                disabled={removeSourcePending}
              >
                <Trash2 className="h-4 w-4" />
                Remove source
              </Button>
            </DialogFooter>
          </>
        ) : null}
      </DialogContent>
    </Dialog>
  );
}
