import { Link, useNavigate } from "react-router";
import { Checkbox } from "~/components/ui/checkbox";
import { Progress } from "~/components/ui/progress";
import { Separator } from "~/components/ui/separator";
import { Tooltip, TooltipContent, TooltipTrigger } from "~/components/ui/tooltip";
import { EmptyState } from "~/components/empty-state";
import { FilterPopover } from "~/components/filter-popover";
import { create, matchesFilters, streak, update, useDb, weekRate, type Habit } from "~/lib/data";
import { isoDay } from "~/lib/format";

const today = () => isoDay(new Date());

function HabitRow({ habit }: { habit: Habit }) {
  const done = habit.history.includes(today());

  return (
    <div className="flex items-center gap-2 text-sm">
      <Checkbox
        checked={done}
        onCheckedChange={() =>
          update("habit", habit.id, {
            history: done ? habit.history.filter((d) => d !== today()) : [...habit.history, today()],
          } as never)
        }
      />
      <Link to={`/habits/${habit.id}`}>{habit.title}</Link>
      <Tooltip>
        <TooltipTrigger asChild>
          <span className="ml-auto text-xs text-muted-foreground">{weekRate(habit)}%</span>
        </TooltipTrigger>
        <TooltipContent>This week</TooltipContent>
      </Tooltip>
    </div>
  );
}

export default function Habits() {
  const db = useDb();
  const navigate = useNavigate();
  const habits = db.habit.filter(matchesFilters);
  const active = habits.filter((h) => h.cadence !== "monthly");
  const overall = active.length ? Math.round(active.reduce((sum, h) => sum + weekRate(h), 0) / active.length) : 0;
  const best = habits.reduce((max, h) => Math.max(max, streak(h)), 0);

  const groups: [string, Habit[]][] = [
    ["Today", habits.filter((h) => h.cadence === "daily")],
    ["This Week", habits.filter((h) => h.cadence === "weekly")],
    ["Later", habits.filter((h) => h.cadence === "monthly")],
  ];

  return (
    <div className="flex flex-col gap-3">
      <header className="flex items-center justify-between gap-2">
        <h1 className="text-3xl font-heading">Habits</h1>
        <FilterPopover />
      </header>

      <div className="flex flex-wrap items-center gap-3 text-sm text-muted-foreground">
        <Progress value={overall} className="h-1 w-32" />
        <span>{overall}% complete</span>
        <Separator orientation="vertical" className="h-4" />
        <span>{active.length} active</span>
        <Separator orientation="vertical" className="h-4" />
        <span>{best} day streak</span>
      </div>

      {habits.length === 0 ? (
        <EmptyState title="No habits yet" hint="Habits are the small things that add up." action={{ label: "Create habit", onClick: async () => { const item = await create("habit", { kind: "habit", title: "New habit", tags: [], blocks: [] }); navigate(`/habits/${item.id}`); } }} />
      ) : (
        <div className="flex flex-col gap-4">
          {groups.map(([label, items]) => (
            <div key={label} className="flex flex-col gap-2">
              <p className="text-sm font-medium">{label}</p>
              <Separator />
              {items.length ? items.map((habit) => <HabitRow key={habit.id} habit={habit} />) : <p className="text-sm text-muted-foreground">Nothing here</p>}
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
