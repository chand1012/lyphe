import { PaperclipIcon, MoreHorizontalIcon } from "lucide-react";
import { toast } from "sonner";
import { Button } from "~/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "~/components/ui/dropdown-menu";
import { update, type Block, type Entity } from "~/lib/data";

type FileBlock = Extract<Block, { type: "file" }>;

export function FileRow({ block, entity }: { block: FileBlock; entity: Entity }) {
  const safeUrl = typeof window !== "undefined" && block.url?.startsWith(`blob:${window.location.origin}/`) ? block.url : undefined;
  const remove = () => {
    update(entity.kind, entity.id, { blocks: entity.blocks.filter((b) => b !== block) });
    toast("Attachment removed");
  };

  return (
    <div className="flex items-center gap-2 py-1 text-sm">
      <PaperclipIcon className="size-4 shrink-0 text-muted-foreground" />
      <div className="min-w-0 flex-1">
        <p className="truncate">{block.name}</p>
        <p className="text-xs text-muted-foreground">{block.name.split(".").pop()?.toUpperCase()} · {block.size}</p>
      </div>
      <DropdownMenu>
        <DropdownMenuTrigger asChild>
          <Button variant="ghost" size="icon" aria-label="File actions">
            <MoreHorizontalIcon />
          </Button>
        </DropdownMenuTrigger>
        <DropdownMenuContent align="end">
          {safeUrl ? (
            <>
              <DropdownMenuItem asChild><a href={safeUrl} target="_blank" rel="noopener noreferrer">Open</a></DropdownMenuItem>
              <DropdownMenuItem asChild><a href={safeUrl} download={block.name}>Download</a></DropdownMenuItem>
            </>
          ) : (
            <>
              <DropdownMenuItem disabled>Open</DropdownMenuItem>
              <DropdownMenuItem disabled>Download</DropdownMenuItem>
            </>
          )}
          <DropdownMenuItem onClick={remove}>Remove</DropdownMenuItem>
        </DropdownMenuContent>
      </DropdownMenu>
    </div>
  );
}

export function FileChip({ name, size, url, onRemove }: { name: string; size: string; url?: string; onRemove: () => void }) {
  const safeUrl = typeof window !== "undefined" && url?.startsWith(`blob:${window.location.origin}/`) ? url : undefined;
  return <DropdownMenu>
    <DropdownMenuTrigger asChild>
      <Button variant="ghost" aria-label={`File actions ${name}`} title={`${name} · ${size}`} className="inline-flex h-auto max-w-full items-center gap-1.5 rounded-md bg-muted/60 px-2 py-1 align-baseline font-sans text-sm font-normal hover:bg-muted">
        <PaperclipIcon className="size-3.5 shrink-0 text-muted-foreground" /><span className="truncate">{name}</span>
      </Button>
    </DropdownMenuTrigger>
    <DropdownMenuContent align="start">
      {safeUrl ? <>
        <DropdownMenuItem asChild><a href={safeUrl} target="_blank" rel="noopener noreferrer">Open</a></DropdownMenuItem>
        <DropdownMenuItem asChild><a href={safeUrl} download={name}>Download</a></DropdownMenuItem>
      </> : <><DropdownMenuItem disabled>Open</DropdownMenuItem><DropdownMenuItem disabled>Download</DropdownMenuItem></>}
      <DropdownMenuItem onSelect={onRemove}>Remove</DropdownMenuItem>
    </DropdownMenuContent>
  </DropdownMenu>;
}
