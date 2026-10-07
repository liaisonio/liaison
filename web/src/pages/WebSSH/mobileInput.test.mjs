import { test } from 'node:test';
import assert from 'node:assert/strict';
import { controlInput } from './mobileInput.ts';
test('mobile Ctrl generates terminal control characters without altering pasted or non-Latin text', () => {
  assert.equal(controlInput('c'), '\x03');
  assert.equal(controlInput('D'), '\x04');
  assert.equal(controlInput('['), '\x1b');
  assert.equal(controlInput(' '), '\x00');
  for (const text of ['你好', 'ß', 'echo example\n', '\x1b[A']) assert.equal(controlInput(text), text);
});
