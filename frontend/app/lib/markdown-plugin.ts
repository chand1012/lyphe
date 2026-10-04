import { createPlatePlugin } from "platejs/react";
import { NodeApi } from "platejs";
import { markdownBlock, markdownInline } from "./markdown-shortcuts";

export const MarkdownPlugin = createPlatePlugin({ key: "markdown-shortcuts" }).overrideEditor(({ editor, tf: { insertText, insertBreak } }) => ({
  transforms: {
    insertText(text, options) {
      const point = editor.selection?.anchor;
      if (!point || !editor.api.isCollapsed() || editor.api.isComposing() || text.length !== 1) return insertText(text, options);
      const block = editor.children[point.path[0]];
      let leaf: any = editor;
      for (const index of point.path) leaf = leaf.children?.[index];
      if (block?.type === "code-block" || leaf?.code || typeof leaf?.text !== "string") return insertText(text, options);
      const start = editor.api.start([point.path[0]]);
      if (!start) return insertText(text, options);
      const range = { anchor: start, focus: point };
      const before = editor.api.string(range);
      const shortcut = text === " " ? markdownBlock(before) : before + text === "---" ? { type: "hr" } : undefined;
      if (shortcut) {
        insertText(text, options);
        const end = editor.selection!.anchor;
        editor.tf.withNewBatch(() => editor.tf.withoutNormalizing(() => {
          editor.tf.delete({ at: { anchor: start, focus: end } });
          editor.tf.setNodes(shortcut, { at: [point.path[0]] });
          if (shortcut.type === "hr") {
            const path = [point.path[0] + 1];
            editor.tf.insertNodes({ type: "p", children: [{ text: "" }] }, { at: path });
            editor.tf.select(editor.api.start(path)!);
          }
        }));
        return;
      }
      const inline = markdownInline(leaf.text.slice(0, point.offset) + text);
      if (inline) {
        insertText(text, options);
        const end = editor.selection!.anchor;
        editor.tf.withNewBatch(() => editor.tf.withoutNormalizing(() => {
          editor.tf.select({ anchor: { path: point.path, offset: inline.start }, focus: end });
          for (const mark of inline.marks) editor.tf.addMark(mark, true);
          insertText(inline.text);
          for (const mark of inline.marks) editor.tf.removeMark(mark);
        }));
        return;
      }
      insertText(text, options);
    },
    insertBreak() {
      const point = editor.selection?.anchor;
      const block = point && editor.children[point.path[0]];
      if (point && block && editor.api.isCollapsed()) {
        const start = editor.api.start([point.path[0]]);
        const before = start && editor.api.string({ anchor: start, focus: point });
        const fence = typeof before === "string" && markdownBlock(before);
        if (fence && fence.type === "code-block") {
          editor.tf.delete({ at: { anchor: start!, focus: point } });
          editor.tf.setNodes(fence, { at: [point.path[0]] });
          return;
        }
        if (block.type === "code-block") {
          if (!before || before.endsWith("\n")) {
            if (before) editor.tf.deleteBackward("character");
            const path = [point.path[0] + 1];
            editor.tf.insertNodes({ type: "p", children: [{ text: "" }] }, { at: path });
            editor.tf.select(editor.api.start(path)!);
          } else insertText("\n");
          return;
        }
        if ((block.type === "list-item" || block.type === "todo") && !NodeApi.string(block)) {
          editor.tf.setNodes({ type: "p" }, { at: [point.path[0]] });
          return;
        }
      }
      insertBreak();
      const next = editor.selection?.anchor;
      if (next && editor.children[next.path[0]]?.type === "todo") editor.tf.setNodes({ checked: false }, { at: [next.path[0]] });
    },
  },
}));
