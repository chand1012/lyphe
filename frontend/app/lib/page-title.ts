import type { Entity, Journal } from "./data";
import { longDate } from "./format";

type TitleData = Record<Entity["kind"], Entity[]>;

export function pageTitle(pathname: string, search: string, entities: TitleData) {
  const [section, id] = pathname.split("/").filter(Boolean);
  const labels: Record<string, string> = {
    login: "Sign in", register: "Create an account", journal: "Journal",
    goals: "Goals", tasks: "Tasks", habits: "Habits", settings: "Settings",
  };
  let title = section ? labels[section] ?? "Page not found" : "";
  let entity: Entity | undefined;
  if (section === "journal") entity = id ? entities.journal.find(item => item.id === id) : entities.journal[0];
  if (section === "goals" && id) entity = entities.goal.find(item => item.id === id);
  if (section === "habits" && id) entity = entities.habit.find(item => item.id === id);
  if (section === "tasks") entity = entities.task.find(item => item.id === new URLSearchParams(search).get("task"));
  if (entity) {
    title = entity.title.trim() || title;
    if (entity.kind === "journal" && ["", "Untitled", "New journal"].includes(entity.title.trim())) {
      title = longDate((entity as Journal).date);
    }
  }
  return title ? `${title} · Lyphe` : "Lyphe";
}
