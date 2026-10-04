import { useState } from "react";
import { XIcon } from "lucide-react";
import { Badge } from "~/components/ui/badge";
import { Button } from "~/components/ui/button";
import { Command, CommandInput, CommandItem, CommandList } from "~/components/ui/command";
import { Popover, PopoverContent, PopoverTrigger } from "~/components/ui/popover";
import { allTags, get, toggleTag, update, useDb, type Entity } from "~/lib/data";

export function TagEditor({ entity }: { entity: Entity }) {
  useDb();
  const [open, setOpen] = useState(false);
  const [query, setQuery] = useState("");
  const [selected, setSelected] = useState("");
  const name = query.trim();
  const tags = allTags();
  const exact = tags.find((tag) => tag.toLocaleLowerCase() === name.toLocaleLowerCase());
  const matches = tags.filter((tag) => tag.toLocaleLowerCase().includes(name.toLocaleLowerCase()))
    .sort((a, b) => Number(b === exact) - Number(a === exact));
  const add = (tag: string) => {
    const current = get(entity.kind, entity.id);
    if (current && !current.tags.some((item) => item.toLocaleLowerCase() === tag.toLocaleLowerCase())) {
      update(entity.kind, entity.id, { tags: [...current.tags, tag] });
    }
    setQuery("");
    setSelected("");
    setOpen(false);
  };
  return (
    <div className="flex flex-wrap items-center gap-1">
      {entity.tags.map((tag) => (
        <Badge key={tag} variant="secondary">
          {tag}
          <button aria-label={`Remove ${tag}`} onClick={() => toggleTag(entity.kind, entity.id, tag)}>
            <XIcon className="size-3" />
          </button>
        </Badge>
      ))}
      <Popover open={open} onOpenChange={(next) => { setOpen(next); if (!next) { setQuery(""); setSelected(""); } }}>
        <PopoverTrigger asChild>
          <Button variant="ghost" size="sm">+ Add</Button>
        </PopoverTrigger>
        <PopoverContent align="start" className="w-56 p-0">
          <Command shouldFilter={false} value={selected} onValueChange={setSelected}>
            <CommandInput placeholder="Add a tag…" value={query} onValueChange={(value) => {
              setQuery(value);
              const typed = value.trim();
              setSelected(tags.find((tag) => tag.toLocaleLowerCase() === typed.toLocaleLowerCase()) ?? (typed ? `Create ${typed}` : ""));
            }} />
            <CommandList>
              {name && !exact && (
                <CommandItem value={`Create ${name}`} onSelect={() => add(name)}>
                  Create “{name}”
                </CommandItem>
              )}
              {matches.map((tag) => (
                <CommandItem key={tag} value={tag} onSelect={() => add(tag)}>
                  {tag}
                </CommandItem>
              ))}
            </CommandList>
          </Command>
        </PopoverContent>
      </Popover>
    </div>
  );
}
