'use strict';

const fs = require('fs');
const os = require('os');
const path = require('path');

/**
 * The bridge between the engine's PreToolUse hook and the panel.
 *
 * The hook runs as its own process and blocks; the extension host is where the
 * user is. Neither can call the other, so they meet on the filesystem: the hook
 * drops a `.request` and waits, this watches for it, asks, and answers with an
 * `.allow` or a `.deny` carrying the reason.
 *
 * Failing closed is the whole design. A request nobody answers, a panel that
 * closed mid-question, a directory that vanished — all deny. The hook does the
 * same on its side, so neither half can be the one that quietly lets a write
 * through.
 */
class ApprovalBridge {
	/**
	 * @param {(request: {id: string, tool: string, input: object}) => void} onRequest
	 */
	constructor(onRequest) {
		this.onRequest = onRequest;
		this.dir = fs.mkdtempSync(path.join(os.tmpdir(), 'gofi-ai-approve-'));
		/** @type {Set<string>} Requests already shown, so a re-read doesn't ask twice. */
		this.seen = new Set();
		/** @type {import('fs').FSWatcher | null} */
		this.watcher = null;
		this.start();
	}

	start() {
		try {
			this.watcher = fs.watch(this.dir, (_event, filename) => {
				if (filename && filename.endsWith('.request')) {
					this.read(filename);
				}
			});
		} catch {
			this.watcher = null; // watch unsupported; the poll below carries it
		}
		// fs.watch misses events on some filesystems (network mounts, WSL
		// crossings). A slow poll costs nothing and makes the bridge dependable
		// rather than usually-working.
		this.poll = setInterval(() => this.sweep(), 400);
	}

	sweep() {
		let entries;
		try {
			entries = fs.readdirSync(this.dir);
		} catch {
			return;
		}
		for (const name of entries) {
			if (name.endsWith('.request')) {
				this.read(name);
			}
		}
	}

	read(filename) {
		const id = filename.slice(0, -'.request'.length);
		if (this.seen.has(id)) {
			return;
		}
		let raw;
		try {
			raw = fs.readFileSync(path.join(this.dir, filename), 'utf8');
		} catch {
			return; // still being written, or already answered
		}
		let payload;
		try {
			payload = JSON.parse(raw);
		} catch {
			return; // partial write; the next sweep will find it whole
		}
		this.seen.add(id);
		this.onRequest({
			id,
			tool: String(payload.tool_name || 'ferramenta'),
			input: payload.tool_input && typeof payload.tool_input === 'object' ? payload.tool_input : {},
		});
	}

	allow(id) {
		this.answer(id, 'allow', '');
	}

	/** @param {string} reason Handed back to the model as the refusal. */
	deny(id, reason) {
		this.answer(id, 'deny', reason || 'O usuário não autorizou esta alteração.');
	}

	answer(id, kind, body) {
		if (!this.seen.has(id)) {
			return;
		}
		this.seen.delete(id);
		try {
			fs.writeFileSync(path.join(this.dir, `${id}.${kind}`), body, 'utf8');
		} catch {
			/* the hook timed out and cleaned up; it already denied */
		}
	}

	/** Denies everything outstanding — used when a turn is cancelled. */
	denyAll(reason) {
		for (const id of [...this.seen]) {
			this.deny(id, reason);
		}
	}

	dispose() {
		if (this.watcher) {
			this.watcher.close();
			this.watcher = null;
		}
		clearInterval(this.poll);
		// Anything still waiting must be released, or the hook blocks until its
		// own timeout with nobody left to answer.
		this.denyAll('O painel foi fechado antes da resposta.');
		try {
			fs.rmSync(this.dir, { recursive: true, force: true });
		} catch {
			/* best effort */
		}
	}
}

/**
 * The `--settings` payload that installs the hook.
 *
 * Only the tools that can change something are matched; reads are never worth
 * a prompt and asking about them would train the user to click through.
 */
function hookSettings(scriptPath, ipcDir) {
	const command = `sh ${shellQuote(scriptPath)} ${shellQuote(ipcDir)}`;
	return {
		hooks: {
			PreToolUse: [
				{
					matcher: 'Edit|Write|NotebookEdit|Bash',
					hooks: [{ type: 'command', command, timeout: 330 }],
				},
			],
		},
	};
}

function shellQuote(value) {
	return `'${String(value).replace(/'/g, `'\\''`)}'`;
}

module.exports = { ApprovalBridge, hookSettings };
