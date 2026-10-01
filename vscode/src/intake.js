'use strict';

const { execFile, spawn } = require('child_process');
const fs = require('fs');
const path = require('path');

/**
 * The shape of `gofi intake --json` this panel reads. A different schema is a
 * gofi the panel does not know — it says so instead of guessing.
 */
const SCHEMA = 'gofi.intake/v1';

/** How long the intake may take: it can consult the light tier, which takes seconds. */
const TIMEOUT_MS = 90000;

/**
 * @typedef {Object} IntakePhase
 * @property {string} role
 * @property {string} produces
 * @property {string} tier
 * @property {string} base_tier
 * @property {string} why
 * @property {string} tier_why
 * @property {string} [turn]        What to send to run the phase (its skill)
 * @property {string} [role_turn]   The same, for a phase the router raised
 * @property {string} [model]       The raised tier's model, set for that phase
 * @property {boolean} [review_after] The plan stops for review after this phase
 * @property {IntakePhase[]} [on_reject] What to run when this audit fails the delivery
 * @property {boolean} [elicit]      A cheaper phase that gathers the next role's open decisions
 * @property {string} [elicit_file]  Where that phase writes them
 */

/**
 * Whether a message goes through the intake: free text only. A skill invoked
 * outright, a shell command and a message with attachments go as typed — the
 * person already said what to do, or is showing the agent something.
 *
 * @param {string} text
 * @param {number} attachments
 */
function shouldPlan(text, attachments) {
	const t = String(text || '').trim();
	return t !== '' && attachments === 0 && !t.startsWith('/') && !t.startsWith('!');
}

/**
 * The arguments of one `gofi intake --json` call.
 *
 * @param {string} text
 * @param {{answers?: Record<string, string>, proceed?: boolean}} [opts]
 */
function intakeArgs(text, opts = {}) {
	const args = ['intake', text, '--json'];
	for (const [id, value] of Object.entries(opts.answers || {})) {
		args.push('--answer', `${id}=${value}`);
	}
	if (opts.proceed) {
		args.push('--proceed');
	}
	return args;
}

/**
 * Runs `gofi intake --json` in the project and returns its plan.
 *
 * @param {string} gofiPath The gofi binary
 * @param {string} root     The project root
 * @param {string} text
 * @param {{answers?: Record<string, string>, proceed?: boolean}} [opts]
 * @returns {Promise<any>}
 */
function runIntake(gofiPath, root, text, opts = {}) {
	return new Promise((resolve, reject) => {
		execFile(gofiPath || 'gofi', intakeArgs(text, opts), { cwd: root, timeout: TIMEOUT_MS, maxBuffer: 4 * 1024 * 1024 }, (err, stdout, stderr) => {
			if (err) {
				const detail = String(stderr || '').trim() || err.message;
				reject(new Error(`gofi intake: ${detail}`));
				return;
			}
			let result;
			try {
				result = JSON.parse(stdout);
			} catch {
				reject(new Error('gofi intake: resposta ilegível'));
				return;
			}
			if (!result || result.schema !== SCHEMA) {
				reject(new Error(`gofi intake: esquema ${result && result.schema} desconhecido — atualize a extensão (gofi install extensions)`));
				return;
			}
			resolve(result);
		});
	});
}

/**
 * One `gofi intake --serve` kept open per gofi binary and project: the index
 * stays loaded between messages, so planning takes milliseconds instead of the
 * ~0.4–0.8 s of opening it every time. Requests go one per line and are
 * answered in order. A gofi without --serve (older than the panel) is noted,
 * and every request then goes through `gofi intake --json` as before.
 */
class IntakeServer {
	constructor(gofiPath, root) {
		this.gofiPath = gofiPath;
		this.root = root;
		this.child = null;
		this.buffer = '';
		/** @type {{resolve: Function, reject: Function, timer: NodeJS.Timeout}[]} */
		this.waiting = [];
		this.unsupported = false;
	}

	/** Plans one request on the kept process. */
	request(text, opts = {}) {
		if (!this.child) {
			this.start();
		}
		const line = JSON.stringify({ request: text, answers: opts.answers || {}, proceed: Boolean(opts.proceed) });
		return new Promise((resolve, reject) => {
			const timer = setTimeout(() => {
				this.stop(Object.assign(new Error('gofi intake: sem resposta a tempo'), { timedOut: true }));
			}, TIMEOUT_MS);
			this.waiting.push({ resolve, reject, timer });
			try {
				this.child.stdin.write(`${line}\n`);
			} catch (err) {
				this.stop(err);
			}
		});
	}

