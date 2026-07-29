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

	/**
	 * Three drawings of the same arm, shown one at a time.
	 *
	 * A whip is not a rigid stick, so swinging one by rotating a curve does not
	 * read as a whip — it reads as a bent wire being waved. Hand-drawn poses are
	 * how cartoons have always done this, and they buy something else too: the
	 * animation is opacity only, with no dependence on how an engine resolves
	 * `transform-origin` inside an SVG. Nothing can rotate about the wrong point
	 * because nothing rotates.
	 *
	 * Each entry is `[name, elbow-to-hand endpoint, the lash from that hand]`.
	 * The strike pose ends at x=44, which is the robot's near edge — that is the
	 * whole point of the picture, and the previous version missed it by six
	 * units, whipping at empty air.
	 */
	const WHIP_POSES = [
		['windup', [3.5, 7.5], 'M3.5 7.5 C 0.5 2.5, 7 -0.2, 13 2.2'],
		['strike', [16, 10], 'M16 10 C 26 3.5, 35 7.5, 44 12'],
		['recoil', [15, 13.5], 'M15 13.5 C 22 18, 29 9, 37 15'],
	];

	/**
	 * The working indicator: someone standing over the robot with a whip.
	 *
	 * A spinner says "wait". This says who is waiting and who is working, which
	 * is the funnier and more accurate description of what is happening — and
	 * being a drawing rather than a spinner, you can tell across the room
	 * whether the panel is busy.
	 *
	 * Drawn here rather than shipped as a file because it has to move, and CSS
	 * can only reach the parts if they are elements: three arm poses, the spark
	 * where the lash lands, and the robot flinching a beat later.
	 */
	function workingMark() {
		const mark = svgEl('svg', {
			class: 'whip-mark',
			viewBox: '0 0 72 32',
			width: '54',
			height: '24',
			'aria-hidden': 'true',
			focusable: 'false',
		});

		// The robot, at work: arms down on the keyboard, legs under the body.
		const robot = svgEl('g', { class: 'robot' });
		robot.appendChild(svgEl('rect', { class: 'robot-body', x: '46', y: '13', width: '16', height: '13', rx: '3' }));
		robot.appendChild(svgEl('rect', { class: 'robot-head', x: '48.5', y: '4', width: '11', height: '8.5', rx: '2.5' }));
		robot.appendChild(svgEl('line', { class: 'wire', x1: '54', y1: '4', x2: '54', y2: '1.6' }));
		robot.appendChild(svgEl('circle', { class: 'bulb', cx: '54', cy: '1.2', r: '1.3' }));
		robot.appendChild(svgEl('circle', { class: 'eye', cx: '51.6', cy: '8.2', r: '1.15' }));
		robot.appendChild(svgEl('circle', { class: 'eye', cx: '56.4', cy: '8.2', r: '1.15' }));
		robot.appendChild(svgEl('line', { class: 'wire', x1: '46', y1: '17.5', x2: '42.5', y2: '21.5' }));
		robot.appendChild(svgEl('line', { class: 'wire', x1: '62', y1: '17.5', x2: '65.5', y2: '21.5' }));
		robot.appendChild(svgEl('line', { class: 'wire', x1: '50', y1: '26', x2: '50', y2: '30' }));
		robot.appendChild(svgEl('line', { class: 'wire', x1: '58', y1: '26', x2: '58', y2: '30' }));
		mark.appendChild(robot);

		// Where the lash lands, drawn at the strike pose's endpoint so the flash
		// and the whip are in the same place at the same instant.
		const spark = svgEl('g', { class: 'spark' });
		spark.appendChild(svgEl('line', { x1: '44.5', y1: '11.8', x2: '41', y2: '8.4' }));
		spark.appendChild(svgEl('line', { x1: '44.5', y1: '11.8', x2: '40.2', y2: '12.6' }));
		spark.appendChild(svgEl('line', { x1: '44.5', y1: '11.8', x2: '41.6', y2: '15.6' }));
		mark.appendChild(spark);

		const human = svgEl('g', { class: 'human' });
		human.appendChild(svgEl('circle', { class: 'head', cx: '8', cy: '7.5', r: '3.2' }));
		human.appendChild(svgEl('line', { class: 'limb', x1: '8', y1: '10.7', x2: '8', y2: '20' }));
		human.appendChild(svgEl('line', { class: 'limb', x1: '8', y1: '20', x2: '4', y2: '29.5' }));
		human.appendChild(svgEl('line', { class: 'limb', x1: '8', y1: '20', x2: '12.5', y2: '29.5' }));
		// The idle arm, on the other side, so the figure is not all one diagonal.
		human.appendChild(svgEl('line', { class: 'limb', x1: '8', y1: '14', x2: '13', y2: '18' }));
		mark.appendChild(human);

		for (const [name, hand, lash] of WHIP_POSES) {
			const pose = svgEl('g', { class: `pose ${name}` });
			pose.appendChild(svgEl('line', { class: 'limb', x1: '8', y1: '13.5', x2: String(hand[0]), y2: String(hand[1]) }));
			pose.appendChild(svgEl('path', { class: 'lash', d: lash }));
			mark.appendChild(pose);
		}

		return mark;
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
			const label = document.createElement('span');
			label.className = 'label';
			label.textContent = 'trabalhando';
			workingRow.appendChild(label);
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
		// newest content. `done` and `error` are the two that end a turn, and
		// they have already taken it down.
		if (running && !replaying && awaitingApprovals === 0 && message.type !== 'done' && message.type !== 'error') {
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

	vscode.postMessage({ type: 'ready' });
})();
