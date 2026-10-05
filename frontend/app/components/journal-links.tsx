import { useState } from "react";
import { XIcon } from "lucide-react";
import { Button } from "~/components/ui/button";
import { Checkbox } from "~/components/ui/checkbox";
import { Input } from "~/components/ui/input";
import { Popover, PopoverContent, PopoverTrigger } from "~/components/ui/popover";
import { EntityRef } from "~/components/entity-ref";
import { update, useDb, type Journal } from "~/lib/data";

function JournalLinkPicker({ entry, kind }: { entry: Journal; kind: "goal" | "task" }) {
  const db = useDb();
  const [query, setQuery] = useState("");
  const field = kind === "goal" ? "goals" : "tasks";
  const ids = entry.related?.[field] ?? [];
  const linked = db[kind].filter((item) => ids.includes(item.id));
  const options = db[kind].filter((item) => item.title.toLowerCase().includes(query.toLowerCase()));
  const toggle = (id: string) => {
    const next = ids.includes(id) ? ids.filter((value) => value !== id) : [...ids, id];
    if (next.length > 10) return;
    update("journal", entry.id, { related: { goals: [], tasks: [], habits: [], ...entry.related, [field]: next } });
  };

  return <div className="flex flex-wrap items-center gap-2">
    <span className="w-12 text-xs text-muted-foreground">{kind === "goal" ? "Goals" : "Tasks"}</span>
    {linked.map((item) => <span key={item.id} className="inline-flex min-w-0 max-w-full items-center gap-0.5">
      <EntityRef kind={kind} id={item.id} />
      <Button variant="ghost" size="icon-xs" aria-label={`Unlink ${kind}: ${item.title}`} onClick={() => toggle(item.id)}><XIcon className="size-3" /></Button>
    </span>)}
    <Popover onOpenChange={() => setQuery("")}>
      <PopoverTrigger asChild><Button variant="ghost" size="sm">Link {field}</Button></PopoverTrigger>
      <PopoverContent align="start" className="w-72 space-y-3">
        <Input aria-label={`Search ${field} to link`} placeholder={`Search ${field}…`} value={query} onChange={(event) => setQuery(event.target.value)} />
        <div className="max-h-60 space-y-2 overflow-y-auto">
          {options.map((item) => <label key={item.id} className="flex items-center gap-2 text-sm">
            <Checkbox aria-label={`Link ${kind}: ${item.title}`} checked={ids.includes(item.id)} disabled={!ids.includes(item.id) && ids.length >= 10} onCheckedChange={() => toggle(item.id)} />
            <span className="min-w-0 break-words">{item.title}</span>
          </label>)}
          {!options.length && <p className="text-sm text-muted-foreground">{db[kind].length ? "No matches" : `Create a ${kind} first to link it here.`}</p>}
        </div>
        {ids.length >= 10 && <p className="text-xs text-muted-foreground">Up to 10 {field} per journal entry.</p>}
      </PopoverContent>
    </Popover>
  </div>;
}

export function JournalLinks({ entry }: { entry: Journal }) {
  return <div className="flex flex-col gap-2" aria-label="Journal links">
    <JournalLinkPicker key={`${entry.id}-goal`} entry={entry} kind="goal" />
    <JournalLinkPicker key={`${entry.id}-task`} entry={entry} kind="task" />
  </div>;
}
