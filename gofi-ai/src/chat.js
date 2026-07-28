'use strict';

const vscode = require('vscode');
const crypto = require('crypto');
const path = require('path');
const { resolveProvider } = require('./providers/index.js');
const { readGofiProject, readGofiSkills, findProjectRoot, SKILLS_DIR, SKILL_FILE } = require('./gofiConfig.js');
const { UsageLedger } = require('./usage.js');
const ragAudit = require('./ragAudit.js');
const { ApprovalBridge, hookSettings } = require('./approvals.js');

const VIEW_ID = 'gofi-ai.chat';

/** Where dismissed suggestions are remembered, per workspace. */
const DISMISSED_KEY = 'gofi-ai.dismissedFindings';
const RUNNING_CONTEXT = 'gofi-ai.running';

/** Never offered as an `@` reference, and never indexed. */
const FILE_EXCLUDE = '**/{node_modules,.git,dist,build,target,coverage,.gofi,vendor}/**';

/** Upper bound on the file index. Big enough for real repos, small enough to hold. */
const FILE_INDEX_LIMIT = 8000;

/**
 * The `@` file index, shared by every chat: it describes the workspace, not a
 * conversation, and rebuilding it per chat would be pure duplication.
 *
 * @type {string[] | null}
 */
let sharedFileIndex = null;

/** Dropped whenever the tree changes, so the next `@` rebuilds it. */
function invalidateFileIndex() {
	sharedFileIndex = null;
}

/**
 * Token deltas arrive far faster than a UI can usefully repaint. Batching them
 * over one frame turns hundreds of postMessage round trips per second into
 * ~30, which is the difference between text that flows and text that stutters.
 */
const DELTA_FLUSH_MS = 33;

/**
 * The retrieval audit stats and parses files. Recomputing it on every tool
 * result during a busy turn is wasted work nobody can read that fast.
 */
const USAGE_THROTTLE_MS = 400;

/** Settle time before re-auditing a corpus that just changed. */
const AUDIT_DEBOUNCE_MS = 600;

/**
 * One conversation.
 *
 * A chat owns its engine session, its transcript, its token ledger and its
 * retrieval audit — nothing is shared with another chat. That separation is
 * the point: two chats mean two independent contexts, so a `/gofi-spec` thread
 * and a `/gofi-eng` thread can run side by side without polluting each other's
 * history or each other's cached prefix.
 *
 * A chat usually renders into one webview, but it can hold several (the same
 * conversation mirrored), so closing one view does not end it.
 */
class Chat {
	/**
	 * @param {vscode.ExtensionContext} context
	 * @param {number} id Ordinal used in the default title
	 */
	constructor(context, id) {
		this.context = context;
		this.id = id;
		/** Shown on the editor tab; replaced by the first message's opening words. */
		this.title = id === 0 ? 'GOFI AI' : `GOFI AI ${id}`;
		/** @type {((title: string) => void) | null} Told when the title changes. */
		this.onTitle = null;
		/** Whether the title has already been taken from a message. */
		this.named = false;
		/**
		 * Per-call approval, when enabled: the engine asks this panel before
		 * every tool that can change something, and a refusal is binding.
		 * @type {ApprovalBridge | null}
		 */
		this.approvals = null;
		/** Tools the user chose to stop being asked about, for this chat. */
		this.alwaysAllow = new Set();
		/** @type {Map<string, {tool: string}>} Requests waiting on an answer. */
		this.pendingApprovals = new Map();
		/** @type {Set<vscode.Webview>} Every surface currently rendering the chat. */
		this.surfaces = new Set();
		/** @type {import('./providers/types.js').ChatSession | null} */
		this.session = null;
		/** True while a turn is in flight. */
		this.running = false;
		/**
		 * Messages the user typed while a turn was already in flight. Each one
		 * is echoed into the transcript immediately (with a queued marker) and
		 * fed to the same session as the next turn — same process, same
		 * session_id, so the model continues the conversation and the cached
		 * prefix is preserved. No new context, no re-billed tokens.
		 * @type {{prompt: string, images: {mediaType: string, data: string}[]}[]}
		 */
		this.pending = [];
		/** @type {vscode.Disposable[]} */
		this.watchers = [];
		/** Token and retrieval accounting for the current session. */
		this.ledger = new UsageLedger(this.cwd || '');
		/** Remediations offered to the user, by id, so a click can find its prompt. */
		this.remediations = new Map();
		/** Buffered stream deltas, flushed one frame at a time. */
		this.deltaBuffer = { text: '', thinking: '' };
		this.deltaTimer = null;
		/** Throttle state for the usage snapshot. */
		this.usageTimer = null;
		this.usageDueAt = 0;
		/** Pending rescan after the corpus changed on disk. */
		this.auditTimer = null;
		/** Findings whose fix is running, so the panel can say so. */
		this.applying = new Set();

		this.watchEditor();
	}

