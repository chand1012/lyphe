import { useEffect, useState } from "react";
import { useNavigate } from "react-router";
import {
  CommandDialog,
  CommandEmpty,
  CommandGroup,
  CommandInput,
  CommandItem,
  CommandList,
  CommandSeparator,
  CommandShortcut,
} from "~/components/ui/command";
import { api, create, useDb, type Kind, type SearchHit } from "~/lib/data";
import { href } from "~/lib/href";

export function CommandSearch({ open, onOpenChange }: { open: boolean; onOpenChange: (open: boolean) => void }) {
  const db = useDb();
  const navigate = useNavigate();
  const [query, setQuery] = useState("");

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === "k") {
        e.preventDefault();
        onOpenChange(true);
      }
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [onOpenChange]);

  const [results, setResults] = useState<SearchHit[]>([]);
  const [failed, setFailed] = useState(false);
  useEffect(() => {
    if (!open) return;
    const controller = new AbortController();
    const timer = setTimeout(() => { void api<SearchHit[]>(`/api/search?q=${encodeURIComponent(query)}`, "GET", undefined, controller.signal).then((hits) => { setResults(hits); setFailed(false); }).catch(() => { if (!controller.signal.aborted) setFailed(true); }); }, 200);
    return () => { clearTimeout(timer); controller.abort(); };
  }, [query, open]);
  const go = (kind: Kind, id: string, placement?: string) => { onOpenChange(false); navigate(href(kind, id) + (placement ? `#attachment-${placement}` : "")); };
  return (
    <CommandDialog open={open} onOpenChange={onOpenChange}>
      <CommandInput value={query} onValueChange={setQuery} placeholder="Search everything…" />
      <CommandList>
        <CommandEmpty>{failed ? "Search is unavailable. Try again." : "Nothing found"}</CommandEmpty>
        {!query && <CommandGroup heading="Create">{(["journal", "task", "goal", "habit"] as Kind[]).map((kind) => <CommandItem key={kind} onSelect={async () => { const entity = await create(kind, { kind, title: `New ${kind}`, tags: [], blocks: [] }); go(kind, entity.id); }}>New {kind}<CommandShortcut>↵</CommandShortcut></CommandItem>)}</CommandGroup>}
        <CommandSeparator />
        <CommandGroup heading="Results">{results.map((result) => <CommandItem key={`${result.kind}-${result.id}-${result.placementId ?? ""}`} value={`${query} ${result.title} ${result.snippet}`} onSelect={() => go(result.kind, result.id, result.placementId)}><div className="min-w-0"><p>{result.title}</p><p className="truncate text-xs text-muted-foreground">{result.placementId ? "Attachment" : result.kind} · {result.snippet}</p></div></CommandItem>)}</CommandGroup>
      </CommandList>
    </CommandDialog>
  );
}
