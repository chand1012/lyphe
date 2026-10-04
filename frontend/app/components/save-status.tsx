import { Button } from "~/components/ui/button";
import { resolveConflict, saveState, useDb, type Entity } from "~/lib/data";
export function SaveStatus({entity}: {entity: Entity}) {
 useDb(); const state = saveState(entity.id);
 if (state !== "failed" && state !== "conflict") return null;
 return <div className="flex items-center gap-2 text-xs text-muted-foreground" role="alert">
  {state === "conflict" ? "Another session changed this document. Your draft is preserved." : "Changes have not been saved."}
  {(state === "failed" || state === "conflict") && <><Button variant="ghost" size="sm" onClick={() => resolveConflict(entity.kind, entity.id, true)}>Save my version</Button><Button variant="ghost" size="sm" onClick={() => resolveConflict(entity.kind, entity.id, false)}>Reload saved version</Button></>}
 </div>;
}