	// ── surfaces ────────────────────────────────────────────────────────────

	/** Wires a webview up to the shared conversation. */
	attach(webview) {
		webview.options = {
			enableScripts: true,
			localResourceRoots: [vscode.Uri.joinPath(this.context.extensionUri, 'media')],
		};
		webview.html = this.html(webview);
		webview.onDidReceiveMessage((message) => this.onWebviewMessage(message));
		this.surfaces.add(webview);

		if (this.watchers.length === 0) {
			this.watchers = this.watchProject();
		}
	}

	detach(webview) {
		this.surfaces.delete(webview);
		if (this.surfaces.size > 0) {
			return;
		}
		this.dispose();
	}

	/** Releases the engine and everything scheduled. The chat is dead after. */
	dispose() {
		this.stop();
		if (this.session) {
			this.session.dispose();
			this.session = null;
		}
		if (this.approvals) {
			this.approvals.dispose();
			this.approvals = null;
		}
		this.alwaysAllow.clear();
		this.pending.length = 0;
		this.running = false;
		if (this.deltaTimer !== null) {
			clearTimeout(this.deltaTimer);
			this.deltaTimer = null;
		}
		if (this.usageTimer !== null) {
			clearTimeout(this.usageTimer);
			this.usageTimer = null;
		}
		if (this.auditTimer !== null) {
			clearTimeout(this.auditTimer);
			this.auditTimer = null;
		}
		this.deltaBuffer = { text: '', thinking: '' };
		for (const watcher of this.watchers) {
			watcher.dispose();
		}
		this.watchers = [];
	}

	// ── messages ────────────────────────────────────────────────────────────

	onWebviewMessage(message) {
		switch (message && message.type) {
			case 'ready':
				this.postIdentity();
				this.postActiveFile();
				this.postUsage();
				break;
			case 'send':
				this.send(String(message.text || ''), message.images);
				break;
			case 'stop':
				this.stop();
				break;
			case 'new':
				this.newSession();
				break;
			case 'optimize':
				this.optimize(String(message.id || ''));
				break;
			case 'approve':
				this.resolveApproval(String(message.id || ''), message.decision, message.reason);
				break;
			case 'dismiss':
				this.dismiss(String(message.id || ''));
				break;
			case 'restoreDismissed':
				this.restoreDismissed();
				break;
			case 'searchFiles':
				this.searchFiles(String(message.query || ''));
				break;
			case 'openFile':
				this.openFile(String(message.path || ''), Number(message.line) || 0);
				break;
			default:
				break;
		}
	}

	// ── workspace context ───────────────────────────────────────────────────

	/** The folder the engine runs in, or null when no folder is open. */
	get cwd() {
		const folders = vscode.workspace.workspaceFolders;
		return folders && folders.length > 0 ? folders[0].uri.fsPath : null;
	}

	get config() {
		return vscode.workspace.getConfiguration('gofiAI');
	}

	/**
	 * The folder holding `.gofi.yaml`, which may sit above the opened folder.
	 * Falls back to the workspace folder so skills are still found in a project
	 * that has `.claude/` but no config yet.
	 */
	get projectRoot() {
		const cwd = this.cwd;
		if (!cwd) {
			return null;
		}
		return findProjectRoot(cwd) || cwd;
	}

