'use strict';

/**
 * Token accounting and retrieval auditing for a chat session.
 *
 * Two questions the panel answers, both live while the agent works:
 *
 *  1. What is this conversation costing? — token counters straight from the
 *     engine, split into new input, cache read, cache write and output, since
 *     those bill at wildly different rates and the split is the whole story.
 *
 *  2. Is the project's retrieval actually working? — every Read/Grep/Glob and
 *     every `gofi graph` query is recorded with how much text it dragged into
 *     the context. A gofi project has an explicit retrieval protocol
 *     (frontmatter first, then `grep '^## '`, then Read of just that section)
 *     and, for code, the graph: `gofi graph explain` answers "where is it, who
 *     calls it" in ~30 lines, which the same question costs many times over
 *     when asked by grepping the tree and opening what matched. An agent that
 *     opens a 900-line spec whole, or that hunts for a symbol with Grep while
 *     the graph sits on disk, is burning context the protocol exists to save,
 *     and that shows up here as a finding rather than as a surprise on the
 *     invoice.
 *
 * Pure logic, no vscode imports — the numbers are testable on their own.
 */

/** Rough chars-per-token. Only used for tool output, which the engine does not
 *  meter separately; the headline token counts are the engine's own. */
const CHARS_PER_TOKEN = 4;

/** A single call above this is worth naming, in approximate tokens. */
const HEAVY_CALL_TOKENS = 4000;

/** Below this a whole-file read is not worth complaining about. */
const SMALL_READ_TOKENS = 500;

/** Cache reads below this share of billed input suggest a churning prefix. */
const LOW_CACHE_RATE = 0.5;

/**
 * Turns to let pass before judging the cache.
 *
 * The first turns of a session *write* the cache — that is what establishes it,
 * and the ratio is meant to look poor while it happens. Warning then reports
 * normal behaviour as a problem.
 */
const CACHE_WARMUP_TURNS = 3;

/** Below this many code searches a session is too young to judge. */
const MIN_CODE_SEARCHES = 3;

/** Tools that pull text into the context — the ones retrieval efficiency is about. */
const RETRIEVAL_TOOLS = new Set(['Read', 'Grep', 'Glob', 'WebFetch', 'NotebookRead']);

/** Folders holding the project's RAG corpora, which the protocol governs. */
const CORPUS_PREFIXES = ['specs/', 'prd/', '.claude/'];

/** Where `gofi graph build` writes the map — a Read in here is a graph query. */
const GRAPH_DIR = '.gofi/graph/';

/**
 * The `gofi graph` subcommands that answer a question about the code.
 *
 * `build`, `open`, `hooks` and `install` maintain the map rather than consult
 * it, so counting them as searches would flatter every session that rebuilds.
 * Matched anywhere in the command line because the agent chains and pipes.
 */
const GRAPH_QUERY = /\bgofi\s+graph\s+explain\b/;

/** The shell command a Bash call ran, whatever key the engine used for it. */
function bashCommand(input) {
	if (!input || typeof input !== 'object') {
		return '';
	}
	return typeof input.command === 'string' ? input.command : '';
}

/** The graph query inside a Bash call, shortened for display, or ''. */
function graphQueryOf(input) {
	const command = bashCommand(input);
	if (!GRAPH_QUERY.test(command)) {
		return '';
	}
	const match = /\bgofi\s+graph\s+(explain[^\n|;&]*)/.exec(command);
	return match ? match[1].trim() : 'graph';
}

/**
 * Which question a retrieval call was asking, which is what decides whether it
 * should have gone through the graph.
 *
 * @returns {'graph'|'doc'|'code'|'web'}
 */
function kindOf(name, target) {
	if (name === 'WebFetch') {
		return 'web';
	}
	if (target.startsWith(GRAPH_DIR)) {
		return 'graph';
	}
	return inCorpus(target) ? 'doc' : 'code';
}

function approxTokens(chars) {
	return Math.round(chars / CHARS_PER_TOKEN);
}

