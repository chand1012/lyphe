import { useRef, useState } from "react";
import { update, type Entity } from "~/lib/data";

/** Inherits the surrounding heading's typography while editing. */
export function EditableEntityTitle({ entity, title = entity.title }: { entity: Entity; title?: string }) {
  const [editing, setEditing] = useState(false);
  const [draft, setDraft] = useState(title);
  const cancelled = useRef(false);
  const label = entity.kind.charAt(0).toUpperCase() + entity.kind.slice(1);
  const save = () => {
    const next = draft.trim();
    if (!cancelled.current && next && next !== title) update(entity.kind, entity.id, { title: next });
    setEditing(false);
  };

  return editing ? <input data-entity-title autoFocus aria-label={`${label} title`} value={draft}
    onChange={(event) => setDraft(event.target.value)}
    onFocus={(event) => event.currentTarget.select()} onBlur={save}
    onKeyDown={(event) => {
      if (event.nativeEvent.isComposing) return;
      if (event.key === "Enter") { event.preventDefault(); event.currentTarget.blur(); }
      if (event.key === "Escape") {
        event.preventDefault();
        event.stopPropagation();
        cancelled.current = true;
        setEditing(false);
      }
    }}
    className="w-full min-w-0 border-0 bg-transparent p-0 outline-none" />
    : <button type="button" aria-label={`Rename ${entity.kind} title`}
        className="w-full cursor-text text-left break-words rounded-sm focus-visible:outline-2 focus-visible:outline-ring"
        onClick={() => { cancelled.current = false; setDraft(title); setEditing(true); }}>{title}</button>;
}
