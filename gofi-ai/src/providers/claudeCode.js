'use strict';

const { spawn, execFile } = require('child_process');
const claudeSessions = require('./claudeSessions.js');

/** @typedef {import('./types.js').ProviderEvent} ProviderEvent */

/**
 * Claude, driven through the Claude Code CLI running in the workspace root.
 *
 * Going through the CLI rather than the raw API is what makes the panel a gofi
 * chat instead of a generic one: the process inherits `.claude/` from the
 * workspace, so every skill the project installed — `/gofi-eng`, `/gofi-spec`,
 * `/gofi-qa` — is a slash command the user can type, and the agent reads the
 * project's own memory, knowledge and specs with no wiring on our side.
 *
 * The process is kept alive across turns and fed over stdin. Starting one per
 * turn is simpler, but measurably worse: on a project with nine skills the CLI
 * spends ~2.6s on startup and discovery before the model sees a byte, and a
 * resumed session replays its history on top of that. Holding the process open
 * pays that once — measured here at 4.8s to first token cold versus 1.8s warm.
 */

const ID = 'claude-code';

function executableFrom(config) {
	const raw = (config.get('claudeCode.executable') || '').trim();
	return raw === '' ? 'claude' : raw;
}

/**
 * Every tool that can change something on disk.
 *
 * While a turn is unauthorised these are removed from the session, so the
 * guarantee is the engine's, not the model's: the tools are not merely
 * discouraged, they do not exist. Bash is on the list because a shell is a
 * write tool; inspection still has Read, Grep and Glob.
 */
const WRITE_TOOLS = ['Edit', 'Write', 'NotebookEdit', 'Bash'];

/** Needs a TTY; in a headless session it errors mid-turn instead of asking. */
const INTERACTIVE_TOOLS = ['AskUserQuestion'];

/**
 * What the engine is told about being driven from a chat.
 *
 * Two things it cannot infer. First, there is no terminal here: its interactive
 * question tool is gone, and without this it would fall silent instead of
 * asking in words. Second, when writes are withheld it should say what it
 * *would* do and ask, rather than treating the missing tools as a dead end.
 */
function chatSystemPrompt(askPerCall) {
	const lines = [
		'Você responde dentro de um painel de chat, não num terminal interativo.',
		'Quando precisar de uma decisão do usuário, escreva a pergunta como texto e encerre o turno —',
		'a resposta virá na próxima mensagem. Nunca tente abrir um prompt interativo.',
	];
	if (askPerCall) {
		lines.push(
			'',
			'Cada alteração de arquivo e cada comando passa por uma aprovação do usuário no painel,',
			'chamada por chamada. Isso é normal, não é erro: faça a chamada e o painel perguntará.',
			'Se uma chamada for recusada, o motivo vem junto — leve-o em conta, não tente contornar,',
			'e não peça que o usuário habilite ferramentas ou permissões.',
		);
	}
	return lines.join('\n');
}

/** Flags for a persistent, streaming session. */
function sessionArgs(config, project, resumeId, writesAllowed, settings, modelOverride) {
	const this_askPerCall = Boolean(settings);
	const args = [
		'--print',
		'--input-format',
		'stream-json',
		'--output-format',
		'stream-json',
		'--verbose',
		'--include-partial-messages',
		'--disallowed-tools',
		...(writesAllowed ? INTERACTIVE_TOOLS : [...INTERACTIVE_TOOLS, ...WRITE_TOOLS]),
		'--append-system-prompt',
		chatSystemPrompt(this_askPerCall),
	];

	// A per-conversation `/model` wins; otherwise an explicit setting; otherwise
	// a gofi project's own `ai.model`, so the chat and the CLI agents run on the
	// same model by default.
	const override = typeof modelOverride === 'string' ? modelOverride.trim() : '';
	const model = override !== '' ? override : ((config.get('model') || '').trim() || (project && project.model) || '');
	if (model !== '') {
		args.push('--model', model);
	}
	// The engine has a second gate of its own, on top of the tool set: paths it
	// considers sensitive — `.claude/` among them — stay blocked under
	// `acceptEdits`, so an authorised turn would still fail on exactly the files
	// the RAG audit exists to fix. `auto` is the mode that actually carries out
	// what the user just approved. It is only ever used once they have.
	// `auto` is the only engine mode that carries out what the panel already
	// approved. The others add a second gate of the engine's own, by path —
	// `acceptEdits` refuses every write under `.claude/`, which is exactly where
	// the RAG audit needs to work. The real gate is the PreToolUse hook below.
	//
	// Note this reads a declared default, never an empty string: VS Code returns
	// the value from package.json when the user has not set one, so a `||`
	// fallback here would be dead code that silently never runs.
	const permissionMode = (config.get('permissionMode') || 'auto').trim();
	if (permissionMode !== 'default') {
		args.push('--permission-mode', permissionMode);
	}
	// The panel's own gate, installed as a PreToolUse hook: the engine asks it
	// before every tool that can change something, and a refusal is binding.
	// `auto` above is what stops the engine's own dialog from blocking a call
	// the user has already been asked about here.
	if (settings) {
		args.push('--settings', JSON.stringify(settings));
	}
	const effort = (config.get('effort') || '').trim();
	if (effort !== '') {
		args.push('--effort', effort);
	}
	// After a cancel, or when the panel reopens a saved conversation: in both
	// cases the process that held the thread is gone, and resuming by id is how
	// the next turn continues it rather than starting another one.
	if (resumeId) {
		args.push('--resume', resumeId);
	}
	return args;
}