	/** Tells the surfaces who they are talking to and what the project offers. */
	postIdentity() {
		const cwd = this.cwd;
		const root = this.projectRoot;
		const project = root ? readGofiProject(root) : null;
		const provider = resolveProvider(this.config);
		this.post({
			type: 'identity',
			providerLabel: provider.label,
			projectName: project ? project.name : null,
			projectRoot: root && cwd && root !== cwd ? vscode.workspace.asRelativePath(root, false) : null,
			skills: root ? readGofiSkills(root, project ? project.agents : []) : [],
			hasWorkspace: cwd !== null,
			hasConfig: project !== null,
			showThinking: this.config.get('showThinking') !== false,
			showToolCalls: this.config.get('showToolCalls') !== false,
			showActiveFile: this.config.get('showActiveFile') !== false,
			approvalRequired: this.approvalRequired(),
			supportsImages: resolveProvider(this.config).supportsImages === true,
		});
	}

	/**
	 * Reports the file the user is looking at, and what is selected in it.
	 *
	 * Chat and editor are side by side, and "this file" is the most common
	 * unstated subject of a question — showing it makes the chat's idea of
	 * context visible instead of leaving the user to guess.
	 */
	postActiveFile() {
		const editor = vscode.window.activeTextEditor;
		if (!editor || editor.document.uri.scheme !== 'file') {
			this.post({ type: 'activeFile', file: null });
			return;
		}
		const relative = vscode.workspace.asRelativePath(editor.document.uri, false);
		const selection = editor.selection;
		this.post({
			type: 'activeFile',
			file: {
				path: relative,
				name: path.basename(relative),
				// 1-based, inclusive — the way the editor's own gutter reads.
				selection: selection.isEmpty ? null : { start: selection.start.line + 1, end: selection.end.line + 1 },
			},
		});
	}

	/** @returns {vscode.Disposable[]} */
	watchEditor() {
		const push = () => {
			if (this.surfaces.size > 0) {
				this.postActiveFile();
			}
		};
		this.context.subscriptions.push(
			vscode.window.onDidChangeActiveTextEditor(push),
			vscode.window.onDidChangeTextEditorSelection(push),
		);
		return [];
	}

	/**
	 * Re-reads the project's skills whenever the CLI changes them, and drops the
	 * file index when the tree changes.
	 *
	 * `gofi init`, `gofi update` and `gofi agent add` all write into
	 * `.claude/skills/`, and a chat that only learned the skill list at startup
	 * would stay wrong until the window reloaded.
	 *
	 * @returns {vscode.Disposable[]}
	 */
	watchProject() {
		const folder = vscode.workspace.workspaceFolders?.[0];
		if (!folder) {
			return [];
		}
		const refreshSkills = () => this.postIdentity();
		const dropIndex = invalidateFileIndex;

		const skills = vscode.workspace.createFileSystemWatcher(new vscode.RelativePattern(folder, `${SKILLS_DIR}/*/${SKILL_FILE}`));
		skills.onDidCreate(refreshSkills);
		skills.onDidDelete(refreshSkills);
		skills.onDidChange(refreshSkills);

		const config = vscode.workspace.createFileSystemWatcher(new vscode.RelativePattern(folder, '.gofi.yaml'));
		config.onDidChange(refreshSkills);

		const tree = vscode.workspace.createFileSystemWatcher(new vscode.RelativePattern(folder, '**/*'));
		tree.onDidCreate(dropIndex);
		tree.onDidDelete(dropIndex);

		// The audit describes what is on disk, so it has to follow disk. Without
		// this the panel keeps offering to index a file that was indexed a
		// moment ago — by this chat, by another one, or by hand in the editor —
		// and the user is left wondering whether the fix took.
		const corpora = ragAudit.CORPORA.map((corpus) => {
			const watcher = vscode.workspace.createFileSystemWatcher(
				new vscode.RelativePattern(folder, `${corpus.prefix}**/*.md`),
			);
			const rescan = () => this.scheduleAudit();
			watcher.onDidCreate(rescan);
			watcher.onDidChange(rescan);
			watcher.onDidDelete(rescan);
			return watcher;
		});

		return [skills, config, tree, ...corpora];
	}

