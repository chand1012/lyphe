import { useState } from "react";
import { Link, useNavigate } from "react-router";
import { Progress } from "~/components/ui/progress";
import { Tabs, TabsList, TabsTrigger } from "~/components/ui/tabs";
import { EntityMenu } from "~/components/entity-menu";
import { FilterPopover } from "~/components/filter-popover";
import { EmptyState } from "~/components/empty-state";
import { create, matchesFilters, useDb } from "~/lib/data";

export default function Goals() {
  const db = useDb();
  const navigate = useNavigate();
  const [tab, setTab] = useState("active");
  const goals = db.goal.filter((goal) => goal.status === tab && matchesFilters(goal));

  return (
    <div className="flex flex-col gap-3">
      <header className="flex items-center justify-between gap-2">
        <h1 className="text-3xl font-heading">Goals</h1>
        <FilterPopover />
      </header>

      <Tabs value={tab} onValueChange={setTab}>
        <TabsList className="w-fit">
          <TabsTrigger value="active">Active</TabsTrigger>
          <TabsTrigger value="completed">Completed</TabsTrigger>
        </TabsList>
      </Tabs>

      {goals.length === 0 ? (
        <EmptyState
          title="No goals yet"
          hint="Goals help connect what you're doing to what you're working toward."
          action={{ label: "Create goal", onClick: async () => { const item = await create("goal", { kind: "goal", title: "New goal", tags: [], blocks: [] }); navigate(`/goals/${item.id}`); } }}
        />
      ) : (
        <div className="flex flex-col gap-3">
          {goals.map((goal) => {
            const tasks = db.task.filter((t) => t.goalId === goal.id).length;
            const habits = db.habit.filter((h) => h.goalId === goal.id).length;

            return (
              <div key={goal.id} className="group flex items-start justify-between gap-2">
                <Link to={`/goals/${goal.id}`} className="flex min-w-0 flex-1 flex-col gap-1">
                  <p className="text-sm font-medium">{goal.title}</p>
                  <Progress value={goal.progress} className="h-1 w-40" />
                  <p className="text-xs text-muted-foreground">
                    {goal.progress}% complete · {tasks} tasks · {habits} habits
                  </p>
                </Link>
                <EntityMenu entity={goal} />
              </div>
            );
          })}
        </div>
      )}
    </div>
  );
}
