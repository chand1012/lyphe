import { useNavigate } from "react-router";
import { CalendarIcon } from "lucide-react";
import { Sheet, SheetContent, SheetDescription, SheetHeader, SheetTitle } from "~/components/ui/sheet";
import { Drawer, DrawerContent, DrawerHeader, DrawerTitle } from "~/components/ui/drawer";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "~/components/ui/select";
import { Popover, PopoverContent, PopoverTrigger } from "~/components/ui/popover";
import { Calendar } from "~/components/ui/calendar";
import { Separator } from "~/components/ui/separator";
import { Input } from "~/components/ui/input";
import { EntityRef } from "~/components/entity-ref";
import { TagEditor } from "~/components/tag-editor";
import { DocumentEditor } from "~/components/document-editor";
import { Button } from "~/components/ui/button";
import { useIsMobile } from "~/hooks/use-mobile";
import { update, useDb, type Task } from "~/lib/data";
import { dayLabel, isoDay } from "~/lib/format";

const statuses = ["todo", "in_progress", "done", "in_review", "blocked", "paused"] as const;

export function TaskSheet({ task }: { task: Task }) {
  const navigate = useNavigate();
  const mobile = useIsMobile();
  const db = useDb();
  const close = () => navigate("/tasks");

  const body = <div className="flex min-h-0 flex-col gap-4 overflow-y-auto px-4 pb-4">
    <Input aria-label="Task title" value={task.title} onChange={(event) => update("task", task.id, { title: event.target.value })} className="border-0 px-0 text-lg font-medium shadow-none focus-visible:ring-0" />
    <div className="flex flex-col gap-1">
      <p className="text-xs text-muted-foreground">Status</p>
      <Select value={task.status} onValueChange={(status) => update("task", task.id, { status })}>
        <SelectTrigger size="sm" className="w-40"><SelectValue /></SelectTrigger>
        <SelectContent>{statuses.map((status) => (
          <SelectItem key={status} value={status}>{status === "in_progress" ? "In Progress" : status === "todo" ? "Todo" : "Done"}</SelectItem>
        ))}</SelectContent>
      </Select>
    </div>
    <div className="flex flex-col gap-1">
      <p className="text-xs text-muted-foreground">Goal</p>
      <Select value={task.goalId ?? "none"} onValueChange={(goalId) => update("task", task.id, { goalId: goalId === "none" ? undefined : goalId })}>
        <SelectTrigger size="sm" className="w-40"><SelectValue placeholder="No goal" /></SelectTrigger>
        <SelectContent>
          <SelectItem value="none">No goal</SelectItem>
          {db.goal.map((goal) => <SelectItem key={goal.id} value={goal.id}>{goal.title}</SelectItem>)}
        </SelectContent>
      </Select>
    </div>
    <div className="flex flex-col gap-1">
      <p className="text-xs text-muted-foreground">Due</p>
      <Popover>
        <PopoverTrigger asChild>
          <Button variant="outline" size="sm" className="w-40 justify-start"><CalendarIcon className="size-4" />{task.due ? dayLabel(task.due) : "No date"}</Button>
        </PopoverTrigger>
        <PopoverContent align="start" className="p-0">
          <Calendar mode="single" selected={task.due ? new Date(`${task.due}T00:00:00`) : undefined}
            onSelect={(date: Date | undefined) => update("task", task.id, { due: date ? isoDay(date) : undefined })} />
        </PopoverContent>
      </Popover>
    </div>
    <div className="flex flex-col gap-1"><p className="text-xs text-muted-foreground">Tags</p><TagEditor entity={task} /></div>
    <Separator />
    <div className="flex flex-col gap-2"><p className="text-xs text-muted-foreground">Notes</p><DocumentEditor key={task.id} entity={task} /></div>
    <Separator />
    <div className="flex flex-col gap-2">
      <p className="text-xs text-muted-foreground">Related Journals</p>
      <div className="flex flex-wrap gap-1">
        {task.journals.map((id) => <EntityRef key={id} kind="journal" id={id} />)}
        {!task.journals.length && <p className="text-sm text-muted-foreground">None yet</p>}
      </div>
    </div>
    <Button variant="ghost" size="sm" onClick={close}>Close</Button>
  </div>;

  return mobile ? (
    <Drawer open onOpenChange={(open) => !open && close()}>
      <DrawerContent className="max-h-[90svh]">
        <DrawerHeader><DrawerTitle>Task detail</DrawerTitle></DrawerHeader>
        {body}
      </DrawerContent>
    </Drawer>
  ) : (
    <Sheet open onOpenChange={(open) => !open && close()}>
      <SheetContent className="flex flex-col gap-0 sm:max-w-md">
        <SheetHeader><SheetTitle>{task.title}</SheetTitle><SheetDescription>Task detail</SheetDescription></SheetHeader>
        {body}
      </SheetContent>
    </Sheet>
  );
}
