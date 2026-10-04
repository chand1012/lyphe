import { useState } from "react";
import { Link, useNavigate, useSearchParams } from "react-router";
import type { Route } from "./+types/tasks";
import { Checkbox } from "~/components/ui/checkbox";
import { ToggleGroup, ToggleGroupItem } from "~/components/ui/toggle-group";
import { ScrollArea, ScrollBar } from "~/components/ui/scroll-area";
import {
  Kanban,
  KanbanBoard,
  KanbanColumn,
  KanbanColumnContent,
  KanbanColumnHandle,
  KanbanItem,
  KanbanItemHandle,
  KanbanOverlay,
} from "~/components/reui/kanban";
import { TaskCard } from "~/components/task-card";
import { TaskSheet } from "~/components/task-sheet";
import { FilterPopover } from "~/components/filter-popover";
import { EmptyState } from "~/components/empty-state";
import { dayLabel } from "~/lib/format";
import { applyBoard, create, get, matchesFilters, update, useDb, type Task } from "~/lib/data";
import { useIsMobile } from "~/hooks/use-mobile";

const baseColumns: [string, string][] = [["todo", "Todo"], ["in_progress", "In Progress"], ["done", "Done"]];

export default function Tasks({}: Route.ComponentProps) {
  const db = useDb();
  const navigate = useNavigate();
  const isMobile = useIsMobile();
  const [chosenView, setView] = useState<string>();
  const view = chosenView ?? (isMobile ? "list" : "board");
  const [params] = useSearchParams();
  const tasks = db.task.filter(matchesFilters);
  const columns: [string, string][] = [...baseColumns, ...(["in_review", "blocked", "paused"] as const).filter((status) => tasks.some((t) => t.status === status)).map((status): [string, string] => [status, status.replace("_", " ")])];
  const open = params.get("task") ? db.task.find((t) => t.id === params.get("task")) : undefined;

  const board: Record<string, Task[]> = Object.fromEntries(
    columns.map(([status]) => [status, tasks.filter((t) => t.status === status).sort((a, b) => (a.order ?? 0) - (b.order ?? 0))]),
  );


  return (
    <div className="flex flex-col gap-3">
      <header className="flex flex-wrap items-center justify-between gap-2">
        <h1 className="text-3xl font-heading">Tasks</h1>
        <ToggleGroup type="single" value={view} onValueChange={(value) => value && setView(value)}>
          <ToggleGroupItem value="board">Board</ToggleGroupItem>
          <ToggleGroupItem value="list">List</ToggleGroupItem>
        </ToggleGroup>
        <FilterPopover />
      </header>

      {tasks.length === 0 ? (
        <EmptyState title="No tasks yet" hint="Tasks are the things that move a goal forward." action={{ label: "Create task", onClick: async () => { const item = await create("task", { kind: "task", title: "New task", tags: [], blocks: [] }); navigate(`/tasks?task=${item.id}`); } }} />
      ) : view === "board" ? (
        <ScrollArea className="w-full">
          <Kanban value={board} onValueChange={applyBoard} getItemValue={(task) => task.id} className="min-w-[720px]">
            <KanbanBoard>
              {columns.map(([status, label]) => (
                <KanbanColumn key={status} value={status}>
                  <KanbanColumnHandle className="flex items-center justify-between pb-2 text-xs text-muted-foreground">
                    <span>{label}</span>
                    <span>{board[status].length}</span>
                  </KanbanColumnHandle>
                  <KanbanColumnContent value={status}>
                    {board[status].map((task) => (
                      <KanbanItem key={task.id} value={task.id}>
                        <KanbanItemHandle className="w-full">
                          <TaskCard task={task} />
                        </KanbanItemHandle>
                      </KanbanItem>
                    ))}
                  </KanbanColumnContent>
                </KanbanColumn>
              ))}
            </KanbanBoard>
            <KanbanOverlay>
              {({ value, variant }) => {
                if (variant === "item") {
                  const task = get("task", String(value));
                  return task && task.kind === "task" ? <TaskCard task={task as Task} /> : null;
                }
                const label = columns.find(([key]) => key === String(value))?.[1] ?? String(value);
                return <p className="text-xs text-muted-foreground">{label}</p>;
              }}
            </KanbanOverlay>
          </Kanban>
          <ScrollBar orientation="horizontal" />
        </ScrollArea>
      ) : (
        <div className="flex flex-col gap-1">
          {tasks.map((task) => (
            <div key={task.id} className="flex items-center gap-2 text-sm">
              <Checkbox
                checked={task.status === "done"}
                onCheckedChange={() => update("task", task.id, { status: task.status === "done" ? "todo" : "done" })}
              />
              <Link to={`/tasks?task=${task.id}`} className="flex-1 truncate">
                {task.title}
              </Link>
              <span className="text-xs text-muted-foreground">{task.goalId ? db.goal.find((g) => g.id === task.goalId)?.title : "—"}</span>
              <span className="text-xs text-muted-foreground">{task.due ? dayLabel(task.due) : "—"}</span>
            </div>
          ))}
        </div>
      )}

      {open && <TaskSheet task={open} />}
    </div>
  );
}
