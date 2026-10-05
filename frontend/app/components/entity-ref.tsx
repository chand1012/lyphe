import { Link } from "react-router";
import { BookOpenIcon, RefreshCwIcon, SquareIcon, TargetIcon } from "lucide-react";
import { get } from "~/lib/data";
import type { Kind } from "~/lib/data";
import { journalLabel } from "~/lib/format";
import type { Journal } from "~/lib/data";
import { href } from "~/lib/href";

const icons = { task: SquareIcon, goal: TargetIcon, habit: RefreshCwIcon, journal: BookOpenIcon };

export function EntityRef({ kind, id }: { kind: Kind; id: string }) {
  const entity = get(kind, id);
  if (!entity) return null;
  const Icon = icons[kind];
  return (
    <Link
      to={href(kind, id)}
      className="inline-flex items-center gap-1.5 rounded-md bg-muted/60 px-2 py-1 text-sm transition-colors hover:bg-muted"
    >
      <Icon className="size-3.5 shrink-0 text-muted-foreground" />
      <span className="truncate">{kind === "journal" ? journalLabel(entity as Journal) : entity.title}</span>
    </Link>
  );
}
