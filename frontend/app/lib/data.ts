import { useSyncExternalStore } from "react";
import type { Value } from "platejs";
import type { RecordModel } from "pocketbase";
import { toast } from "sonner";
import { pb } from "./pocketbase";
import { documentValue, inlineFiles } from "./document";
import { isoDay } from "./format";

export type Kind = "journal" | "task" | "goal" | "habit";
type StoredFile = { fileId?: string; placementId?: string };
export type Block =
  | { type: "paragraph" | "heading" | "quote"; text: string; value?: Value }
  | { type: "divider" }
  | { type: "ref"; kind: Kind; id: string }
  | ({ type: "image"; name: string; url?: string } & StoredFile)
  | ({ type: "video"; name: string; url?: string } & StoredFile)
  | ({ type: "audio"; name: string; duration: number; url?: string } & StoredFile)
  | ({ type: "file"; name: string; size: string; url?: string } & StoredFile);
export type Entity = { id: string; kind: Kind; title: string; tags: string[]; blocks: Block[]; revision: number; editorVersion?: number; related?: {goals: string[]; tasks: string[]; habits: string[]} };
export type Goal = Entity & { kind: "goal"; status: "active" | "completed"; progress: number };
export type Task = Entity & { kind: "task"; status: "todo" | "in_progress" | "done" | "in_review" | "blocked" | "paused"; goalId?: string; due?: string; journals: string[]; order?: number };
export type Habit = Entity & { kind: "habit"; cadence: "daily" | "weekly" | "monthly"; goalId?: string; history: string[]; notes: Record<string, string> };
export type Journal = Entity & { kind: "journal"; date: string; folder: string; folderId?: string };
export const kinds: Kind[] = ["journal", "task", "goal", "habit"];
export const db: { journal: Journal[]; task: Task[]; goal: Goal[]; habit: Habit[] } = { journal: [], task: [], goal: [], habit: [] };
export const folders: string[] = [];
export const tagNames: string[] = [];
export const fileRecords = new Map<string, RecordModel>();
export type Job = RecordModel & { status: string; raw_text: string; cleaned_text: string; anchor_id: string; applied: boolean; file: string; cleanup_outcome: string };
export const jobs: Job[] = [];
export let preferences: RecordModel | undefined;
export let aiStatus: { completion: { available: boolean; state: string }; transcription: { available: boolean } } | undefined;
const listeners = new Set<() => void>(); let version = 0; let session = ""; let connectedUser = ""; let loading: Promise<void> | undefined; let fileToken = ""; let poll: ReturnType<typeof setInterval> | undefined; let tokenPoll: ReturnType<typeof setInterval> | undefined;
export function emit() { version++; recomputeBacklinks(); for (const listener of listeners) listener(); }
export function useDb() { useSyncExternalStore((l) => { listeners.add(l); return () => { listeners.delete(l); }; }, () => version, () => 0); return db; }
export const api = async <T = RecordModel>(path: string, method = "GET", body?: unknown, signal?: AbortSignal): Promise<T> => { const user = pb.authStore.record?.id; const result = await pb.send<T>(path, {method, body, signal, requestKey: null}); if (user !== pb.authStore.record?.id) throw new Error("Account changed"); return result; };
export function get(kind: Kind, id: string): Entity | undefined { return db[kind].find((item) => item.id === id); }
export function useFileURL(id?: string) { useDb(); return fileURL(id); }
export function fileURL(id?: string) { const record = id ? fileRecords.get(id) : undefined; return record ? pb.files.getURL(record, record.file, { token: fileToken }) : undefined; }
function media(id: string, type: "audio" | "image" | "video", placementId = id): Block { const file = fileRecords.get(id); return type === "audio" ? { type, fileId: id, placementId, name: file?.name ?? "Audio", duration: file?.duration ?? 0, url: fileURL(id) } : { type, fileId: id, placementId, name: file?.name ?? "Attachment", url: fileURL(id) }; }
function hydrate(value: Value): Value { const copy = structuredClone(value); const visit = (nodes: any[]) => { for (const node of nodes) { if (node.fileId) { const file = fileRecords.get(node.fileId); node.url = fileURL(node.fileId); node.name = file?.name ?? node.name; node.size = file ? `${Math.ceil(file.size / 1024)} KB` : node.size; } if (node.children) visit(node.children); } }; visit(copy); return copy; }
export function decodeEntity(record: RecordModel): Entity {
  const d = record.content ?? { version: 1, value: [{ type: "p", children: [{ text: "" }] }], audioFileIds: [] };
  const blocks: Block[] = [{ type: "paragraph", text: record.content_text ?? "", value: hydrate(d.value) }, ...(d.audioFileIds ?? []).map((id: string) => media(id, "audio")), ...(d.media ?? []).map((node: any) => media(node.fileId, node.type, node.placementId))];
  const common = { id: record.id, kind: record.kind, title: record.title, tags: record.tags ?? [], revision: record.revision ?? 0, related: {goals: record.goals ?? [], tasks: record.tasks ?? [], habits: record.habits ?? []}, blocks };
  return { ...common, status: record.status || (record.kind === "goal" ? "active" : "todo"), progress: record.progress ?? 0, date: record.date, folder: record.folder ?? "Unfiled", folderId: record.folderId, goalId: record.goal || undefined, due: record.due_on || undefined, order: record.position, journals: [], cadence: record.cadence || "daily", history: [], notes: {} } as Entity;
}
export function encodeDocument(blocks: Block[]) {
  const value = structuredClone(documentValue(blocks)); const visit = (nodes: any[]) => { for (const node of nodes) { delete node.url; if (node.type && !node.id) node.id = crypto.randomUUID(); if (node.fileId && !node.placementId) node.placementId = crypto.randomUUID(); if (node.children) visit(node.children); } }; visit(value);
  return { version: 1, value, audioFileIds: blocks.flatMap((b) => b.type === "audio" && b.fileId ? [b.fileId] : []), media: blocks.flatMap((b) => (b.type === "image" || b.type === "video") && b.fileId ? [{ type: b.type, fileId: b.fileId, placementId: b.placementId ?? b.fileId, children: [{ text: "" }] }] : []) };
}
function replace(record: RecordModel, remote = false) { const entity = decodeEntity(record); const old = get(entity.kind, entity.id); if (remote && old && record.revision <= old.revision) return; if (old && pending.get(entity.id)?.state !== "saved" && pending.has(entity.id)) return; entity.editorVersion = (old?.editorVersion ?? 0) + (remote ? 1 : 0); if (old && entity.kind === "habit") { (entity as Habit).history = (old as Habit).history; (entity as Habit).notes = (old as Habit).notes; } if (old) Object.assign(old, entity); else (db[entity.kind] as Entity[]).push(entity); emit(); }
function applyHistory(records: RecordModel[]) { for (const habit of db.habit) { habit.history = []; habit.notes = {}; } for (const row of records) { const habit = db.habit.find((h) => h.id === row.habit); if (!habit) continue; if (row.is_completed) habit.history.push(row.date); if (row.notes) { const node = document.createElement("div"); node.innerHTML = row.notes; habit.notes[row.date] = node.textContent ?? ""; } } }
export async function loadData(force = false) {
  const user = pb.authStore.record?.id; if (!user) return; if (!force && session === user && loading) return loading; if (!force && session === user && db.journal.length + db.task.length + db.goal.length + db.habit.length) return;
  session = user; loading = (async () => {
    const [files, folderRows, tags, settings, history] = await Promise.all([pb.collection("files").getFullList(), pb.collection("folders").getFullList(), pb.collection("tags").getFullList(), api("/api/preferences"), pb.collection("habit_completions").getFullList({ filter: pb.filter("date >= {:date}", { date: isoDay(new Date(Date.now() - 400 * 864e5)) }) })]);
    if (session !== user) return; fileToken = await pb.files.getToken(); fileRecords.clear(); files.forEach((f) => fileRecords.set(f.id, f)); folders.splice(0, folders.length, ...folderRows.map((f) => f.name)); tagNames.splice(0, tagNames.length, ...tags.map((t) => t.name)); preferences = settings;
    for (const kind of kinds) { const records: RecordModel[] = []; let page = 1; while (true) { const result = await api<{items: RecordModel[]; hasMore: boolean}>(`/api/entities/${kind}?page=${page++}`); records.push(...result.items); if (!result.hasMore) break; } if (session !== user) return; const dirty = db[kind].filter((e) => pending.has(e.id) && pending.get(e.id)?.state !== "saved"); (db[kind] as Entity[]).splice(0, db[kind].length, ...records.filter((r) => !dirty.some((d) => d.id === r.id)).map(decodeEntity), ...dirty); }
    applyHistory(history); await refreshAI(); await refreshJobs(); emit();
  })().finally(() => { loading = undefined; }); return loading;
}
export async function connectData() { await loadData(); const user = session; if (!user || connectedUser === user) return; connectedUser = user; for (const [kind, table] of Object.entries({ journal: "journal", task: "tasks", goal: "goals", habit: "habits" })) await pb.collection(table).subscribe("*", async (event) => { if (session !== user || pending.get(event.record.id)?.state === "saving") return; if (event.action === "delete" || event.record.deleted_at) { if (pending.get(event.record.id)?.state !== "saved" && pending.has(event.record.id)) return; const list = db[kind as Kind]; const index = list.findIndex((e) => e.id === event.record.id); if (index >= 0) list.splice(index, 1); emit(); return; } try { const record = await api(`/api/entities/${kind}/${event.record.id}`); replace(record, true); } catch { /* a record may have been deleted */ } });
  await pb.collection("habit_completions").subscribe("*", (event) => {
    if (session !== user || pending.get(event.record.habit)?.state === "saving") return;
    const habit = db.habit.find((h) => h.id === event.record.habit); if (!habit) return;
    habit.history = habit.history.filter((d) => d !== event.record.date);
    delete habit.notes[event.record.date];
    if (event.action !== "delete") { if (event.record.is_completed) habit.history.push(event.record.date); if (event.record.notes) { const node = document.createElement("div"); node.innerHTML = event.record.notes; habit.notes[event.record.date] = node.textContent ?? ""; } }
    emit();
  });
  await pb.collection("files").subscribe("*", (event) => { if (session !== user) return; if (event.action === "delete") fileRecords.delete(event.record.id); else fileRecords.set(event.record.id,event.record); emit(); });
  await pb.collection("folders").subscribe("*", () => { void pb.collection("folders").getFullList().then((rows) => { if (session !== user) return; folders.splice(0,folders.length,...rows.map((r) => r.name)); emit(); }).catch(() => {}); });
  await pb.collection("tags").subscribe("*", () => { void pb.collection("tags").getFullList().then((rows) => { if (session !== user) return; tagNames.splice(0,tagNames.length,...rows.map((r) => r.name)); emit(); }).catch(() => {}); });
  await pb.collection("transcriptions").subscribe("*", () => { void refreshJobs().catch(() => {}); });
  poll = setInterval(() => { void refreshAI(); void refreshJobs().catch(() => {}); }, 10000);
  tokenPoll = setInterval(() => { void pb.files.getToken().then((token) => { if (session !== user) return; fileToken = token; emit(); }).catch(() => {}); }, 90000);
}
export function clearData() { session = ""; connectedUser = ""; for (const p of pending.values()) if (p.timer) clearTimeout(p.timer); pending.clear(); kinds.forEach((k) => db[k].splice(0)); folders.splice(0); tagNames.splice(0); jobs.splice(0); fileRecords.clear(); preferences = undefined; aiStatus = undefined; filters.kinds = []; filters.tags = []; filters.days = 0; if (poll) clearInterval(poll); poll = undefined; if (tokenPoll) clearInterval(tokenPoll); tokenPoll = undefined; void pb.realtime.unsubscribe(); emit(); }
if (typeof window !== "undefined") { pb.authStore.onChange(() => { if (session && session !== pb.authStore.record?.id) clearData(); }); window.addEventListener("beforeunload", (event) => { if ([...pending.values()].some((p) => p.state !== "saved")) { event.preventDefault(); event.returnValue = ""; } }); }
export async function createFolder(name: string) { name = name.trim(); if (!name || folders.includes(name)) return; await pb.collection("folders").create({ user: pb.authStore.record?.id, name }); folders.push(name); emit(); }
export async function create(kind: Kind, input: Omit<Entity, "id" | "revision"> & Record<string, unknown>): Promise<Entity> { try { const record = await api(`/api/entities/${kind}`, "POST", { ...input, date: input.date ?? isoDay(new Date()) }); replace(record); return get(kind, record.id)!; } catch (error) { toast.error("Could not create item"); throw error; } }
type Pending = { patch: Record<string, unknown>; state: "saved" | "saving" | "failed" | "conflict"; timer?: ReturnType<typeof setTimeout>; promise?: Promise<void>; error?: string; habitDates?: Set<string> };
const pending = new Map<string, Pending>();
export function hasPendingChanges() { return [...pending.values()].some((p) => p.state !== "saved"); }
export function saveState(id: string) { return pending.get(id)?.state ?? "saved"; }
export function update(kind: Kind, id: string, patch: Partial<Entity> & Record<string, unknown>) {
  const entity = get(kind, id); if (!entity) return; const state: Pending = pending.get(id) ?? { patch: {}, state: "saved" }; if (kind === "habit" && (patch.history || patch.notes)) { const habit = entity as Habit; state.habitDates ??= new Set(); const nextHistory = patch.history as string[] | undefined; const nextNotes = patch.notes as Record<string, string> | undefined; const dates = new Set([...habit.history, ...(nextHistory ?? []), ...Object.keys(habit.notes), ...Object.keys(nextNotes ?? {})]); for (const date of dates) { if ((nextHistory && habit.history.includes(date) !== nextHistory.includes(date)) || (nextNotes && (habit.notes[date] ?? "") !== (nextNotes[date] ?? ""))) state.habitDates.add(date); } } Object.assign(entity, patch); Object.assign(state.patch, patch); if (state.timer) clearTimeout(state.timer); if (state.state !== "conflict") state.state = "saving"; state.timer = setTimeout(() => { state.timer = undefined; void persist(kind, id); }, "blocks" in patch ? 750 : 200); pending.set(id, state); emit();
}
function metadata(patch: Record<string, unknown>) { const result = { ...patch }; delete result.blocks; delete result.history; delete result.notes; delete result.kind; delete result.id; delete result.revision; delete result.editorVersion; if ("goalId" in result) { result.goal = result.goalId ?? ""; delete result.goalId; } if ("due" in result) { result.due_on = result.due ?? ""; delete result.due; } if ("order" in result) { result.position = result.order; delete result.order; } return result; }
async function persist(kind: Kind, id: string): Promise<void> {
  const state = pending.get(id); if (!state || state.state === "conflict") return; if (state.promise) { await state.promise; if (Object.keys(state.patch).length && state.state !== "failed") return persist(kind, id); return; }
  const entity = get(kind, id); if (!entity || !Object.keys(state.patch).length) return; const patch = state.patch; const habitDates = state.habitDates; state.habitDates = undefined; state.patch = {}; state.state = "saving";
  state.promise = (async () => { try {
    if (kind === "habit" && habitDates) { for (const date of habitDates) { await api(`/api/habits/${id}/days/${date}`, "PUT", { ...(patch.history ? { completed: (patch.history as string[]).includes(date) } : {}), ...(patch.notes ? { notes: (patch.notes as Record<string, string>)[date] ?? "" } : {}) }); } }
    const fields = metadata(patch); let record: RecordModel | undefined; if (Object.keys(fields).length) { record = await api(`/api/entities/${kind}/${id}`, "PATCH", { baseRevision: entity.revision, patch: fields }); entity.revision = record!.revision; }
    if (patch.blocks) { record = await api(`/api/entities/${kind}/${id}/document`, "PUT", { baseRevision: entity.revision, document: encodeDocument(patch.blocks as Block[]) }); entity.revision = record!.revision; }
    state.state = Object.keys(state.patch).length ? "saving" : "saved";
  } catch (error: any) { state.patch = { ...patch, ...state.patch }; if (habitDates) { state.habitDates ??= new Set(); habitDates.forEach((date) => state.habitDates!.add(date)); } state.state = error.status === 409 ? "conflict" : "failed"; state.error = error.message; toast.error(state.state === "conflict" ? "This document changed in another session. Your draft is still here." : "Changes could not be saved. Your draft is still here."); } finally { state.promise = undefined; emit(); } })(); await state.promise; if (Object.keys(state.patch).length && state.state === "saving") await persist(kind, id);
}
export async function flush(kind: Kind, id: string) { const p = pending.get(id); if (p?.timer) clearTimeout(p.timer); await persist(kind, id); if (p && p.state !== "saved") throw new Error("Save pending changes first"); }
export async function flushAll() { for (const kind of kinds) for (const entity of db[kind]) await flush(kind, entity.id); }
export async function resolveConflict(kind: Kind, id: string, keep: boolean) { const record = await api(`/api/entities/${kind}/${id}`); const state = pending.get(id); if (keep && state) { get(kind, id)!.revision = record.revision; state.state = "saving"; await persist(kind, id); } else { pending.delete(id); replace(record, true); } }
export async function remove(kind: Kind, id: string) { await flush(kind, id); const copy = get(kind, id); await api(`/api/entities/${kind}/${id}/delete`, "POST"); const list = db[kind]; const i = list.findIndex((e) => e.id === id); if (i >= 0) list.splice(i, 1); emit(); return copy; }
export async function restore(entity: Entity) { replace(await api(`/api/entities/${entity.kind}/${entity.id}/restore`, "POST"), true); }
export async function duplicateEntity(entity: Entity) { await flush(entity.kind, entity.id); const record = await api(`/api/entities/${entity.kind}/${entity.id}/duplicate`, "POST"); replace(record); return get(entity.kind, record.id)!; }
export function toggleTag(kind: Kind, id: string, tag: string) { const e = get(kind, id); if (e) update(kind, id, { tags: e.tags.includes(tag) ? e.tags.filter((t) => t !== tag) : [...e.tags, tag] }); }
export function allTags() { return [...new Set([...tagNames, ...Object.values(db).flat().flatMap((e) => e.tags)])].sort(); }
export function streak(habit: Habit) { if (habit.cadence !== "daily") return 0; let n = 0; const start = new Date(); if (!habit.history.includes(isoDay(start))) start.setDate(start.getDate() - 1); while (habit.history.includes(isoDay(start))) { n++; start.setDate(start.getDate() - 1); } return n; }
export function weekRate(habit: Pick<Habit, "cadence" | "history">) { const now = new Date(); if (habit.cadence === "monthly") return habit.history.some((d) => d.startsWith(isoDay(now).slice(0, 7))) ? 100 : 0; const monday = new Date(now); monday.setDate(now.getDate() - ((now.getDay() + 6) % 7)); const start = isoDay(monday); const end = isoDay(now); const done = new Set(habit.history.filter((d) => d >= start && d <= end)).size; return habit.cadence === "weekly" ? (done ? 100 : 0) : Math.round(done / (((now.getDay() + 6) % 7) + 1) * 100); }
export async function applyBoard(board: Record<string, Task[]>) { const changed = Object.entries(board).flatMap(([status, list]) => list.flatMap((t, position) => t.status !== status || t.order !== position ? [{ id: t.id, status, position, revision: t.revision }] : [])); if (!changed.length) return; try { for (const t of changed) await flush("task", t.id); const records = await api<RecordModel[]>("/api/tasks/reorder", "POST", { tasks: changed.map((t) => ({ ...t, revision: get("task", t.id)!.revision })) }); records.forEach((r) => replace(r)); } catch { toast.error("Could not move tasks. Reloading the saved board."); await loadData(true); } }
export const filters = { kinds: [] as Kind[], tags: [] as string[], days: 0 };
export function useFilters() { useDb(); return filters; }
export function setFilters(next: Partial<typeof filters>) { Object.assign(filters, next); emit(); }
export function matchesFilters(e: Entity) { if (filters.kinds.length && !filters.kinds.includes(e.kind)) return false; if (filters.tags.length && !filters.tags.some((t) => e.tags.includes(t))) return false; const date = e.kind === "journal" ? (e as Journal).date : e.kind === "task" ? (e as Task).due : undefined; return !filters.days || !date || new Date(`${date}T00:00:00`) >= new Date(Date.now() - filters.days * 864e5); }
export type SearchHit = {kind: Kind; id: string; title: string; snippet: string; placementId?: string};
export function search(q: string): Record<Kind, Entity[]> & {files: {name: string; kind: Kind; id: string; index: number}[]} { const s = q.toLowerCase(); const all = Object.values(db).flat(); const hit = (e: Entity) => e.title.toLowerCase().includes(s) || e.tags.some((t) => t.toLowerCase().includes(s)) || e.blocks.some((b) => "text" in b && b.text.toLowerCase().includes(s)); return { ...Object.fromEntries(kinds.map((k) => [k, db[k].filter(hit)])) as unknown as Record<Kind, Entity[]>, files: all.flatMap((e) => e.blocks.flatMap((b, index) => "name" in b ? [{ name: b.name, kind: e.kind, id: e.id, index }] : "value" in b && b.value ? inlineFiles(b.value).map((f) => ({ ...f, kind: e.kind, id: e.id })) : [])).filter((f) => f.name.toLowerCase().includes(s)) }; }
export async function upload(file: File | Blob, name: string) { const form = new FormData(); form.set("file", file, name); const record = await api("/api/files/upload", "POST", form); fileRecords.set(record.id, record); return record; }
export async function refreshAI() { try { const user = session; const result = await api<any>("/api/ai/status"); if (user !== session) return; aiStatus = result; preferences = result.preferences; emit(); } catch { /* normal editing works without AI */ } }
export async function refreshJobs() { const user = session; const records = await pb.collection("transcriptions").getFullList<Job>({ sort: "-created" }); if (user !== session) return; jobs.splice(0, jobs.length, ...records); emit(); }
export async function updatePreferences(patch: Record<string, unknown>) { preferences = await api("/api/preferences", "PATCH", patch); emit(); }
export async function startTranscription(entity: Entity, fileId: string, anchorId: string) { await flush(entity.kind, entity.id); const job = await api<Job>("/api/ai/transcribe", "POST", { kind: entity.kind, entityId: entity.id, fileId, anchorId, requestKey: crypto.randomUUID() }); jobs.unshift(job); emit(); return job; }
export async function applyTranscript(entity: Entity, job: Job, anchorId?: string, raw = false) { await flush(entity.kind, entity.id); const record = await api(`/api/transcriptions/${job.id}/apply`, "POST", { baseRevision: entity.revision, anchorId, raw }); pending.delete(entity.id); replace(record, true); await refreshJobs(); }

export function mentions(entity: Entity, kind: Kind, id: string) { let found = false; const visit = (nodes: any[]) => { for (const node of nodes) { if (node.type === "entity-mention" && node.kind === kind && node.entityId === id) found = true; if (node.children) visit(node.children); } }; for (const block of entity.blocks) { if (block.type === "ref" && block.kind === kind && block.id === id) found = true; if ("value" in block && block.value) visit(block.value); } const key = kind === "goal" ? "goals" : kind === "task" ? "tasks" : "habits"; return found || (kind !== "journal" && !!entity.related?.[key].includes(id)); }
function recomputeBacklinks() { for (const task of db.task) task.journals = db.journal.filter((j) => mentions(j, "task", task.id)).map((j) => j.id); }
