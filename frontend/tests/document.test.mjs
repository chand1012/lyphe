import assert from "node:assert/strict";
import { test } from "node:test";
import { documentValue, saveDocument } from "../app/lib/document.ts";

test("legacy references become inline nodes with editable text between and after them", () => {
  const value = documentValue([
    { type: "paragraph", text: "Today" },
    { type: "ref", kind: "task", id: "t1" },
    { type: "ref", kind: "goal", id: "g1" },
    { type: "ref", kind: "habit", id: "h1" },
  ]);
  const links = value[1].children;
  assert.deepEqual(links.filter((node) => node.type === "entity-mention").map((node) => node.entityId), ["t1", "g1", "h1"]);
  assert.ok(links[0].text === "" && links[2].text === " " && links.at(-1).text === " ");
  assert.equal(value.at(-1).type, "p");
  assert.equal(value.at(-1).children[0].text, "");
});

test("saving the continuous document preserves multiple audio attachments and rich inline content", () => {
  const first = { type: "audio", name: "one.wav", duration: 1, url: "blob:one" };
  const second = { type: "audio", name: "two.wav", duration: 2, url: "blob:two" };
  const file = { type: "file", name: "notes.pdf", size: "1 KB" };
  const blocks = [first, { type: "paragraph", text: "Before" }, { type: "ref", kind: "task", id: "t1" }, second, file];
  const value = documentValue(blocks);
  value[1].children[2].text = " and after the link";
  const saved = saveDocument(blocks, value, "After");
  assert.deepEqual(saved.map((block) => block.type), ["audio", "paragraph", "audio"]);
  assert.equal(saved[0], first);
  assert.equal(saved[2], second);
  assert.equal(value[1].children.at(-2).type, "file-attachment");
  assert.equal(value[1].children.at(-2).name, file.name);
  assert.deepEqual(documentValue(saved), value);
});

test("an entry with only attachments still has an editable paragraph", () => {
  const audio = { type: "audio", name: "note.wav", duration: 1 };
  const value = documentValue([audio]);
  assert.deepEqual(value, [{ type: "p", children: [{ text: "" }] }]);
  assert.deepEqual(saveDocument([audio], value, ""), [audio, { type: "paragraph", text: "", value }]);
});

test("inline files remain searchable after editing and retain their attachment anchor", async () => {
  const { inlineFiles } = await import("../app/lib/document.ts");
  const value = documentValue([{ type: "file", name: "one.pdf", size: "1 KB" }, { type: "file", name: "two.pdf", size: "2 KB" }]);
  value[0].children[2].text = " between files ";
  const saved = saveDocument([], value, "between files");
  assert.deepEqual(inlineFiles(saved[0].value), [{ name: "one.pdf", index: 0 }, { name: "two.pdf", index: 1 }]);
  assert.equal(value.at(-1).type, "p");
});

test("removing an inline attachment removes it from derived file search", async () => {
  const { inlineFiles } = await import("../app/lib/document.ts");
  const value = documentValue([{ type: "file", name: "notes.pdf", size: "1 KB" }]);
  assert.equal(inlineFiles(value).length, 1);
  value[0].children = [{text: "File removed"}];
  assert.equal(inlineFiles(value).length, 0);
});