/** The path-ish argument a retrieval tool was pointed at. */
function targetOf(name, input) {
	if (!input || typeof input !== 'object') {
		return '';
	}
	const candidates = name === 'Grep'
		? [input.path, input.glob, input.pattern]
		: [input.file_path, input.path, input.pattern, input.url, input.notebook_path];
	for (const value of candidates) {
		if (typeof value === 'string' && value !== '') {
			return value;
		}
	}
	return '';
}

/** Strips the workspace prefix so findings read as project-relative paths. */
function relativise(target, root) {
	if (root && target.startsWith(root)) {
		return target.slice(root.length).replace(/^[/\\]+/, '');
	}
	return target;
}

function inCorpus(path) {
	return CORPUS_PREFIXES.some((prefix) => path.startsWith(prefix));
}

/**
 * Whether the call narrowed its own scope. A `Read` with offset/limit asked
 * for a section; one without asked for the file.
 */
function isScoped(name, input) {
	if (!input || typeof input !== 'object') {
		return false;
	}
	if (name === 'Read' || name === 'NotebookRead') {
		return typeof input.offset === 'number' || typeof input.limit === 'number';
	}
	if (name === 'Grep') {
		// A grep confined to a path or glob is targeted; one over the whole tree
		// is a sweep.
		return typeof input.path === 'string' || typeof input.glob === 'string';
	}
	return true;
}

class UsageLedger {
	/** @param {string} root Workspace folder, used to shorten paths */
	constructor(root) {
		this.root = root || '';
		/**
		 * Whether the project has a built graph. Set by the host, which is the
		 * side that can stat disk; without it there is nothing to prefer over a
		 * Grep, and saying otherwise would be advice the project cannot follow.
		 */
		this.graphAvailable = false;
		this.reset();
	}

	reset() {
		/** Engine-reported counters, accumulated across turns. */
		this.tokens = { input: 0, output: 0, cacheRead: 0, cacheWrite: 0 };
		this.costUsd = 0;
		this.turns = 0;
		/** @type {Map<string, object>} Retrieval calls awaiting their result. */
		this.pending = new Map();
		/** @type {object[]} Completed retrieval calls, in order. */
		this.rows = [];
	}

	/**
	 * Accumulates token counters.
	 *
	 * Only per-message (`turn`) usage is summed. The engine also reports a
	 * session total on the final result, which already includes every message —
	 * adding both would double-count.
	 */
	noteUsage(scope, usage) {
		if (scope !== 'turn' || !usage) {
			return;
		}
		this.tokens.input += usage.input || 0;
		this.tokens.output += usage.output || 0;
		this.tokens.cacheRead += usage.cacheRead || 0;
		this.tokens.cacheWrite += usage.cacheWrite || 0;
	}

	noteCost(costUsd) {
		if (typeof costUsd === 'number' && Number.isFinite(costUsd)) {
			this.costUsd += costUsd;
		}
		this.turns += 1;
	}

	/** Opens a retrieval row the moment the agent reaches for a file. */
	noteToolUse(id, name, input) {
		// A graph query arrives as a shell command, not as a retrieval tool, and
		// is the one search that is supposed to happen — so it is measured beside
		// the reads it replaces instead of disappearing into the Bash calls.
		const query = name === 'Bash' ? graphQueryOf(input) : '';
		if (query === '' && !RETRIEVAL_TOOLS.has(name)) {
			return;
		}
		const target = query !== '' ? query : relativise(targetOf(name, input), this.root);
		this.pending.set(id, {
			id,
			name: query !== '' ? 'gofi graph' : name,
			target,
			kind: query !== '' ? 'graph' : kindOf(name, target),
			scoped: query !== '' ? true : isScoped(name, input),
			tokens: null, // filled when the result arrives
			/** Files a graph answer named, measured off disk by the host. */
			cited: null,
			failed: false,
		});
	}

	/** Closes the row with what the call actually cost in context. */
	noteToolResult(id, chars, isError) {
		const row = this.pending.get(id);
		if (!row) {
			return;
		}
		this.pending.delete(id);
		row.tokens = approxTokens(chars || 0);
		row.failed = isError === true;
		this.rows.push(row);
	}

	/** The row a tool call opened, still pending or already closed. */
	rowById(id) {
		return this.pending.get(id) || this.rows.find((row) => row.id === id) || null;
	}

