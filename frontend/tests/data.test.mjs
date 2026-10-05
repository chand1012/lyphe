import assert from "node:assert/strict";
import { test } from "node:test";
import { encodeDocument, decodeEntity, weekRate, mentions, metadata, db } from "../app/lib/data.ts";
import { isoDay } from "../app/lib/format.ts";

test("persistent documents use file IDs and strip temporary URLs", () => {
 const blocks = [{type: "paragraph", text: "Text", value: [{type: "p", children: [{text: "before "}, {type: "file-attachment", fileId: "file123", placementId: "stable", url: "blob:temporary", children: [{text: ""}]}, {text: " after"}]}]}, {type: "audio", fileId: "audio123", name: "voice.webm", duration: 2, url: "blob:audio"}];
 const document = encodeDocument(blocks);
 assert.equal(document.value[0].children[1].fileId, "file123");
 assert.equal(document.value[0].children[1].placementId, "stable");
 assert.equal(document.value[0].children[1].url, undefined);
 assert.deepEqual(document.audioFileIds, ["audio123"]);
 assert.equal(blocks[0].value[0].children[1].url, "blob:temporary");
});
test("backend adapter preserves calendar dates and extended task statuses", () => {
 const task = decodeEntity({id: "task123", kind: "task", title: "Work", status: "blocked", due_on: "2026-10-02", position: 4, revision: 7, goal: "goal123", content: {version: 1, value: [{type: "p", children: [{text: "Notes"}]}], audioFileIds: []}});
 assert.equal(task.status, "blocked"); assert.equal(task.due, "2026-10-02"); assert.equal(task.goalId, "goal123"); assert.equal(task.order, 4); assert.equal(task.revision, 7);
});
test("backlinks find mentions inside a continuous document", () => {
 const entity = {blocks: [{type: "paragraph", text: "", value: [{type: "p", children: [{text: ""}, {type: "entity-mention", kind: "habit", entityId: "habit123", children: [{text: ""}]}]}]}]};
 assert.equal(mentions(entity, "habit", "habit123"), true);
 assert.equal(mentions(entity, "habit", "other"), false);
});
test("production cache starts empty", () => { assert.equal(Object.values(db).flat().length, 0); });
test("dates use local calendar days", () => { assert.equal(isoDay(new Date(2026, 9, 1)), "2026-10-01"); });
test("weekly and monthly habits count one completion per calendar period", () => {
 const today = isoDay(new Date()); assert.equal(weekRate({cadence: "weekly", history: [today]}), 100); assert.equal(weekRate({cadence: "monthly", history: [today]}), 100); assert.equal(weekRate({cadence: "weekly", history: []}), 0);
});

test('journal sidebar labels use renamed titles and fall back to the date', async () => {
 const { journalLabel } = await import('../app/lib/format.ts');
 const date = isoDay(new Date());
 assert.equal(journalLabel({title: 'My first entry', date}), 'My first entry');
 for (const title of ['', 'Untitled', 'New journal']) assert.equal(journalLabel({title, date}), 'Today');
});


test("journal links encode to persisted relations and decode as backlinks", () => {
 const related = {goals: ["goal123"], tasks: ["task123"], habits: []};
 const patch = metadata({related, title: "Journal", revision: 3});
 assert.deepEqual(patch, {title: "Journal", goals: ["goal123"], tasks: ["task123"], habits: []});
 assert.deepEqual(related, {goals: ["goal123"], tasks: ["task123"], habits: []});
 const entry = decodeEntity({id: "journal123", kind: "journal", ...patch});
 assert.equal(mentions(entry, "goal", "goal123"), true);
 assert.equal(mentions(entry, "task", "task123"), true);
 assert.equal(mentions(entry, "task", "other"), false);
 const unlinked = decodeEntity({id: "journal123", kind: "journal", ...metadata({related: {...related, tasks: []}})});
 assert.equal(mentions(unlinked, "task", "task123"), false);
 assert.equal(mentions(unlinked, "goal", "goal123"), true);
});
