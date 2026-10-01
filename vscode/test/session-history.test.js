'use strict';

/**
 * Conversations survive the panel that had them.
 *
 * Two promises are pinned here. A transcript written by the panel comes back
 * exactly as it was rendered — that is what makes closing the chat safe. And a
 * conversation is a single engine session, not a panel copy of one: the id
 * travels with the record, so the same thread can be continued from the chat
 * or from a terminal, and neither one is a second version of it.
 *
 * Run with `node test/session-history.test.js`.
 */

const assert = require('assert');
const fs = require('fs');
const os = require('os');
const path = require('path');

const { installVscodeStub, makeContext, runner } = require('./vscode-stub.js');

installVscodeStub();

const { Chat } = require('../src/chat.js');
const { SessionStore } = require('../src/history.js');
const claudeSessions = require('../src/providers/claudeSessions.js');

const { test, run } = runner();

/** A scratch directory that goes away with the process that made it. */
function tempDir(prefix) {
	const dir = fs.mkdtempSync(path.join(os.tmpdir(), prefix));
	process.on('exit', () => fs.rmSync(dir, { recursive: true, force: true }));
	return dir;
}

function makeChat(id, storagePath) {
	return new Chat(makeContext(storagePath), id);
}

// ── the panel's own transcripts ─────────────────────────────────────────────

test('a conversation is written and comes back', () => {
	const chat = makeChat(1, tempDir('gofi-history-'));
	const posted = [];
	chat.surfaces.add({ postMessage: (m) => posted.push(m) });

	chat.nameFrom('como está a spec de cobrança?');
	chat.post({ type: 'user', text: 'como está a spec de cobrança?', images: 0, queued: false });
	chat.post({ type: 'blocks', blocks: [{ type: 'text', text: 'está pronta' }] });
	chat.post({ type: 'done', isError: false, costUsd: null, durationMs: 1200 });
	chat.saveNow();

	const listed = chat.store.list();
	assert.strictEqual(listed.length, 1, 'the conversation must be on disk');
	assert.strictEqual(listed[0].title, 'como está a spec de cobrança?');
	assert.strictEqual(listed[0].messages, 1, 'one question was asked');

	const loaded = chat.store.load(listed[0].id);
	assert.deepStrictEqual(
		loaded.events.map((event) => event.type),
		['user', 'blocks', 'done'],
		'the transcript must come back in the order it was rendered',
	);
});

test('what is not part of the transcript is not written', () => {
	const chat = makeChat(1, tempDir('gofi-history-'));

	chat.post({ type: 'user', text: 'oi', images: 0 });
	chat.post({ type: 'usage', snapshot: { tokens: {} } });
	chat.post({ type: 'identity', skills: [] });
	chat.post({ type: 'delta', kind: 'text', text: 'parcial' });

	assert.deepStrictEqual(
		chat.record.events.map((event) => event.type),
		['user'],
		'live-only chatter must not end up in a saved conversation',
	);
});

test('an empty conversation is not saved', () => {
	const chat = makeChat(1, tempDir('gofi-history-'));
	chat.saveNow();
	assert.strictEqual(chat.store.list().length, 0, 'nothing was said — there is nothing to keep');
});

test('starting a new conversation keeps the old one', () => {
	const chat = makeChat(1, tempDir('gofi-history-'));
	chat.nameFrom('primeira pergunta');
	chat.post({ type: 'user', text: 'primeira pergunta', images: 0 });

	const first = chat.record.id;
	chat.newSession();

	assert.notStrictEqual(chat.record.id, first, 'the new conversation must be a new record');
	assert.strictEqual(chat.store.list().length, 1, 'and the old one must still be on disk');
	assert.strictEqual(chat.record.events.length, 0, 'starting over means starting empty');
});

test('reopening a conversation replays it and carries its engine session', () => {
	const storage = tempDir('gofi-history-');
	const chat = makeChat(1, storage);

	chat.nameFrom('pergunta antiga');
	chat.post({ type: 'user', text: 'pergunta antiga', images: 0 });
	chat.post({ type: 'blocks', blocks: [{ type: 'text', text: 'resposta antiga' }] });
	chat.record.engineSessionId = 'aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee';
	chat.saveNow();
	const oldId = chat.record.id;

	// A later window: another Chat, the same storage.
	const later = makeChat(2, storage);
	const posted = [];
	later.surfaces.add({ postMessage: (m) => posted.push(m) });
	later.restoreSession(oldId, '');

	assert.strictEqual(later.record.id, oldId, 'the reopened conversation must be that conversation');
	assert.strictEqual(
		later.record.engineSessionId,
		'aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee',
		'the engine session id is what lets the next turn continue rather than start over',
	);

	const replay = posted.find((message) => message.type === 'replay');
	assert.ok(replay, 'the panel must be told to rebuild the transcript');
	assert.deepStrictEqual(replay.events.map((event) => event.type), ['user', 'blocks']);
});

