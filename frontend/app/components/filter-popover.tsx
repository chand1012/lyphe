import { FilterIcon } from "lucide-react";
import { Badge } from "~/components/ui/badge";
import { Button } from "~/components/ui/button";
import { Command, CommandInput, CommandItem, CommandList } from "~/components/ui/command";
import { Popover, PopoverContent, PopoverTrigger } from "~/components/ui/popover";
import { Separator } from "~/components/ui/separator";
import { allTags, setFilters, useFilters, type Kind } from "~/lib/data";

const kinds: Kind[] = ["journal", "task", "goal", "habit"];
const dates: [string, number][] = [["Last 7 days", 7], ["Last 30 days", 30], ["All time", 0]];

export function FilterPopover() {
  const f = useFilters();
  const active = f.kinds.length > 0 || f.tags.length > 0 || f.days > 0;

  return (
    <Popover>
      <PopoverTrigger asChild>
        <Button variant="ghost" size="sm">
          <FilterIcon className="size-4" />
          Filter{active ? " · on" : ""}
        </Button>
      </PopoverTrigger>
      <PopoverContent align="start" className="w-64 flex flex-col gap-3">
        <div className="flex flex-col gap-1">
          <p className="text-xs text-muted-foreground">Type</p>
          <div className="flex flex-wrap gap-1">
            {kinds.map((kind) => (
              <Badge key={kind} asChild variant={f.kinds.includes(kind) ? "secondary" : "outline"}>
                <button type="button" aria-pressed={f.kinds.includes(kind)} onClick={() => setFilters({
                  kinds: f.kinds.includes(kind) ? f.kinds.filter((k) => k !== kind) : [...f.kinds, kind],
                })}>{kind[0].toUpperCase() + kind.slice(1)}</button>
              </Badge>
            ))}
          </div>
        </div>

        <Separator />

        <div className="flex flex-col gap-1">
          <p className="text-xs text-muted-foreground">Tags</p>
          <Command>
            <CommandInput placeholder="Tags…" />
            <CommandList>
              {allTags().map((tag) => (
                <CommandItem
                  key={tag}
                  value={tag}
                  data-selected={f.tags.includes(tag)}
                  onSelect={() =>
                    setFilters({ tags: f.tags.includes(tag) ? f.tags.filter((t) => t !== tag) : [...f.tags, tag] })
                  }
                >
                  {tag}
                </CommandItem>
              ))}
            </CommandList>
          </Command>
        </div>

        <Separator />

        <div className="flex flex-col gap-1">
          <p className="text-xs text-muted-foreground">Date</p>
          <div className="flex flex-wrap gap-1">
            {dates.map(([label, days]) => (
              <Badge key={label} asChild variant={f.days === days ? "secondary" : "outline"}>
                <button type="button" aria-pressed={f.days === days} onClick={() => setFilters({ days })}>{label}</button>
              </Badge>
            ))}
          </div>
        </div>
      </PopoverContent>
    </Popover>
  );
}
