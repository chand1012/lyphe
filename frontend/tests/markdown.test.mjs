import assert from 'node:assert/strict';
import { test } from 'node:test';
import { createPlateEditor, createPlatePlugin } from 'platejs/react';
import { BasicBlocksPlugin, BasicMarksPlugin, HorizontalRulePlugin } from '@platejs/basic-nodes/react';
import { NodeApi } from 'platejs';
import { MarkdownPlugin } from '../app/lib/markdown-plugin.ts';

function editorFor(text = '') {
  const editor = createPlateEditor({
    plugins: [BasicBlocksPlugin, BasicMarksPlugin, HorizontalRulePlugin,
      ...['list-item', 'todo', 'code-block'].map(key => createPlatePlugin({key, node: {isElement: true}})), MarkdownPlugin],
    value: [{type: 'p', children: [{text}]}],
  });
  editor.tf.select(editor.api.end([]));
  return editor;
}
function type(editor, text) { for (const letter of text) editor.tf.insertText(letter); }

test('Markdown prefixes convert blocks and remove the markers', () => {
  for (const [prefix, typeName] of [['# ', 'h1'], ['### ', 'h3'], ['> ', 'blockquote'], ['- ', 'list-item'], ['2. ', 'list-item'], ['[ ] ', 'todo'], ['[x] ', 'todo']]) {
    const editor = editorFor(); type(editor, prefix);
    assert.equal(editor.children[0].type, typeName, prefix);
    assert.equal(NodeApi.string(editor.children[0]), '', prefix);
  }
});

test('Inline Markdown preserves surrounding text and applies marks', () => {
  for (const [source, content, marks] of [['**bold**', 'bold', ['bold']], ['*italic*', 'italic', ['italic']], ['***both***', 'both', ['bold','italic']], ['~~removed~~', 'removed', ['strikethrough']], ['`code`', 'code', ['code']]]) {
    const editor = editorFor('Before '); type(editor, source); type(editor, ' after');
    assert.equal(NodeApi.string(editor.children[0]), `Before ${content} after`, source);
    const leaf = editor.children[0].children.find(node => node.text === content);
    for (const mark of marks) assert.equal(leaf?.[mark], true, `${source}: ${mark}`);
    assert.ok(!editor.children[0].children.at(-1)[marks[0]], 'following text should be unformatted');
  }
});

test('Dividers leave a writable paragraph and fences keep code literal', () => {
  const divider = editorFor(); type(divider, '---');
  assert.equal(divider.children[0].type, 'hr');
  assert.equal(divider.children[1].type, 'p');
  type(divider, 'After'); assert.equal(NodeApi.string(divider.children[1]), 'After');
  const code = editorFor(); type(code, '```js'); code.tf.insertBreak();
  assert.equal(code.children[0].type, 'code-block');
  type(code, '**literal**'); assert.equal(NodeApi.string(code.children[0]), '**literal**');
  code.tf.insertBreak(); code.tf.insertBreak();
  assert.equal(code.children[1].type, 'p');
});

test('Ordinary underscores and editing away from the cursor stay literal', () => {
  const editor = editorFor(); type(editor, 'some_variable_name');
  assert.equal(NodeApi.string(editor.children[0]), 'some_variable_name');
});

test('Undo restores typed Markdown instead of removing the author’s words', () => {
  const inline = editorFor(); type(inline, '**bold**'); inline.tf.undo();
  assert.equal(NodeApi.string(inline.children[0]), '**bold**');
  const heading = editorFor(); type(heading, '# '); heading.tf.undo();
  assert.equal(heading.children[0].type, 'p');
  assert.equal(NodeApi.string(heading.children[0]), '# ');
});
