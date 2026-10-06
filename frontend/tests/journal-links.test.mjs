import assert from "node:assert/strict";
import { test } from "node:test";
import { createElement } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { MemoryRouter } from "react-router";
import { JournalLinks } from "../app/components/journal-links.tsx";
import { db } from "../app/lib/data.ts";

test("journal summary follows inline links, deduplicates relations, and hides empty rows", () => {
 const originalGoals = [...db.goal];
 const originalTasks = [...db.task];
 const entry = { id: "journal123", kind: "journal", title: "Entry", tags: [], blocks: [], related: {goals: [], tasks: [], habits: []} };
 const render = () => renderToStaticMarkup(createElement(MemoryRouter, null, createElement(JournalLinks, {entry})));
 try {
  db.goal.splice(0, db.goal.length, {id: "goal123", kind: "goal", title: "My goal"});
  db.task.splice(0, db.task.length, {id: "task123", kind: "task", title: "My task"});
  assert.equal(render(), "");
  entry.blocks = [{type: "paragraph", text: "", value: [{type: "p", children: [
   {text: ""}, {type: "entity-mention", kind: "goal", entityId: "goal123", children: [{text: ""}]}],
  }]}];
  let html = render();
  assert.ok(html.includes("My goal"));
  assert.ok(!html.includes("Tasks"));
  assert.ok(!html.includes("Link goals"));
  entry.blocks.push({type: "ref", kind: "task", id: "task123"});
  entry.related.tasks = ["task123"];
  html = render();
  assert.equal(html.match(/My task/g)?.length, 1);
  assert.ok(!html.includes("Link tasks"));
  assert.ok(!html.includes("Unlink task"));
  entry.blocks = [];
  html = render();
  assert.ok(!html.includes("Goals"));
  assert.ok(html.includes("Unlink task"));
  entry.related.tasks = ["deleted-task"];
  assert.equal(render(), "");
 } finally {
  db.goal.splice(0, db.goal.length, ...originalGoals);
  db.task.splice(0, db.task.length, ...originalTasks);
 }
});
