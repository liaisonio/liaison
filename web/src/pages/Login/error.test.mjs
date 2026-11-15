import assert from 'node:assert/strict';
import test from 'node:test';
import { loginError } from './error.ts';

test('login failures are localized for HTTP errors and application envelopes', () => {
  for (const reason of ['INVALID_PASSWORD', 'USER_NOT_FOUND']) {
    for (const error of [{ code: 401, reason, message: '密码错误' }, { response: { status: 401, data: { reason, message: '密码错误' } } }]) {
      assert.equal(loginError(error, (_, en) => en), 'Incorrect email or password');
      assert.equal(loginError(error, zh => zh), '邮箱或密码错误');
    }
  }
  assert.match(loginError({ reason: 'ACCOUNT_DISABLED' }, (_, en) => en), /account is disabled/);
  assert.match(loginError({ reason: 'ACCOUNT_DISABLED' }, zh => zh), /账号已禁用/);
  assert.match(loginError({ response: { status: 429 } }, (_, en) => en), /Too many/);
  assert.match(loginError({ code: 429 }, zh => zh), /过于频繁/);
});

test('unknown server and network errors never expose raw details', () => {
  for (const error of [null, undefined, {}, new Error('secret'), { code: 500, message: 'secret' }, { response: { status: 500, data: { message: 'secret' } } }]) {
    assert.equal(loginError(error, (_, en) => en), 'Unable to sign in. Please try again later');
    assert.equal(loginError(error, zh => zh), '登录失败，请稍后重试');
  }
});
