'use strict';

/**
 * The working row stays alive for the whole turn.
 *
 * It is the only thing that says the agent is still there, and twice now it has
 * been broken by a change that read as correct: once by the first token
 * dismissing it, once by a node move that silently restarts CSS animations.
 * Neither shows up in a code review, and both are obvious on screen — so this
 * boots the real webview against a DOM shim and asks where the row actually
 * ends up.
 *
 * The insertion count is the load-bearing assertion. Chromium restarts every
 * animation on an element when it is re-inserted, so a row that is appended
 * twice in one turn is a whip that never gets past its first frame.
 *
 * Run with `node test/working-row.test.js`.
 */

const fs = require('fs');
const path = require('path');
const vm = require('vm');

const MEDIA = path.join(__dirname, '..', 'media');

/** Counts every insertion, so "was it re-added?" is answerable. */
const inserts = new Map();

class El {
	constructor(tag, ns) {
		this.tagName = tag;
		this.ns = ns || null;
		this.children = [];
		this.parentElement = null;
		this.attrs = {};
		this.dataset = {};
		this.style = {};
		this.hidden = false;
		this._text = '';
		this._classes = new Set();
		this.listeners = {};
		this.selectionStart = 0;
		this.value = '';
	}

	get className() {
		return [...this._classes].join(' ');
	}
	set className(v) {
		this._classes = new Set(String(v).split(/\s+/).filter(Boolean));
	}
	get classList() {
		const self = this;
		return {
			add: (...c) => c.forEach((x) => self._classes.add(x)),
			remove: (...c) => c.forEach((x) => self._classes.delete(x)),
			toggle: (c, force) => {
				const on = force === undefined ? !self._classes.has(c) : force;
				if (on) self._classes.add(c);
				else self._classes.delete(c);
				return on;
			},
			contains: (c) => self._classes.has(c),
		};
	}

	set textContent(v) {
		for (const child of this.children) child.parentElement = null;
		this.children = [];
		this._text = String(v);
	}
	get textContent() {
		return this.children.length === 0 ? this._text : this.children.map((c) => c.textContent).join('');
	}

	setAttribute(k, v) {
		this.attrs[k] = String(v);
	}
	getAttribute(k) {
		return this.attrs[k];
	}
	removeAttribute(k) {
		delete this.attrs[k];
	}

	_detach(node) {
		if (node.parentElement) {
			const i = node.parentElement.children.indexOf(node);
			if (i !== -1) node.parentElement.children.splice(i, 1);
		}
	}

	appendChild(node) {
		this._detach(node);
		this.children.push(node);
		node.parentElement = this;
		inserts.set(node, (inserts.get(node) || 0) + 1);
		return node;
	}

	insertBefore(node, ref) {
		this._detach(node);
		const i = ref ? this.children.indexOf(ref) : -1;
		if (i === -1) this.children.push(node);
		else this.children.splice(i, 0, node);
		node.parentElement = this;
		inserts.set(node, (inserts.get(node) || 0) + 1);
		return node;
	}

	remove() {
		this._detach(this);
		this.parentElement = null;
	}

	get lastElementChild() {
		return this.children.length ? this.children[this.children.length - 1] : null;
	}
	get childElementCount() {
		return this.children.length;
	}

	matches(selector) {
		return selector
			.split(/(?=\.)/)
			.filter(Boolean)
			.every((part) => (part.startsWith('.') ? this._classes.has(part.slice(1)) : this.tagName === part));
	}

	querySelector(selector) {
		for (const child of this.children) {
			if (child.matches && child.matches(selector)) return child;
			const nested = child.querySelector ? child.querySelector(selector) : null;
			if (nested) return nested;
		}
		return null;
	}

	addEventListener(type, fn) {
		(this.listeners[type] = this.listeners[type] || []).push(fn);
	}
	removeEventListener() {}
	focus() {}
	select() {}
	setSelectionRange() {}
	scrollIntoView() {}
	appendData(text) {
		this._text += text;
	}
}

