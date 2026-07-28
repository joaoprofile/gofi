// @ts-check
'use strict';

/**
 * Webview side of the GOFI AI chat.
 *
 * Everything here builds DOM nodes with textContent — no innerHTML anywhere.
 * The transcript is model output plus tool results plus file contents, i.e.
 * fully untrusted text, and this document has the extension's message channel
 * in scope.
 */

(function () {
	const vscode = acquireVsCodeApi();

	const log = el('log');
	const chips = el('chips');
	const picker = el('picker');
	const input = /** @type {HTMLTextAreaElement} */ (el('input'));
	const submit = /** @type {HTMLButtonElement} */ (el('submit'));
	const cancel = /** @type {HTMLButtonElement} */ (el('cancel'));
	const subtitle = el('subtitle');
	const usageBar = /** @type {HTMLButtonElement} */ (el('usageBar'));
	const usageSummary = el('usageSummary');
	const usageFlag = el('usageFlag');
	const usagePanel = el('usagePanel');
	const activeFile = el('activeFile');
	const attachmentsBar = el('attachments');
	const writeBadge = el('writeBadge');

	function el(id) {
		return /** @type {HTMLElement} */ (document.getElementById(id));
	}

	/** Options mirrored from settings on every identity message. */
	let showThinking = true;
	let showToolCalls = true;
	let showActiveFile = true;

	/** @type {{slug: string, title: string, enabled: boolean}[]} */
	let skills = [];
	/** Whether the engine accepts images at all. */
	let supportsImages = false;
	/** Images pasted into the composer, waiting to go with the next message. */
	/** @type {{id: number, mediaType: string, data: string, url: string}[]} */
	let attachments = [];
	let attachmentSeq = 0;
	/** From `.gofi.yaml`; null when the project has none. */
	let projectName = null;
	let hasConfig = false;
	/** Until the first identity message lands we don't know if there are any. */
	let skillsLoaded = false;

	/**
	 * The turn currently being built, and the block being streamed into it.
	 *
	 * `streamingText` keeps the bubble, the Text node being appended to, and the
	 * raw string. Appending to a live Text node is O(1); rebuilding the bubble
	 * from the accumulated string on every delta is O(n) per delta, i.e. O(n²)
	 * over a long answer — which is exactly what made the text lag behind.
	 */
	let currentTurn = null;
	/** @type {{bubble: HTMLElement, node: Text, raw: string}|null} */
	let streamingText = null;
	let streamingThinking = null;
	/** Deltas land here and are painted once per animation frame. */
	let paintQueued = false;
	let followTail = true;
	/** The "working…" row shown between turns. */
	let pendingIndicator = null;
	/** Tool rows awaiting their result, keyed by tool_use id. */
	const pendingTools = new Map();

	renderSkills();
	showEmptyState();

	// ── composer ────────────────────────────────────────────────────────────

	function autoGrow() {
		input.style.height = 'auto';
		input.style.height = `${Math.min(input.scrollHeight, window.innerHeight * 0.4)}px`;
	}

	function send() {
		const text = input.value.trim();
		if (text === '' && attachments.length === 0) {
			return;
		}
		// Submit is never disabled while running — the backend queues the
		// message onto the same session and drains it after the current turn.
		vscode.postMessage({
			type: 'send',
			text,
			images: attachments.map((a) => ({ mediaType: a.mediaType, data: a.data })),
		});
		input.value = '';
		attachments = [];
		renderAttachments();
		closePicker();
		autoGrow();
	}

	// ── pasted images ───────────────────────────────────────────────────────

	/**
	 * A screenshot is often the fastest way to say what is wrong, so Ctrl+V of
	 * an image attaches it to the next message instead of pasting nothing.
	 *
	 * The bytes go to the engine as an image block — no temp file, no Read tool,
	 * no permission prompt.
	 */
	input.addEventListener('paste', (event) => {
		if (!supportsImages || !event.clipboardData) {
			return;
		}
		const images = [...event.clipboardData.items].filter((item) => item.type.startsWith('image/'));
		if (images.length === 0) {
			return; // ordinary text paste — let the textarea handle it
		}
		event.preventDefault();
		for (const item of images) {
			const file = item.getAsFile();
			if (file) {
				attach(file);
			}
		}
	});

	function attach(file) {
		const reader = new FileReader();
		reader.onload = () => {
			const url = String(reader.result);
			const comma = url.indexOf(',');
			if (comma === -1) {
				return;
			}
			attachments.push({
				id: ++attachmentSeq,
				mediaType: file.type || 'image/png',
				data: url.slice(comma + 1),
				url,
			});
			renderAttachments();
		};
		reader.readAsDataURL(file);
	}

	function renderAttachments() {
		attachmentsBar.textContent = '';
		attachmentsBar.hidden = attachments.length === 0;
		for (const item of attachments) {
			const chip = document.createElement('span');
			chip.className = 'attachment';

			const thumb = document.createElement('img');
			thumb.src = item.url;
			thumb.alt = 'imagem colada';
			chip.appendChild(thumb);

			const remove = document.createElement('button');
			remove.type = 'button';
			remove.className = 'remove';
			remove.textContent = '×';
			remove.title = 'Remover';
			remove.addEventListener('click', () => {
				attachments = attachments.filter((a) => a.id !== item.id);
				renderAttachments();
			});
			chip.appendChild(remove);
			attachmentsBar.appendChild(chip);
		}
	}

	input.addEventListener('input', () => {
		autoGrow();
		refreshPicker();
	});
	// Keys the picker owns must not trigger a refresh on the way back up —
	// that was resetting the highlight to the first row on every ArrowDown.
	const PICKER_KEYS = new Set(['ArrowDown', 'ArrowUp', 'Enter', 'Tab', 'Escape']);
	input.addEventListener('keyup', (event) => {
		if (!PICKER_KEYS.has(event.key)) {
			refreshPicker(); // the caret can move without the value changing
		}
	});
	input.addEventListener('click', refreshPicker);
	input.addEventListener('keydown', (event) => {
		// The picker owns the keyboard while it is open, so Enter commits a
		// choice instead of sending a half-typed `/gofi-e`.
		if (pickerOpen() && handlePickerKey(event)) {
			return;
		}
		// Enter sends; Shift+Enter is a newline. Matches every chat the user
		// already has open.
		if (event.key === 'Enter' && !event.shiftKey) {
			event.preventDefault();
			send();
		}
	});
	input.addEventListener('blur', () => {
		// Let a click on a picker row land before the picker disappears.
		setTimeout(closePicker, 120);
	});
	submit.addEventListener('click', send);
	cancel.addEventListener('click', () => vscode.postMessage({ type: 'stop' }));

	// ── `/` skills and `@` files ────────────────────────────────────────────

	/**
	 * One menu, two triggers.
	 *
	 * `/` lists the project's skills, which the extension already knows. `@`
	 * lists project files, which it has to look up — so that half is async: the
	 * menu renders what it has and re-renders when results arrive, and a stale
	 * response for a query the user has already moved past is discarded.
	 *
	 * ↑/↓ move, Enter or Tab commit, Esc dismisses.
	 */
	let pickerMode = null; // 'skill' | 'file' | null
	let pickerIndex = 0;
	let pickerStart = -1;
	let pickerQuery = '';
	/** @type {{value: string, label: string, detail: string}[]} */
	let pickerItems = [];

	function pickerOpen() {
		return pickerMode !== null;
	}

	function closePicker() {
		pickerMode = null;
		pickerStart = -1;
		pickerQuery = '';
		pickerItems = [];
		picker.hidden = true;
		picker.textContent = '';
	}

	/**
	 * Finds the `/word` or `@word` the caret sits in. Anchored to the start of
	 * the message or to whitespace, so `src/main.js` and an e-mail address never
	 * open it.
	 */
	function tokenAtCaret() {
		const caret = input.selectionStart ?? 0;
		const before = input.value.slice(0, caret);
		const match = /(^|\s)([/@])([\w./-]*)$/.exec(before);
		if (!match) {
			return null;
		}
		return {
			mode: match[2] === '/' ? 'skill' : 'file',
			start: caret - match[3].length - 1,
			query: match[3],
		};
	}

	function refreshPicker() {
		const token = tokenAtCaret();
		if (!token) {
			closePicker();
			return;
		}
		// Nothing about the token changed, so neither should the menu — and in
		// particular neither should the highlighted row.
		if (pickerMode === token.mode && pickerStart === token.start && pickerQuery === token.query) {
			return;
		}

		pickerMode = token.mode;
		pickerStart = token.start;
		pickerQuery = token.query;

		if (token.mode === 'skill') {
			const needle = token.query.toLowerCase();
			pickerItems = skills
				.filter((s) => s.slug.toLowerCase().includes(needle))
				.map((s) => ({ value: `/${s.slug}`, label: `/${s.slug}`, detail: s.title }));
			if (pickerItems.length === 0) {
				closePicker();
				return;
			}
			pickerIndex = 0;
			renderPicker();
			return;
		}

		// Files: ask the extension, render a placeholder meanwhile.
		pickerIndex = 0;
		vscode.postMessage({ type: 'searchFiles', query: token.query });
		if (pickerItems.length === 0) {
			renderPickerLoading();
		} else {
			renderPicker();
		}
	}

	function onFileResults(query, files) {
		// The user kept typing while the lookup was in flight — this answer is
		// about a query that no longer exists.
		if (pickerMode !== 'file' || query !== pickerQuery) {
			return;
		}
		pickerItems = files.map((file) => ({
			value: `@${file}`,
			label: file.slice(file.lastIndexOf('/') + 1),
			detail: file,
		}));
		if (pickerItems.length === 0) {
			picker.textContent = '';
			picker.appendChild(pickerMessage('nenhum arquivo com esse nome'));
			picker.hidden = false;
			return;
		}
		if (pickerIndex >= pickerItems.length) {
			pickerIndex = 0;
		}
		renderPicker();
	}

	function pickerMessage(text) {
		const row = document.createElement('div');
		row.className = 'row empty';
		row.textContent = text;
		return row;
	}

	function renderPickerLoading() {
		picker.textContent = '';
		const row = pickerMessage('procurando…');
		row.appendChild(dots());
		picker.appendChild(row);
		picker.hidden = false;
	}

	function renderPicker() {
		picker.textContent = '';
		pickerItems.forEach((item, index) => {
			const row = document.createElement('div');
			row.className = index === pickerIndex ? 'row active' : 'row';
			row.setAttribute('role', 'option');

			const label = document.createElement('span');
			label.className = 'slug';
			label.textContent = item.label;
			row.appendChild(label);

			if (item.detail) {
				const detail = document.createElement('span');
				detail.className = 'desc';
				detail.textContent = item.detail;
				row.appendChild(detail);
			}

			// mousedown, not click: blur fires first on click and would close the
			// picker before the selection landed.
			row.addEventListener('mousedown', (event) => {
				event.preventDefault();
				commit(item.value);
			});
			picker.appendChild(row);
		});
		picker.hidden = false;
		const active = picker.querySelector('.row.active');
		if (active) {
			active.scrollIntoView({ block: 'nearest' });
		}
	}

	/** @returns {boolean} true when the key was consumed by the picker. */
	function handlePickerKey(event) {
		if (pickerItems.length === 0 && event.key !== 'Escape') {
			return false;
		}
		switch (event.key) {
			case 'ArrowDown':
				event.preventDefault();
				pickerIndex = (pickerIndex + 1) % pickerItems.length;
				renderPicker();
				return true;
			case 'ArrowUp':
				event.preventDefault();
				pickerIndex = (pickerIndex - 1 + pickerItems.length) % pickerItems.length;
				renderPicker();
				return true;
			case 'Enter':
			case 'Tab':
				event.preventDefault();
				commit(pickerItems[pickerIndex].value);
				return true;
			case 'Escape':
				event.preventDefault();
				closePicker();
				return true;
			default:
				return false;
		}
	}

	/** Replaces the partial token under the caret with the chosen value. */
	function commit(value) {
		const caret = input.selectionStart ?? input.value.length;
		const head = input.value.slice(0, pickerStart);
		const tail = input.value.slice(caret);
		const insert = `${value} `;
		input.value = head + insert + tail;
		const position = head.length + insert.length;
		input.setSelectionRange(position, position);
		closePicker();
		input.focus();
		autoGrow();
	}

	// ── transcript ──────────────────────────────────────────────────────────

	function atBottom() {
		return log.scrollHeight - log.scrollTop - log.clientHeight < 60;
	}

	function showEmptyState() {
		const empty = document.createElement('div');
		empty.className = 'empty-state';

		const mark = document.createElement('span');
		mark.className = 'mark big';
		empty.appendChild(mark);

		const title = document.createElement('p');
		title.className = 'title';
		if (projectName) {
			title.appendChild(document.createTextNode('Converse com os agentes do projeto '));
			const name = document.createElement('strong');
			name.textContent = projectName;
			title.appendChild(name);
			title.appendChild(document.createTextNode('.'));
		} else {
			title.textContent = 'Converse com os agentes do projeto.';
		}
		empty.appendChild(title);

		// No `.gofi.yaml` is a legitimate state — the chat still works — but it
		// explains why there is no project name and no enabled-agent list.
		if (!hasConfig) {
			const note = document.createElement('p');
			note.className = 'subtle';
			note.textContent = 'Sem .gofi.yaml aqui — rode `gofi init` para o chat conhecer o projeto.';
			empty.appendChild(note);
		}

		const hints = document.createElement('ul');
		for (const [key, text] of [['/', 'chama uma skill do projeto'], ['@', 'referencia um arquivo']]) {
			const item = document.createElement('li');
			const kbd = document.createElement('kbd');
			kbd.textContent = key;
			item.appendChild(kbd);
			item.appendChild(document.createTextNode(` ${text}`));
			hints.appendChild(item);
		}
		empty.appendChild(hints);
		log.appendChild(empty);
	}

	function clearEmptyState() {
		const empty = log.querySelector('.empty-state');
		if (empty) {
			empty.remove();
		}
	}

	function turn(who, label) {
		clearEmptyState();
		const el = document.createElement('article');
		el.className = `turn ${who}`;

		const heading = document.createElement('div');
		heading.className = 'who';

		const avatar = document.createElement('span');
		avatar.setAttribute('aria-hidden', 'true');
		if (who === 'user') {
			avatar.className = 'avatar user';
			avatar.textContent = 'eu';
		} else {
			// Same two-part mark as the indicator, so it can keep animating
			// while this turn is the live one.
			avatar.className = 'avatar agent gofi-thinking';
			avatar.appendChild(Object.assign(document.createElement('i'), { className: 'bar' }));
			avatar.appendChild(Object.assign(document.createElement('i'), { className: 'chev' }));
		}
		heading.appendChild(avatar);

		const name = document.createElement('span');
		name.textContent = label;
		heading.appendChild(name);

		el.appendChild(heading);

		const body = document.createElement('div');
		body.className = 'body';
		el.appendChild(body);

		log.appendChild(el);
		return body;
	}

	function assistantTurn() {
		if (!currentTurn) {
			clearIndicator();
			currentTurn = turn('assistant', 'GOFI AI');
			currentTurn.parentElement.classList.add('live');
		}
		return currentTurn;
	}

	function endTurn() {
		if (currentTurn && currentTurn.parentElement) {
			currentTurn.parentElement.classList.remove('live');
		}
		currentTurn = null;
		streamingText = null;
		streamingThinking = null;
	}

	/** Three animated dots — used inside the picker while a lookup is in flight. */
	function dots() {
		const el = document.createElement('span');
		el.className = 'dots';
		el.setAttribute('aria-hidden', 'true');
		for (let i = 0; i < 3; i++) {
			el.appendChild(document.createElement('i'));
		}
		return el;
	}

	/**
	 * The gofi mark, animated.
	 *
	 * Built from two elements rather than the usual gradient background so the
	 * bar and the chevron can move independently: the bar lights up, then the
	 * chevron follows and nudges forward. It reads as the play mark advancing,
	 * which says "working" in the product's own language instead of borrowing a
	 * generic spinner.
	 */
	function thinkingMark() {
		const mark = document.createElement('span');
		mark.className = 'gofi-thinking';
		mark.setAttribute('aria-hidden', 'true');
		const bar = document.createElement('i');
		bar.className = 'bar';
		const chevron = document.createElement('i');
		chevron.className = 'chev';
		mark.appendChild(bar);
		mark.appendChild(chevron);
		return mark;
	}

	/** A visible "the agent is working" row while nothing has streamed yet. */
	function showIndicator() {
		if (pendingIndicator) {
			return;
		}
		clearEmptyState();
		pendingIndicator = document.createElement('div');
		pendingIndicator.className = 'working';
		pendingIndicator.appendChild(thinkingMark());
		const label = document.createElement('span');
		label.className = 'label';
		label.textContent = 'pensando';
		pendingIndicator.appendChild(label);
		log.appendChild(pendingIndicator);
	}

	/**
	 * A small inline gofi mark that lives at the tail of the streaming bubble.
	 * It's a sibling of the growing Text node — appendData() on that node keeps
	 * the cursor visually right after the last character. Disappears with the
	 * bubble when `blocks` replaces the streamed preview, so "cursor gone"
	 * means "this stream is done".
	 */
	function streamingCursor() {
		const c = document.createElement('span');
		c.className = 'gofi-thinking gofi-cursor';
		c.setAttribute('aria-hidden', 'true');
		c.appendChild(Object.assign(document.createElement('i'), { className: 'bar' }));
		c.appendChild(Object.assign(document.createElement('i'), { className: 'chev' }));
		return c;
	}

	function clearIndicator() {
		if (pendingIndicator) {
			pendingIndicator.remove();
			pendingIndicator = null;
		}
	}

	// Paths in rendered Markdown open in the editor — the agents emit them
	// constantly, and a path you can click beats one you have to retype.
	window.gofiMarkdown.setPathHandler((path) => vscode.postMessage({ type: 'openFile', path }));

	/**
	 * Renders the agent's Markdown into `el`.
	 *
	 * Only called for finished messages. While a turn streams, text is appended
	 * raw (see the `delta` handler): half-parsed Markdown looks worse than
	 * plain text, and re-parsing the whole message on every token is the O(n²)
	 * that made the chat lag.
	 */
	function renderInto(el, text) {
		el.classList.add('md');
		window.gofiMarkdown.render(el, text);
	}

	function textBubble(parent, text) {
		const bubble = document.createElement('div');
		bubble.className = 'bubble';
		renderInto(bubble, text);
		parent.appendChild(bubble);
		return bubble;
	}

	function thinkingBlock(parent, text) {
		const details = document.createElement('details');
		details.className = 'thinking';
		const summary = document.createElement('summary');
		summary.textContent = 'raciocínio';
		details.appendChild(summary);
		const bubble = document.createElement('div');
		bubble.className = 'bubble';
		bubble.textContent = text;
		details.appendChild(bubble);
		parent.appendChild(details);
		return bubble;
	}

	/** The one line of a tool's input worth showing next to its name. */
	function toolArgument(input) {
		if (!input || typeof input !== 'object') {
			return '';
		}
		for (const key of ['file_path', 'path', 'command', 'pattern', 'query', 'url', 'prompt', 'description']) {
			const value = input[key];
			if (typeof value === 'string' && value !== '') {
				const line = value.split('\n')[0];
				return line.length > 120 ? `${line.slice(0, 120)}…` : line;
			}
		}
		return '';
	}

	function toolRow(parent, name, input) {
		const row = document.createElement('div');
		row.className = 'tool running';
		const label = document.createElement('span');
		label.className = 'name';
		label.textContent = name;
		row.appendChild(label);
		const arg = document.createElement('span');
		arg.className = 'arg';
		arg.textContent = toolArgument(input);
		row.appendChild(arg);
		parent.appendChild(row);
		return row;
	}

	// ── token + retrieval readout ───────────────────────────────────────────

	/**
	 * The always-visible bar summarises; the panel explains.
	 *
	 * Both update on every provider event, so the numbers move while the agent
	 * reads files rather than appearing only once the turn ends — watching a
	 * Read pull in 6k tokens as it happens is what makes the cost of a bad
	 * retrieval obvious.
	 */
	let usageExpanded = false;
	/** @type {object|null} */
	let lastSnapshot = null;

	usageBar.addEventListener('click', () => {
		usageExpanded = !usageExpanded;
		usageBar.setAttribute('aria-expanded', String(usageExpanded));
		usageBar.querySelector('.caret').textContent = usageExpanded ? '▾' : '▸';
		usagePanel.hidden = !usageExpanded;
		if (usageExpanded) {
			renderUsagePanel();
		}
	});

	/** 12345 → "12.3k"; small numbers stay exact. */
	function compact(n) {
		return n >= 1000 ? `${(n / 1000).toFixed(1)}k` : String(n);
	}

	function renderUsage(snapshot) {
		lastSnapshot = snapshot;
		const { tokens, cacheRate, retrieval } = snapshot;

		// Tokens only. The ledger still tracks cost — it comes free with the
		// stream — but showing it turns every glance into a price check.
		const parts = [
			`${compact(tokens.input + tokens.cacheRead + tokens.cacheWrite)} entrada`,
			`${compact(tokens.output)} saída`,
		];
		if (cacheRate !== null) {
			parts.push(`${Math.round(cacheRate * 100)}% cache`);
		}
		usageSummary.textContent = parts.join('  ·  ');

		// The bar carries the headline so the user notices without opening
		// anything; the count says how much more is inside.
		const warnings = snapshot.findings.filter((f) => f.level === 'warn');
		if (retrieval.inFlight.length > 0) {
			usageFlag.hidden = false;
			usageFlag.className = 'flag busy';
			usageFlag.textContent = `lendo ${retrieval.inFlight.length}`;
		} else if (warnings.length > 0) {
			usageFlag.hidden = false;
			usageFlag.className = 'flag warn';
			usageFlag.textContent = `${warnings.length} a melhorar`;
		} else if (retrieval.calls > 0) {
			usageFlag.hidden = false;
			usageFlag.className = 'flag good';
			usageFlag.textContent = 'rag ok';
		} else {
			usageFlag.hidden = true;
		}

		if (usageExpanded) {
			renderUsagePanel();
		}
	}

	function renderUsagePanel() {
		usagePanel.textContent = '';
		if (!lastSnapshot) {
			return;
		}
		const { tokens, retrieval, findings } = lastSnapshot;

		usagePanel.appendChild(
			statGrid([
				['entrada nova', compact(tokens.input)],
				['lida do cache', compact(tokens.cacheRead)],
				['gravada no cache', compact(tokens.cacheWrite)],
				['saída', compact(tokens.output)],
			]),
		);

		if (retrieval.inFlight.length > 0 || retrieval.rows.length > 0) {
			usagePanel.appendChild(sectionTitle(`recuperação — ~${compact(retrieval.tokens)} tokens em ${retrieval.calls} buscas`));
			const list = document.createElement('div');
			list.className = 'reads';
			for (const row of retrieval.inFlight) {
				list.appendChild(readRow(row, true));
			}
			for (const row of retrieval.rows) {
				list.appendChild(readRow(row, false));
			}
			usagePanel.appendChild(list);
		}

		usagePanel.appendChild(sectionTitle('o que dá para melhorar'));
		if (findings.length === 0) {
			const none = document.createElement('p');
			none.className = 'note';
			none.textContent = 'Nada a apontar — a recuperação deste projeto está indo bem.';
			usagePanel.appendChild(none);
		} else {
			const list = document.createElement('ul');
			list.className = 'findings';
			const applying = new Set(lastSnapshot.applying || []);
			for (const finding of findings) {
				list.appendChild(findingItem(finding, applying.has(finding.id)));
			}
			usagePanel.appendChild(list);
		}

		if (lastSnapshot.dismissedCount > 0) {
			const restore = document.createElement('button');
			restore.type = 'button';
			restore.className = 'restore';
			restore.textContent =
				lastSnapshot.dismissedCount === 1
					? '1 sugestão ignorada — mostrar de novo'
					: `${lastSnapshot.dismissedCount} sugestões ignoradas — mostrar de novo`;
			restore.addEventListener('click', () => vscode.postMessage({ type: 'restoreDismissed' }));
			usagePanel.appendChild(restore);
		}

		const note = document.createElement('p');
		note.className = 'note';
		note.textContent = 'Tokens de entrada/saída vêm do motor. O tamanho de cada busca é estimado a partir do texto devolvido.';
		usagePanel.appendChild(note);
	}

	/**
	 * One finding: what is wrong, exactly where to change it, and the button
	 * that changes it.
	 *
	 * "specs/x.md não tem frontmatter" is a diagnosis; "no topo do arquivo,
	 * antes da linha 1" plus the lines to paste is somewhere to start. The
	 * location is a link — clicking puts the cursor on that line.
	 */
	function findingItem(finding, isApplying) {
		const item = document.createElement('li');
		item.className = finding.level;
		item.appendChild(document.createTextNode(finding.text));

		if (finding.where) {
			const where = document.createElement('div');
			where.className = 'where';

			const label = document.createElement('span');
			label.className = 'tag';
			label.textContent = 'onde';
			where.appendChild(label);

			if (finding.anchor) {
				const link = document.createElement('button');
				link.className = 'goto';
				link.type = 'button';
				link.textContent = finding.where;
				link.title = `Abrir ${finding.anchor.path}:${finding.anchor.line}`;
				link.addEventListener('click', () =>
					vscode.postMessage({ type: 'openFile', path: finding.anchor.path, line: finding.anchor.line }),
				);
				where.appendChild(link);
			} else {
				where.appendChild(document.createTextNode(finding.where));
			}
			item.appendChild(where);
		}

		if (finding.insert) {
			const pre = document.createElement('pre');
			pre.className = 'insert';
			const code = document.createElement('code');
			code.textContent = finding.insert;
			pre.appendChild(code);
			item.appendChild(pre);
		}

		// While its fix runs the finding shows progress instead of buttons: the
		// agent is working on this exact file, and offering to start again would
		// be an invitation to do it twice.
		if (isApplying) {
			const running = document.createElement('div');
			running.className = 'finding-running';
			running.appendChild(dots());
			const label = document.createElement('span');
			label.textContent = 'aplicando…';
			running.appendChild(label);
			item.appendChild(running);
			return item;
		}

		// A finding that can be fixed carries the button that fixes it, and the
		// one that makes it go away — not every suggestion deserves acting on,
		// and one you cannot silence is one you learn to ignore wholesale.
		if (finding.id && finding.actionLabel) {
			const actions = document.createElement('div');
			actions.className = 'finding-actions';

			const apply = document.createElement('button');
			apply.className = 'apply';
			apply.type = 'button';
			apply.textContent = finding.actionLabel;
			apply.addEventListener('click', () => vscode.postMessage({ type: 'optimize', id: finding.id }));
			actions.appendChild(apply);

			const skip = document.createElement('button');
			skip.className = 'dismiss';
			skip.type = 'button';
			skip.textContent = 'Ignorar';
			skip.title = 'Não sugerir isto de novo neste projeto';
			skip.addEventListener('click', () => vscode.postMessage({ type: 'dismiss', id: finding.id }));
			actions.appendChild(skip);

			item.appendChild(actions);
		}
		return item;
	}

	function statGrid(pairs) {
		const grid = document.createElement('div');
		grid.className = 'stats';
		for (const [label, value] of pairs) {
			const cell = document.createElement('div');
			const v = document.createElement('span');
			v.className = 'value';
			v.textContent = value;
			const l = document.createElement('span');
			l.className = 'label';
			l.textContent = label;
			cell.appendChild(v);
			cell.appendChild(l);
			grid.appendChild(cell);
		}
		return grid;
	}

	function sectionTitle(text) {
		const el = document.createElement('h2');
		el.textContent = text;
		return el;
	}

	function readRow(row, inFlight) {
		const el = document.createElement('div');
		el.className = 'read';
		if (row.failed) {
			el.classList.add('failed');
		}

		const badge = document.createElement('span');
		badge.className = row.scoped ? 'badge scoped' : 'badge full';
		badge.textContent = row.scoped ? 'alvo' : 'inteiro';
		badge.title = row.scoped
			? 'A busca limitou o próprio escopo (offset/limit, ou um caminho).'
			: 'A busca não limitou o escopo — trouxe o arquivo ou a árvore toda.';
		el.appendChild(badge);

		const target = document.createElement('span');
		target.className = 'target';
		target.textContent = row.target || row.name;
		target.title = `${row.name} ${row.target}`.trim();
		el.appendChild(target);

		const size = document.createElement('span');
		size.className = 'size';
		size.textContent = inFlight ? '…' : `~${compact(row.tokens || 0)}`;
		el.appendChild(size);
		return el;
	}

	// ── skills and active file ──────────────────────────────────────────────

	/**
	 * One chip per skill installed in `.claude/skills/`, so the project's agents
	 * are visible without the user knowing their names. Skills not listed under
	 * `agents:` in `.gofi.yaml` are still shown — the file is on disk, so it is
	 * callable — just dimmed, so the enabled set reads at a glance.
	 *
	 * Before the first identity message we render placeholders rather than an
	 * empty bar, so the row doesn't jump into existence a beat later.
	 */
	function renderSkills() {
		chips.textContent = '';
		if (!skillsLoaded) {
			for (let i = 0; i < 4; i++) {
				const ghost = document.createElement('span');
				ghost.className = 'chip ghost';
				ghost.setAttribute('aria-hidden', 'true');
				chips.appendChild(ghost);
			}
			return;
		}
		for (const skill of skills) {
			const chip = document.createElement('button');
			chip.className = skill.enabled ? 'chip' : 'chip off';
			chip.type = 'button';
			chip.textContent = `/${skill.slug}`;
			chip.title = skill.title
				? `${skill.title}${skill.enabled ? '' : ' (fora do agents: do .gofi.yaml)'}`
				: skill.slug;
			chip.addEventListener('click', () => {
				const value = input.value;
				const prefix = value.trim() === '' || /\s$/.test(value) ? value : `${value} `;
				input.value = `${prefix}/${skill.slug} `;
				input.focus();
				input.setSelectionRange(input.value.length, input.value.length);
				autoGrow();
			});
			chips.appendChild(chip);
		}
	}

	// ── per-call approval ───────────────────────────────────────────────────

	/** Cards awaiting an answer, so a resolution can find and retire its own. */
	const approvalCards = new Map();

	/**
	 * The dialog the engine waits on before touching anything.
	 *
	 * Shown inline in the transcript rather than as a modal: it belongs to the
	 * turn that raised it, several can be outstanding, and a modal would steal
	 * focus from whatever the user is reading to decide.
	 */
	function showApproval(request) {
		clearEmptyState();
		clearIndicator();

		const card = document.createElement('div');
		card.className = 'approval';

		const head = document.createElement('div');
		head.className = 'approval-head';
		head.textContent = approvalTitle(request.tool);
		card.appendChild(head);

		const detail = approvalDetail(request.tool, request.input);
		if (detail.path) {
			const target = document.createElement('button');
			target.type = 'button';
			target.className = 'approval-path';
			target.textContent = detail.path;
			target.title = `Abrir ${detail.path}`;
			target.addEventListener('click', () => vscode.postMessage({ type: 'openFile', path: detail.path }));
			card.appendChild(target);
		}
		if (detail.body) {
			const pre = document.createElement('pre');
			pre.className = 'approval-body';
			const code = document.createElement('code');
			code.textContent = detail.body;
			pre.appendChild(code);
			card.appendChild(pre);
		}

		const actions = document.createElement('div');
		actions.className = 'approval-actions';
		const choose = (decision, reason) => {
			vscode.postMessage({ type: 'approve', id: request.id, decision, reason });
		};

		actions.appendChild(approvalButton('Permitir', 'yes', () => choose('allow')));
		actions.appendChild(
			approvalButton(`Sempre permitir ${request.tool}`, 'always', () => choose('always')),
		);
		actions.appendChild(approvalButton('Recusar', 'no', () => choose('deny', instruction.value.trim())));
		card.appendChild(actions);

		// A refusal carries a reason straight to the model, so "no, do it this
		// other way" is one step instead of a refusal plus a follow-up message.
		const instruction = document.createElement('input');
		instruction.className = 'approval-instruction';
		instruction.type = 'text';
		instruction.placeholder = 'Ao recusar: diga o que fazer em vez disso (opcional)';
		instruction.addEventListener('keydown', (event) => {
			if (event.key === 'Enter') {
				event.preventDefault();
				choose('deny', instruction.value.trim());
			}
		});
		card.appendChild(instruction);

		log.appendChild(card);
		approvalCards.set(request.id, card);
		queuePaint();
	}

	function approvalButton(label, kind, onClick) {
		const button = document.createElement('button');
		button.type = 'button';
		button.className = `approval-option ${kind}`;
		button.textContent = label;
		button.addEventListener('click', onClick);
		return button;
	}

	function approvalTitle(tool) {
		switch (tool) {
			case 'Bash':
				return 'Executar este comando?';
			case 'Write':
				return 'Criar ou sobrescrever este arquivo?';
			case 'Edit':
			case 'NotebookEdit':
				return 'Aplicar esta alteração?';
			default:
				return `Permitir ${tool}?`;
		}
	}

	/** What is worth showing so the decision is informed rather than reflexive. */
	function approvalDetail(tool, input) {
		const data = input || {};
		if (tool === 'Bash') {
			return { path: '', body: String(data.command || '') };
		}
		const path = String(data.file_path || data.notebook_path || '');
		if (tool === 'Write') {
			const content = String(data.content || '');
			return { path, body: content.length > 1200 ? `${content.slice(0, 1200)}\n…` : content };
		}
		// An edit reads as a diff — what leaves, what arrives.
		const before = String(data.old_string || '');
		const after = String(data.new_string || '');
		const trim = (text) => (text.length > 600 ? `${text.slice(0, 600)}\n…` : text);
		const body = [
			before ? trim(before).split('\n').map((l) => `- ${l}`).join('\n') : '',
			after ? trim(after).split('\n').map((l) => `+ ${l}`).join('\n') : '',
		]
			.filter(Boolean)
			.join('\n');
		return { path, body };
	}

	/** Replaces the card with what was decided, so the transcript records it. */
	function resolveApproval(id, decision) {
		const card = approvalCards.get(id);
		approvalCards.delete(id);
		if (!card) {
			return;
		}
		card.textContent = '';
		card.className = `approval settled ${decision === 'deny' ? 'denied' : 'allowed'}`;
		const line = document.createElement('div');
		line.className = 'approval-head';
		line.textContent =
			decision === 'deny' ? 'Recusado' : decision === 'always' ? 'Permitido — e não perguntar mais' : 'Permitido';
		card.appendChild(line);
	}

	/**
	 * Says whether the agent can currently change files.
	 *
	 * Worth a permanent spot: the difference between "it will answer" and "it
	 * will edit your repo" is the one thing about a session you should never
	 * have to guess at.
	 */
	function renderWriteMode(mode) {
		if (!mode) {
			return;
		}
		writeBadge.hidden = false;
		if (mode === 'guarded') {
			writeBadge.className = 'write blocked';
			writeBadge.textContent = 'aprovação por ação';
			writeBadge.title = 'Cada alteração de arquivo e cada comando pede sua aprovação antes de acontecer.';
		} else {
			writeBadge.className = 'write open';
			writeBadge.textContent = 'sem aprovação';
			writeBadge.title = 'O agente altera arquivos e roda comandos sem perguntar.';
		}
	}

	/** The file the user is looking at, shown just above the composer. */
	function renderActiveFile(file) {
		activeFile.textContent = '';
		if (!file || !showActiveFile) {
			activeFile.hidden = true;
			return;
		}

		const label = document.createElement('span');
		label.className = 'label';
		label.textContent = 'em foco';
		activeFile.appendChild(label);

		const button = document.createElement('button');
		button.className = 'file';
		button.type = 'button';
		button.title = `Inserir @${file.path} na mensagem`;

		const name = document.createElement('span');
		name.className = 'name';
		name.textContent = file.name;
		button.appendChild(name);

		const dir = file.path.slice(0, file.path.length - file.name.length).replace(/\/$/, '');
		if (dir !== '') {
			const dirEl = document.createElement('span');
			dirEl.className = 'dir';
			dirEl.textContent = dir;
			button.appendChild(dirEl);
		}
		if (file.selection) {
			const lines = document.createElement('span');
			lines.className = 'lines';
			lines.textContent =
				file.selection.start === file.selection.end
					? `L${file.selection.start}`
					: `L${file.selection.start}–${file.selection.end}`;
			button.appendChild(lines);
		}

		button.addEventListener('click', () => {
			const value = input.value;
			const prefix = value.trim() === '' || /\s$/.test(value) ? value : `${value} `;
			input.value = `${prefix}@${file.path} `;
			input.focus();
			input.setSelectionRange(input.value.length, input.value.length);
			autoGrow();
		});
		activeFile.appendChild(button);
		activeFile.hidden = false;
	}

	// ── extension messages ──────────────────────────────────────────────────

	window.addEventListener('message', (event) => {
		const message = event.data;
		// Sample "was the user at the bottom" before mutating, then scroll once
		// per frame — scrolling synchronously on every message forces a layout
		// per delta, which is its own source of stutter.
		followTail = followTail && atBottom();

		switch (message.type) {
			case 'identity': {
				showThinking = message.showThinking !== false;
				showToolCalls = message.showToolCalls !== false;
				showActiveFile = message.showActiveFile !== false;
				supportsImages = message.supportsImages === true;
				renderWriteMode(message.approvalRequired ? 'guarded' : 'open');
				skills = Array.isArray(message.skills) ? message.skills : [];
				skillsLoaded = true;
				projectName = message.projectName || null;
				hasConfig = message.hasConfig === true;
				subtitle.textContent = projectName
					? `${projectName}  ·  ${message.providerLabel}`
					: message.providerLabel;
				renderSkills();
				// The greeting is written before identity arrives; rewrite it now
				// that we know whose project this is.
				if (log.querySelector('.empty-state')) {
					log.textContent = '';
					showEmptyState();
				}
				refreshPicker();
				if (!message.hasWorkspace) {
					notice('Nenhuma pasta aberta.', 'O motor roda na raiz do workspace — abra o projeto para conversar.');
				}
				break;
			}

			case 'approval':
				showApproval(message);
				break;

			case 'approvalResolved':
				resolveApproval(message.id, message.decision);
				break;

			case 'activeFile':
				renderActiveFile(message.file);
				break;

			case 'fileResults':
				onFileResults(message.query, message.files || []);
				break;

			case 'meta':
				if (message.model) {
					const line = document.createElement('div');
					line.className = 'meta';
					line.textContent = message.model;
					assistantTurn().appendChild(line);
				}
				break;

			case 'user': {
				endTurn();
				const body = turn('user', 'você');
				if (message.queued) {
					// Waiting behind another turn — badge it and skip the
					// bottom "pensando" row (the current turn is still
					// streaming and that would give two indicators).
					body.parentElement.classList.add('queued');
				}
				if (message.images > 0) {
					const note = document.createElement('div');
					note.className = 'attached-note';
					note.textContent = message.images === 1 ? '1 imagem anexada' : `${message.images} imagens anexadas`;
					body.appendChild(note);
				}
				if (message.text !== '') {
					userBubble(body, message.text);
				}
				if (!message.queued) {
					showIndicator();
				}
				break;
			}

			case 'dequeued': {
				// The next queued message just started running. Clear its badge
				// and cover the gap between the previous turn's `done` and this
				// turn's first delta with the usual pending row.
				const first = log.querySelector('.turn.user.queued');
				if (first) {
					first.classList.remove('queued');
				}
				showIndicator();
				break;
			}

			case 'delta': {
				const el = assistantTurn();
				if (message.kind === 'text') {
					if (!streamingText) {
						const bubble = document.createElement('div');
						bubble.className = 'bubble streaming';
						const node = document.createTextNode('');
						bubble.appendChild(node);
						// Cursor as the next sibling of the Text node: appendData
						// grows the text, cursor stays visually at the tail.
						const cursor = streamingCursor();
						bubble.appendChild(cursor);
						el.appendChild(bubble);
						streamingText = { bubble, node, cursor, raw: '' };
					}
					// Raw append, always: the authoritative `blocks` event that
					// closes the message renders the Markdown properly, and
					// re-parsing on every token is what made this lag.
					streamingText.raw += message.text;
					streamingText.node.appendData(message.text);
				} else if (showThinking) {
					if (!streamingThinking) {
						streamingThinking = thinkingBlock(el, '');
					}
					streamingThinking.appendChild(document.createTextNode(message.text));
				}
				break;
			}

			case 'blocks': {
				const el = assistantTurn();
				// The streamed preview is replaced by the authoritative blocks —
				// the deltas were for latency, these are the record.
				if (streamingText) {
					streamingText.bubble.remove();
					streamingText = null;
				}
				if (streamingThinking) {
					streamingThinking.parentElement?.remove();
					streamingThinking = null;
				}
				for (const block of message.blocks) {
					if (block.type === 'text') {
						textBubble(el, block.text);
					} else if (block.type === 'thinking' && showThinking) {
						thinkingBlock(el, block.text);
					} else if (block.type === 'tool_use' && showToolCalls) {
						pendingTools.set(block.id, toolRow(el, block.name, block.input));
					}
				}
				break;
			}

			case 'toolResult': {
				const row = pendingTools.get(message.toolUseId);
				pendingTools.delete(message.toolUseId);
				if (!row) {
					break;
				}
				row.classList.remove('running');
				if (message.isError) {
					row.classList.add('failed');
				}
				const outcome = document.createElement('span');
				outcome.className = 'outcome';
				outcome.textContent = message.isError ? 'erro' : message.preview;
				row.appendChild(outcome);
				break;
			}

			case 'usage':
				renderUsage(message.snapshot);
				break;

			case 'running':
				// Submit stays enabled while running so new messages can queue.
				cancel.hidden = !message.running;
				document.body.classList.toggle('busy', message.running);
				if (!message.running) {
					clearIndicator();
					pendingTools.clear();
				}
				break;

			case 'done': {
				const el = assistantTurn();
				if (message.isError && message.error) {
					notice(message.error);
				}
				if (typeof message.durationMs === 'number') {
					const line = document.createElement('div');
					line.className = 'meta';
					line.textContent = `${(message.durationMs / 1000).toFixed(1)}s`;
					el.appendChild(line);
				}
				clearIndicator();
				endTurn();
				break;
			}

			case 'error':
				clearIndicator();
				notice(message.message, message.hint);
				endTurn();
				break;

			case 'cleared':
				log.textContent = '';
				pendingTools.clear();
				clearIndicator();
				endTurn();
				showEmptyState();
				break;

			default:
				break;
		}

		queuePaint();
	});

	/** Coalesces scroll-to-tail into one write per animation frame. */
	function queuePaint() {
		if (paintQueued) {
			return;
		}
		paintQueued = true;
		requestAnimationFrame(() => {
			paintQueued = false;
			if (followTail) {
				log.scrollTop = log.scrollHeight;
			}
			followTail = atBottom();
		});
	}

	// The user scrolling up is the signal to stop chasing the tail.
	log.addEventListener('scroll', () => {
		if (!paintQueued) {
			followTail = atBottom();
		}
	});

	function notice(text, hint) {
		clearEmptyState();
		const el = document.createElement('div');
		el.className = 'notice';
		el.appendChild(document.createTextNode(text));
		if (hint) {
			const line = document.createElement('span');
			line.className = 'hint';
			line.textContent = hint;
			el.appendChild(line);
		}
		log.appendChild(el);
	}

	vscode.postMessage({ type: 'ready' });
})();
