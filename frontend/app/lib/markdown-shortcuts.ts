export function markdownBlock(prefix: string): Record<string, unknown> | undefined {
  const heading = prefix.match(/^(#{1,6})$/);
  if (heading) return { type: `h${heading[1].length}` };
  if (prefix === ">") return { type: "blockquote" };
  if (/^[-*+]$/.test(prefix)) return { type: "list-item", ordered: false };
  if (/^\d+[.)]$/.test(prefix)) return { type: "list-item", ordered: true };
  if (/^(?:- )?\[([ xX])\]$/.test(prefix)) return { type: "todo", checked: /[xX]/.test(prefix) };
  if (/^(?:---|\*\*\*|___)$/.test(prefix)) return { type: "hr" };
  const fence = prefix.match(/^```([\w+-]*)$/);
  if (fence) return { type: "code-block", language: fence[1] };
}

// Match only the plain text leaf at the cursor, so inline references are never
// replaced and existing formatting on neighboring text is preserved.
export function markdownInline(text: string) {
  const rules: [RegExp, string[]][] = [
    [/(?:^|\s)(\*\*\*([^*\n]+)\*\*\*)$/, ["bold", "italic"]],
    [/(?:^|\s)(___([^_\n]+)___)$/, ["bold", "italic"]],
    [/(?:^|[^*])(\*\*([^*\n]+)\*\*)$/, ["bold"]],
    [/(?:^|\W)(__([^_\n]+)__)$/, ["bold"]],
    [/(~~([^~\n]+)~~)$/, ["strikethrough"]],
    [/(`([^`\n]+)`)$/, ["code"]],
    [/(?:^|[^*])(\*([^*\n]+)\*)$/, ["italic"]],
    [/(?:^|\W)(_([^_\n]+)_)$/, ["italic"]],
  ];
  for (const [pattern, marks] of rules) {
    const match = text.match(pattern);
    if (match && match[2].trim() && match[2] === match[2].trim()) {
      return { start: text.length - match[1].length, text: match[2], marks };
    }
  }
}
