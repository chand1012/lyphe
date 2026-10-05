import { useState } from "react";
import { Link } from "react-router";
import { PlusIcon } from "lucide-react";
import type { Route } from "./+types/goal";
import { Badge } from "~/components/ui/badge";
import { Button } from "~/components/ui/button";
import { Checkbox } from "~/components/ui/checkbox";
import { Input } from "~/components/ui/input";
import { Label } from "~/components/ui/label";
import { Popover, PopoverContent, PopoverTrigger } from "~/components/ui/popover";
import { Progress } from "~/components/ui/progress";
import { Slider } from "~/components/ui/slider";
import { Separator } from "~/components/ui/separator";
import { DocumentEditor } from "~/components/document-editor";
import { EditableEntityTitle } from "~/components/editable-entity-title";
import { EntityMenu } from "~/components/entity-menu";
import { EntityRef } from "~/components/entity-ref";
import { EmptyState } from "~/components/empty-state";
import { useDb, update, mentions, create } from "~/lib/data";

function CreateGoalItem({ goalId, kind }: { goalId: string; kind: "task" | "habit" }) {
  const [open, setOpen] = useState(false);
  const [title, setTitle] = useState("");
  const [saving, setSaving] = useState(false);

  return <Popover open={open} onOpenChange={(next) => { if (!saving) { setOpen(next); if (!next) setTitle(""); } }}>
    <PopoverTrigger asChild>
      <Button variant="ghost" size="icon-sm" aria-label={`Create ${kind} for this goal`} title={`Create ${kind}`}><PlusIcon className="size-4" /></Button>
    </PopoverTrigger>
    <PopoverContent align="end" className="w-72">
      <form className="flex flex-col gap-3" onSubmit={async (event) => {
        event.preventDefault();
        const name = title.trim();
        if (!name || saving) return;
        setSaving(true);
        try {
          await create(kind, { kind, title: name, goal: goalId, tags: [], blocks: [] });
          setOpen(false);
          setTitle("");
        } catch { /* create displays the error; keep the name for retry. */ }
        finally { setSaving(false); }
      }}>
        <p className="text-sm font-medium">New {kind}</p>
        <Input aria-label={`New ${kind} name`} placeholder={`${kind === "task" ? "Task" : "Habit"} name`} value={title} disabled={saving} onChange={(event) => setTitle(event.target.value)} />
        <Button type="submit" size="sm" disabled={!title.trim() || saving}>{saving ? "Creating…" : `Create ${kind}`}</Button>
      </form>
    </PopoverContent>
  </Popover>;
}

