'use strict';

/**
 * The scope of "always allow", pinned.
 *
 * This is the guarantee the whole approval flow rests on: granting a tool
 * blanket permission must reach exactly one conversation, and must not outlive
 * it. Everything else in the panel is a convenience; this one is a promise
 * about what can happen to the user's repository, so it gets a test rather
 * than a careful reading.
 *
 * Run with `node test/permission-scope.test.js`.
 */

const assert = require('assert');
const fs = require('fs');
const path = require('path');
const Module = require('module');

// The panel runs inside the extension host, so `vscode` only exists there.
// A stub with the handful of members Chat touches is enough to exercise the
// permission logic without one.
const vscodeStub = {
	Uri: { joinPath: (base, ...parts) => ({ fsPath: path.join(base.fsPath, ...parts) }) },
	workspace: {
		workspaceFolders: undefined,
		getConfiguration: () => ({ get: () => undefined }),
		createFileSystemWatcher: () => ({
			onDidCreate() {},
			onDidChange() {},
			onDidDelete() {},
			dispose() {},
		}),
		asRelativePath: (p) => String(p),
	},
	window: {
		activeTextEditor: undefined,
		onDidChangeActiveTextEditor: () => ({ dispose() {} }),
		onDidChangeTextEditorSelection: () => ({ dispose() {} }),
		showWarningMessage() {},
		showInformationMessage: async () => undefined,
	},
	commands: { executeCommand() {} },
	RelativePattern: class {},
	ViewColumn: { One: 1, Beside: 2 },
	Position: class {},
	Selection: class {},
	Range: class {},
	TextEditorRevealType: { InCenter: 2 },
	ThemeIcon: class {},
};

const realResolve = Module._resolveFilename;
Module._resolveFilename = function (request, ...rest) {
	if (request === 'vscode') {
		return 'vscode';
	}
	return realResolve.call(this, request, ...rest);
};
require.cache.vscode = { id: 'vscode', filename: 'vscode', loaded: true, exports: vscodeStub };

const { Chat } = require('../src/chat.js');

function makeChat(id) {
	const state = new Map();
	const context = {
		extensionUri: { fsPath: path.join(__dirname, '..') },
		subscriptions: [],
		workspaceState: {
			get: (key, fallback) => (state.has(key) ? state.get(key) : fallback),
			update: (key, value) => state.set(key, value),
		},
	};
	return new Chat(context, id);
}

/** Stands in for the bridge so no engine or filesystem handshake is involved. */
function stubApprovals(chat) {
	const decisions = [];
	chat.approvals = {
		dir: '/tmp/none',
		allow: (id) => decisions.push(['allow', id]),
		deny: (id, reason) => decisions.push(['deny', id, reason]),
		denyAll() {},
		dispose() {},
	};
	return decisions;
}

const tests = [];
const test = (name, fn) => tests.push([name, fn]);

test('granting is remembered for the rest of the conversation', () => {
	const chat = makeChat(1);
	const decisions = stubApprovals(chat);

	chat.onApprovalRequest({ id: 'r1', tool: 'Edit', input: {} });
	assert.strictEqual(chat.pendingApprovals.size, 1, 'the first Edit must be asked about');

	chat.resolveApproval('r1', 'always');
	assert.ok(chat.alwaysAllow.has('Edit'));

	// The second one is answered without troubling the user.
	chat.onApprovalRequest({ id: 'r2', tool: 'Edit', input: {} });
	assert.strictEqual(chat.pendingApprovals.size, 0, 'a granted tool must not ask again');
	assert.deepStrictEqual(decisions[decisions.length - 1], ['allow', 'r2']);
});

test('granting one tool says nothing about another', () => {
	const chat = makeChat(1);
	stubApprovals(chat);

	chat.onApprovalRequest({ id: 'r1', tool: 'Edit', input: {} });
	chat.resolveApproval('r1', 'always');

	chat.onApprovalRequest({ id: 'r2', tool: 'Bash', input: {} });
	assert.strictEqual(chat.pendingApprovals.size, 1, 'Bash must still be asked about');
});

test('the grant does not survive clearing the conversation', () => {
	const chat = makeChat(1);
	stubApprovals(chat);

	chat.onApprovalRequest({ id: 'r1', tool: 'Edit', input: {} });
	chat.resolveApproval('r1', 'always');
	assert.ok(chat.alwaysAllow.has('Edit'));

	chat.newSession();
	assert.strictEqual(chat.alwaysAllow.size, 0, 'clearing the chat must clear the authority granted in it');
});

test('the grant does not survive closing the conversation', () => {
	const chat = makeChat(1);
	stubApprovals(chat);

	chat.onApprovalRequest({ id: 'r1', tool: 'Edit', input: {} });
	chat.resolveApproval('r1', 'always');

	chat.dispose();
	assert.strictEqual(chat.alwaysAllow.size, 0);
});

test('one conversation cannot grant permission for another', () => {
	const first = makeChat(1);
	const second = makeChat(2);
	stubApprovals(first);
	stubApprovals(second);

	first.onApprovalRequest({ id: 'r1', tool: 'Edit', input: {} });
	first.resolveApproval('r1', 'always');

	second.onApprovalRequest({ id: 'r2', tool: 'Edit', input: {} });
	assert.strictEqual(second.pendingApprovals.size, 1, 'the second chat must ask on its own behalf');
	assert.strictEqual(second.alwaysAllow.size, 0);
});

test('a single allow grants nothing beyond that call', () => {
	const chat = makeChat(1);
	stubApprovals(chat);

	chat.onApprovalRequest({ id: 'r1', tool: 'Edit', input: {} });
	chat.resolveApproval('r1', 'allow');
	assert.strictEqual(chat.alwaysAllow.size, 0, '"allow" is for one call, not for the tool');

	chat.onApprovalRequest({ id: 'r2', tool: 'Edit', input: {} });
	assert.strictEqual(chat.pendingApprovals.size, 1, 'the next Edit must be asked about again');
});

test('a refusal carries the reason back to the model', () => {
	const chat = makeChat(1);
	const decisions = stubApprovals(chat);

	chat.onApprovalRequest({ id: 'r1', tool: 'Bash', input: {} });
	chat.resolveApproval('r1', 'deny', 'não rode isso');
	assert.deepStrictEqual(decisions[decisions.length - 1], ['deny', 'r1', 'não rode isso']);
});

test('an unknown request id is ignored', () => {
	const chat = makeChat(1);
	const decisions = stubApprovals(chat);

	chat.resolveApproval('nunca-visto', 'always');
	assert.strictEqual(decisions.length, 0, 'nothing may be answered that was never asked');
	assert.strictEqual(chat.alwaysAllow.size, 0, 'and nothing may be granted by it');
});

let failed = 0;
for (const [name, fn] of tests) {
	try {
		fn();
		console.log(`  ok    ${name}`);
	} catch (err) {
		failed++;
		console.log(`  FALHA ${name}\n        ${err.message}`);
	}
}
console.log(`\n${tests.length - failed}/${tests.length} passaram`);
process.exit(failed === 0 ? 0 : 1);

// Keep the linter from flagging the unused import in a script with no exports.
void fs;
