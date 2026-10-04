import type { Kind } from "~/lib/data";

const paths: Record<Kind, string> = { journal: "/journal", goal: "/goals", habit: "/habits", task: "/tasks?task=" };

export const href = (kind: Kind, id: string) => `${paths[kind]}${kind === "task" ? id : `/${id}`}`;
