import { EditableEntityTitle } from "~/components/editable-entity-title";
import { useNavigate } from "react-router";
import type { Route } from "./+types/journal";
import { DocumentEditor } from "~/components/document-editor";
import { EntityMenu } from "~/components/entity-menu";
import { JournalLinks } from "~/components/journal-links";
import { TagEditor } from "~/components/tag-editor";
import { EmptyState } from "~/components/empty-state";
import { longDate } from "~/lib/format";
import { create, useDb } from "~/lib/data";

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
        <h1 className="min-w-0 flex-1 text-3xl font-heading"><EditableEntityTitle key={entry.id} entity={entry} title={entry.title && !["Untitled", "New journal"].includes(entry.title) ? entry.title : longDate(entry.date)} /></h1>
        <EntityMenu entity={entry} context="page" />
      </header>

      <TagEditor entity={entry} />
      <JournalLinks entry={entry} />

      <DocumentEditor key={entry.id} entity={entry} audioFirst />
    </div>
  );
}
