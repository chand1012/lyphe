import assert from 'node:assert/strict';
import { test } from 'node:test';
import { pageTitle } from '../app/lib/page-title.ts';
import { longDate } from '../app/lib/format.ts';

const empty = { journal: [], goal: [], task: [], habit: [] };

test('every page has a readable title before data loads', () => {
  for (const [path, title] of Object.entries({ '/': 'Lyphe', '/login': 'Sign in · Lyphe', '/register': 'Create an account · Lyphe', '/journal': 'Journal · Lyphe', '/journal/missing': 'Journal · Lyphe', '/goals': 'Goals · Lyphe', '/goals/missing': 'Goals · Lyphe', '/tasks': 'Tasks · Lyphe', '/habits': 'Habits · Lyphe', '/habits/missing': 'Habits · Lyphe', '/settings': 'Settings · Lyphe', '/unknown': 'Page not found · Lyphe' })) {
    assert.equal(pageTitle(path, '', empty), title);
  }
});

test('detail titles follow the displayed entity and its renamed title', () => {
  const entities = { journal: [{id: 'j1', kind: 'journal', title: 'Untitled', date: '2026-10-05'}], goal: [{id: 'g1', kind: 'goal', title: 'Finish the app'}], task: [{id: 't1', kind: 'task', title: 'Fix bugs'}], habit: [{id: 'h1', kind: 'habit', title: 'Walk daily'}] };
  assert.equal(pageTitle('/journal/j1', '', entities), `${longDate('2026-10-05')} · Lyphe`);
  entities.journal[0].title = 'My first entry';
  assert.equal(pageTitle('/journal/j1', '', entities), 'My first entry · Lyphe');
  assert.equal(pageTitle('/journal', '', entities), 'My first entry · Lyphe');
  assert.equal(pageTitle('/goals/g1', '', entities), 'Finish the app · Lyphe');
  assert.equal(pageTitle('/habits/h1', '', entities), 'Walk daily · Lyphe');
  assert.equal(pageTitle('/tasks', '?task=t1', entities), 'Fix bugs · Lyphe');
  assert.equal(pageTitle('/tasks', '?task=missing', entities), 'Tasks · Lyphe');
});
