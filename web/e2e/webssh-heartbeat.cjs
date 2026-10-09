// Deterministic lifecycle/timer tests of the production heartbeat controller.
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');
const ts = require('typescript');
const source = fs.readFileSync(path.join(__dirname, '../src/pages/WebSSH/heartbeat.ts'), 'utf8');
const compiled = ts.transpileModule(source, {compilerOptions: {module: ts.ModuleKind.CommonJS}}).outputText;
function setup() {
  let now = 0, next = 0, closed = false, pings = 0;
  const intervals = new Map(), timeouts = new Map(), listeners = new Map();
  const add = (event, fn) => { if (!listeners.has(event)) listeners.set(event, new Set()); listeners.get(event).add(fn); };
  const remove = (event, fn) => listeners.get(event)?.delete(fn);
  const doc = {visibilityState: 'visible', addEventListener: add, removeEventListener: remove};
  const socket = {readyState: 1, send: data => {assert.deepEqual(JSON.parse(data), {type: 'ping'}); pings++;}};
  const exports = {};
  vm.runInNewContext(compiled, {exports, Date: {now: () => now}, WebSocket: {OPEN: 1}, document: doc, window: {
    setInterval: (fn, ms) => {assert.equal(ms, 25000); intervals.set(++next, fn); return next;},
    clearInterval: id => intervals.delete(id),
    setTimeout: (fn, ms) => {timeouts.set(++next, {fn, at: now + ms}); return next;},
    clearTimeout: id => timeouts.delete(id), addEventListener: add, removeEventListener: remove,
  }});
  const controller = exports.startWebSSHHeartbeat(socket, () => {closed = true;});
  return {
    controller, doc, socket,
    tickAt(ms) {now = ms; for (const fn of [...intervals.values()]) fn();},
    advance(ms) {now = ms; for (const [id, t] of [...timeouts]) if (t.at <= now) {timeouts.delete(id); t.fn();}},
    emit(event) {for (const fn of listeners.get(event) || []) fn();},
    get closed() {return closed;}, get pings() {return pings;},
    get pending() {return timeouts.size + intervals.size + [...listeners.values()].reduce((n, s) => n + s.size, 0);},
  };
}
const delayed = setup();
delayed.tickAt(76000); assert.equal(delayed.closed, false);
delayed.tickAt(600001); assert.equal(delayed.closed, false, 'probe before closing a delayed timer');
delayed.advance(605000); delayed.controller.alive(); delayed.advance(620000);
assert.equal(delayed.closed, false, 'queued pong/output cancels timeout');
delayed.controller.stop(); assert.equal(delayed.pending, 0);
const dead = setup();
dead.tickAt(600001); dead.advance(605000); dead.emit('online'); dead.emit('pageshow');
dead.advance(610001); assert.equal(dead.closed, true, 'events cannot extend an unanswered probe');
assert.equal(dead.pending, 0);
const idle = setup();
for (let now = 25000; now <= 30 * 60000; now += 25000) {idle.tickAt(now); idle.controller.alive();}
assert.equal(idle.closed, false); assert.equal(idle.pings, 72);
idle.emit('pageshow'); idle.emit('online'); idle.emit('visibilitychange');
assert.equal(idle.pings, 75);
idle.doc.visibilityState = 'hidden'; idle.emit('visibilitychange'); assert.equal(idle.pings, 75);
idle.controller.stop(); assert.equal(idle.pending, 0); idle.emit('online'); assert.equal(idle.pings, 75);
const replaced = setup(); replaced.tickAt(600001); replaced.controller.stop(); replaced.advance(620000);
assert.equal(replaced.closed, false, 'old session timers cannot close a replacement session');
const failed = setup(); failed.socket.send = () => {throw new Error('transport unavailable');};
failed.tickAt(25000); assert.equal(failed.pending, 0);
console.log('PASS WebSSH heartbeat: idle, delayed timers, resume events, bounded probe, cleanup, send failure');
