import { Popover, PopoverContent, PopoverTrigger } from "~/components/ui/popover";
import { Toggle } from "~/components/ui/toggle";
import { Button } from "~/components/ui/button";
import { Separator } from "~/components/ui/separator";
import { EntityRef } from "~/components/entity-ref";
import { mentions, update, useDb, type Habit } from "~/lib/data";
import { dayLabel, isoDay } from "~/lib/format";

function Day({ habit, date }: { habit: Habit; date: string }) {
  const db = useDb();
  const done = habit.history.includes(date);
  const journal = db.journal.find(
    (j) => j.date === date && mentions(j, "habit", habit.id),
  );

  return (
    <Popover>
      <PopoverTrigger asChild>
        <Toggle
          pressed={done}
          aria-label={date}
          className="size-6 rounded-md border p-0 data-[state=on]:bg-primary/20 data-[state=on]:text-primary"
        >
          <span className="size-1.5 rounded-full bg-current" />
        </Toggle>
      </PopoverTrigger>
      <PopoverContent align="start" className="w-64 flex flex-col gap-2">
        <p className="text-sm font-medium">{dayLabel(date)}</p>
        <Button
          variant="outline"
          size="sm"
          onClick={() =>
            update(habit.kind, habit.id, {
              history: done ? habit.history.filter((d) => d !== date) : [...habit.history, date],
            } as never)
        }
        >
          {done ? "Mark missed" : "Mark done"}
        </Button>
        <Separator />
        <textarea
          rows={2}
          placeholder="Note"
          defaultValue={habit.notes[date] ?? ""}
          onBlur={(e) => update(habit.kind, habit.id, { notes: { ...habit.notes, [date]: e.target.value } } as never)}
          className="w-full resize-none bg-transparent text-sm outline-none field-sizing-content"
        />
        {journal && <EntityRef kind="journal" id={journal.id} />}
      </PopoverContent>
    </Popover>
  );
}

export function HabitGrid({ habit }: { habit: Habit }) {
  const start = new Date();
  start.setHours(0, 0, 0, 0);
  start.setDate(start.getDate() - ((start.getDay() + 6) % 7) - 21);
  const days = Array.from({ length: 28 }, (_, i) => {
    const date = new Date(start);
    date.setDate(start.getDate() + i);
    return isoDay(date);
  });
  const weeks = [0, 1, 2, 3].map((w) => days.slice(w * 7, w * 7 + 7));

  return (
    <div className="flex flex-col gap-1">
      <div className="flex items-center gap-1 text-xs text-muted-foreground">
        {["M", "T", "W", "T", "F", "S", "S"].map((d, i) => (
          <span key={i} className="size-6 text-center">
            {d}
          </span>
        ))}
      </div>
      {weeks.map((week) => (
        <div key={week[0]} className="flex items-center gap-1">
          {week.map((date) => (
            <Day key={date} habit={habit} date={date} />
          ))}
        </div>
      ))}
    </div>
  );
}
