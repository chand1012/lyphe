import { Link } from "react-router";
import type { Route } from "./+types/habit";
import { Separator } from "~/components/ui/separator";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "~/components/ui/select";
import { DocumentEditor } from "~/components/document-editor";
import { EditableEntityTitle } from "~/components/editable-entity-title";
import { EntityMenu } from "~/components/entity-menu";
import { EmptyState } from "~/components/empty-state";
import { HabitGrid } from "~/components/habit-grid";
import { streak, update, useDb, weekRate } from "~/lib/data";

export default function Habit({ params }: Route.ComponentProps) {
  const db = useDb();
  const habit = db.habit.find((h) => h.id === params.id);

  if (!habit) return <EmptyState title="No habit" hint="Pick a habit from the sidebar." action={{ label: "Habits", href: "/habits" }} />;

  return (
    <div className="flex flex-col gap-3">
      <header className="flex items-center justify-between gap-2">
        <h1 className="min-w-0 flex-1 text-3xl font-heading"><EditableEntityTitle key={habit.id} entity={habit} /></h1>
        <EntityMenu entity={habit} context="page" />
      </header>

      <div className="flex flex-wrap items-center gap-2 text-sm text-muted-foreground">
        <span>{weekRate(habit)}% {habit.cadence === "monthly" ? "this month" : "this week"}</span>
        {habit.cadence === "daily" && <span>· {streak(habit)} day streak</span>}
        <Select value={habit.cadence} onValueChange={(cadence) => update("habit", habit.id, { cadence })}>
          <SelectTrigger size="sm" className="w-28" aria-label="Habit cadence"><SelectValue /></SelectTrigger>
          <SelectContent>
            <SelectItem value="daily">Daily</SelectItem>
            <SelectItem value="weekly">Weekly</SelectItem>
            <SelectItem value="monthly">Monthly</SelectItem>
          </SelectContent>
        </Select>
      </div>

      <HabitGrid habit={habit} />

      <Separator />

      <p className="text-sm font-medium">Notes</p>
      <DocumentEditor key={habit.id} entity={habit} />

      {habit.goalId && (
        <p className="text-sm text-muted-foreground">
          Related goal:{" "}
          <Link to={`/goals/${habit.goalId}`}>{db.goal.find((g) => g.id === habit.goalId)?.title}</Link>
        </p>
      )}
    </div>
  );
}
