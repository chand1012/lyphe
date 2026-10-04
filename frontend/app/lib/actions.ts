import { toast } from "sonner";
import { duplicateEntity, remove, restore, type Entity } from "~/lib/data";

export async function deleteEntity(entity: Entity) {
  const copy = await remove(entity.kind, entity.id);
  if (!copy) return;
  toast(`Deleted ${copy.title}`, { action: { label: "Undo", onClick: () => restore(copy) } });
}

export async function duplicate(entity: Entity) {
  const copy = await duplicateEntity(entity);
  toast(`Duplicated ${entity.title}`, { action: { label: "Undo", onClick: () => remove(entity.kind, copy.id) } });
}