test('a conversation forgotten here is not one deleted from the engine', () => {
	const chat = makeChat(1, tempDir('gofi-history-'));
	chat.nameFrom('descartável');
	chat.post({ type: 'user', text: 'descartável', images: 0 });
	chat.record.engineSessionId = 'aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee';
	chat.saveNow();

	const id = chat.record.id;
	chat.deleteSession(id);

	assert.strictEqual(chat.store.list().length, 0, 'the panel forgets its own copy');
	assert.strictEqual(chat.store.load(id), null);
});

test('an id from the panel cannot escape the sessions folder', () => {
	const store = new SessionStore(makeContext(tempDir('gofi-history-')));
	assert.strictEqual(store.load('../../../etc/passwd'), null);
	assert.strictEqual(store.load('..'), null);
});

// ── the engine's own store ──────────────────────────────────────────────────

/**
 * Writes a session file where the engine keeps them, for a fake home.
 *
 * `os.homedir()` reads `HOME` on this platform, so a temporary one is all it
 * takes to exercise the reader against a real file rather than a mock.
 */
function withEngineStore(cwd, sessionId, lines) {
	const home = tempDir('gofi-home-');
	process.env.HOME = home;
	const dir = path.join(home, '.claude', 'projects', cwd.replace(/[^A-Za-z0-9]/g, '-'));
	fs.mkdirSync(dir, { recursive: true });
	fs.writeFileSync(path.join(dir, `${sessionId}.jsonl`), lines.map((l) => JSON.stringify(l)).join('\n'));
	return dir;
}

test('a conversation started in the terminal is listed and readable', () => {
	const cwd = '/work/projeto';
	const id = '11111111-2222-3333-4444-555555555555';
	withEngineStore(cwd, id, [
		{ type: 'queue-operation', operation: 'enqueue' },
		{ type: 'user', isSidechain: false, message: { role: 'user', content: 'quais specs faltam?' } },
		{
			type: 'assistant',
			isSidechain: false,
			message: { role: 'assistant', content: [{ type: 'text', text: 'faltam duas' }] },
		},
		{ type: 'ai-title', aiTitle: 'Specs pendentes do projeto', sessionId: id },
	]);

	const listed = claudeSessions.list(cwd);
	assert.strictEqual(listed.length, 1);
	assert.strictEqual(listed[0].engineId, id);
	assert.strictEqual(listed[0].title, 'Specs pendentes do projeto', 'the engine names its own sessions — use its name');

	const events = claudeSessions.transcript(cwd, id);
	assert.deepStrictEqual(events.map((event) => event.type), ['user', 'blocks']);
	assert.strictEqual(events[0].text, 'quais specs faltam?');
	assert.strictEqual(events[1].blocks[0].text, 'faltam duas');
});

test('the engine transcript leaves out what is not the conversation', () => {
	const cwd = '/work/outro';
	const id = '66666666-7777-8888-9999-000000000000';
	withEngineStore(cwd, id, [
		{ type: 'file-history-snapshot', snapshot: {} },
		{
			type: 'user',
			isSidechain: false,
			message: { role: 'user', content: 'rode os testes<system-reminder>ignore isto</system-reminder>' },
		},
		{
			type: 'assistant',
			isSidechain: true,
			message: { role: 'assistant', content: [{ type: 'text', text: 'conversa de sub-agente' }] },
		},
		{
			type: 'user',
			isSidechain: false,
			message: {
				role: 'user',
				content: [{ type: 'tool_result', tool_use_id: 't1', content: 'ok\ndetalhes' }],
			},
		},
	]);

	const events = claudeSessions.transcript(cwd, id);
	assert.deepStrictEqual(events.map((event) => event.type), ['user', 'toolResult']);
	assert.strictEqual(events[0].text, 'rode os testes', 'reminders are addressed to the model, not to the reader');
	assert.strictEqual(events[1].preview, 'ok');
});

test('an unknown folder simply has no conversations', () => {
	process.env.HOME = tempDir('gofi-home-');
	assert.deepStrictEqual(claudeSessions.list('/nao/existe'), []);
	assert.deepStrictEqual(claudeSessions.transcript('/nao/existe', '11111111-2222-3333-4444-555555555555'), []);
});

run();