	/**
	 * Re-runs the audit shortly after the corpus changes.
	 *
	 * Debounced because writing one file raises several events, and an agent
	 * applying a fix touches the doc and its index back to back — one rescan
	 * for the whole burst.
	 */
	scheduleAudit() {
		if (this.auditTimer !== null) {
			clearTimeout(this.auditTimer);
		}
		this.auditTimer = setTimeout(() => {
			this.auditTimer = null;
			this.postUsage({ immediate: true });
		}, AUDIT_DEBOUNCE_MS);
	}

	// ── `@` file references ─────────────────────────────────────────────────

	/**
	 * Answers the webview's `@` picker.
	 *
	 * The index is built once and filtered in memory: `findFiles` takes a glob,
	 * and globs are case-sensitive and can't express "contains these characters
	 * in order", which is what typing a few letters of a filename means.
	 */
	async searchFiles(query) {
		if (!this.cwd) {
			this.post({ type: 'fileResults', query, files: [] });
			return;
		}
		if (sharedFileIndex === null) {
			const uris = await vscode.workspace.findFiles('**/*', FILE_EXCLUDE, FILE_INDEX_LIMIT);
			sharedFileIndex = uris.map((uri) => vscode.workspace.asRelativePath(uri, false)).sort();
		}
		this.post({ type: 'fileResults', query, files: rankFiles(sharedFileIndex, query) });
	}

	/**
	 * Opens a referenced file, so a path in the chat is one click from the code.
	 * A line puts the cursor on the exact spot a finding is about — pointing at
	 * a file is advice, pointing at a line is a place to start typing.
	 */
	async openFile(relative, line) {
		const folder = vscode.workspace.workspaceFolders?.[0];
		if (!folder || relative === '') {
			return;
		}
		const uri = vscode.Uri.joinPath(folder.uri, relative);
		try {
			const editor = await vscode.window.showTextDocument(uri, {
				preview: true,
				viewColumn: vscode.ViewColumn.One,
			});
			if (line > 0) {
				const target = new vscode.Position(Math.min(line - 1, editor.document.lineCount - 1), 0);
				editor.selection = new vscode.Selection(target, target);
				editor.revealRange(new vscode.Range(target, target), vscode.TextEditorRevealType.InCenter);
			}
		} catch {
			vscode.window.showWarningMessage(`GOFI AI: não consegui abrir ${relative}.`);
		}
	}

	// ── conversation ────────────────────────────────────────────────────────

	/**
	 * Accepts a message. Runs it now if idle, queues it if a turn is already
	 * in flight — the queue drains automatically as each turn completes, on
	 * the same session, so the conversation and its cached prefix stay intact.
	 *
	 * @param {string} text
	 * @param {{mediaType: string, data: string}[]} [images]
	 */
	async send(text, images) {
		const prompt = text.trim();
		const attached = images || [];
		if (prompt === '' && attached.length === 0) {
			return;
		}

		const cwd = this.cwd;
		if (!cwd) {
			this.post({
				type: 'error',
				message: 'Abra uma pasta antes de conversar.',
				hint: 'O motor roda na raiz do workspace — sem pasta aberta ele não tem onde ler o projeto.',
			});
			return;
		}

		const queued = this.running;
		this.nameFrom(prompt);
		// Echo immediately so the user sees their message landed even when it
		// is going to wait behind another turn.
		this.post({ type: 'user', text: prompt, images: attached.length, queued });

		if (queued) {
			this.pending.push({ prompt, images: attached });
			return;
		}

		this.startTurn(prompt, attached);
	}