const ids = [
	'log', 'chips', 'picker', 'input', 'submit', 'cancel', 'subtitle', 'usageBar', 'usageSummary',
	'usageFlag', 'usagePanel', 'activeFile', 'attachments', 'writeBadge', 'historyBtn', 'history',
	'historyNew', 'historySearch', 'historyList', 'title', 'working',
];
const byId = new Map(ids.map((id) => [id, Object.assign(new El('div'), { attrs: { id } })]));

const document = {
	getElementById: (id) => byId.get(id) || null,
	createElement: (tag) => new El(tag),
	createElementNS: (ns, tag) => new El(tag, ns),
	createTextNode: (text) => Object.assign(new El('#text'), { _text: text }),
	body: new El('body'),
	documentElement: new El('html'),
	addEventListener() {},
};

const messageListeners = [];
const sandbox = {
	console,
	document,
	setTimeout,
	clearTimeout,
	requestAnimationFrame: (fn) => setTimeout(fn, 0),
	acquireVsCodeApi: () => ({ postMessage() {}, getState() {}, setState() {} }),
	FileReader: class {},
	window: {
		addEventListener: (type, fn) => {
			if (type === 'message') messageListeners.push(fn);
		},
		innerHeight: 800,
		matchMedia: () => ({ matches: false }),
	},
};
sandbox.window.document = document;
sandbox.globalThis = sandbox;

const context = vm.createContext(sandbox);
for (const file of ['markdown.js', 'main.js']) {
	vm.runInContext(fs.readFileSync(path.join(MEDIA, file), 'utf8'), context, { filename: file });
}

const send = (message) => messageListeners.forEach((fn) => fn({ data: message }));
const log = byId.get('log');
const working = () => log.children.find((c) => c._classes.has('working'));
const tail = () => (log.lastElementChild ? log.lastElementChild.className : '(vazio)');

const assert = require('assert');
const { runner } = require('./vscode-stub.js');
const { test, run } = runner();

/** Each check is its own test, so a failure names itself. */
function check(label, condition, detail) {
	test(label, () => assert.ok(condition, detail));
}

// ── um turno completo, como a extensão o emite ─────────────────────────────
send({ type: 'identity', skills: [], providerLabel: 'Claude', hasWorkspace: true });
send({ type: 'user', text: 'audite as specs', images: 0, queued: false });
check('a linha de trabalho entra ao enviar', Boolean(working()), `último = ${tail()}`);

const row = working();
send({ type: 'running', running: true });
send({ type: 'meta', model: 'claude-opus-5' });
send({ type: 'delta', kind: 'text', text: 'vou ler as specs…' });
check('sobrevive ao primeiro token', row.parentElement === log, 'a linha foi removida quando a resposta começou');
check('continua sendo a última linha', log.lastElementChild === row, `último = ${tail()}`);

send({ type: 'blocks', blocks: [{ type: 'text', text: 'lendo' }, { type: 'tool_use', id: 't1', name: 'Read', input: { file_path: 'specs/a.md' } }] });
send({ type: 'toolResult', toolUseId: 't1', isError: false, preview: '120 linhas' });
send({ type: 'delta', kind: 'text', text: ' pronto' });
send({ type: 'blocks', blocks: [{ type: 'text', text: 'terminei' }] });
check('segue a última linha depois de ferramentas e blocos', log.lastElementChild === row, `último = ${tail()}`);

const insertions = inserts.get(row);
check('nunca foi reinserida durante o turno', insertions === 1, `inserções = ${insertions} (cada uma reinicia a animação)`);

send({ type: 'done', isError: false, costUsd: 0.01, durationMs: 4200 });
send({ type: 'running', running: false });
check('sai quando o turno acaba', !working(), `último = ${tail()}`);

// ── enquanto uma aprovação espera, quem está parado é o agente ──────────────
send({ type: 'user', text: 'aplique', images: 0, queued: false });
send({ type: 'running', running: true });
send({ type: 'approval', id: 'a1', tool: 'Edit', input: { file_path: 'x.md' } });
check('some enquanto a aprovação espera', !working(), `último = ${tail()}`);
send({ type: 'approvalResolved', id: 'a1', decision: 'allow', tool: 'Edit' });
check('volta quando a aprovação é respondida', Boolean(working()), `último = ${tail()}`);

run();