	/**
	 * Records the files a graph answer pointed at, sized off disk.
	 *
	 * Arrives after the row closed — the measurement is async on purpose — so it
	 * is attached by id rather than to whatever is on top.
	 */
	noteAvoided(id, cited) {
		const row = this.rowById(id);
		if (row) {
			row.cited = cited;
		}
	}

	/**
	 * What the graph answered without anything being opened.
	 *
	 * A file the session went on to read anyway was not avoided, so it does not
	 * count; neither does the same file cited by two queries.
	 */
	avoided() {
		const read = new Set();
		for (const row of this.rows) {
			if (row.name === 'Read' && row.target !== '') {
				read.add(row.target);
			}
		}
		const counted = new Set();
		let tokens = 0;
		for (const row of this.rows) {
			for (const file of row.cited || []) {
				if (read.has(file.path) || counted.has(file.path)) {
					continue;
				}
				counted.add(file.path);
				tokens += file.tokens;
			}
		}
		return { tokens, files: counted.size };
	}

	/** Everything the panel renders, recomputed on demand. */
	snapshot() {
		const billedInput = this.tokens.input + this.tokens.cacheRead + this.tokens.cacheWrite;
		const cacheRate = billedInput > 0 ? this.tokens.cacheRead / billedInput : null;
		const retrievalTokens = this.rows.reduce((sum, row) => sum + (row.tokens || 0), 0);

		// Rows still open are shown as in-flight, so a long Read is visible while
		// it runs rather than appearing only once it lands.
		const inFlight = [...this.pending.values()].map((row) => ({ ...row }));

		const byKind = this.countByKind();
		const avoided = this.avoided();

		return {
			tokens: { ...this.tokens },
			billedInput,
			cacheRate,
			costUsd: this.costUsd,
			turns: this.turns,
			retrieval: {
				tokens: retrievalTokens,
				calls: this.rows.length,
				graph: byKind.graph,
				code: byKind.code,
				docs: byKind.doc,
				codeTokens: byKind.codeTokens,
				graphTokens: byKind.graphTokens,
				avoided,
				rows: [...this.rows].reverse().slice(0, 40),
				inFlight,
			},
			findings: this.findings(cacheRate, retrievalTokens),
		};
	}

	/** How the session's searches split between the graph, the docs and the tree. */
	countByKind() {
		const out = { graph: 0, code: 0, doc: 0, graphTokens: 0, codeTokens: 0 };
		for (const row of this.rows) {
			if (row.kind === 'graph') {
				out.graph += 1;
				out.graphTokens += row.tokens || 0;
			} else if (row.kind === 'code') {
				out.code += 1;
				out.codeTokens += row.tokens || 0;
			} else if (row.kind === 'doc') {
				out.doc += 1;
			}
		}
		return out;
	}

