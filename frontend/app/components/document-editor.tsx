import { useEffect, useRef, useState, type MouseEvent } from "react";
import { useLocation } from "react-router";
import { BasicBlocksPlugin, BasicMarksPlugin, HorizontalRulePlugin } from "@platejs/basic-nodes/react";
import { Plate, PlateContent, usePlateEditor, createPlatePlugin, type PlateElementProps, type PlateEditor } from "platejs/react";
import { NodeApi, type TRange, type Value } from "platejs";
import { ImageIcon, PaperclipIcon, PlusIcon, MoreHorizontalIcon, ChevronDownIcon, BoldIcon, ItalicIcon, UnderlineIcon, StrikethroughIcon, CodeIcon, TypeIcon, Heading1Icon, Heading2Icon, Heading3Icon, QuoteIcon, ListIcon, ListOrderedIcon } from "lucide-react";
import { toast } from "sonner";
import { Button } from "~/components/ui/button";
import {
  Command,
  CommandEmpty,
  CommandGroup,
  CommandInput,
  CommandItem,
  CommandList,
  CommandSeparator,
} from "~/components/ui/command";
import { Popover, PopoverContent, PopoverTrigger } from "~/components/ui/popover";
import { Separator } from "~/components/ui/separator";
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuLabel, DropdownMenuTrigger } from "~/components/ui/dropdown-menu";
import { AudioPlayer } from "~/components/audio-player";
import { EntityRef } from "~/components/entity-ref";
import { FileRow, FileChip } from "~/components/file-row";
import { VoiceRecorder } from "~/components/voice-recorder";
import { create, db, get, update, type Block, type Entity, type Kind, type Job, api, aiStatus, preferences, upload, fileURL, useFileURL, useDb, flush, startTranscription, applyTranscript } from "~/lib/data";
import { SaveStatus } from "~/components/save-status";
import { TranscriptActions } from "~/components/transcript-actions";
import { documentValue, isDocumentBlock, saveDocument } from "~/lib/document";
import { MarkdownPlugin } from "~/lib/markdown-plugin";
import { indentList, listDepth, listNumber } from "~/lib/list";
import { Checkbox } from "~/components/ui/checkbox";

type TextType = "paragraph" | "heading" | "quote";
type TextBlock = Extract<Block, { type: TextType }>;

const MentionPlugin = createPlatePlugin({
  key: "entity-mention",
  node: { isElement: true, isInline: true, isVoid: true },
}).withComponent(({ attributes, children, element }: PlateElementProps) => (
  <span {...attributes}>
    <span contentEditable={false}><EntityRef kind={element.kind as Kind} id={element.entityId as string} /></span>
    {children}
  </span>
));

const FilePlugin = createPlatePlugin({
  key: "file-attachment",
  node: { isElement: true, isInline: true, isVoid: true },
}).withComponent(({ attributes, children, element, editor }: PlateElementProps) => {
 const url = useFileURL(typeof element.fileId === "string" ? element.fileId : undefined);
 return (
  <span {...attributes} id={`attachment-${element.placementId ?? element.attachmentIndex}`}>
    <span contentEditable={false}><FileChip name={String(element.name)} size={String(element.size)} url={url ?? (typeof element.url === "string" ? element.url : undefined)} onRemove={() => {
      const path = editor.api.findPath(element);
      if (path) editor.tf.removeNodes({ at: path });
      editor.tf.focus();
    }} /></span>
    {children}
  </span>
); });

const ListPlugin = createPlatePlugin({ key: "list-item", node: { isElement: true } })
  .withComponent(({ attributes, children, element, editor }: PlateElementProps) => {
    const path = editor.api.findPath(element);
    const style = { marginLeft: `${listDepth(element) * 24}px` };
    return element.ordered
      ? <ol {...attributes} start={path ? listNumber(editor.children, path[0]) : 1} style={style} className="list-decimal pl-6"><li>{children}</li></ol>
      : <ul {...attributes} style={style} className="list-disc pl-6"><li>{children}</li></ul>;
  });