/**
 * A live engine process, and the turn currently running on it.
 *
 * Owned by the panel: one session per conversation, disposed when the
 * conversation is reset or the last surface closes.
 */
class ClaudeCodeSession {
	/** @param {import('./types.js').ProviderContext} ctx */
	constructor(ctx) {
		this.ctx = ctx;
		/** @type {import('child_process').ChildProcess | null} */
		this.child = null;
		/**
		 * Engine-side conversation id, learned from the init event — or handed in
		 * when the panel is reopening a conversation from its history, in which
		 * case the first process starts with `--resume` and the agent picks the
		 * thread back up instead of meeting it as a transcript.
		 */
		this.sessionId = ctx.resumeSessionId || null;
		/** Set while a turn is in flight. */
		this.emit = null;
		this.settled = true;
		this.buffer = '';
		this.stderr = '';
		this.cancelled = false;
		this.disposed = false;
		/** Tool set the running process was started with. */
		this.childWritesAllowed = false;
	}

	/**
	 * Starts the engine, or reuses the one already running.
	 *
	 * A session's tool set is fixed when the process starts, so granting or
	 * withdrawing write access means a new process — resumed by id, so the
	 * conversation carries over and the agent applies exactly what it just
	 * described. That respawn is the price of the guarantee, and it is only
	 * paid when the authorisation actually changes.
	 */
	ensureChild(writesAllowed) {
		if (this.child && !this.child.killed && this.childWritesAllowed === writesAllowed) {
			return true;
		}
		if (this.child) {
			this.killChild();
		}
		const exe = executableFrom(this.ctx.config);
		const args = sessionArgs(
			this.ctx.config,
			this.ctx.project,
			this.sessionId,
			writesAllowed,
			this.ctx.hookSettings,
			this.ctx.modelOverride || null,
		);
		this.childWritesAllowed = writesAllowed;

		let child;
		try {
			child = spawn(exe, args, { cwd: this.ctx.cwd, stdio: ['pipe', 'pipe', 'pipe'] });
		} catch (err) {
			this.child = null;
			this.fail(`não deu para iniciar \`${exe}\`: ${err.message}`);
			return false;
		}
		// Every handler below closes over `child` and compares it with
		// `this.child`, which is how it tells "this process died" from "this
		// process was replaced". A flag cleared synchronously around the kill is
		// already false by the time `close` fires — that reported every
		// deliberate respawn as the engine crashing with SIGTERM.
		this.child = child;

		this.buffer = '';
		this.stderr = '';

		// The child may exit while we are mid-write; the exit handler reports it.
		child.stdin.on('error', () => {});

		child.stdout.setEncoding('utf8');
		child.stdout.on('data', (chunk) => this.consume(chunk));

		child.stderr.setEncoding('utf8');
		child.stderr.on('data', (chunk) => {
			// Cap it: a wedged engine can produce megabytes, and all we ever show
			// is the tail as a failure reason.
			this.stderr = (this.stderr + chunk).slice(-4000);
		});

		child.on('error', (err) => {
			if (this.child !== child) {
				return;
			}
			this.child = null;
			this.fail(
				err.code === 'ENOENT' ? `\`${exe}\` não encontrado no PATH` : String(err.message || err),
				err.code === 'ENOENT' ? 'Rode `gofi doctor`, ou ajuste `gofiAI.claudeCode.executable`.' : undefined,
			);
		});

		child.on('close', (code) => {
			if (this.child !== child) {
				return; // swapped out on purpose, to change the tool set
			}
			this.child = null;
			if (this.settled) {
				return; // idle between turns — nothing to report
			}
			if (this.cancelled) {
				this.finish({ type: 'done', isError: false, costUsd: null, durationMs: null, error: 'interrompido' });
				return;
			}
			this.fail(this.stderr.trim() || `o motor encerrou com código ${code} sem devolver um resultado`);
		});

		return true;
	}

