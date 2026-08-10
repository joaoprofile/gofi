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
	const attachBtn = /** @type {HTMLButtonElement} */ (el('attachBtn'));
	const attachMenu = el('attachMenu');
	const attachUpload = /** @type {HTMLButtonElement} */ (el('attachUpload'));
	const attachProject = /** @type {HTMLButtonElement} */ (el('attachProject'));
	const fileInput = /** @type {HTMLInputElement} */ (el('fileInput'));
	const writeBadge = el('writeBadge');
	const historyBtn = /** @type {HTMLButtonElement} */ (el('historyBtn'));
	const history = el('history');
	const historyNew = /** @type {HTMLButtonElement} */ (el('historyNew'));
	const historySearch = /** @type {HTMLInputElement} */ (el('historySearch'));
	const historyList = el('historyList');

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
	/**
	 * What is riding along with the next message.
	 *
	 * Two kinds, because they travel differently: an image goes as bytes (the
	 * only way to show a picture to a model) and a document goes as a path (the
	 * agent opens it if it needs it — see `filesPreamble` on the host).
	 *
	 * @type {({id: number, kind: 'image', name: string, mediaType: string, data: string, url: string}
	 *   | {id: number, kind: 'file', name: string, path: string, rel: string|null, size: number})[]}
	 */
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
	/**
	 * The live "working" row: the last line of the transcript while a turn runs.
	 *
	 * Built once and kept, because re-inserting a node restarts every CSS
	 * animation on it. Everything else is added to the log *before* it, so it
	 * stays at the end without ever being moved — which is what lets the whip
	 * keep swinging for the whole turn instead of resetting to its first frame
	 * each time the agent writes a line.
	 */
	let workingRow = null;
	/**
	 * The label inside that row, and the word currently in it.
	 *
	 * `activity` is the verb for what the agent is doing — set from the tools it
	 * calls and the kind of tokens it streams, null when there is nothing more
	 * specific to say than that it is working.
	 */
	let workingLabel = null;
	let workingVerb = null;
	/** @type {string|null} */
	let activity = null;
	/** Tool rows awaiting their result, keyed by tool_use id. */
	const pendingTools = new Map();
	/**
	 * Set while a saved transcript is being rebuilt.
	 *
	 * Replay reuses the live handlers — the same events, so the same rendering —
	 * but a few of them are about what is happening *now*: the working indicator
	 * and the scroll-follow only make sense for a turn in flight.
	 */
	let replaying = false;
	/**
	 * Whether a turn is in flight, and whether it is currently blocked on the
	 * user.
	 *
	 * The working row follows both: it belongs on screen for as long as the
	 * agent is working, and off it while the agent is waiting for an answer to
	 * an approval — at that point the one holding things up is you, and a row
	 * saying "trabalhando" would be a lie.
	 */
	let running = false;
	let awaitingApprovals = 0;

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
			images: attachments
				.filter((a) => a.kind === 'image')
				.map((a) => ({ mediaType: a.mediaType, data: a.data })),
			// Whole items, not a hand-picked subset: a file from outside the project
			// carries the text the host already read for it, and re-listing fields
			// here is how that content would get silently dropped.
			files: attachments.filter((a) => a.kind === 'file'),
		});
		input.value = '';
		attachments = [];
		renderAttachments();
		closePicker();
		closeAttachMenu();
		autoGrow();
	}

	// ── the `+` menu ────────────────────────────────────────────────────────

	/**
	 * Two ways to put a file in front of the agent, in one menu.
	 *
	 * "Do computador" is the operating system's own dialog — any folder, any
	 * drive, wherever the user is sitting. "Do projeto" is the `@` picker, which
	 * is already the cheapest way to name a file the agent can reach; the menu
	 * just tells the user it exists.
	 */
	function attachMenuOpen() {
		return !attachMenu.hidden;
	}

	function openAttachMenu() {
		attachMenu.hidden = false;
		attachBtn.setAttribute('aria-expanded', 'true');
	}

	function closeAttachMenu() {
		attachMenu.hidden = true;
		attachBtn.setAttribute('aria-expanded', 'false');
	}

	attachBtn.addEventListener('click', (event) => {
		event.stopPropagation();
		if (attachMenuOpen()) {
			closeAttachMenu();
		} else {
			closePicker();
			openAttachMenu();
		}
	});

	attachUpload.addEventListener('click', () => {
		closeAttachMenu();
		fileInput.click();
	});

	fileInput.addEventListener('change', () => {
		for (const file of [...(fileInput.files || [])]) {
			attachFromDisk(file);
		}
		// Clearing is what makes picking the same file twice work: `change` does
		// not fire when the value has not changed.
		fileInput.value = '';
	});

	attachProject.addEventListener('click', () => {
		closeAttachMenu();
		// Hand the user the trigger rather than explaining it: an `@` at the
		// caret is the file picker, already open.
		const caret = input.selectionStart ?? input.value.length;
		const before = input.value.slice(0, caret);
		const prefix = before === '' || /\s$/.test(before) ? '' : ' ';
		input.value = `${before}${prefix}@${input.value.slice(caret)}`;
		const at = caret + prefix.length + 1;
		input.focus();
		input.setSelectionRange(at, at);
		autoGrow();
		refreshPicker();
	});

	// Anywhere else closes it — a menu that needs its own button clicked again to
	// go away is a menu in the way.
	document.addEventListener('click', (event) => {
		if (attachMenuOpen() && !attachMenu.contains(event.target) && event.target !== attachBtn) {
			closeAttachMenu();
		}
	});

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

	/** Bigger than this and neither the engine nor the window wants it. */
	const MAX_IMAGE_BYTES = 5 * 1024 * 1024;
	const MAX_TEXT_BYTES = 2 * 1024 * 1024;
	const MAX_INLINE_CHARS = 40000;

	/**
	 * A file the user picked in the OS dialog.
	 *
	 * An image goes the same way a pasted one does — bytes, as an image block.
	 * Anything else is read here as text and travels in the prompt, because a file
	 * on the user's machine is not something the agent can open: the engine runs in
	 * the project root, and on a remote window it is not even the same computer.
	 * A file *inside* the project should go by path instead, which is what the
	 * other menu item is for.
	 */
	function attachFromDisk(file) {
		const isImage = (file.type || '').startsWith('image/');
		if (isImage && !supportsImages) {
			notice(`${file.name} não foi anexado.`, 'este motor não aceita imagens.');
			return;
		}
		if (isImage) {
			if (file.size > MAX_IMAGE_BYTES) {
				notice(
					`${file.name} não foi anexado.`,
					`imagem de ${Math.round(file.size / 1024)} KB — o limite é ${MAX_IMAGE_BYTES / 1024 / 1024} MB.`,
				);
				return;
			}
			attach(file);
			return;
		}
		attachDocument(file);
	}

	/**
	 * A document, read in the window and carried in the message.
	 *
	 * The states are the chip's whole vocabulary and the prompt's: too big to
	 * carry, binary (nothing useful to say about the bytes), or text — possibly
	 * truncated, and if it was, that is stated in the prompt as well. A truncated
	 * file that looks whole is how someone concludes the model ignored it.
	 */
	function attachDocument(file) {
		const chip = {
			id: ++attachmentSeq,
			kind: 'file',
			name: file.name || 'arquivo',
			path: null,
			rel: null,
			size: file.size || 0,
		};
		if (chip.size > MAX_TEXT_BYTES) {
			attachments.push({ ...chip, state: 'too-big' });
			renderAttachments();
			return;
		}
		const reader = new FileReader();
		reader.onload = () => {
			const bytes = new Uint8Array(/** @type {ArrayBuffer} */ (reader.result));
			// A NUL byte in the head of the file is the cheap, reliable tell: no
			// UTF-8 text has one, every binary format we might be handed does.
			if (bytes.subarray(0, 8192).includes(0)) {
				attachments.push({ ...chip, state: 'binary' });
			} else {
				const text = new TextDecoder('utf-8').decode(bytes);
				attachments.push({
					...chip,
					state: 'text',
					text: text.slice(0, MAX_INLINE_CHARS),
					truncated: text.length > MAX_INLINE_CHARS,
				});
			}
			renderAttachments();
		};
		reader.onerror = () => notice(`${chip.name} não foi anexado.`, 'não consegui ler o arquivo.');
		reader.readAsArrayBuffer(file);
	}

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
				kind: 'image',
				name: file.name || 'imagem colada',
				mediaType: file.type || 'image/png',
				data: url.slice(comma + 1),
				url,
			});
			renderAttachments();
		};
		reader.readAsDataURL(file);
	}

	/** 2048 → "2 KB". Chips have no room for exact bytes and nobody reads them. */
	function fileSize(bytes) {
		if (bytes >= 1024 * 1024) {
			return `${(bytes / 1024 / 1024).toFixed(1)} MB`;
		}
		return `${Math.max(1, Math.round(bytes / 1024))} KB`;
	}

	/**
	 * What will happen to this file, in two words on the chip.
	 *
	 * Worth the words: "no projeto" means the agent opens it if it needs to, and
	 * anything else means the file's text is going into the prompt — including
	 * whether all of it made it. A truncated attachment that looks whole is how
	 * someone concludes the model ignored the file.
	 */
	function attachmentDetail(item) {
		if (item.rel) {
			return 'no projeto';
		}
		if (item.state === 'too-big') {
			return 'grande demais para anexar';
		}
		if (item.state === 'binary') {
			return 'externo · binário';
		}
		return item.truncated ? 'externo · truncado' : 'externo';
	}

	function renderAttachments() {
		attachmentsBar.textContent = '';
		attachmentsBar.hidden = attachments.length === 0;
		for (const item of attachments) {
			const chip = document.createElement('span');
			chip.className = `attachment ${item.kind}`;

			if (item.kind === 'image') {
				const thumb = document.createElement('img');
				thumb.src = item.url;
				thumb.alt = item.name;
				chip.appendChild(thumb);
			} else {
				// A document has nothing to preview, so the chip says the two
				// things that matter: which file, and whether the agent will be
				// reading it from the project or from outside it.
				const name = document.createElement('span');
				name.className = 'file-name';
				name.textContent = item.name;
				chip.appendChild(name);

				const detail = document.createElement('span');
				detail.className = 'file-detail';
				detail.textContent = `${attachmentDetail(item)} · ${fileSize(item.size)}`;
				chip.appendChild(detail);
				chip.title = item.rel || item.path || item.name;
			}

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
		if (event.key === 'Escape' && attachMenuOpen()) {
			event.preventDefault();
			closeAttachMenu();
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
			// `/model` is a client-side switch, not a project skill — but it lives
			// in the same picker so users find it the same way. Prepended, and
			// only when the query still matches it.
			const items = [];
			if ('model'.startsWith(needle)) {
				items.push({ value: '/model', label: '/model', detail: 'trocar o modelo desta conversa' });
			}
			for (const s of skills) {
				if (s.slug.toLowerCase().includes(needle)) {
					items.push({ value: `/${s.slug}`, label: `/${s.slug}`, detail: s.title });
				}
			}
			pickerItems = items;
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
		for (const [key, text] of [
			['/', 'chama uma skill do projeto  ·  /model troca o modelo'],
			['@', 'referencia um arquivo'],
		]) {
			const item = document.createElement('li');
			const kbd = document.createElement('kbd');
			kbd.textContent = key;
			item.appendChild(kbd);
			item.appendChild(document.createTextNode(` ${text}`));
			hints.appendChild(item);
		}
		empty.appendChild(hints);
		addToLog(empty);
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

		addToLog(el);
		return body;
	}

	function assistantTurn() {
		if (!currentTurn) {
			currentTurn = turn('assistant', 'GOFI AI');
			currentTurn.parentElement.classList.add('live');
			// The indicator belongs *below* the answer being written, so it moves
			// to the end rather than being dismissed by the first token: the turn
			// has only started, and a long one keeps working for minutes after.
			showIndicator();
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

	const SVG_NS = 'http://www.w3.org/2000/svg';

	function svgEl(name, attributes) {
		const el = document.createElementNS(SVG_NS, name);
		for (const key of Object.keys(attributes)) {
			el.setAttribute(key, attributes[key]);
		}
		return el;
	}

	/** Appends `[tag, attributes, text?]` triples to a node, in order. */
	function draw(parent, shapes) {
		for (const [name, attributes, text] of shapes) {
			const node = svgEl(name, attributes);
			if (text !== undefined) {
				node.textContent = text;
			}
			parent.appendChild(node);
		}
		return parent;
	}

	/**
	 * Six drawings of the same arm, shown one at a time — one full turn of the whip.
	 *
	 * A whip is not a rigid stick, so swinging one by rotating a curve does not
	 * read as a whip — it reads as a bent wire being waved. Hand-drawn poses are
	 * how cartoons have always done this, and they buy something else too: the
	 * animation is opacity only, with no dependence on how an engine resolves
	 * `transform-origin` inside an SVG. Nothing can rotate about the wrong point
	 * because nothing rotates.
	 *
	 * The hands trace a circle around the shoulder at (25.2, 23.4) — down and
	 * behind, up behind the head, over the top, forward, and back again. That
	 * circle is the animation: a whip that only appears and disappears in front
	 * reads as a flicker, while a whip that comes round from behind reads as one
	 * being *swung*, with the force that implies.
	 *
	 * `behind: true` puts that pose's lash under the mascot instead of over it, so
	 * the tail genuinely disappears behind the body on the way round. It is the
	 * cheapest depth an SVG has — drawing order — and the only reason the swing
	 * looks like it leaves the picture plane.
	 *
	 * The strike lash ends at x=63.5, the robot's near edge: landing it on the
	 * robot is the whole point, so that number is the one to preserve if the scene
	 * is ever re-laid-out.
	 */
	const WHIP_POSES = [
		// Trailing low behind, where the swing starts and ends.
		{ name: 'back', hand: [18.5, 29.5], lash: 'M18.5 29.5 C 12 34, 5 33.5, 0.5 29', behind: true },
		// Coming up behind the mascot's own head, hidden by it. The hand stays
		// outside the head (centre 17, radius 8.8) — an arm drawn across the face
		// is not a raised whip, it is a mascot punching itself — and the lash goes
		// up and to the left, where the body swallows it.
		{ name: 'rise', hand: [26.5, 15], lash: 'M26.5 15 C 23 5.5, 14 2, 7.5 6.5', behind: true },
		// Over the top: the highest point of the circle, now in front.
		{ name: 'over', hand: [28.5, 13], lash: 'M28.5 13 C 36 3.5, 47 3, 53 8.5' },
		// Coming down onto the robot. This is the hit.
		{ name: 'strike', hand: [34, 19], lash: 'M34 19 C 46 11.5, 55 13.5, 63.5 18.5' },
		// Taut, into the first coil's left end — this is the pose the wrap hangs
		// off, so its lash has to arrive exactly where the coil starts (63.5).
		{ name: 'wrap', hand: [34, 21], lash: 'M34 21 C 44 16.5, 53 19.5, 63.5 22.5' },
		// Slack, on the way back down and round.
		{ name: 'pull', hand: [30, 25.5], lash: 'M30 25.5 C 38 31, 47 23, 54 28' },
	];

	/**
	 * The coils the lash makes around the robot, as `[cx, cy, rx, ry]`.
	 *
	 * Three of them, head to lap, because "wrapped around the robot" is a whole
	 * body being tied up and not a rope touching it somewhere. Each is drawn
	 * twice: the far half behind the robot, where its own navy hides it, and the
	 * near half in front. That over-and-under is the only thing that makes a
	 * stroke read as *around* a shape rather than on top of it — and it is why
	 * the endpoints (cx ± rx) sit a unit and a half outside the silhouette, where
	 * the rope has to be visible for the two halves to join.
	 */
	const COILS = [
		// Neck, not face: a loop at eye level (y=11.8) covers the eyes, and the
		// eyes are the reaction — the joke dies if you cannot see the robot take it.
		[74.5, 17.4, 8, 1.8],
		[74.5, 23, 11, 2],
		[74.5, 29.5, 10, 1.8],
	];

	/** The near half of a coil: sags towards the viewer. */
	function coilFront([cx, cy, rx, ry]) {
		return `M${cx - rx} ${cy} Q ${cx} ${cy + ry * 2}, ${cx + rx} ${cy}`;
	}

	/** The far half: arcs away, behind the body. */
	function coilBack([cx, cy, rx, ry]) {
		return `M${cx - rx} ${cy} Q ${cx} ${cy - ry * 2}, ${cx + rx} ${cy}`;
	}

	/**
	 * The working indicator: the gofi mascot standing over the robot with a whip.
	 *
	 * A spinner says "wait". This says who is waiting and who is working, which
	 * is the funnier and more accurate description of what is happening — and
	 * being a drawing rather than a spinner, you can tell across the room whether
	 * the panel is busy.
	 *
	 * The two characters are the product's own: the armoured mascot holds the
	 * whip, the robot at the keyboard takes it. Drawn here rather than shipped as
	 * a file because it has to move, and CSS can only reach the parts if they are
	 * elements — six arm poses around a full swing, the spark where the lash lands,
	 * the coils it leaves wrapped around the robot, the robot flinching and
	 * squirming, the sweat that follows, and hands that never stop typing through
	 * any of it.
	 */
	function workingMark() {
		const mark = svgEl('svg', {
			class: 'whip-mark',
			viewBox: '0 0 104 44',
			width: '104',
			height: '44',
			'aria-hidden': 'true',
			focusable: 'false',
		});

		// The lash burns from yellow at the grip to red at the tip — a gradient
		// rather than a flat stroke, because "hot" is a change along the length and
		// a single red line is just a red line. In user space, so the colour belongs
		// to the scene: wherever a pose puts the tip, the tip is the red end.
		const defs = svgEl('defs', {});
		const fire = svgEl('linearGradient', {
			id: 'gofi-lash-fire',
			gradientUnits: 'userSpaceOnUse',
			x1: '22',
			y1: '0',
			x2: '92',
			y2: '0',
		});
		draw(fire, [
			['stop', { offset: '0', 'stop-color': '#fde047' }],
			['stop', { offset: '0.35', 'stop-color': '#fb923c' }],
			['stop', { offset: '0.7', 'stop-color': '#ef4444' }],
			['stop', { offset: '1', 'stop-color': '#dc2626' }],
		]);
		defs.appendChild(fire);
		mark.appendChild(defs);

		// The floor, so the two characters read as one scene rather than two
		// drawings that happen to be side by side.
		draw(mark, [['line', { class: 'floor', x1: '6', y1: '39.4', x2: '98', y2: '39.4' }]]);

		// The poses whose lash passes behind the mascot go down before it, so the
		// tail disappears behind the body on the way round. Their arms are added
		// with the others further down, in front, where an arm belongs.
		const trailing = svgEl('g', { class: 'trailing' });
		for (const pose of WHIP_POSES.filter((entry) => entry.behind)) {
			const group = svgEl('g', { class: `pose ${pose.name}` });
			draw(group, [['path', { class: 'lash', d: pose.lash }]]);
			trailing.appendChild(group);
		}
		mark.appendChild(trailing);

		// The far half of each coil goes down first, so the robot drawn next
		// covers it: that is the whole trick behind the rope looking wrapped.
		const behind = svgEl('g', { class: 'coil back' });
		draw(behind, COILS.map((coil) => ['path', { class: 'rope', d: coilBack(coil) }]));
		mark.appendChild(behind);

		const robot = svgEl('g', { class: 'robot' });
		draw(robot, [
			['rect', { class: 'robot-body', x: '65', y: '18', width: '19', height: '15.5', rx: '4' }],
			['rect', { class: 'robot-head', x: '68', y: '7', width: '13', height: '10', rx: '3' }],
			['line', { class: 'wire', x1: '74.5', y1: '7', x2: '74.5', y2: '3.2' }],
			['circle', { class: 'bulb', cx: '74.5', cy: '2.2', r: '1.5' }],
			['circle', { class: 'eye', cx: '71.3', cy: '11.8', r: '1.4' }],
			['circle', { class: 'eye', cx: '77.7', cy: '11.8', r: '1.4' }],
			['line', { class: 'mouth', x1: '72.2', y1: '15', x2: '76.8', y2: '15' }],
			// Arms down and out to the keyboard, where the hands are.
			['line', { class: 'wire', x1: '65', y1: '22', x2: '61.2', y2: '28.4' }],
			['line', { class: 'wire', x1: '84', y1: '22', x2: '87.8', y2: '28.4' }],
			['line', { class: 'wire', x1: '69', y1: '33.5', x2: '69', y2: '38.6' }],
			['line', { class: 'wire', x1: '80', y1: '33.5', x2: '80', y2: '38.6' }],
		]);
		mark.appendChild(robot);

		// And the near half on top of it. Same group name, opposite side: one
		// animation rule drives both halves, so they can never fall out of step.
		const front = svgEl('g', { class: 'coil front' });
		draw(front, COILS.map((coil) => ['path', { class: 'rope', d: coilFront(coil) }]));
		mark.appendChild(front);

		// It is not the whip that gets the work done — the typing never stops,
		// which is the other half of the joke.
		draw(mark, [
			['circle', { class: 'hand left', cx: '61.2', cy: '28.4', r: '1.7' }],
			['circle', { class: 'hand right', cx: '87.8', cy: '28.4', r: '1.7' }],
		]);

		// The keyboard goes on top of the robot's lower body: it is in front of
		// the desk, and drawing order is the only depth an SVG has.
		draw(mark, [
			['rect', { class: 'keyboard', x: '58.5', y: '29.6', width: '32', height: '4.4', rx: '1.5' }],
			['line', { class: 'key', x1: '65', y1: '30.8', x2: '65', y2: '32.8' }],
			['line', { class: 'key', x1: '72', y1: '30.8', x2: '72', y2: '32.8' }],
			['line', { class: 'key', x1: '79', y1: '30.8', x2: '79', y2: '32.8' }],
			['line', { class: 'key', x1: '86', y1: '30.8', x2: '86', y2: '32.8' }],
		]);

		// A drop, one beat after the lash lands. It overlaps the head's top-right
		// corner (the head ends at x=81) so it reads as coming off the robot
		// rather than floating beside it.
		draw(mark, [['path', { class: 'sweat', d: 'M82 7.4 C 83.5 10, 83.5 11.6, 82 11.6 C 80.5 11.6, 80.5 9.9, 82 7.4 Z' }]]);

		// Where the lash lands, drawn at the strike pose's endpoint so the flash
		// and the whip are in the same place at the same instant.
		const spark = svgEl('g', { class: 'spark' });
		draw(spark, [
			['line', { x1: '64', y1: '18.5', x2: '60', y2: '14.6' }],
			['line', { x1: '64', y1: '18.5', x2: '59.2', y2: '19.4' }],
			['line', { x1: '64', y1: '18.5', x2: '60.6', y2: '23' }],
		]);
		mark.appendChild(spark);

		// The mascot: the gofi bear in armour, drawn as a caricature of itself —
		// at this size the plate lettering would be a smudge, so the chest plate
		// carries the gold and the shape carries the rest.
		const mascot = svgEl('g', { class: 'mascot' });
		draw(mascot, [
			['circle', { class: 'fur', cx: '10.5', cy: '7.4', r: '2.9' }],
			['circle', { class: 'fur', cx: '23.5', cy: '7.4', r: '2.9' }],
			['rect', { class: 'armor', x: '9.6', y: '21.6', width: '14.8', height: '12.6', rx: '3.4' }],
			['rect', { class: 'plate', x: '12.2', y: '24', width: '9.6', height: '7', rx: '1.6' }],
			// The G on the breastplate: who is holding the whip. At this size the
			// full name would be a smudge, so the initial carries it — centred on
			// the plate, in the navy the gold was chosen to sit against.
			//
			// The baseline is a number rather than `dominant-baseline: central`: the
			// plate spans y 24–31, a 6.4px cap is ~4.6 tall, so 29.8 puts the letter
			// in the middle of it without depending on how an engine resolves
			// baseline keywords inside an SVG.
			['text', { class: 'emblem', x: '17', y: '29.8', 'text-anchor': 'middle' }, 'G'],
			['rect', { class: 'armor', x: '12', y: '33.4', width: '4', height: '5.6', rx: '1.6' }],
			['rect', { class: 'armor', x: '18.2', y: '33.4', width: '4', height: '5.6', rx: '1.6' }],
			// The idle arm, on the other side, so the figure is not all one diagonal.
			['line', { class: 'armor-limb', x1: '10.2', y1: '25.4', x2: '6.4', y2: '30.4' }],
			['circle', { class: 'fur', cx: '17', cy: '13.5', r: '8.8' }],
			['circle', { class: 'sclera', cx: '13.4', cy: '12', r: '3.1' }],
			['circle', { class: 'sclera', cx: '20.6', cy: '12', r: '3.1' }],
			['circle', { class: 'pupil', cx: '14.1', cy: '12.3', r: '1.35' }],
			['circle', { class: 'pupil', cx: '21.3', cy: '12.3', r: '1.35' }],
			['ellipse', { class: 'pupil', cx: '17', cy: '16', rx: '1.7', ry: '1.2' }],
			['rect', { class: 'tooth', x: '15.7', y: '17', width: '1.3', height: '2.8', rx: '0.4' }],
			['rect', { class: 'tooth', x: '17.4', y: '17', width: '1.3', height: '2.8', rx: '0.4' }],
			// The pauldron sits over the shoulder the whip comes out of, so the
			// arm poses read as attached to something.
			['circle', { class: 'armor', cx: '25.2', cy: '23.4', r: '3.4' }],
		]);
		mark.appendChild(mascot);

		for (const pose of WHIP_POSES) {
			const group = svgEl('g', { class: `pose ${pose.name}` });
			// The arm is always in front; the lash only when it is not the tail
			// already drawn behind the mascot.
			draw(group, [['line', { class: 'armor-limb', x1: '25.2', y1: '23.4', x2: String(pose.hand[0]), y2: String(pose.hand[1]) }]]);
			if (!pose.behind) {
				draw(group, [['path', { class: 'lash', d: pose.lash }]]);
			}
			mark.appendChild(group);
		}

		return mark;
	}

	/**
	 * The two characters from the working mark, both standing still: the
	 * mascot with the whip hanging slack in its hand, the robot waiting beside
	 * it. Same cast, same stage, but the whip is cold — this is the one moment
	 * the engine itself said stop, and there is nothing left to strike with
	 * until it says otherwise.
	 *
	 * Static on purpose, unlike `workingMark`. The mascot's face is doing the
	 * telling: brows down, eyes half-closed, mouth turned — disappointed at
	 * being unable to use the whip, not at the robot, which is why the robot's
	 * own face stays as calm as the mascot's is not.
	 */
	function waitingMark() {
		const mark = svgEl('svg', {
			class: 'wait-mark',
			viewBox: '0 0 104 44',
			width: '104',
			height: '44',
			'aria-hidden': 'true',
			focusable: 'false',
		});

		draw(mark, [['line', { class: 'floor', x1: '4', y1: '39.4', x2: '98', y2: '39.4' }]]);

		// The mascot, built from the same parts as `workingMark`'s — the point is
		// that it is recognisably the same character, just not mid-swing.
		const mascot = svgEl('g', { class: 'mascot' });
		draw(mascot, [
			['circle', { class: 'fur', cx: '10.5', cy: '7.4', r: '2.9' }],
			['circle', { class: 'fur', cx: '23.5', cy: '7.4', r: '2.9' }],
			['rect', { class: 'armor', x: '9.6', y: '21.6', width: '14.8', height: '12.6', rx: '3.4' }],
			['rect', { class: 'plate', x: '12.2', y: '24', width: '9.6', height: '7', rx: '1.6' }],
			['text', { class: 'emblem', x: '17', y: '29.8', 'text-anchor': 'middle' }, 'G'],
			['rect', { class: 'armor', x: '12', y: '33.4', width: '4', height: '5.6', rx: '1.6' }],
			['rect', { class: 'armor', x: '18.2', y: '33.4', width: '4', height: '5.6', rx: '1.6' }],
			// The off hand, idle at its side.
			['line', { class: 'armor-limb', x1: '10.2', y1: '25.4', x2: '6.4', y2: '30.4' }],
			['circle', { class: 'fur', cx: '17', cy: '13.5', r: '8.8' }],
			['circle', { class: 'sclera', cx: '13.4', cy: '12', r: '3.1' }],
			['circle', { class: 'sclera', cx: '20.6', cy: '12', r: '3.1' }],
			// Pupils cast down rather than centred — looking at the whip, not ahead.
			['circle', { class: 'pupil', cx: '13.4', cy: '13', r: '1.35' }],
			['circle', { class: 'pupil', cx: '20.6', cy: '13', r: '1.35' }],
			// Eyelids drawn over the top third of each eye: half-closed reads as
			// tired/resigned in a way two full circles never could.
			['ellipse', { class: 'eyelid', cx: '13.4', cy: '10.3', rx: '3.3', ry: '2.1' }],
			['ellipse', { class: 'eyelid', cx: '20.6', cy: '10.3', rx: '3.3', ry: '2.1' }],
			// Brows angled in and up — worried, not angry (angry tilts the other way).
			['line', { class: 'brow', x1: '10.8', y1: '8.6', x2: '14.6', y2: '7.6' }],
			['line', { class: 'brow', x1: '23.2', y1: '8.6', x2: '19.4', y2: '7.6' }],
			// A frown, not the strike pose's bared teeth.
			['path', { class: 'mouth-frown', d: 'M14.3 18.3 Q 17 16.4, 19.7 18.3' }],
			['circle', { class: 'armor', cx: '25.2', cy: '23.4', r: '3.4' }],
			// The whip arm, hanging rather than raised — the pose the whole drawing
			// exists to show.
			['line', { class: 'armor-limb', x1: '25.2', y1: '23.4', x2: '27.5', y2: '31' }],
			// The lash itself, slack from hand to floor. No fire gradient here —
			// that belongs to a whip in motion, and this one is not.
			['path', { class: 'lash-idle', d: 'M27.5 31 C 30 34, 24 36, 27 39.2' }],
		]);
		mark.appendChild(mascot);

		// The robot, waiting rather than typing: no keyboard, arms down, standing
		// on its own feet instead of at a desk.
		const robot = svgEl('g', { class: 'robot' });
		draw(robot, [
			['rect', { class: 'robot-body', x: '65', y: '18', width: '19', height: '15.5', rx: '4' }],
			['rect', { class: 'robot-body', x: '70', y: '33.5', width: '3.5', height: '5.2', rx: '1.2' }],
			['rect', { class: 'robot-body', x: '78.5', y: '33.5', width: '3.5', height: '5.2', rx: '1.2' }],
			['rect', { class: 'robot-head', x: '68', y: '7', width: '13', height: '10', rx: '3' }],
			['line', { class: 'wire', x1: '74.5', y1: '7', x2: '74.5', y2: '3.2' }],
			['circle', { class: 'bulb', cx: '74.5', cy: '2.2', r: '1.5' }],
			['circle', { class: 'eye', cx: '71.3', cy: '11.8', r: '1.4' }],
			['circle', { class: 'eye', cx: '77.7', cy: '11.8', r: '1.4' }],
			['line', { class: 'mouth', x1: '72.2', y1: '15', x2: '76.8', y2: '15' }],
			['line', { class: 'wire', x1: '65', y1: '22', x2: '61.5', y2: '30' }],
			['circle', { class: 'hand', cx: '61.5', cy: '30', r: '1.7' }],
			['line', { class: 'wire', x1: '84', y1: '22', x2: '87.5', y2: '30' }],
			['circle', { class: 'hand', cx: '87.5', cy: '30', r: '1.7' }],
		]);
		mark.appendChild(robot);

		return mark;
	}

	/**
	 * What the row says the agent is doing, by tool name.
	 *
	 * The verb is not decoration: "lendo" for two minutes and "executando" for
	 * two minutes mean very different things about a turn that is taking long,
	 * and the transcript above may be scrolled away while this row is not.
	 */
	const TOOL_VERBS = {
		Read: 'lendo',
		NotebookRead: 'lendo',
		Grep: 'procurando',
		Glob: 'procurando',
		LS: 'procurando',
		Write: 'escrevendo',
		Edit: 'editando',
		MultiEdit: 'editando',
		NotebookEdit: 'editando',
		Bash: 'executando',
		BashOutput: 'executando',
		KillShell: 'executando',
		WebSearch: 'pesquisando',
		WebFetch: 'pesquisando',
		Task: 'delegando',
		Agent: 'delegando',
		TodoWrite: 'planejando',
		ExitPlanMode: 'planejando',
		Skill: 'aplicando a skill',
		SlashCommand: 'aplicando a skill',
	};

	/** The words the row cycles through when nothing more specific is known. */
	const IDLE_VERBS = ['pensando', 'trabalhando', 'analisando'];

	function verbForTool(name) {
		if (TOOL_VERBS[name]) {
			return TOOL_VERBS[name];
		}
		// An MCP tool is `mcp__server__tool` and there is no useful verb for an
		// arbitrary one — but "consultando" beats falling back to nothing.
		return typeof name === 'string' && name.startsWith('mcp__') ? 'consultando' : 'trabalhando';
	}

	/**
	 * Says what the agent is doing right now, or nothing in particular.
	 *
	 * Passing null hands the label back to the cycling words, which is the honest
	 * state between one tool result and whatever the model decides next. The verb
	 * node is replaced rather than rewritten so its entrance animation plays on
	 * every change — replacing a child of the label never touches the row itself,
	 * which is what keeps the whip swinging (see `addToLog`).
	 */
	function setActivity(verb) {
		activity = verb || null;
		if (!workingLabel) {
			return;
		}
		workingLabel.classList.toggle('acting', activity !== null);
		if (workingVerb) {
			workingVerb.remove();
		}
		workingVerb = document.createElement('span');
		workingVerb.className = 'verb';
		workingVerb.textContent = activity || '';
		workingLabel.appendChild(workingVerb);
	}

	/**
	 * Shows the working row, for as long as the work lasts.
	 *
	 * It used to be a gap-filler, dismissed by the first token — what said "still
	 * working" after that was the mark animating on the speaker's avatar. With
	 * the mark now still, this row is the only signal, and a turn that reads six
	 * files and writes two spends minutes past its first token. So it stays: the
	 * last line of the transcript, alive, under whatever is being written, until
	 * the turn actually ends.
	 */
	function showIndicator() {
		if (replaying) {
			return;
		}
		if (!workingRow) {
			workingRow = document.createElement('div');
			workingRow.className = 'working';
			workingRow.appendChild(workingMark());

			// Two labels in one: a stack of generic words cross-faded by CSS while
			// nothing specific is known, and one verb when it is. The cycling is
			// CSS rather than a timer because a timer would have to be cancelled
			// on every path that ends a turn, and forgetting one leaves a word
			// changing under a finished conversation.
			workingLabel = document.createElement('span');
			workingLabel.className = 'label';
			const cycle = document.createElement('span');
			cycle.className = 'cycle';
			// The cycling words are decoration for a screen reader — the verb,
			// when there is one, is the part worth announcing.
			cycle.setAttribute('aria-hidden', 'true');
			for (const word of IDLE_VERBS) {
				const slot = document.createElement('i');
				slot.textContent = word;
				cycle.appendChild(slot);
			}
			workingLabel.appendChild(cycle);
			workingRow.appendChild(workingLabel);
			workingVerb = null;
			setActivity(activity);
		}
		if (workingRow.parentElement !== log) {
			clearEmptyState();
			log.appendChild(workingRow);
		}
	}

	/**
	 * The caret at the tail of the streaming bubble.
	 *
	 * A sibling of the growing Text node — appendData() on that node keeps the
	 * caret visually right after the last character. Deliberately a plain
	 * blinking block rather than the product mark: the mark identifies who is
	 * speaking, and it says that once, in the line above.
	 */
	function streamingCursor() {
		const caret = document.createElement('span');
		caret.className = 'caret';
		caret.setAttribute('aria-hidden', 'true');
		return caret;
	}

	/**
	 * Takes the row off the transcript. The element is kept, not destroyed —
	 * showing it again is the only moment its animation restarts, and that is
	 * the start of a turn, which is where a restart belongs.
	 */
	function clearIndicator() {
		if (workingRow && workingRow.parentElement) {
			workingRow.remove();
		}
		// Whatever it was doing, it is not doing it now: the next time the row
		// appears it starts from the generic words rather than resuming a verb
		// that belonged to the turn before.
		setActivity(null);
	}

	/**
	 * Adds a node to the transcript, keeping the working row last.
	 *
	 * Every append goes through here for one reason: the row must never be
	 * moved. Inserting before it costs nothing and is what makes the animation
	 * continuous — `appendChild` on a node the log already holds is a remove and
	 * an insert, and Chromium restarts CSS animations on re-insertion.
	 */
	function addToLog(node) {
		if (workingRow && workingRow.parentElement === log) {
			log.insertBefore(node, workingRow);
		} else {
			log.appendChild(node);
		}
		return node;
	}

	// Paths in rendered Markdown open in the editor — the agents emit them
	// constantly, and a path you can click beats one you have to retype.
	window.gofiMarkdown.setPathHandler((path) => vscode.postMessage({ type: 'openFile', path }));

	// Copying goes through the extension host: the editor's own clipboard works
	// the same in every VSCode build, which is more than the webview's does.
	window.gofiMarkdown.setCopyHandler((text) => vscode.postMessage({ type: 'copy', text }));

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

	/**
	 * A finished message.
	 *
	 * It carries its own copy button — what you usually want out of an answer is
	 * the text of it, and selecting a rendered bubble by hand picks up the
	 * markup around it. The copy is of the Markdown the agent actually wrote,
	 * not of what the renderer made of it, so it pastes into an editor as the
	 * agent meant it.
	 */
	function textBubble(parent, text) {
		const bubble = document.createElement('div');
		bubble.className = 'bubble';
		renderInto(bubble, text);
		const copy = window.gofiMarkdown.copyButton(text);
		copy.classList.add('bubble-copy');
		bubble.appendChild(copy);
		parent.appendChild(bubble);
		return bubble;
	}

	/**
	 * What you typed, shown as you typed it.
	 *
	 * Deliberately not run through the Markdown renderer: your message is not a
	 * document, and a path or an asterisk in a question should come back looking
	 * like the question you asked. Line breaks are kept by the stylesheet.
	 */
	function userBubble(parent, text) {
		const bubble = document.createElement('div');
		bubble.className = 'bubble plain';
		bubble.textContent = text;
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
		// Graph queries earn a place on the closed bar: they are the searches the
		// protocol asks for, and the count only moves when the agent honours it.
		if (retrieval.graph > 0) {
			parts.push(`${retrieval.graph} grafo`);
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
			usagePanel.appendChild(retrievalSplit(retrieval));
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

	/**
	 * Where the session's searches went.
	 *
	 * The headline counts every retrieval; this line says how many of them asked
	 * the graph instead of opening the tree, which is the number the project's
	 * protocol is actually about.
	 */
	function retrievalSplit(retrieval) {
		const el = document.createElement('p');
		el.className = 'split';
		const parts = [`${retrieval.graph || 0} pelo grafo`];
		if (retrieval.code > 0) {
			parts.push(`${retrieval.code} por grep/read no código`);
		}
		if (retrieval.docs > 0) {
			parts.push(`${retrieval.docs} em docs`);
		}
		el.textContent = parts.join('  ·  ');
		el.title = 'Grafo = `gofi graph explain`, ou leitura de .gofi/graph/.';

		const avoided = retrieval.avoided;
		if (avoided && avoided.files > 0) {
			const saved = document.createElement('span');
			saved.className = 'saved';
			saved.textContent = `−~${compact(avoided.tokens)} tokens`;
			saved.title = `As respostas do grafo citaram ${avoided.files} arquivos que a sessão não precisou abrir. O tamanho deles é o que um grep seguido de leitura teria trazido para o contexto.`;
			el.appendChild(document.createTextNode('  ·  '));
			el.appendChild(saved);
		}
		return el;
	}

	function readRow(row, inFlight) {
		const el = document.createElement('div');
		el.className = 'read';
		if (row.failed) {
			el.classList.add('failed');
		}

		const graph = row.kind === 'graph';
		const badge = document.createElement('span');
		badge.className = graph ? 'badge graph' : row.scoped ? 'badge scoped' : 'badge full';
		badge.textContent = graph ? 'grafo' : row.scoped ? 'alvo' : 'inteiro';
		badge.title = graph
			? 'A busca passou pelo grafo, sem abrir arquivo.'
			: row.scoped
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

		addToLog(card);
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

	/**
	 * Replaces the card with what was decided, so the transcript records it.
	 *
	 * On replay there is no card to replace — the question was asked in another
	 * window — so the decision is written straight into the log. What the user
	 * authorised is part of what happened, and a transcript that omits it reads
	 * as if the agent acted unasked.
	 */
	function resolveApproval(id, decision, tool) {
		let card = approvalCards.get(id);
		approvalCards.delete(id);
		if (!card) {
			if (!tool) {
				return;
			}
			clearEmptyState();
			card = document.createElement('div');
			addToLog(card);
		}
		card.textContent = '';
		card.className = `approval settled ${decision === 'deny' ? 'denied' : 'allowed'}`;
		const line = document.createElement('div');
		line.className = 'approval-head';
		const what =
			decision === 'deny' ? 'Recusado' : decision === 'always' ? 'Permitido — e não perguntar mais' : 'Permitido';
		line.textContent = tool ? `${what} · ${tool}` : what;
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

	// ── saved conversations ─────────────────────────────────────────────────

	/**
	 * The conversations kept for this project, and the one currently open.
	 *
	 * A chat that ends when the panel closes is a chat you can only use for
	 * questions you can finish in one sitting. The list is what makes the other
	 * kind possible: pick a thread up tomorrow, in another window, with the
	 * agent still inside the conversation rather than reading a summary of it.
	 */
	/** @type {{id: string|null, engineId: string|null, title: string, updatedAt: number, messages: number, source: string}[]} */
	let sessions = [];
	let activeSessionId = null;
	let activeEngineId = null;
	let canOpenTerminal = false;
	let historyOpen = false;

	historyBtn.addEventListener('click', () => toggleHistory(!historyOpen));
	historyNew.addEventListener('click', () => {
		vscode.postMessage({ type: 'new' });
		toggleHistory(false);
	});
	historySearch.addEventListener('input', renderSessions);
	historySearch.addEventListener('keydown', (event) => {
		if (event.key === 'Escape') {
			toggleHistory(false);
		}
	});

	function toggleHistory(open) {
		historyOpen = open;
		history.hidden = !open;
		// The list takes the transcript's place rather than floating over it:
		// one thing on screen at a time, and no z-index fight with the picker.
		log.hidden = open;
		historyBtn.setAttribute('aria-expanded', String(open));
		historyBtn.classList.toggle('on', open);
		if (open) {
			// Render what we have before asking for more, so opening the list is
			// instant and the answer only ever refreshes it.
			historySearch.value = '';
			renderSessions();
			vscode.postMessage({ type: 'sessions' });
			historySearch.focus();
		} else {
			input.focus();
		}
	}

	/** "agora", "12m", "3h", "5d" — the age at a glance, not a full date. */
	function age(timestamp) {
		const seconds = Math.max(0, Math.round((Date.now() - timestamp) / 1000));
		if (seconds < 60) {
			return 'agora';
		}
		if (seconds < 3600) {
			return `${Math.floor(seconds / 60)}m`;
		}
		if (seconds < 86400) {
			return `${Math.floor(seconds / 3600)}h`;
		}
		return `${Math.floor(seconds / 86400)}d`;
	}

	function fullDate(timestamp) {
		try {
			return new Date(timestamp).toLocaleString();
		} catch {
			return '';
		}
	}

	function renderSessions() {
		historyList.textContent = '';
		const needle = historySearch.value.trim().toLowerCase();
		const shown = needle === '' ? sessions : sessions.filter((s) => s.title.toLowerCase().includes(needle));

		if (shown.length === 0) {
			const empty = document.createElement('p');
			empty.className = 'history-empty';
			empty.textContent =
				sessions.length === 0
					? 'Nenhuma conversa salva ainda neste projeto.'
					: 'Nenhuma conversa com esse nome.';
			historyList.appendChild(empty);
			return;
		}

		for (const session of shown) {
			const isCurrent =
				(session.id !== null && session.id === activeSessionId) ||
				(session.engineId !== null && session.engineId === activeEngineId);

			const row = document.createElement('div');
			row.className = isCurrent ? 'history-row current' : 'history-row';
			row.setAttribute('role', 'listitem');

			const open = document.createElement('button');
			open.type = 'button';
			open.className = 'history-open';
			open.title = [
				session.title,
				fullDate(session.updatedAt),
				session.engineId ? `sessão ${session.engineId}` : 'sem sessão no motor',
			].join('\n');

			const name = document.createElement('span');
			name.className = 'history-title';
			name.textContent = session.title;
			open.appendChild(name);

			// Where the conversation was had. Only worth saying when it was not
			// here — a thread from the terminal opening in the chat is the sort
			// of thing you want to know before you click it.
			if (session.source === 'engine') {
				const badge = document.createElement('span');
				badge.className = 'history-badge';
				badge.textContent = 'terminal';
				badge.title = 'Começou fora do painel. Abrir continua a mesma conversa.';
				open.appendChild(badge);
			}

			const when = document.createElement('span');
			when.className = 'history-age';
			when.textContent = age(session.updatedAt);
			open.appendChild(when);

			open.addEventListener('click', () => {
				vscode.postMessage({ type: 'openSession', id: session.id, engineId: session.engineId });
				toggleHistory(false);
			});
			row.appendChild(open);

			// The same conversation, in the engine's own interface. One session,
			// two doors — nothing is copied or exported to go through either.
			if (canOpenTerminal && session.engineId) {
				const terminal = document.createElement('button');
				terminal.type = 'button';
				terminal.className = 'history-terminal';
				terminal.title = 'Continuar esta conversa num terminal';
				terminal.setAttribute('aria-label', `Continuar ${session.title} num terminal`);
				terminal.textContent = '›_';
				terminal.addEventListener('click', (event) => {
					event.stopPropagation();
					vscode.postMessage({ type: 'openSessionInTerminal', engineId: session.engineId });
				});
				row.appendChild(terminal);
			}

			// Only what the panel wrote is the panel's to throw away. The engine
			// keeps its own copy, and a row that lives only there stays.
			if (session.id) {
				const remove = document.createElement('button');
				remove.type = 'button';
				remove.className = 'history-delete';
				remove.title = 'Esquecer aqui — a conversa continua no motor';
				remove.setAttribute('aria-label', `Esquecer ${session.title}`);
				remove.textContent = '×';
				remove.addEventListener('click', (event) => {
					event.stopPropagation();
					vscode.postMessage({ type: 'deleteSession', id: session.id });
				});
				row.appendChild(remove);
			}

			historyList.appendChild(row);
		}
	}

	/**
	 * Rebuilds a saved conversation from its events.
	 *
	 * They are the same events the live chat renders, so replay is the live
	 * renderer run over a recording — nothing about a restored transcript can
	 * drift from how it looked the first time.
	 */
	function replay(events, title) {
		log.textContent = '';
		pendingTools.clear();
		approvalCards.clear();
		clearIndicator();
		endTurn();

		replaying = true;
		for (const event of events) {
			try {
				apply(event);
			} catch {
				// One malformed event from an older version of the extension is
				// not a reason to lose the rest of the conversation.
			}
		}
		replaying = false;

		clearIndicator();
		endTurn();
		if (log.childElementCount === 0) {
			showEmptyState();
		} else {
			const mark = document.createElement('div');
			mark.className = 'resume-mark';
			mark.textContent = title ? `conversa retomada · ${title}` : 'conversa retomada';
			addToLog(mark);
		}
		followTail = true;
		queuePaint();
	}

	// ── extension messages ──────────────────────────────────────────────────

	window.addEventListener('message', (event) => {
		// Sample "was the user at the bottom" before mutating, then scroll once
		// per frame — scrolling synchronously on every message forces a layout
		// per delta, which is its own source of stutter.
		followTail = followTail && atBottom();
		apply(event.data);
		queuePaint();
	});

	/** Renders one message from the extension — live, or replayed from disk. */
	function apply(message) {
		if (!message || typeof message.type !== 'string') {
			return;
		}
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
				awaitingApprovals++;
				showApproval(message);
				break;

			case 'approvalResolved':
				awaitingApprovals = Math.max(0, awaitingApprovals - 1);
				resolveApproval(message.id, message.decision, message.tool);
				break;

			case 'sessions':
				sessions = Array.isArray(message.items) ? message.items : [];
				activeSessionId = message.activeId || null;
				activeEngineId = message.activeEngineId || null;
				canOpenTerminal = message.canOpenTerminal === true;
				if (historyOpen) {
					renderSessions();
				}
				break;

			case 'replay':
				replay(Array.isArray(message.events) ? message.events : [], message.title);
				break;

			case 'toggleHistory':
				toggleHistory(!historyOpen);
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
				if (Array.isArray(message.files) && message.files.length > 0) {
					// The names, not the contents: what was attached is part of the
					// conversation, what was inside it is the agent's business.
					const note = document.createElement('div');
					note.className = 'attached-note';
					note.textContent = `anexado: ${message.files.join(', ')}`;
					body.appendChild(note);
				}
				if (message.text !== '') {
					userBubble(body, message.text);
				}
				if (!message.queued) {
					setActivity(null);
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
				setActivity(null);
				showIndicator();
				break;
			}

			case 'delta': {
				const el = assistantTurn();
				// The tokens themselves say what it is doing: prose is an answer
				// being written, reasoning is thinking. Both beat "trabalhando".
				setActivity(message.kind === 'text' ? 'respondendo' : 'pensando');
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
					} else if (block.type === 'tool_use') {
						// The verb follows the tool whether or not the row for it is
						// shown: hiding tool calls is about the transcript, not about
						// what the indicator is allowed to know.
						setActivity(verbForTool(block.name));
						if (showToolCalls) {
							pendingTools.set(block.id, toolRow(el, block.name, block.input));
						}
					}
				}
				break;
			}

			case 'toolResult': {
				// The tool is done and the model has not said what comes next, so
				// the row goes back to saying it is working — which is all anyone
				// knows at this instant.
				setActivity(null);
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
				running = message.running === true;
				cancel.hidden = !running;
				document.body.classList.toggle('busy', running);
				if (running) {
					showIndicator();
				} else {
					clearIndicator();
					pendingTools.clear();
					awaitingApprovals = 0;
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

			case 'rateLimited':
				// The engine itself said stop — not a failure to explain, a pause
				// to sit through. No whip-mark here: that animation says gofi is
				// still driving the robot, which is the one thing not happening.
				clearIndicator();
				waitingNotice(message.message, message.reset);
				endTurn();
				break;

			case 'notice':
				// Informative aside from the host (a `/model` switch, for now).
				// Not an error and not a turn — no indicator, no `endTurn`.
				notice(message.text, message.hint);
				break;

			case 'cleared':
				log.textContent = '';
				pendingTools.clear();
				approvalCards.clear();
				clearIndicator();
				endTurn();
				running = false;
				awaitingApprovals = 0;
				showEmptyState();
				break;

			default:
				break;
		}

		// Whatever just landed, the turn is still running and nobody is waiting
		// on the user: the row goes back to the foot of the transcript, under the
		// newest content. `done`, `error` and `rateLimited` are the ones that end
		// a turn, and they have already taken it down.
		if (
			running &&
			!replaying &&
			awaitingApprovals === 0 &&
			message.type !== 'done' &&
			message.type !== 'error' &&
			message.type !== 'rateLimited'
		) {
			showIndicator();
		}
	}

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
		addToLog(el);
	}

	/**
	 * The rate-limit banner, with the robot asking to wait instead of the plain
	 * error box. This is the engine itself saying stop, not a bug — showing it
	 * as a red failure invites exactly the retry that got the session capped.
	 */
	function waitingNotice(text, reset) {
		clearEmptyState();
		const el = document.createElement('div');
		el.className = 'wait-notice';
		el.appendChild(waitingMark());
		const body = document.createElement('span');
		body.className = 'text';
		body.appendChild(document.createTextNode(text));
		if (reset) {
			const line = document.createElement('span');
			line.className = 'hint';
			line.textContent = `Reinicia às ${reset}.`;
			body.appendChild(line);
		}
		el.appendChild(body);
		addToLog(el);
	}

	vscode.postMessage({ type: 'ready' });
})();
