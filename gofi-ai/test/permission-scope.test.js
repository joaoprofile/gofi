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
const { installVscodeStub, makeContext, runner } = require('./vscode-stub.js');

// The panel runs inside the extension host, so `vscode` only exists there. The
// stub is enough to exercise the permission logic without one.
installVscodeStub();

const { Chat } = require('../src/chat.js');
const { test, run } = runner();

function makeChat(id) {
	return new Chat(makeContext(), id);
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

run();
