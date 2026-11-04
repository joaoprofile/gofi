// /model writes ai.model and ai.models only: host, tiers, guard and every
// other line of .gofi.yaml stay as written.
const assert = require('assert');
const fs = require('fs');
const os = require('os');
const path = require('path');
const { persistSelectedModel } = require('../src/gofiConfig.js');

function project(yaml) {
	const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'gofi-model-'));
	fs.writeFileSync(path.join(dir, '.gofi.yaml'), yaml);
	return dir;
}
const read = (dir) => fs.readFileSync(path.join(dir, '.gofi.yaml'), 'utf8');

// Tiers and guard after the models list, a blank line inside the block.
let dir = project([
	'project:',
	'    name: x',
	'ai:',
	'    host: codex',
	'    model: old-model',
	'    models:',
	'        - old-model',
	'        # - other-model',
	'',
	'    tiers:',
	'        deep: big',
	'    guard: warn',
	'loop:',
	'    max_agents: 2',
	'',
].join('\n'));
assert.strictEqual(persistSelectedModel(dir, 'new-model'), 'written');
let out = read(dir);
assert.match(out, /\n    host: codex\n/);
assert.match(out, /\n    model: new-model\n/);
assert.match(out, /\n    models:\n        - new-model\n(        # - .*\n)*/);
assert.match(out, /# - old-model/);
assert.match(out, /# - other-model/);
assert.match(out, /\n    tiers:\n        deep: big\n    guard: warn\nloop:\n    max_agents: 2\n/);
assert.ok(!/claude-vscode/.test(out), 'no host is invented');

// No host, the model after the list: nothing invented, both rewritten.
dir = project('ai:\n    models:\n        - a\n    model: a\n');
persistSelectedModel(dir, 'b');
out = read(dir);
assert.ok(!/host:/.test(out), out);
assert.match(out, /\n    model: b\n/);
assert.match(out, /^ai:\n    models:\n        - b\n/);

// No model, no list: both go right after ai:.
dir = project('ai:\n    host: claude-code\nother: 1\n');
persistSelectedModel(dir, 'c');
out = read(dir);
assert.match(out, /^ai:\n    model: c\n    models:\n        - c\n/);
assert.match(out, /    host: claude-code\nother: 1\n$/);

// The chat runs on Claude Code: another host is named, so the panel says so.
const { readGofiProject, hostWithoutChat } = require('../src/gofiConfig.js');
dir = project('project:\n    name: x\nai:\n    host: codex\n    model: gpt\n');
assert.strictEqual(hostWithoutChat(readGofiProject(dir)), 'codex');
dir = project('project:\n    name: x\nai:\n    host: claude-code\n');
assert.strictEqual(hostWithoutChat(readGofiProject(dir)), '');
assert.strictEqual(hostWithoutChat(null), '');

console.log('model-persist: ok');