	/**
	 * Sends one turn to the engine. Caller guarantees no other turn is in
	 * flight on this session.
	 *
	 * @param {string} prompt
	 * @param {{mediaType: string, data: string}[]} attached
	 */
	startTurn(prompt, attached) {
		// The session outlives the turn. Opening one per message would repay the
		// engine's startup every time — measured at ~2.6s here — and rebuild the
		// cached prefix, turning cheap cache reads back into expensive writes.
		if (!this.session) {
			const config = this.config;
			let settings;
			if (this.approvalRequired()) {
				// The bridge must exist before the engine starts: its directory
				// is baked into the hook command.
				this.approvals = new ApprovalBridge((request) => this.onApprovalRequest(request));
				settings = hookSettings(
					vscode.Uri.joinPath(this.context.extensionUri, 'media', 'permission-hook.sh').fsPath,
					this.approvals.dir,
				);
			}
			this.session = resolveProvider(config).createSession({
				cwd: this.cwd,
				config,
				project: readGofiProject(this.projectRoot || this.cwd),
				hookSettings: settings,
			});
		}

		if (!this.running) {
			this.setRunning(true);
		}

		// The tools are present; the hook is what decides, call by call.
		this.session.send({ prompt, images: attached, allowWrites: true }, (event) => this.onProviderEvent(event));
	}

	/**
	 * Suggestions the user has waved off.
	 *
	 * Kept in workspace state, not in the conversation: a finding dismissed
	 * because "that doc is meant to be read whole" is a fact about the project,
	 * and having it come back on the next turn — or in the next chat — would
	 * make the panel nagging rather than useful.
	 *
	 * @returns {Set<string>}
	 */
	dismissedIds() {
		return new Set(this.context.workspaceState.get(DISMISSED_KEY, []));
	}

	dismiss(id) {
		if (id === '') {
			return;
		}
		const ids = this.dismissedIds();
		ids.add(id);
		this.context.workspaceState.update(DISMISSED_KEY, [...ids]);
		this.postUsage({ immediate: true });
	}

	/** Nothing here is a one-way door. */
	restoreDismissed() {
		this.context.workspaceState.update(DISMISSED_KEY, []);
		this.postUsage({ immediate: true });
	}

	/** Whether every file change and command must be approved. */
	approvalRequired() {
		return this.config.get('requireApproval') !== false;
	}

	// ── per-call approval ───────────────────────────────────────────────────

	/** The engine wants to run a tool. Ask, unless the user said not to. */
	onApprovalRequest(request) {
		if (this.alwaysAllow.has(request.tool)) {
			this.approvals.allow(request.id);
			return;
		}
		this.pendingApprovals.set(request.id, { tool: request.tool });
		this.post({ type: 'approval', id: request.id, tool: request.tool, input: request.input });
	}

	/**
	 * @param {string} id
	 * @param {'allow'|'deny'|'always'} decision
	 * @param {string} [reason] What to tell the model when denying.
	 */
	resolveApproval(id, decision, reason) {
		const pending = this.pendingApprovals.get(id);
		if (!pending || !this.approvals) {
			return;
		}
		this.pendingApprovals.delete(id);
		if (decision === 'always') {
			this.alwaysAllow.add(pending.tool);
		}
		if (decision === 'deny') {
			this.approvals.deny(id, reason);
		} else {
			this.approvals.allow(id);
		}
		this.post({ type: 'approvalResolved', id, decision });
	}

	/**
	 * Names the chat after its opening question, once.
	 *
	 * Several tabs called "GOFI AI" are indistinguishable; the first thing
	 * asked is what the user remembers the thread by.
	 */
	nameFrom(prompt) {
		if (this.named || prompt === '') {
			return;
		}
		this.named = true;
		const firstLine = prompt.split('\n')[0].trim();
		this.title = firstLine.length > 40 ? `${firstLine.slice(0, 40)}…` : firstLine;
		if (this.onTitle) {
			this.onTitle(this.title);
		}
	}

