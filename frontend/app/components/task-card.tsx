import { Link } from "react-router";
import { MoreHorizontalIcon } from "lucide-react";
import { Button } from "~/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "~/components/ui/dropdown-menu";
import { get, update, type Task } from "~/lib/data";
import { deleteEntity, duplicate } from "~/lib/actions";
import { dayLabel } from "~/lib/format";

const statuses = ["todo", "in_progress", "done"] as const;

export function TaskCard({ task }: { task: Task }) {
  const goal = task.goalId ? get("goal", task.goalId) : undefined;

  return (
    <div className="group rounded-md border bg-card/40 p-3 text-sm transition-colors hover:bg-muted/40">
      <div className="flex items-start justify-between gap-2">
        <Link to={`/tasks?task=${task.id}`} className="min-w-0 flex-1 truncate font-medium">
          {task.title}
        </Link>
        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <Button variant="ghost" size="icon" className="size-6 opacity-100 sm:opacity-0 sm:group-hover:opacity-100 sm:group-focus-within:opacity-100" aria-label={`${task.title} actions`}>
              <MoreHorizontalIcon />
            </Button>
          </DropdownMenuTrigger>
          <DropdownMenuContent align="end">
            {statuses.map((status) => (
              <DropdownMenuItem key={status} onClick={() => update("task", task.id, { status })}>
                {status === "in_progress" ? "In Progress" : status === "todo" ? "Todo" : "Done"}
              </DropdownMenuItem>
            ))}
            <DropdownMenuItem asChild>
              <Link to={`/tasks?task=${task.id}`}>Edit</Link>
            </DropdownMenuItem>
            <DropdownMenuItem onClick={() => duplicate(task)}>Duplicate</DropdownMenuItem>
            <DropdownMenuItem onClick={() => deleteEntity(task)}>Delete</DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
      </div>
      {goal && <p className="mt-1 truncate text-xs text-muted-foreground">{goal.title}</p>}
      <p className="mt-1 text-xs text-muted-foreground">{task.due ? dayLabel(task.due) : "—"}</p>
    </div>
  );
}