const TodoPlugin = createPlatePlugin({ key: "todo", node: { isElement: true } })
  .withComponent(({ attributes, children, element, editor }: PlateElementProps) => (
    <div {...attributes} className="flex items-baseline gap-2">
      <span contentEditable={false}><Checkbox aria-label="Complete checklist item" checked={!!element.checked} onCheckedChange={(checked) => {
        const path = editor.api.findPath(element);
        if (path) editor.tf.setNodes({ checked: !!checked }, { at: path });
      }} /></span>
      <span className={element.checked ? "text-muted-foreground line-through" : ""}>{children}</span>
    </div>
  ));

const CodeBlockPlugin = createPlatePlugin({ key: "code-block", node: { isElement: true } })
  .withComponent(({ attributes, children }: PlateElementProps) => (
    <pre {...attributes} className="my-2 rounded-md bg-muted/50 p-3 font-mono text-sm whitespace-pre-wrap">{children}</pre>
  ));


function TextBlockEditor({ block, entity, index, onFocus, onCaret, onSlash, continuous = false, onReady, onSelection }: {
  block: TextBlock;
  continuous?: boolean;
  onReady?: (editor: PlateEditor) => void;
  onSelection?: (editor: PlateEditor) => void;
  entity: Entity;
  index: number;
  onFocus: (index: number, editor: PlateEditor) => void;
  onCaret: (event?: MouseEvent<HTMLDivElement>) => void;
  onSlash: () => void;
}) {
  useDb();
  const [ghost, setGhost] = useState<{ text: string; point: any; left: number; top: number } | null>(null);
  const completionTimer = useRef<ReturnType<typeof setTimeout> | null>(null);
  const completionAbort = useRef<AbortController | null>(null);
  const completionVersion = useRef(0);
  const completionPoint = useRef<TRange["anchor"] | null>(null);
  const textSurface = useRef<HTMLDivElement>(null);
  useEffect(() => () => { if (completionTimer.current) clearTimeout(completionTimer.current); completionAbort.current?.abort(); }, []);
  const [mention, setMention] = useState<{ query: string; range: TRange } | null>(null);
  const [active, setActive] = useState(0);
  const [dismissed, setDismissed] = useState(false);
  const editor = usePlateEditor({
    plugins: [BasicBlocksPlugin, BasicMarksPlugin, MentionPlugin, FilePlugin, ListPlugin, TodoPlugin, CodeBlockPlugin, HorizontalRulePlugin, MarkdownPlugin],
    value: block.value ?? [{ type: block.type === "heading" ? "h2" : block.type === "quote" ? "blockquote" : "p", children: [{ text: block.text }] }],
  });
  useEffect(() => { onReady?.(editor); }, [editor]);
  const cancelCompletion = () => { completionVersion.current++; completionPoint.current = null; if (completionTimer.current) clearTimeout(completionTimer.current); completionAbort.current?.abort(); setGhost(null); };
  const suggest = () => {
    cancelCompletion();
    if (!aiStatus?.completion.available || !preferences?.enable_ai || !preferences?.autocomplete || !editor.selection || !editor.api.isCollapsed()) return;
    const point = structuredClone(editor.selection.anchor);
    let leaf: any = editor;
    for (const index of point.path) leaf = leaf.children?.[index];
    if (!leaf || typeof leaf.text !== "string" || leaf.code || point.offset !== leaf.text.length || String(editor.children[point.path[0]]?.type).startsWith("code")) return;
    const start = editor.api.start([point.path[0]]); if (!start) return;
    const currentParagraph = editor.api.string({ anchor: start, focus: point });
    // Keep paragraph boundaries in the model's context instead of running blocks together.
    const context = [...editor.children.slice(0, point.path[0]).map((node) => NodeApi.string(node)), currentParagraph].join("\n").slice(-3500);
    if (currentParagraph.trim().length < 8 || /[.!?。！？]["'”’\])}]*\s*$/.test(currentParagraph) || /(?:^|\s)@[^\n]*$/.test(currentParagraph)) return;
    const version = completionVersion.current;
    completionPoint.current = point;
    completionTimer.current = setTimeout(() => {
      const controller = new AbortController(); completionAbort.current = controller;
      void api<{text: string; verbatim?: boolean}>("/api/ai/completion", "POST", {context, requestId: crypto.randomUUID()}, controller.signal).then((result) => {
        if (controller.signal.aborted || version !== completionVersion.current || !editor.api.isFocused() || !result.text || JSON.stringify(editor.selection?.anchor) !== JSON.stringify(point) || !editor.api.isCollapsed()) return;
        const selection = window.getSelection(); if (!selection?.rangeCount || !textSurface.current) return;
        const rect = selection.getRangeAt(0).getBoundingClientRect(); const bounds = textSurface.current.getBoundingClientRect();
        const text = result.verbatim ? result.text : /\s$/.test(context) || /^\s|^[.,!?;:]/.test(result.text) ? result.text : ` ${result.text}`;
        setGhost({text, point, left: rect.left - bounds.left, top: rect.top - bounds.top});
      }).catch(() => {});
    }, 600);
  };
  const matches = [...db.task, ...db.goal, ...db.habit].filter((item) =>
    item.title.toLowerCase().includes(mention?.query.toLowerCase() ?? ""),
  ).slice(0, 8);
  const link = (item: Entity) => {
    if (!mention) return;
    editor.tf.select(mention.range);
    editor.tf.insertNodes({ type: "entity-mention", kind: item.kind, entityId: item.id, children: [{ text: "" }] });
    editor.tf.move({ distance: 1 });
    editor.tf.insertText(" ");
    setMention(null);
    editor.tf.focus();
  };
  return (
    <div ref={textSurface} className="relative" onMouseMove={onCaret}>
      <Plate editor={editor} onSelectionChange={() => { onSelection?.(editor); if (completionPoint.current && (!editor.api.isCollapsed() || JSON.stringify(editor.selection?.anchor) !== JSON.stringify(completionPoint.current))) cancelCompletion(); }} onValueChange={({ value }) => {
        suggest();
        const point = editor.selection?.anchor;
        if (point && editor.api.isCollapsed()) {
          const start = editor.api.start([point.path[0]]);
          if (!start) return;
          const before = editor.api.string({ anchor: start, focus: point });
          const match = before.match(/(?:^|\s)@([^@\n]*)$/);
          if (match && !dismissed) {
            const anchor = editor.api.before(point, { distance: match[1].length + 1 });
            if (anchor) setMention({ query: match[1], range: { anchor, focus: point } });
            setActive(0);
          } else setMention(null);
        } else setMention(null);
        if (JSON.stringify(value) === JSON.stringify(block.value)) return;
        const text = value.map((node) => NodeApi.string(node)).join("\n");
        update(entity.kind, entity.id, {
          blocks: continuous ? saveDocument(get(entity.kind, entity.id)?.blocks ?? entity.blocks, value as Value, text) : entity.blocks.map((item, i) => i === index ? { ...block, text, value: value as Value } : item),
        });
      }}>
        <PlateContent aria-label="Document text" placeholder="Write…" onFocus={() => onFocus(index, editor)} onBlur={cancelCompletion}
          onKeyDown={(event) => {
            if (event.nativeEvent.isComposing) { cancelCompletion(); return; }
            if (event.key === "Tab" && (event.shiftKey || !mention || !matches[active]) && indentList(editor, event.shiftKey)) { event.preventDefault(); cancelCompletion(); return; }
            if (ghost && event.key === "Tab" && !event.shiftKey) { event.preventDefault(); const text = ghost.text; cancelCompletion(); editor.tf.insertText(text); return; }
            if (ghost && event.key === "Escape") { event.preventDefault(); cancelCompletion(); return; }
            cancelCompletion();
            if (event.key === "@") setDismissed(false);
            if (mention) {
              if (event.key === "Escape") { event.preventDefault(); setMention(null); setDismissed(true); return; }
              if (event.key === "ArrowDown" || event.key === "ArrowUp") {
                event.preventDefault();
                setActive((n) => matches.length ? (n + (event.key === "ArrowDown" ? 1 : -1) + matches.length) % matches.length : 0);
                return;
              }
              if ((event.key === "Enter" || event.key === "Tab") && matches[active]) { event.preventDefault(); link(matches[active]); return; }
            }
            const point = editor.selection?.anchor;
            if (!point || !editor.api.isCollapsed()) return;
            const start = editor.api.start([point.path[0]]);
            if (!start) return;
            const range = { anchor: start, focus: point };
            const before = editor.api.string(range);
            if (event.key === "/" && !before) { event.preventDefault(); onSlash(); return; }

          }}
          className="min-h-48 w-full font-content text-[17px] leading-relaxed outline-none whitespace-pre-wrap break-words [&_:is(h1,h2,h3,h4,h5,h6)]:pb-3 [&_h1]:text-3xl [&_h2]:text-2xl [&_h3]:text-xl [&_h4]:text-lg [&_h5]:font-semibold [&_h6]:font-semibold [&_blockquote]:border-l-2 [&_blockquote]:border-border [&_blockquote]:pl-4 [&_blockquote]:text-muted-foreground [&_code]:rounded [&_code]:bg-muted [&_code]:px-1 [&_code]:font-mono [&_code]:text-sm"
        />
      </Plate>
      {ghost && <span aria-hidden="true" className="pointer-events-none absolute max-w-full whitespace-pre-wrap font-content text-[17px] leading-relaxed text-muted-foreground/60" style={{left: ghost.left, top: ghost.top}}>{ghost.text}</span>}
      {mention && <div className="absolute top-full left-0 z-40 mt-2 w-72 max-w-full rounded-lg bg-popover p-1 font-sans shadow-md ring-1 ring-border" role="listbox" aria-label="Link a task, goal, or habit">
        {matches.length ? matches.map((item, i) => <Button key={item.id} role="option" aria-selected={i === active} variant="ghost" className={`h-auto w-full justify-between gap-3 px-3 py-2 ${i === active ? "bg-muted" : ""}`} onMouseDown={(event) => event.preventDefault()} onClick={() => link(item)}>
          <span className="truncate">{item.title}</span><span className="text-xs text-muted-foreground">{item.kind}</span>
        </Button>) : <p className="px-3 py-2 text-sm text-muted-foreground">No matching items</p>}
      </div>}
    </div>
  );
}

const slashGroups: { heading: string; items: string[] }[] = [
  { heading: "Basic", items: ["paragraph", "heading", "quote", "divider"] },
  { heading: "Personal", items: ["task", "goal", "habit"] },
  { heading: "Media", items: ["image", "audio", "video", "file"] },
];

function newBlock(type: TextType | "divider"): Block {
  return type === "divider" ? { type: "divider" } : { type, text: "" };
}

function BlockView({
  block,
  entity,
  index,
  onFocus,
  onCaret,
  onSlash,
}: {
  block: Block;
  entity: Entity;
  index: number;
  onFocus: (index: number, editor: PlateEditor) => void;
  onCaret: (event?: MouseEvent<HTMLDivElement>) => void;
  onSlash: () => void;
}) {
  const mediaURL = useFileURL("fileId" in block ? block.fileId : undefined) ?? ("url" in block ? block.url : undefined);
  if (block.type === "divider") return <Separator className="my-2" />;
  if (block.type === "ref") return <EntityRef kind={block.kind} id={block.id} />;
  if (block.type === "audio") return <AudioPlayer name={block.name} duration={block.duration} url={mediaURL} />;
  if (block.type === "file") return <FileRow block={block} entity={entity} />;
  if (block.type === "image") return mediaURL
    ? <figure><img src={mediaURL} alt={block.name} className="max-h-96 max-w-full rounded-md object-contain" /><figcaption className="text-xs text-muted-foreground">{block.name}</figcaption></figure>
    : <p className="text-sm text-muted-foreground"><ImageIcon className="inline size-4" /> {block.name} · no file attached</p>;
  if (block.type === "video") return mediaURL
    ? <video src={mediaURL} controls className="max-h-96 max-w-full rounded-md" aria-label={block.name} />
    : <p className="text-sm text-muted-foreground">{block.name} · no file attached</p>;

  return <TextBlockEditor key={`${entity.id}-${index}-${block.type}`} block={block} entity={entity} index={index} onFocus={onFocus} onCaret={onCaret} onSlash={onSlash} />;
}

export function DocumentEditor({ entity, audioFirst = false }: { entity: Entity; audioFirst?: boolean }) {
  useDb();
  const location = useLocation();
  const [caret, setCaret] = useState<{ left: number; top: number } | null>(null);
  const surface = useRef<HTMLDivElement>(null);
  const activeEditor = useRef<PlateEditor | null>(null);
  const hoveredBlock = useRef(0);
  const selectedRange = useRef<TRange | null>(null);
  const [selectionToolbar, setSelectionToolbar] = useState<{ left: number; top: number } | null>(null);
  const [blockMenu, setBlockMenu] = useState(false);
  const [textMenu, setTextMenu] = useState(false);
  const positionCaret = (event?: MouseEvent<HTMLDivElement>) => {
    if (!surface.current || window.getSelection()?.isCollapsed === false) return;
    const bounds = surface.current.getBoundingClientRect();
    let block = event?.target instanceof Element ? event.target.closest('[data-slate-node="element"]') : null;
    const content = surface.current.querySelector('[data-slate-editor]');
    if (block && content) {
      while (block.parentElement && block.parentElement !== content) block = block.parentElement;
      if (block.parentElement === content) {
        hoveredBlock.current = Array.from(content.children).indexOf(block);
        const rect = block.getBoundingClientRect();
        const left = Math.max(8 - bounds.left, rect.left - bounds.left - 58);
        const top = rect.top - bounds.top;
        setCaret((current) => current?.left === left && current?.top === top ? current : { left, top });
        return;
      }
    }
    const selection = window.getSelection();
    if (!selection?.rangeCount || !surface.current.contains(selection.anchorNode)) return;
    const rect = selection.getRangeAt(0).getBoundingClientRect();
    if (rect.height) {
      hoveredBlock.current = activeEditor.current?.selection?.anchor.path[0] ?? 0;
      setCaret({ left: -58, top: rect.top - bounds.top });
    }
  };
  const selectionFrame = useRef<number | null>(null);
  useEffect(() => () => { if (selectionFrame.current !== null) cancelAnimationFrame(selectionFrame.current); }, []);
  const syncSelection = (editor: PlateEditor) => {
    if (selectionFrame.current !== null) cancelAnimationFrame(selectionFrame.current);
    selectionFrame.current = requestAnimationFrame(() => {
      const selection = window.getSelection();
      const root = surface.current;
      if (!root || !selection?.rangeCount || !root.contains(selection.anchorNode)) return;
      if (selection.isCollapsed) { if (!textMenu) setSelectionToolbar(null); return; }
      if (editor.selection) selectedRange.current = structuredClone(editor.selection);
      const rect = selection.getRangeAt(0).getBoundingClientRect();
      const bounds = root.getBoundingClientRect();
      setSelectionToolbar({ left: Math.max(0, Math.min(rect.left - bounds.left, bounds.width - 310)), top: rect.top - bounds.top - 42 });
    });
  };
  useEffect(() => {
    const dismiss = (event: PointerEvent) => {
      const target = event.target;
      if (!(target instanceof Element) || surface.current?.contains(target) || target.closest('[role="menu"]')) return;
      setSelectionToolbar(null);
      setCaret(null);
    };
    document.addEventListener("pointerdown", dismiss);
    return () => document.removeEventListener("pointerdown", dismiss);
  }, []);
  const transformBlock = (type: string, selection = false) => {
    const editor = activeEditor.current;
    if (!editor) return;
    if (selection && selectedRange.current) editor.tf.select(selectedRange.current);
    const index = selection ? editor.selection?.anchor.path[0] ?? 0 : hoveredBlock.current;
    editor.tf.setNodes({ type: type === "bullet" || type === "numbered" ? "list-item" : type, ordered: type === "numbered", indent: 0 }, { at: [index] });
    editor.tf.focus();
  };
  const types = [
    { type: "p", label: "Text", icon: TypeIcon },
    { type: "h1", label: "Heading 1", icon: Heading1Icon },
    { type: "h2", label: "Heading 2", icon: Heading2Icon },
    { type: "h3", label: "Heading 3", icon: Heading3Icon },
    { type: "blockquote", label: "Quote", icon: QuoteIcon },
    { type: "bullet", label: "Bulleted list", icon: ListIcon },
    { type: "numbered", label: "Numbered list", icon: ListOrderedIcon },
  ];
  const [slash, setSlash] = useState(false);
  const fileInput = useRef<HTMLInputElement>(null);
  const mediaType = useRef<"image" | "audio" | "video" | "file">("file");
  useEffect(() => {
    if (!/^#attachment-[\w-]+$/.test(location.hash)) return;
    const frame = requestAnimationFrame(() => document.getElementById(location.hash.slice(1))?.scrollIntoView({ block: "center" }));
    return () => cancelAnimationFrame(frame);
  }, [location.hash, entity.id]);

  const removeAudio = (block: Block) => {
    update(entity.kind, entity.id, { blocks: entity.blocks.filter((item) => item !== block) });
    if ("url" in block && block.url?.startsWith("blob:") && !Object.values(db).flat().some((item) => item.blocks.some((other) => "url" in other && other.url === block.url))) {
      URL.revokeObjectURL(block.url);
    }
  };
  const chooseMedia = (type: "image" | "audio" | "video" | "file") => {
    mediaType.current = type;
    if (!fileInput.current) return;
    fileInput.current.accept = type === "file" ? "" : `${type}/*`;
    fileInput.current.multiple = type === "audio";
    fileInput.current.click();
  };
  const add = (block: Block) => update(entity.kind, entity.id, { blocks: [...(get(entity.kind, entity.id)?.blocks ?? []), block] });

  const reserve = () => {
    const editor = activeEditor.current; const id = crypto.randomUUID();
    if (editor) { editor.tf.insertNodes({type: "p", id, children: [{text: ""}]}); const blocks = get(entity.kind, entity.id)?.blocks ?? entity.blocks; update(entity.kind, entity.id, {blocks: saveDocument(blocks, structuredClone(editor.children), "")}); }
    else { const blocks = get(entity.kind, entity.id)?.blocks ?? entity.blocks; const value = documentValue(blocks); value.push({type: "p", id, children: [{text: ""}]}); update(entity.kind, entity.id, {blocks: saveDocument(blocks, value, "")}); }
    return id;
  };
  const transcribe = async (fileId: string) => { const anchorId = reserve(); await startTranscription(entity, fileId, anchorId); };
  const insertTranscript = async (job: Job) => { const anchorId = reserve(); await applyTranscript(entity, job, anchorId); };
  const runSlash = async (type: string) => {
    setSlash(false);
    setCaret(null);
    if (type === "task" || type === "goal" || type === "habit") {
      const kind = type as Kind;
      const item = await create(kind, { kind, title: `New ${type}`, tags: [], blocks: [] });
      const editor = activeEditor.current;
      if (editor) {
        editor.tf.insertNodes({ type: "entity-mention", kind, entityId: item.id, children: [{ text: "" }] });
        editor.tf.move({ distance: 1 });
        editor.tf.insertText(" ");
        editor.tf.focus();
      }
      return;
    }
    if (type === "image" || type === "audio" || type === "video" || type === "file") {
      chooseMedia(type);
      return;
    }
    const editor = activeEditor.current;
    if (type === "divider" && editor?.selection) {
      editor.tf.insertNodes([{ type: "hr", children: [{ text: "" }] }, { type: "p", children: [{ text: "" }] }]);
      editor.tf.focus();
    } else if (editor?.selection) {
      editor.tf.setNodes({ type: type === "heading" ? "h2" : type === "quote" ? "blockquote" : "p" }, { at: [editor.selection.anchor.path[0]] });
      editor.tf.focus();
    } else add(newBlock(type as TextType | "divider"));
  };

  return (
    <div ref={surface} className="relative pb-[clamp(10rem,30dvh,25rem)]" onKeyDownCapture={() => setCaret(null)} onMouseLeave={() => { if (!slash && !blockMenu) setCaret(null); }}>
      <input ref={fileInput} type="file" className="hidden" aria-label="Attach file" onChange={async (event) => {
        const files = Array.from(event.target.files ?? []); event.target.value = ""; if (!files.length) return;
        const type = mediaType.current; const editor = activeEditor.current; const selection = editor?.selection ? structuredClone(editor.selection) : null;
        try {
          for (const file of files) {
            const record = await upload(file, file.name); const placementId = crypto.randomUUID();
            if (type === "file" && editor) {
              if (selection) editor.tf.select(selection);
              editor.tf.insertNodes([{type: "file-attachment", fileId: record.id, placementId, name: record.name, size: `${Math.ceil(record.size / 1024)} KB`, children: [{text: ""}]}, {text: " "}]); editor.tf.focus();
            } else add(type === "audio" ? {type, fileId: record.id, placementId: record.id, name: record.name, duration: 0, url: fileURL(record.id)} : {type: type === "image" ? "image" : "video", fileId: record.id, placementId, name: record.name, url: fileURL(record.id)});
          }
          await flush(entity.kind, entity.id);
        } catch { toast.error("Could not upload attachment"); }
      }} />
      {audioFirst && <div className="mb-8 flex flex-col gap-3">
        {entity.blocks.map((block, i) => block.type === "audio" ? <div key={block.url ?? i} id={`attachment-${block.placementId ?? i}`}><AudioPlayer name={block.name} duration={block.duration} url={fileURL(block.fileId) ?? block.url} onRemove={() => removeAudio(block)} />{block.fileId && <TranscriptActions entity={entity} fileId={block.fileId} onTranscribe={() => { void transcribe(block.fileId!).catch(() => toast.error("Could not start transcription")); }} onInsert={insertTranscript} />}</div> : null)}
        <div className="flex items-center gap-1">
          <Button variant="ghost" size="sm" className="text-muted-foreground" onClick={() => chooseMedia("audio")}><PaperclipIcon className="size-4" />Add audio</Button>
          <VoiceRecorder onFinish={async (blob, duration) => {
            try { const record = await upload(blob, `Voice note-${Date.now()}.${blob.type.includes("mp4") ? "m4a" : "webm"}`); add({type: "audio", fileId: record.id, placementId: record.id, name: record.name, duration, url: fileURL(record.id)}); await flush(entity.kind, entity.id); if (preferences?.enable_transcription) await transcribe(record.id); }
            catch { toast.error("Could not save or transcribe the recording"); }
          }} />
        </div>
      </div>}
      {selectionToolbar && <div role="toolbar" aria-label="Text formatting" className="absolute z-40 flex items-center rounded-md bg-popover p-1 font-sans shadow-lg ring-1 ring-border" style={selectionToolbar} onMouseDown={(event) => event.preventDefault()}>
        <DropdownMenu open={textMenu} onOpenChange={setTextMenu}>
          <DropdownMenuTrigger asChild><Button variant="ghost" size="sm" className="h-7 gap-1 px-2 text-xs" aria-label="Turn selected text into">{types.find((item) => item.type === activeEditor.current?.children[activeEditor.current?.selection?.anchor.path[0] ?? 0]?.type)?.label ?? "Text"}<ChevronDownIcon className="size-3" /></Button></DropdownMenuTrigger>
          <DropdownMenuContent className="w-48" align="start">
            <DropdownMenuLabel>Turn into</DropdownMenuLabel>
            {types.map(({ type, label, icon: Icon }) => <DropdownMenuItem key={type} onSelect={() => transformBlock(type, true)}><Icon className="size-4" />{label}</DropdownMenuItem>)}
          </DropdownMenuContent>
        </DropdownMenu>
        <Separator orientation="vertical" className="mx-1 h-5" />
        {[{ mark: "bold", label: "Bold", icon: BoldIcon }, { mark: "italic", label: "Italic", icon: ItalicIcon }, { mark: "underline", label: "Underline", icon: UnderlineIcon }, { mark: "strikethrough", label: "Strikethrough", icon: StrikethroughIcon }, { mark: "code", label: "Code", icon: CodeIcon }].map(({ mark, label, icon: Icon }) => <Button key={mark} variant="ghost" size="icon" className="size-7 rounded-sm" aria-label={label} title={label} onClick={() => {
          const editor = activeEditor.current;
          if (!editor || !selectedRange.current) return;
          editor.tf.select(selectedRange.current);
          editor.tf.toggleMark(mark);
          editor.tf.focus();
        }}><Icon className="size-3.5" /></Button>)}
      </div>}
      <div
        className={`absolute z-30 flex items-center gap-0.5 text-muted-foreground ${caret || slash || blockMenu ? "" : "invisible pointer-events-none"}`}
        style={{ left: caret?.left ?? -58, top: caret?.top ?? 0 }}
        onMouseDown={(event) => event.preventDefault()}
        aria-label="Block controls"
      >
        <Popover open={slash} onOpenChange={setSlash}>
          <PopoverTrigger asChild>
            <Button variant="ghost" size="icon" className="size-6 rounded-sm" aria-label="Insert block" title="Insert block" onClick={() => {
              const editor = activeEditor.current;
              if (editor) {
                const index = hoveredBlock.current + 1;
                editor.tf.insertNodes({ type: "p", children: [{ text: "" }] }, { at: [index] });
                hoveredBlock.current = index;
                const start = editor.api.start([index]);
                if (start) editor.tf.select(start);
              }
            }}><PlusIcon className="size-4" /></Button>
          </PopoverTrigger>
          <PopoverContent align="start" className="w-64 p-0">
            <Command>
              <CommandInput placeholder="Type a command…" />
              <CommandList>
                <CommandEmpty>Nothing found</CommandEmpty>
                {slashGroups.map((group) => (
                  <div key={group.heading}>
                    <CommandSeparator />
                    <CommandGroup heading={group.heading}>
                      {group.items.map((item) => (
                        <CommandItem key={item} value={item} onSelect={() => runSlash(item)}>
                          {item === "paragraph" ? "Text" : item[0].toUpperCase() + item.slice(1)}
                        </CommandItem>
                      ))}
                    </CommandGroup>
                  </div>
                ))}
              </CommandList>
            </Command>
          </PopoverContent>
        </Popover>

        <DropdownMenu open={blockMenu} onOpenChange={setBlockMenu}>
          <DropdownMenuTrigger asChild><Button variant="ghost" size="icon" className="size-6 rounded-sm" aria-label="Block options" title="Turn into"><MoreHorizontalIcon className="size-4" /></Button></DropdownMenuTrigger>
          <DropdownMenuContent align="start" className="w-48">
            <DropdownMenuLabel>Turn into</DropdownMenuLabel>
            {types.map(({ type, label, icon: Icon }) => <DropdownMenuItem key={type} onSelect={() => transformBlock(type)}><Icon className="size-4" />{label}</DropdownMenuItem>)}
          </DropdownMenuContent>
        </DropdownMenu>
      </div>

      <TextBlockEditor
        key={`${entity.id}-${entity.editorVersion ?? 0}`}
        block={{ type: "paragraph", text: "", value: documentValue(entity.blocks) }}
        entity={entity}
        index={0}
        continuous
        onReady={(editor) => { activeEditor.current = editor; }}
        onSelection={syncSelection}
        onFocus={(_i, editor) => { activeEditor.current = editor; }}
        onCaret={positionCaret}
        onSlash={() => { positionCaret(); setSlash(true); }}
      />
      <div className="mt-4 flex flex-col gap-2">
        {entity.blocks.map((block, i) => isDocumentBlock(block) || (audioFirst && block.type === "audio") ? null : (
          <div key={"url" in block ? block.url ?? i : i} id={`attachment-${"placementId" in block ? block.placementId ?? i : i}`}>
            {block.type === "audio" ? <><AudioPlayer name={block.name} duration={block.duration} url={fileURL(block.fileId) ?? block.url} onRemove={() => removeAudio(block)} />{block.fileId && <TranscriptActions entity={entity} fileId={block.fileId} onTranscribe={() => { void transcribe(block.fileId!).catch(() => toast.error("Could not start transcription")); }} onInsert={insertTranscript} />}</> : <BlockView block={block} entity={entity} index={i} onFocus={(_i, editor) => { activeEditor.current = editor; }} onCaret={positionCaret} onSlash={() => { positionCaret(); setSlash(true); }} />}
          </div>
        ))}
      </div>

      <SaveStatus entity={entity} />
    </div>
  );
}