	/** @param {import('./providers/types.js').ProviderEvent} event */
	onProviderEvent(event) {
		switch (event.type) {
			case 'meta':
				this.post({ type: 'meta', model: event.model });
				break;

			case 'delta':
				this.bufferDelta(event.kind, event.text);
				break;

			case 'message':
				this.flushDeltas();
				this.post({ type: 'blocks', blocks: event.blocks });
				// Opening the retrieval row here — rather than when the result
				// lands — is what makes a slow Read visible while it runs.
				for (const block of event.blocks) {
					if (block.type === 'tool_use') {
						this.ledger.noteToolUse(block.id, block.name, block.input);
					}
				}
				this.postUsage();
				break;

			case 'usage':
				this.ledger.noteUsage(event.scope, event.usage);
				this.postUsage();
				break;

			case 'tool_result':
				this.flushDeltas();
				this.post({ type: 'toolResult', toolUseId: event.toolUseId, isError: event.isError, preview: event.preview });
				this.ledger.noteToolResult(event.toolUseId, event.chars, event.isError);
				this.postUsage();
				break;

			case 'done':
				// Whatever the turn did, the audit is about to re-read disk; a
				// finding that survived should be actionable again.
				this.applying.clear();
				this.flushDeltas();
				this.ledger.noteCost(event.costUsd);
				this.postUsage({ immediate: true });
				this.post({ type: 'done', isError: event.isError, costUsd: event.costUsd, durationMs: event.durationMs, error: event.error });
				if (this.pending.length > 0) {
					// Chain into the next queued turn without dropping running:
					// same process, same session_id, same cached prefix. Defer
					// one tick so the frontend renders the finish first.
					const next = this.pending.shift();
					this.post({ type: 'dequeued' });
					setImmediate(() => this.startTurn(next.prompt, next.images));
				} else {
					this.running = false;
					this.setRunning(false);
				}
				break;

			case 'error':
				// An error breaks the current turn's semantics; queued messages
				// were written assuming success. Safer to drop them than to run
				// them against a conversation the user just saw fail.
				this.pending.length = 0;
				this.running = false;
				this.flushDeltas();
				this.setRunning(false);
				this.post({ type: 'error', message: event.message, hint: event.hint });
				break;

			default:
				break;
		}
	}

	/**
	 * Accumulates a stream delta, scheduling a flush if one isn't already due.
	 *
	 * Ordering is preserved because text and thinking are buffered separately
	 * and flushed together, and every event that must appear after the streamed
	 * text flushes the buffer first.
	 */
	bufferDelta(kind, text) {
		this.deltaBuffer[kind === 'thinking' ? 'thinking' : 'text'] += text;
		if (this.deltaTimer === null) {
			this.deltaTimer = setTimeout(() => this.flushDeltas(), DELTA_FLUSH_MS);
		}
	}

	flushDeltas() {
		if (this.deltaTimer !== null) {
			clearTimeout(this.deltaTimer);
			this.deltaTimer = null;
		}
		const { text, thinking } = this.deltaBuffer;
		this.deltaBuffer = { text: '', thinking: '' };
		if (thinking !== '') {
			this.post({ type: 'delta', kind: 'thinking', text: thinking });
		}
		if (text !== '') {
			this.post({ type: 'delta', kind: 'text', text });
		}
	}

	/**
	 * Pushes the live token/retrieval snapshot, enriched with indexing advice.
	 *
	 * The ledger measures; the audit explains and offers a fix. Running the
	 * audit on every event keeps it honest — a finding disappears the moment
	 * the agent fixes the file it was about.
	 */
	postUsage(options) {
		// Coalesce bursts: the numbers still move continuously, they just don't
		// move more often than an eye can follow.
		if (!(options && options.immediate)) {
			const now = Date.now();
			if (now < this.usageDueAt) {
				if (this.usageTimer === null) {
					this.usageTimer = setTimeout(() => {
						this.usageTimer = null;
						this.postUsage({ immediate: true });
					}, this.usageDueAt - now);
				}
				return;
			}
		}
		if (this.usageTimer !== null) {
			clearTimeout(this.usageTimer);
			this.usageTimer = null;
		}
		this.usageDueAt = Date.now() + USAGE_THROTTLE_MS;

		const snapshot = this.ledger.snapshot();
		const root = this.projectRoot || '';
		// Evidence first — a finding that can say "this just cost you 10k
		// tokens" is worth more than the same problem stated in the abstract.
		const advice = ragAudit.review(root, this.ledger.rows);
		const seen = new Set(advice.map((f) => f.id));
		advice.push(...ragAudit.scanCorpora(root, seen));

		const dismissed = this.dismissedIds();
		const kept = advice.filter((f) => !dismissed.has(f.id));
		snapshot.dismissedCount = dismissed.size;
		snapshot.applying = [...this.applying];

		this.remediations.clear();
		for (const finding of kept) {
			this.remediations.set(finding.id, finding.action);
		}

		snapshot.findings = [
			...kept.map((f) => ({
				level: f.level,
				text: f.text,
				id: f.id,
				actionLabel: f.action.label,
				where: f.where,
				insert: f.insert,
				anchor: f.anchor,
			})),
			...snapshot.findings,
		];
		this.post({ type: 'usage', snapshot });
	}

