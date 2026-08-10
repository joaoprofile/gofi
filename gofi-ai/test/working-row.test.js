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
 * The same booted webview answers two more questions that are only visible on
 * screen: whether the row says what the agent is actually doing, and what the
 * composer puts on the wire when a file is attached to a message.
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
		// SVG marks go through `setAttribute('class', …)`, not the `className`
		// setter, so `_classes` alone misses them — fold the raw attribute in too,
		// same as a real DOM element keeps both in sync.
		const classes = new Set([...this._classes, ...String(this.attrs.class || '').split(/\s+/).filter(Boolean)]);
		return selector
			.split(/(?=\.)/)
			.filter(Boolean)
			.every((part) => (part.startsWith('.') ? classes.has(part.slice(1)) : this.tagName === part));
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
	/** Counted, not performed: opening a real OS dialog is the editor's business. */
	click() {
		this.clicks = (this.clicks || 0) + 1;
	}
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
	'historyNew', 'historySearch', 'historyList', 'title', 'working', 'attachBtn', 'attachMenu',
	'attachUpload', 'attachProject', 'fileInput',
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
/** Everything the webview asked the extension to do, in order. */
const posted = [];
const sandbox = {
	console,
	document,
	setTimeout,
	clearTimeout,
	requestAnimationFrame: (fn) => setTimeout(fn, 0),
	acquireVsCodeApi: () => ({ postMessage: (message) => posted.push(message), getState() {}, setState() {} }),
	TextDecoder,
	/**
	 * Enough of a FileReader to answer with the bytes of a fake File. Synchronous
	 * where the real one is not, which is fine: what is under test is what the
	 * composer does with the bytes, not when they arrive.
	 */
	FileReader: class {
		readAsArrayBuffer(file) {
			this.result = file.bytes.buffer;
			if (this.onload) this.onload();
		}
		readAsDataURL(file) {
			this.result = `data:${file.type};base64,${Buffer.from(file.bytes).toString('base64')}`;
			if (this.onload) this.onload();
		}
	},
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
send({ type: 'identity', skills: [], providerLabel: 'Claude', hasWorkspace: true, supportsImages: true });
send({ type: 'user', text: 'audite as specs', images: 0, queued: false });
check('a linha de trabalho entra ao enviar', Boolean(working()), `último = ${tail()}`);

const row = working();
/** The word the row is showing, and whether it is a specific one. */
const label = () => row.querySelector('.label');
const verb = () => {
	const found = label() && label().querySelector('.verb');
	return found ? found.textContent : '(nenhum)';
};
const acting = () => Boolean(label() && label().classList.contains('acting'));

check('começa sem verbo específico — as palavras genéricas giram', !acting(), `verbo = ${verb()}`);

send({ type: 'running', running: true });
send({ type: 'meta', model: 'claude-opus-5' });
send({ type: 'delta', kind: 'thinking', text: 'preciso achar a spec' });
check('raciocínio chegando é "pensando"', acting() && verb() === 'pensando', `verbo = ${verb()}`);

send({ type: 'delta', kind: 'text', text: 'vou ler as specs…' });
check('sobrevive ao primeiro token', row.parentElement === log, 'a linha foi removida quando a resposta começou');
check('continua sendo a última linha', log.lastElementChild === row, `último = ${tail()}`);
check('texto chegando é "respondendo"', verb() === 'respondendo', `verbo = ${verb()}`);

send({ type: 'blocks', blocks: [{ type: 'text', text: 'lendo' }, { type: 'tool_use', id: 't1', name: 'Read', input: { file_path: 'specs/a.md' } }] });
check('a ferramenta em uso dá o verbo', verb() === 'lendo', `verbo = ${verb()}`);

send({ type: 'toolResult', toolUseId: 't1', isError: false, preview: '120 linhas' });
check('sem ferramenta em uso volta ao genérico', !acting(), `verbo = ${verb()}`);

send({ type: 'blocks', blocks: [{ type: 'tool_use', id: 't2', name: 'Bash', input: { command: 'go test ./...' } }] });
check('cada ferramenta tem seu verbo', verb() === 'executando', `verbo = ${verb()}`);
send({ type: 'toolResult', toolUseId: 't2', isError: false, preview: 'ok' });

send({ type: 'delta', kind: 'text', text: ' pronto' });
send({ type: 'blocks', blocks: [{ type: 'text', text: 'terminei' }] });
check('segue a última linha depois de ferramentas e blocos', log.lastElementChild === row, `último = ${tail()}`);

const insertions = inserts.get(row);
check('nunca foi reinserida durante o turno', insertions === 1, `inserções = ${insertions} (cada uma reinicia a animação)`);

send({ type: 'done', isError: false, costUsd: 0.01, durationMs: 4200 });
send({ type: 'running', running: false });
check('sai quando o turno acaba', !working(), `último = ${tail()}`);

// ── limite de sessão: o robô pede para aguardar, em vez de o chicote insistir ──
send({ type: 'user', text: 'mais uma pergunta', images: 0, queued: false });
send({ type: 'running', running: true });
check('a linha de trabalho volta a entrar', Boolean(working()), `último = ${tail()}`);

send({
	type: 'rateLimited',
	message: "You've hit your session limit · resets 4:50pm (America/Sao_Paulo)",
	reset: '4:50pm (America/Sao_Paulo)',
});
send({ type: 'running', running: false });
check('some o chicote ao esbarrar no limite', !working(), `último = ${tail()}`);
const waitRow = log.children.find((c) => c._classes.has('wait-notice'));
check('aparece o aviso de espera', Boolean(waitRow), `último = ${tail()}`);
check('o robô aparece no aviso, não a marca de trabalho', Boolean(waitRow && waitRow.querySelector('.wait-mark')), 'faltou o ícone do robô');
check('o horário de reinício aparece no aviso', Boolean(waitRow) && waitRow.textContent.includes('4:50pm'), waitRow && waitRow.textContent);

// ── enquanto uma aprovação espera, quem está parado é o agente ──────────────
send({ type: 'user', text: 'aplique', images: 0, queued: false });
send({ type: 'running', running: true });
send({ type: 'approval', id: 'a1', tool: 'Edit', input: { file_path: 'x.md' } });
check('some enquanto a aprovação espera', !working(), `último = ${tail()}`);
send({ type: 'approvalResolved', id: 'a1', decision: 'allow', tool: 'Edit' });
check('volta quando a aprovação é respondida', Boolean(working()), `último = ${tail()}`);

// ── anexos: o que o compositor coloca no fio ────────────────────────────────
//
// O `+` abre o diálogo do sistema operacional, e os bytes chegam na janela — não
// um caminho. Uma imagem viaja como bytes (é a única forma de mostrar uma tela ao
// modelo); um documento viaja como texto, porque o motor roda na raiz do projeto
// e, em janela remota, nem é a mesma máquina.
const bar = byId.get('attachments');

/** A File as the OS dialog would hand it over. */
function fakeFile(name, type, text) {
	const bytes = new TextEncoder().encode(text);
	return { name, type, size: bytes.length, bytes };
}

const fileInput = byId.get('fileInput');
const upload = byId.get('attachUpload');

upload.listeners.click[0]();
check('o item do menu abre o diálogo do sistema', fileInput.clicks === 1, `clicks = ${fileInput.clicks}`);

fileInput.files = [
	fakeFile('notas.txt', 'text/plain', 'trocar o provedor'),
	fakeFile('foto.png', 'image/png', 'PNG'),
];
fileInput.listeners.change[0]();

check('o que foi escolhido aparece no compositor', !bar.hidden && bar.childElementCount === 2, `chips = ${bar.childElementCount}`);
check('o chip diz que o arquivo vem de fora do projeto', bar.children[0].textContent.includes('externo'), bar.textContent);
check('escolher o mesmo arquivo de novo continua funcionando', fileInput.value === '', `value = ${fileInput.value}`);

byId.get('input').value = 'o que tem aqui?';
byId.get('submit').listeners.click[0]();
const sent = posted[posted.length - 1];
check('o documento viaja com o conteúdo já lido na janela', sent.type === 'send' && sent.files.length === 1 && sent.files[0].text === 'trocar o provedor', JSON.stringify(sent.files));
check('e sem caminho, porque não é um caminho que o agente possa abrir', sent.files[0].path === null, JSON.stringify(sent.files[0]));
check('a imagem viaja em bytes, num campo separado', sent.images.length === 1 && typeof sent.images[0].data === 'string', JSON.stringify(sent.images.length));
check('enviar limpa a barra de anexos', bar.hidden && bar.childElementCount === 0, `chips = ${bar.childElementCount}`);

run();