	start() {
		const child = spawn(this.gofiPath || 'gofi', ['intake', '--serve'], { cwd: this.root, stdio: ['pipe', 'pipe', 'pipe'] });
		this.child = child;
		this.buffer = '';
		let stderr = '';
		child.stdin.on('error', () => {});
		child.stdout.setEncoding('utf8');
		child.stdout.on('data', (chunk) => this.consume(chunk));
		child.stderr.setEncoding('utf8');
		child.stderr.on('data', (chunk) => {
			stderr = (stderr + chunk).slice(-2000);
		});
		child.on('error', (err) => this.stop(err));
		child.on('close', () => {
			if (this.child !== child) {
				return;
			}
			if (/unknown flag: --serve/.test(stderr)) {
				this.unsupported = true;
			}
			this.stop(new Error(`gofi intake: ${stderr.trim() || 'o processo terminou'}`));
		});
	}

	consume(chunk) {
		this.buffer += chunk;
		let newline;
		while ((newline = this.buffer.indexOf('\n')) !== -1) {
			const line = this.buffer.slice(0, newline).trim();
			this.buffer = this.buffer.slice(newline + 1);
			if (line === '') {
				continue;
			}
			const next = this.waiting.shift();
			if (!next) {
				continue;
			}
			clearTimeout(next.timer);
			let result;
			try {
				result = JSON.parse(line);
			} catch {
				next.reject(new Error('gofi intake: resposta ilegível'));
				continue;
			}
			if (result && result.error) {
				next.reject(new Error(`gofi intake: ${result.error}`));
			} else if (!result || result.schema !== SCHEMA) {
				next.reject(new Error(`gofi intake: esquema ${result && result.schema} desconhecido — atualize a extensão (gofi install extensions)`));
			} else {
				next.resolve(result);
			}
		}
	}

	/** Ends the process; whatever was waiting fails with err. */
	stop(err) {
		const child = this.child;
		this.child = null;
		if (child) {
			try {
				child.kill();
			} catch {
				// Already gone.
			}
		}
		for (const w of this.waiting.splice(0)) {
			clearTimeout(w.timer);
			w.reject(err || new Error('gofi intake: encerrado'));
		}
	}
}

/** @type {Map<string, IntakeServer>} */
const servers = new Map();

/**
 * Plans a request on the kept `gofi intake --serve` of this project, falling
 * back to one `gofi intake --json` call when the server cannot answer — an
 * older gofi, a crash — so a message is never lost to the optimisation.
 */
async function runIntakeKept(gofiPath, root, text, opts = {}) {
	const key = `${gofiPath}\0${root}`;
	let server = servers.get(key);
	if (!server) {
		server = new IntakeServer(gofiPath, root);
		servers.set(key, server);
	}
	if (server.unsupported) {
		return runIntake(gofiPath, root, text, opts);
	}
	try {
		return await server.request(text, opts);
	} catch (err) {
		if (err.timedOut) {
			// It already had its time; a second try would double the wait.
			throw err;
		}
		return runIntake(gofiPath, root, text, opts);
	}
}

/** Ends every kept server — the extension is going away. */
function stopIntakeServers() {
	for (const server of servers.values()) {
		server.stop();
	}
	servers.clear();
}

/**
 * What to send for a phase, and the model to set for it: a raised phase runs
 * on its tier's model, as the role; any other invokes its skill.
 *
 * @param {IntakePhase} phase
 * @returns {{text: string, model: string|null}}
 */
function phaseTurn(phase) {
	if (phase.role_turn && phase.model) {
		return { text: phase.role_turn, model: phase.model };
	}
	return { text: phase.turn || `/${phase.role}`, model: null };
}

/**
 * What the plan asks of the person before anything runs: an answer, a
 * confirmation (a question was answered, or a default assumed), or nothing.
 *
 * @param {any} result
 * @param {boolean} answered
 * @returns {'answer'|'confirm'|'answerOnly'|'raw'|'direct'|'go'}
 */