	/**
	 * Applies an indexing fix — but only after the user says so.
	 *
	 * The fix is a turn like any other: the agent edits the file, and the change
	 * lands in the working tree where it can be reviewed and reverted. Nothing
	 * is written behind the user's back, and the extension never edits a spec
	 * with its own idea of what a spec should look like.
	 */
	async optimize(id) {
		const action = this.remediations.get(id);
		if (!action) {
			return;
		}

		const choice = await vscode.window.showInformationMessage(
			`GOFI AI — ${action.label}?`,
			{
				modal: true,
				detail: `O agente vai editar arquivos do projeto para isso. A instrução enviada será:\n\n${action.prompt}`,
			},
			'Aplicar',
		);
		if (choice !== 'Aplicar') {
			return;
		}
		// Mark it before the turn starts: between the click and the first token
		// the panel would otherwise look like the button did nothing.
		this.applying.add(id);
		this.postUsage({ immediate: true });
		this.send(action.prompt);
	}

	stop() {
		if (this.approvals) {
			this.approvals.denyAll('O usuário interrompeu a resposta.');
		}
		this.pendingApprovals.clear();
		// A stop means "cancel everything I asked for" — dropping the queue is
		// the honest read of that intent. If the user wanted to keep some, they
		// can retype it.
		this.pending.length = 0;
		if (this.session && this.running) {
			this.session.cancel();
		}
	}

	/**
	 * Drops the transcript and the engine session, so the next turn starts with
	 * a clean context instead of resuming.
	 */
	newSession() {
		this.stop();
		if (this.session) {
			this.session.dispose();
			this.session = null;
		}
		// Clearing the conversation clears the authority granted inside it. A
		// blanket "always allow" that survived the reset would be a permission
		// the user has no reason to believe is still in force.
		if (this.approvals) {
			this.approvals.dispose();
			this.approvals = null;
		}
		this.alwaysAllow.clear();
		this.applying.clear();
		this.pending.length = 0;
		this.running = false;
		this.ledger = new UsageLedger(this.cwd || '');
		this.post({ type: 'cleared' });
		this.postIdentity();
		this.postActiveFile();
		this.postUsage();
	}

	setRunning(running) {
		this.running = running;
		vscode.commands.executeCommand('setContext', RUNNING_CONTEXT, running);
		this.post({ type: 'running', running });
	}

	/**
	 * Broadcasts to every open surface — they render one conversation.
	 *
	 * A surface can be torn down between the moment something is scheduled and
	 * the moment it fires: closing the tab, collapsing the view, reloading the
	 * window. VSCode answers a post to a dead webview with "Webview is
	 * disposed", and `onDidDispose` does not always land first. So a failed
	 * post retires that surface instead of surfacing as an error the user can
	 * do nothing about.
	 */
	post(message) {
		for (const webview of [...this.surfaces]) {
			try {
				const posted = webview.postMessage(message);
				if (posted && typeof posted.then === 'function') {
					posted.then(undefined, () => this.surfaces.delete(webview));
				}
			} catch {
				this.surfaces.delete(webview);
			}
		}
	}

