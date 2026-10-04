import { useState } from "react";
import { Input } from "~/components/ui/input";
import { useNavigate } from "react-router";
import type { Route } from "./+types/journal";
import { DocumentEditor } from "~/components/document-editor";
import { EntityMenu } from "~/components/entity-menu";
import { TagEditor } from "~/components/tag-editor";
import { EmptyState } from "~/components/empty-state";
import { longDate } from "~/lib/format";
import { create, update, useDb, type Journal as JournalEntry } from "~/lib/data";

function JournalTitle({ entry }: { entry: JournalEntry }) {
  const title = entry.title && !["Untitled", "New journal"].includes(entry.title) ? entry.title : longDate(entry.date);
  const [editing, setEditing] = useState(false);
  const [draft, setDraft] = useState(title);
  const save = () => {
    const next = draft.trim();
    if (next && next !== title) update("journal", entry.id, { title: next });
    setEditing(false);
  };
  return <h1 className="min-w-0 flex-1 text-3xl font-heading">
    {editing ? <Input autoFocus aria-label="Journal title" value={draft} onChange={(event) => setDraft(event.target.value)}
      onFocus={(event) => event.currentTarget.select()} onBlur={save}
      onKeyDown={(event) => {
        if (event.nativeEvent.isComposing) return;
        if (event.key === "Enter") { event.preventDefault(); event.currentTarget.blur(); }
        if (event.key === "Escape") { event.preventDefault(); setEditing(false); }
      }}
      className="h-auto rounded-none border-0 bg-transparent p-0 text-3xl md:text-3xl font-heading shadow-none focus-visible:ring-0" />
      : <button type="button" aria-label="Rename journal title" className="w-full cursor-text text-left break-words rounded-sm focus-visible:outline-2 focus-visible:outline-ring"
          onClick={() => { setDraft(title); setEditing(true); }}>{title}</button>}
  </h1>;
}

export default function Journal({ params }: Route.ComponentProps) {
  const db = useDb();
  const navigate = useNavigate();
  const entry = params.id ? db.journal.find((e) => e.id === params.id) : db.journal[0];

  if (!entry) {
    return <EmptyState title="No journal entries" hint="Journal is where the day lands." action={{ label: "New journal", onClick: async () => { const item = await create("journal", { kind: "journal", title: "Untitled", tags: [], blocks: [] }); navigate(`/journal/${item.id}`); } }} />;
  }

  return (
    <div className="mx-auto flex w-full max-w-3xl flex-col gap-5 py-4 sm:px-6 sm:py-8">
      <header className="flex items-center justify-between gap-2">
        <JournalTitle key={entry.id} entry={entry} />
        <EntityMenu entity={entry} context="page" />
      </header>

      <TagEditor entity={entry} />

      <DocumentEditor key={entry.id} entity={entry} audioFirst />
    </div>
  );
}