export default function Goal({ params }: Route.ComponentProps) {
  const db = useDb();
  const goal = db.goal.find((g) => g.id === params.id);
  const [picked, setPicked] = useState<string[]>([]);

  if (!goal) return <EmptyState title="No goal" hint="Pick a goal from the sidebar." action={{ label: "Goals", href: "/goals" }} />;

  const tasks = db.task.filter((t) => t.goalId === goal.id);
  const habits = db.habit.filter((h) => h.goalId === goal.id);
  const journals = db.journal.filter((entry) => mentions(entry, "goal", goal.id));
  const linked = new Set([...tasks.map((t) => t.id), ...habits.map((h) => h.id)]);
  const suggestions = [...db.task, ...db.habit].filter((e) => !linked.has(e.id));

  const addSelected = () => {
    for (const id of picked) {
      const entity = [...db.task, ...db.habit].find((e) => e.id === id);
      if (entity) update(entity.kind, entity.id, { goalId: goal.id });
    }
    setPicked([]);
  };

  return (
    <div className="flex flex-col gap-3">
      <header className="flex items-center justify-between gap-2">
        <h1 className="min-w-0 flex-1 text-3xl font-heading"><EditableEntityTitle key={goal.id} entity={goal} /></h1>
        <EntityMenu entity={goal} context="page" />
      </header>

      <div className="flex flex-wrap items-center gap-2 text-xs text-muted-foreground">
        <Badge variant="secondary" asChild>
          <button type="button" aria-label={`Mark goal ${goal.status === "active" ? "completed" : "active"}`} onClick={() => update("goal", goal.id, { status: goal.status === "active" ? "completed" : "active" })}>{goal.status}</button>
        </Badge>
        <Progress value={goal.progress} className="h-1 w-40" />
        <Popover>
          <PopoverTrigger asChild><Button variant="ghost" size="sm">{goal.progress}% complete</Button></PopoverTrigger>
          <PopoverContent align="start" className="w-48 space-y-2">
            <p className="text-xs text-muted-foreground">Progress · {goal.progress}%</p>
            <Slider value={[goal.progress]} max={100} aria-label="Goal progress" onValueChange={([progress]) => update("goal", goal.id, { progress })} />
          </PopoverContent>
        </Popover>
      </div>

      <DocumentEditor key={goal.id} entity={goal} />

      <Separator />

      <p className="text-sm font-medium">Related Journals</p>
      <div className="flex flex-wrap gap-1">
        {journals.map((entry) => <EntityRef key={entry.id} kind="journal" id={entry.id} />)}
        {!journals.length && <p className="text-sm text-muted-foreground">None yet</p>}
      </div>

      <Separator />

      <div className="flex items-center justify-between gap-2">
        <p className="text-sm font-medium">Tasks</p>
        <div className="flex items-center gap-1">
          <CreateGoalItem key={`${goal.id}-task`} goalId={goal.id} kind="task" />
          <Popover>
            <PopoverTrigger asChild>
              <Button variant="ghost" size="sm">
                Add related items
              </Button>
            </PopoverTrigger>
            <PopoverContent align="end" className="w-64 flex flex-col gap-2">
              <p className="text-sm font-medium">Related items</p>
              <p className="text-xs text-muted-foreground">Choose existing tasks or habits</p>
              {suggestions.length === 0 && <p className="text-sm text-muted-foreground">Nothing to suggest</p>}
              {suggestions.map((entity) => (
                <div key={entity.id} className="flex items-center gap-2 text-sm">
                  <Checkbox
                    id={entity.id}
                    checked={picked.includes(entity.id)}
                    onCheckedChange={(checked) =>
                      setPicked(checked ? [...picked, entity.id] : picked.filter((p) => p !== entity.id))
                    }
                  />
                  <Label htmlFor={entity.id}>{entity.kind}: {entity.title}</Label>
                </div>
              ))}
              <Button size="sm" disabled={!picked.length} onClick={addSelected}>
                Add selected
              </Button>
            </PopoverContent>
          </Popover>
        </div>
      </div>

      <div className="flex flex-col gap-1">
        {tasks.map((task) => (
          <div key={task.id} className="flex items-center gap-2 text-sm">
            <Checkbox checked={task.status === "done"} onCheckedChange={() => update("task", task.id, { status: task.status === "done" ? "todo" : "done" })} />
            <Link to={`/tasks?task=${task.id}`}>{task.title}</Link>
          </div>
        ))}
        {!tasks.length && <p className="text-sm text-muted-foreground">No tasks yet</p>}
      </div>

      <Separator />

      <div className="flex items-center justify-between gap-2">
        <p className="text-sm font-medium">Habits</p>
        <CreateGoalItem key={`${goal.id}-habit`} goalId={goal.id} kind="habit" />
      </div>
      <div className="flex flex-col gap-1">
        {habits.map((habit) => (
          <div key={habit.id} className="flex items-center gap-2 text-sm">
            <span className="size-2 rounded-full bg-primary" />
            <Link to={`/habits/${habit.id}`}>{habit.title}</Link>
          </div>
        ))}
        {!habits.length && <p className="text-sm text-muted-foreground">No habits yet</p>}
      </div>
    </div>
  );
}
