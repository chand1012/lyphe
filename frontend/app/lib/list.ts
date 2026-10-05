import type { PlateEditor } from "platejs/react";

type ListNode = { type?: unknown; indent?: unknown; ordered?: unknown };
export const listDepth = (node: ListNode) => typeof node.indent === "number" && Number.isFinite(node.indent) ? Math.max(0, Math.floor(node.indent)) : 0;

export function listNumber(nodes: ListNode[], index: number) {
  const depth = listDepth(nodes[index]);
  let number = 1;
  for (let i = index - 1; i >= 0; i--) {
    const node = nodes[i];
    if (node.type !== "list-item" || listDepth(node) < depth) break;
    if (listDepth(node) > depth) continue;
    if (!node.ordered) break;
    number++;
  }
  return number;
}

// The document stores lists as flat blocks. Shift descendants with their parent
// so Tab/Shift+Tab keeps the hierarchy intact and persists through normal saves.
export function indentList(editor: PlateEditor, outdent = false) {
  if (!editor.selection) return false;
  const indices = [editor.selection.anchor.path[0], editor.selection.focus.path[0]].sort((a, b) => a - b);
  const [first, last] = indices;
  if (editor.children[first]?.type !== "list-item") return false;
  if (editor.children.slice(first, last + 1).some(node => node.type !== "list-item")) return true;
  const depth = listDepth(editor.children[first]);
  const previous = editor.children[first - 1];
  // Consume Tab even at a boundary; it should never leave a focused list.
  if (outdent ? depth === 0 : !previous || previous.type !== "list-item" || depth > listDepth(previous)) return true;
  let end = last + 1;
  const lastDepth = listDepth(editor.children[last]);
  while (end < editor.children.length && editor.children[end].type === "list-item" && listDepth(editor.children[end]) > lastDepth) end++;
  editor.tf.withNewBatch(() => editor.tf.withoutNormalizing(() => {
    for (let i = first; i < end; i++) editor.tf.setNodes({ indent: Math.max(0, listDepth(editor.children[i]) + (outdent ? -1 : 1)) }, { at: [i] });
  }));
  return true;
}
