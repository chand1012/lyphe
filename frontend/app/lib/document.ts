import type { Value } from "platejs";
import type { Block } from "./data";

export function isDocumentBlock(block: Block) {
  return ["paragraph", "heading", "quote", "ref", "divider", "file"].includes(block.type);
}

// Keep older block-based entries editable as one continuous Slate document.
export function documentValue(blocks: Block[]): Value {
  const value: Value = [];
  let references: Value[number] | undefined;
  for (const [index, block] of blocks.entries()) {
    if (block.type === "ref" || block.type === "file") {
      if (!references) {
        references = { type: "p", children: [{ text: "" }] };
        value.push(references);
      }
      references.children.push(
        block.type === "ref"
          ? { type: "entity-mention", kind: block.kind, entityId: block.id, children: [{ text: "" }] }
          : { type: "file-attachment", name: block.name, size: block.size, url: block.url, fileId: block.fileId, placementId: block.placementId, attachmentIndex: index, children: [{ text: "" }] },
        { text: " " },
      );
    } else if (block.type === "paragraph" || block.type === "heading" || block.type === "quote") {
      references = undefined;
      value.push(...(block.value ?? [{ type: block.type === "heading" ? "h2" : block.type === "quote" ? "blockquote" : "p", children: [{ text: block.text }] }]));
    } else if (block.type === "divider") {
      references = undefined;
      value.push({ type: "hr", children: [{ text: "" }] });
    }
  }
  if (!value.length || value[value.length - 1].type !== "p" || references) {
    value.push({ type: "p", children: [{ text: "" }] });
  }
  return value;
}

export function saveDocument(blocks: Block[], value: Value, text: string): Block[] {
  const body: Block = { type: "paragraph", text, value };
  const first = blocks.findIndex(isDocumentBlock);
  if (first < 0) return [...blocks, body];
  return blocks.flatMap((block, index) => index === first ? [body] : isDocumentBlock(block) ? [] : [block]);
}

export function inlineFiles(value: Value): { name: string; index: number }[] {
  const files: { name: string; index: number }[] = [];
  const visit = (nodes: Value[number]["children"]) => {
    for (const node of nodes) {
      if (node.type === "file-attachment" && typeof node.name === "string" && typeof node.attachmentIndex === "number") files.push({ name: node.name, index: node.attachmentIndex });
      if ("children" in node && Array.isArray(node.children)) visit(node.children);
    }
  };
  for (const node of value) visit([node]);
  return files;
}
