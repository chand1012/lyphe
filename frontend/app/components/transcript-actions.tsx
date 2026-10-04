import { useEffect, useRef } from "react";
import { toast } from "sonner";
import { Button } from "~/components/ui/button";
import { Popover, PopoverContent, PopoverTrigger } from "~/components/ui/popover";
import { api, applyTranscript, jobs, useDb, type Entity, type Job } from "~/lib/data";
export function TranscriptActions({entity, fileId, onTranscribe, onInsert}: {entity: Entity; fileId: string; onTranscribe: () => void; onInsert: (job: Job) => Promise<void>}) {
 useDb(); const current = jobs.find((j) => j.file === fileId && j[entity.kind] === entity.id); const attempted = useRef(new Set<string>());
 useEffect(() => { if (current?.status !== "completed" || current.applied || attempted.current.has(current.id) || !current.anchor_id) return; attempted.current.add(current.id); void applyTranscript(entity, current).catch(() => {}); }, [current?.id, current?.status, current?.applied]);
 if (!current) return <Button variant="ghost" size="sm" className="w-fit text-xs text-muted-foreground" onClick={onTranscribe}>Transcribe</Button>;
 return <div className="flex items-center gap-2 text-xs text-muted-foreground">
  {current.status === "queued" || current.status === "transcribing" || current.status === "cleaning" ? <span role="status">{current.status === "queued" ? "Queued…" : current.status === "cleaning" ? "Cleaning transcript…" : "Transcribing…"}</span> : <>
   {current.status === "failed" && <span>{current.error || "Transcription failed"}</span>}
   <Popover><PopoverTrigger asChild><Button variant="ghost" size="sm" className="text-xs">Transcript</Button></PopoverTrigger><PopoverContent className="w-80 max-h-96 overflow-auto"><p className="text-xs font-medium">{current.cleanup_outcome === "raw_fallback" ? "Original transcript · cleanup could not preserve wording" : "Transcript"}</p><p className="my-2 whitespace-pre-wrap text-sm">{current.cleaned_text || current.raw_text || "No transcript yet"}</p>{current.cleaned_text !== current.raw_text && <details className="text-xs"><summary>Original transcript</summary><p className="whitespace-pre-wrap">{current.raw_text}</p></details>}</PopoverContent></Popover>
   {current.status === "completed" && !current.applied && <Button variant="ghost" size="sm" onClick={() => { void onInsert(current).catch(() => toast.error("Could not insert transcript")); }}>Insert at cursor</Button>}
   {(current.status === "failed" || (current.cleanup_outcome === "raw_fallback" && !current.applied)) && <Button variant="ghost" size="sm" onClick={() => { void api(`/api/transcriptions/${current.id}/retry`, "POST").catch(() => toast.error("Could not retry transcription")); }}>Retry</Button>}
  </>}
 </div>;
}