	/**
	 * Runs one turn. The caller guarantees only one is in flight at a time.
	 *
	 * @param {import('./types.js').Turn} turn
	 * @param {(e: ProviderEvent) => void} emit
	 */
	send(turn, emit) {
		this.emit = emit;
		this.settled = false;
		this.cancelled = false;

		if (!this.ensureChild(turn.allowWrites === true)) {
			return;
		}

		/** @type {object[]} */
		const content = [];
		// Images first: the question almost always refers to what was pasted.
		for (const image of turn.images || []) {
			content.push({
				type: 'image',
				source: { type: 'base64', media_type: image.mediaType, data: image.data },
			});
		}
		content.push({ type: 'text', text: turn.prompt });

		try {
			this.child.stdin.write(`${JSON.stringify({ type: 'user', message: { role: 'user', content } })}\n`);
		} catch (err) {
			this.fail(`falha ao enviar a mensagem: ${err.message}`);
		}
	}

	/**
	 * Ends the running process without reporting it as a failure.
	 *
	 * Clearing `this.child` first is what makes the close handler recognise the
	 * exit as deliberate — it compares against the process it was attached to.
	 */
	killChild() {
		const child = this.child;
		this.child = null;
		if (!child) {
			return;
		}
		try {
			child.stdin.end();
		} catch {
			/* already gone */
		}
		child.kill('SIGTERM');
		setTimeout(() => {
			if (!child.killed) {
				child.kill('SIGKILL');
			}
		}, 2000).unref();
	}

	/** Splits the output stream into lines and translates each one. */
	consume(chunk) {
		this.buffer += chunk;
		let newline;
		while ((newline = this.buffer.indexOf('\n')) !== -1) {
			const line = this.buffer.slice(0, newline).trim();
			this.buffer = this.buffer.slice(newline + 1);
			if (line !== '') {
				this.translate(line);
			}
		}
	}

	/**
	 * Maps one line of the CLI's stream-json output onto a provider event.
	 *
	 * Unknown line types are dropped on purpose: the engine emits bookkeeping
	 * events (rate limits, hook lifecycle) that the chat has nothing to say
	 * about, and new ones appear between versions.
	 */
	translate(line) {
		let event;
		try {
			event = JSON.parse(line);
		} catch {
			return; // not JSON — engine chatter, not ours to render
		}

		switch (event.type) {
			case 'system':
				if (event.subtype === 'init') {
					if (event.session_id) {
						this.sessionId = event.session_id;
					}
					this.push({
						type: 'meta',
						sessionId: this.sessionId,
						model: event.model || null,
						slashCommands: Array.isArray(event.slash_commands) ? event.slash_commands : [],
					});
				}
				return;

			case 'stream_event':
				this.translateStreamEvent(event.event);
				return;

			case 'assistant':
				this.push({ type: 'message', blocks: normaliseBlocks(event.message && event.message.content) });
				if (event.message && event.message.usage) {
					this.push({ type: 'usage', scope: 'turn', usage: normaliseUsage(event.message.usage) });
				}
				return;

			case 'user':
				for (const block of contentBlocks(event.message && event.message.content)) {
					if (block.type === 'tool_result') {
						const text = textOf(block.content);
						this.push({
							type: 'tool_result',
							toolUseId: block.tool_use_id || '',
							isError: block.is_error === true,
							preview: firstLine(text),
							chars: text.length,
							// The full answer stays inside the extension process: a
							// graph query is audited by the files it names, and the
							// text is already here. Only the preview reaches the
							// webview.
							text,
						});
					}
				}
				return;

			case 'result':
				if (event.usage) {
					this.push({ type: 'usage', scope: 'total', usage: normaliseUsage(event.usage) });
				}
				this.finish({
					type: 'done',
					isError: event.is_error === true,
					costUsd: typeof event.total_cost_usd === 'number' ? event.total_cost_usd : null,
					durationMs: typeof event.duration_ms === 'number' ? event.duration_ms : null,
					error: event.is_error === true ? String(event.result || event.subtype || 'a execução falhou') : undefined,
				});
				return;

			default:
				return;
		}
	}

	/** Token-level deltas, present because we pass `--include-partial-messages`. */
	translateStreamEvent(inner) {
		if (!inner || inner.type !== 'content_block_delta' || !inner.delta) {
			return;
		}
		if (inner.delta.type === 'text_delta' && typeof inner.delta.text === 'string') {
			this.push({ type: 'delta', kind: 'text', text: inner.delta.text });
		} else if (inner.delta.type === 'thinking_delta' && typeof inner.delta.thinking === 'string') {
			this.push({ type: 'delta', kind: 'thinking', text: inner.delta.thinking });
		}
	}

	push(event) {
		if (this.emit && !this.settled) {
			this.emit(event);
		}
	}