function nextStep(result, answered) {
	if (result.direct || result.chat) {
		return 'direct';
	}
	if (result.ask) {
		return 'answer';
	}
	if (result.answer) {
		return 'answerOnly';
	}
	if (!result.plan || result.plan.length === 0) {
		return 'raw';
	}
	if (answered || (result.assumptions && result.assumptions.length > 0)) {
		return 'confirm';
	}
	return 'go';
}

/**
 * The context's memory as it is now, where the QA writes its verdict — to
 * tell a verdict the audit just wrote from one left by an earlier run. Null
 * when there is none.
 *
 * @param {string} root
 * @param {any} result
 * @returns {string|null}
 */
function readVerdict(root, result) {
	if (!result.verdict_file) {
		return null;
	}
	try {
		return fs.readFileSync(path.join(root, result.verdict_file), 'utf8');
	} catch {
		return null;
	}
}

/** The status field of a document's frontmatter. */
function statusOf(doc) {
	const m = /^---\r?\n([\s\S]*?)\r?\n---/.exec(String(doc || ''));
	if (!m) {
		return '';
	}
	const line = m[1].split(/\r?\n/).find((l) => l.startsWith('status:'));
	return line ? line.slice('status:'.length).trim().replace(/^["']|["']$/g, '').toLowerCase() : '';
}

/**
 * Whether the audit that just ran failed the delivery: the memory changed
 * since before the phase and its status is now reprovado. An audit that wrote
 * nothing rejects nothing.
 *
 * @param {string|null} before
 * @param {string|null} now
 */
function rejected(before, now) {
	return now !== null && now !== before && statusOf(now) === 'reprovado';
}

/** The shape of an elicitation file (`gofi.elicit/v1`). */
const ELICIT_SCHEMA = 'gofi.elicit/v1';

/**
 * What an elicitation phase left for the person: null when it left nothing.
 * A file in another shape is an error — the phase did not do its job.
 *
 * @param {string} root
 * @param {string} rel
 */
function readElicitation(root, rel) {
	if (!rel) {
		return null;
	}
	let text;
	try {
		text = fs.readFileSync(path.join(root, rel), 'utf8');
	} catch {
		return null;
	}
	const e = JSON.parse(text);
	if (!e || e.schema !== ELICIT_SCHEMA) {
		throw new Error(`${rel}: esquema ${e && e.schema}, esperado ${ELICIT_SCHEMA}`);
	}
	e.questions = (e.questions || []).map((q, i) => ({ ...q, id: q.id || `q${i + 1}` }));
	e.derived = e.derived || [];
	return e;
}

/** Removes what a previous run left, so a phase is judged by what it writes now. */
function clearElicitation(root, rel) {
	if (rel) {
		try {
			fs.rmSync(path.join(root, rel), { force: true });
		} catch {
			// Nothing to clear.
		}
	}
}

/** The decisions as the panel asks them: the recommended option first. */
function elicitQuestions(e) {
	return e.questions.map((q) => {
		const options = q.recommended ? [q.recommended] : [];
		for (const o of q.options || []) {
			if (o !== q.recommended) {
				options.push(o);
			}
		}
		const text = q.why && q.recommended ? `${q.text} (recomendado: ${q.recommended} — ${q.why})` : q.text;
		return { id: q.id, text, options };
	});
}

/**
 * The turn of the role an elicitation prepared, with the decisions at its
 * end; an unanswered one is its recommendation, marked as assumed.
 */
function withDecisions(turn, e, answers) {
	if (!e || (e.questions.length === 0 && e.derived.length === 0)) {
		return turn;
	}
	const lines = [String(turn).replace(/\n+$/, ''), '', '**Decisões confirmadas** (não pergunte de novo):', ''];
	for (const q of e.questions) {
		const v = String((answers || {})[q.id] || '').trim();
		const fallback = q.recommended || (q.options || [])[0] || '';
		lines.push(v ? `- ${q.text} → ${v}` : `- ${q.text} → ${fallback} (assumido: a recomendação, sem resposta)`);
	}
	for (const d of e.derived) {
		lines.push(`- ${d}`);
	}
	return `${lines.join('\n')}\n`;
}

module.exports = {
	SCHEMA, shouldPlan, intakeArgs, runIntake, runIntakeKept, stopIntakeServers, IntakeServer, phaseTurn, nextStep, readVerdict, statusOf, rejected,
	readElicitation, clearElicitation, elicitQuestions, withDecisions,
};
