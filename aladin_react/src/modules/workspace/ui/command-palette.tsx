import { FilePlus2, FolderPlus, Link2, Mic, Upload } from "lucide-react";
import {
  CommandDialog,
  CommandEmpty,
  CommandGroup,
  CommandInput,
  CommandItem,
  CommandList,
} from "@/components/ui/command";

export interface CommandPaletteActions {
  onCreateFolder: () => void;
  onCreateNote: () => void;
  onCreateLink: () => void;
  onCreateVoice: () => void;
  onCreateFile: () => void;
}

/**
 * Minimal ⌘K command palette. For now it only exposes the create actions; navigation
 * and ask-my-graph land in later phases.
 */
export function CommandPalette({
  open,
  onOpenChange,
  actions,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  actions: CommandPaletteActions;
}) {
  const run = (fn: () => void) => {
    onOpenChange(false);
    fn();
  };

  return (
    <CommandDialog open={open} onOpenChange={onOpenChange}>
      <CommandInput placeholder="Type a command…" />
      <CommandList>
        <CommandEmpty>No commands found.</CommandEmpty>
        <CommandGroup heading="Create">
          <CommandItem value="new note" onSelect={() => run(actions.onCreateNote)}>
            <FilePlus2 className="h-[15px] w-[15px] text-ink-3" strokeWidth={1.75} />
            New note
          </CommandItem>
          <CommandItem value="new folder" onSelect={() => run(actions.onCreateFolder)}>
            <FolderPlus className="h-[15px] w-[15px] text-ink-3" strokeWidth={1.75} />
            New folder
          </CommandItem>
          <CommandItem value="new link" onSelect={() => run(actions.onCreateLink)}>
            <Link2 className="h-[15px] w-[15px] text-ink-3" strokeWidth={1.75} />
            New link
          </CommandItem>
          <CommandItem value="new voice note" onSelect={() => run(actions.onCreateVoice)}>
            <Mic className="h-[15px] w-[15px] text-ink-3" strokeWidth={1.75} />
            New voice note
          </CommandItem>
          <CommandItem value="upload file" onSelect={() => run(actions.onCreateFile)}>
            <Upload className="h-[15px] w-[15px] text-ink-3" strokeWidth={1.75} />
            Upload file
          </CommandItem>
        </CommandGroup>
      </CommandList>
    </CommandDialog>
  );
}