	/** Ends the current turn. Further events for it are dropped. */
	finish(event) {
		if (this.settled) {
			return;
		}
		const emit = this.emit;
		this.settled = true;
		this.emit = null;
		if (emit) {
			emit(event);
		}
	}

	fail(message, hint) {
		this.finish({ type: 'error', message, hint });
	}

	/**
	 * Stops the turn in flight.
	 *
	 * The engine has no interrupt on this protocol, so the process goes and the
	 * next turn resumes the conversation by id — a cold start, but only for the
	 * turn after a cancel.
	 */
	cancel() {
		if (this.settled || !this.child) {
			return;
		}
		this.cancelled = true;
		const child = this.child;
		child.kill('SIGTERM');
		// SIGTERM is a request. Escalate if the process ignores it.
		setTimeout(() => {
			if (!child.killed) {
				child.kill('SIGKILL');
			}
		}, 2000).unref();
	}

	dispose() {
		this.disposed = true;
		this.settled = true;
		this.emit = null;
		if (this.child) {
			const child = this.child;
			this.child = null;
			try {
				child.stdin.end();
			} catch {
				/* already gone */
			}
			child.kill('SIGTERM');
			setTimeout(() => {
				if (!child.killed) {
					child.kill('SIGKILL');
				}
			}, 2000).unref();
		}
	}
}

/** @type {import('./types.js').ChatProvider} */
const claudeCodeProvider = {
	id: ID,
	label: 'Claude',
	supportsImages: true,

	probe(ctx) {
		const exe = executableFrom(ctx.config);
		return new Promise((resolve) => {
			execFile(exe, ['--version'], { cwd: ctx.cwd, timeout: 15000 }, (err, stdout) => {
				if (err) {
					resolve({
						ok: false,
						detail: err.code === 'ENOENT' ? `\`${exe}\` não encontrado no PATH` : String(err.message || err),
						hint: 'Instale o Claude Code, ou aponte `gofiAI.claudeCode.executable` para o binário.',
					});
					return;
				}
				resolve({ ok: true, detail: String(stdout).trim() });
			});
		});
	},

	createSession(ctx) {
		return new ClaudeCodeSession(ctx);
	},

	/**
	 * The engine's own conversations for this folder — including the ones
	 * started in a terminal, which is the point: there is one set of sessions,
	 * not the panel's and the CLI's.
	 */
	savedSessions(ctx) {
		return claudeSessions.list(ctx.cwd);
	},

	savedTranscript(ctx, engineId) {
		return claudeSessions.transcript(ctx.cwd, engineId);
	},

	/** How to continue a given conversation in a terminal, as a command line. */
	resumeCommand(ctx, engineId) {
		return `${executableFrom(ctx.config)} --resume ${engineId}`;
	},
};

function contentBlocks(content) {
	return Array.isArray(content) ? content : [];
}

/** @returns {import('./types.js').Block[]} */
function normaliseBlocks(content) {
	const blocks = [];
	for (const block of contentBlocks(content)) {
		switch (block.type) {
			case 'text':
				if (typeof block.text === 'string' && block.text !== '') {
					blocks.push({ type: 'text', text: block.text });
				}
				break;
			case 'thinking':
				if (typeof block.thinking === 'string' && block.thinking !== '') {
					blocks.push({ type: 'thinking', text: block.thinking });
				}
				break;
			case 'tool_use':
				blocks.push({ type: 'tool_use', id: block.id || '', name: block.name || 'tool', input: block.input });
				break;
			default:
				break;
		}
	}
	return blocks;
}

/**
 * Flattens a tool result to plain text.
 *
 * The chat only ever shows the first line, but the full length is what the
 * token ledger measures — how much a Read or a Grep actually dragged into the
 * context is the whole retrieval-efficiency signal.
 */
function textOf(content) {
	if (typeof content === 'string') {
		return content;
	}
	const parts = [];
	for (const block of contentBlocks(content)) {
		if (block.type === 'text' && typeof block.text === 'string') {
			parts.push(block.text);
		}
	}
	return parts.join('\n');
}

/** Picks the token counters out of an engine usage object, defaulting to 0. */
function normaliseUsage(usage) {
	const num = (v) => (typeof v === 'number' && Number.isFinite(v) ? v : 0);
	return {
		input: num(usage.input_tokens),
		output: num(usage.output_tokens),
		cacheRead: num(usage.cache_read_input_tokens),
		cacheWrite: num(usage.cache_creation_input_tokens),
	};
}

function firstLine(text) {
	const trimmed = text.trim();
	const cut = trimmed.indexOf('\n');
	const line = cut === -1 ? trimmed : trimmed.slice(0, cut);
	return line.length > 160 ? `${line.slice(0, 160)}…` : line;
}

module.exports = { claudeCodeProvider };