	/** @param {vscode.Webview} webview */
	html(webview) {
		const asset = (name) => webview.asWebviewUri(vscode.Uri.joinPath(this.context.extensionUri, 'media', name));
		// A per-load nonce is what lets the CSP forbid inline script wholesale
		// while still allowing our own bundle.
		const nonce = crypto.randomBytes(16).toString('base64');
		const csp = [
			"default-src 'none'",
			`style-src ${webview.cspSource}`,
			`img-src ${webview.cspSource} data:`,
			`script-src 'nonce-${nonce}'`,
		].join('; ');

		return `<!DOCTYPE html>
<html lang="pt-BR">
<head>
<meta charset="UTF-8">
<meta http-equiv="Content-Security-Policy" content="${csp}">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<link href="${asset('main.css')}" rel="stylesheet">
<title>GOFI AI</title>
</head>
<body>
<header id="header">
  <span class="mark" aria-hidden="true"></span>
  <span id="title">GOFI AI</span>
  <span id="writeBadge" hidden></span>
  <span id="subtitle"></span>
</header>
<button id="usageBar" type="button" aria-expanded="false" aria-controls="usagePanel" title="Tokens e eficiência da recuperação">
  <span class="caret" aria-hidden="true">▸</span>
  <span id="usageSummary">sem uso ainda</span>
  <span id="usageFlag" hidden></span>
</button>
<section id="usagePanel" hidden aria-label="Uso de tokens e recuperação"></section>
<main id="log" role="log" aria-live="polite"></main>
<div id="chips" aria-label="Skills do projeto"></div>
<div id="picker" role="listbox" hidden></div>
<div id="attachments" hidden aria-label="Imagens anexadas"></div>
<div id="activeFile" hidden></div>
<footer>
  <div id="composer">
    <textarea id="input" rows="1" placeholder="Pergunte  ·  / skill  ·  @ arquivo  ·  Ctrl+V cola imagem" aria-label="Mensagem"></textarea>
    <div id="actions">
      <button id="submit" title="Enviar (Enter)" aria-label="Enviar">Enviar</button>
      <button id="cancel" title="Interromper" aria-label="Interromper" hidden>Parar</button>
    </div>
  </div>
</footer>
<script nonce="${nonce}" src="${asset('markdown.js')}"></script>
<script nonce="${nonce}" src="${asset('main.js')}"></script>
</body>
</html>`;
	}
}

/**
 * Subsequence match over the file index, best matches first.
 *
 * Typing `chatview` should find `gofi-ai/src/chatViewProvider.js`, and typing
 * `src/prov` should find things under that folder — so the query matches in
 * order but not necessarily contiguously, and a hit in the filename outranks
 * one buried in the directory path.
 */
function rankFiles(index, query, limit = 30) {
	const needle = query.toLowerCase();
	if (needle === '') {
		return index.slice(0, limit);
	}

	const scored = [];
	for (const file of index) {
		const haystack = file.toLowerCase();
		const base = haystack.slice(haystack.lastIndexOf('/') + 1);

		let score;
		if (base.startsWith(needle)) {
			score = 0; // filename starts with what was typed — the strongest signal
		} else if (base.includes(needle)) {
			score = 1;
		} else if (haystack.includes(needle)) {
			score = 2;
		} else if (isSubsequence(needle, haystack)) {
			score = 3;
		} else {
			continue;
		}
		// Shorter paths win ties: the top-level match is usually the intended one.
		scored.push({ file, score, length: file.length });
	}

	scored.sort((a, b) => a.score - b.score || a.length - b.length || a.file.localeCompare(b.file));
	return scored.slice(0, limit).map((entry) => entry.file);
}

function isSubsequence(needle, haystack) {
	let i = 0;
	for (let j = 0; j < haystack.length && i < needle.length; j++) {
		if (haystack[j] === needle[i]) {
			i++;
		}
	}
	return i === needle.length;
}

module.exports = { Chat, VIEW_ID, RUNNING_CONTEXT, rankFiles, invalidateFileIndex };