	/**
	 * Turns the raw numbers into things worth changing.
	 *
	 * Each finding names the offending call and says what to do instead. Nothing
	 * is reported on a hunch: every rule below fires on a measured quantity.
	 *
	 * @returns {{level: 'warn'|'info'|'good', text: string}[]}
	 */
	findings(cacheRate, retrievalTokens) {
		const out = [];

		// 1. Whole-file reads of the RAG corpora — the protocol violation that
		//    costs the most, because those docs are long by design.
		const wholeCorpusReads = this.rows.filter(
			(row) => row.name === 'Read' && !row.scoped && inCorpus(row.target) && (row.tokens || 0) > SMALL_READ_TOKENS,
		);
		for (const row of wholeCorpusReads.slice(0, 5)) {
			out.push({
				level: 'warn',
				text: `${row.target} lido inteiro (~${row.tokens} tokens). O protocolo do projeto pede frontmatter → grep '^## ' → Read só da seção.`,
			});
		}

		// 2. The same file pulled in more than once in one session — the second
		//    read is pure duplication, the content was already in context.
		const byTarget = new Map();
		for (const row of this.rows) {
			if (row.name !== 'Read' || row.target === '') {
				continue;
			}
			const entry = byTarget.get(row.target) || { count: 0, tokens: 0 };
			entry.count += 1;
			entry.tokens += row.tokens || 0;
			byTarget.set(row.target, entry);
		}
		for (const [target, entry] of byTarget) {
			if (entry.count > 1) {
				out.push({
					level: 'warn',
					text: `${target} lido ${entry.count}× (~${entry.tokens} tokens no total) — releituras não somam contexto novo.`,
				});
			}
		}

		// 3. Any single call that dominated the context on its own.
		for (const row of this.rows) {
			if ((row.tokens || 0) >= HEAVY_CALL_TOKENS && !wholeCorpusReads.includes(row)) {
				out.push({
					level: 'info',
					text: `${row.name} ${row.target || ''} trouxe ~${row.tokens} tokens de uma vez — vale restringir o alvo.`.trim(),
				});
			}
		}

		// 4. Cache health. A low read share means the stable prefix keeps
		//    changing, re-billing context that should have been nearly free —
		//    but only once the cache has had a chance to establish itself.
		if (cacheRate !== null && this.turns > 0) {
			const percent = Math.round(cacheRate * 100);
			if (this.turns < CACHE_WARMUP_TURNS && cacheRate < LOW_CACHE_RATE) {
				out.push({
					level: 'info',
					text: `${percent}% da entrada veio do cache — normal nos primeiros turnos, que são justamente os que gravam o cache. A partir daqui deve subir.`,
				});
			} else if (cacheRate < LOW_CACHE_RATE) {
				out.push({
					level: 'warn',
					text: `Só ${percent}% da entrada veio do cache depois de ${this.turns} turnos — algo no prefixo muda a cada volta, e isso recobra contexto que deveria sair quase de graça.`,
				});
			} else {
				out.push({ level: 'good', text: `${percent}% da entrada veio do cache.` });
			}
		}

		// 5. Code hunted through the tree while the graph was on disk. `gofi graph
		//    explain` answers "where is it / who calls it" in about thirty lines;
		//    the same question asked with Grep costs every file that matched, and
		//    still cannot list the callers.
		const kinds = this.countByKind();
		if (this.graphAvailable && kinds.code >= MIN_CODE_SEARCHES) {
			const searches = kinds.code + kinds.graph;
			if (kinds.graph === 0) {
				out.push({
					level: 'warn',
					text: `${kinds.code} buscas de código (~${kinds.codeTokens} tokens) foram por Grep/Glob/Read e nenhuma pelo grafo — \`gofi graph explain <termo>\` acha o símbolo e diz quem o chama sem abrir arquivo.`,
				});
			} else if (kinds.code > kinds.graph) {
				out.push({
					level: 'info',
					text: `Só ${kinds.graph} de ${searches} buscas de código passaram pelo grafo; as outras ${kinds.code} custaram ~${kinds.codeTokens} tokens abrindo a árvore.`,
				});
			}
		}

		// 6. The positive signal, so the panel says what is working, not only
		//    what isn't. When the cited files were measured, it says it in the
		//    only currency that settles the argument.
		const avoided = this.avoided();
		if (avoided.files > 0) {
			const files =
				avoided.files === 1
					? '1 arquivo que não precisou ser aberto'
					: `${avoided.files} arquivos que não precisaram ser abertos`;
			out.push({
				level: 'good',
				text: `O grafo apontou ${files}: ~${avoided.tokens} tokens que um grep seguido de leitura teria trazido, contra os ~${kinds.graphTokens} que as consultas ao grafo custaram.`,
			});
		} else if (kinds.graph > 0 && kinds.graph >= kinds.code) {
			out.push({
				level: 'good',
				text: `${kinds.graph} de ${kinds.graph + kinds.code} buscas de código foram pelo grafo (~${kinds.graphTokens} tokens).`,
			});
		}
		const scoped = this.rows.filter((row) => row.scoped).length;
		if (this.rows.length > 0) {
			if (scoped === this.rows.length) {
				out.push({ level: 'good', text: `Todas as ${this.rows.length} buscas foram direcionadas (~${retrievalTokens} tokens).` });
			} else if (scoped > 0) {
				out.push({ level: 'info', text: `${scoped} de ${this.rows.length} buscas foram direcionadas.` });
			}
		}

		return out;
	}
}

module.exports = { UsageLedger, approxTokens, CHARS_PER_TOKEN };
