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
	if (result.blocked) {
		return 'blocked';
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

/** The flow that delivers what a plan phase builds (gofi decision 0002). */
// One flow delivers every build: which role implements is dispatched inside it.
const DELIVERY_FLOWS = { code: 'delivery', ui: 'delivery', infra: 'delivery' };

/** The flow that delivers an artifact, or null when the loop does not build it. */
function deliveryFlow(produces) {
	return DELIVERY_FLOWS[produces] || null;
}

/** Whether a phase is part of a delivery: the build, or the audit the flow holds. */
function deliverable(phase) {
	return !phase.elicit && (Boolean(deliveryFlow(phase.produces)) || phase.produces === 'audit');
}

/**
 * Hands what a plan builds to the loop: `gofi run --context <context> --flow
 * <flow> --events` delivers every sealed change of the context, in order, and
 * writes its events as NDJSON — each one handed to onEvent as it comes. The
 * panel runs nothing of the delivery itself.
 *
 * @param {string} gofiPath
 * @param {string} root
 * @param {string} flow
 * @param {string} context
 * @param {(e: {ev: string, node?: string, to?: string, why?: string, run?: string, cost_usd?: number}) => void} onEvent
 * @param {{stop?: () => void}} [control] given stop(): it ends the delivery
 *   as gofi cancel does — at its next checkpoint, to go on with --resume.
 * @returns {Promise<{ok: boolean, outcome: string, output: string}>}
 */
function deliverContext(gofiPath, root, flow, context, onEvent, control) {
	return new Promise((resolve) => {
		const child = spawn(gofiPath || 'gofi', ['run', '--context', context, '--flow', flow, '--events'], { cwd: root, stdio: ['ignore', 'pipe', 'pipe'] });
		if (control) {
			control.stop = () => {
				if (child.exitCode === null && !child.killed) {
					child.kill('SIGTERM');
				}
			};
		}
		let buffer = '';
		let stderr = '';
		let outcome = '';
		child.stdout.on('data', (chunk) => {
			buffer += chunk;
			let nl;
			while ((nl = buffer.indexOf('\n')) >= 0) {
				const line = buffer.slice(0, nl).trim();
				buffer = buffer.slice(nl + 1);
				if (!line) {
					continue;
				}
				try {
					const e = JSON.parse(line);
					if (e.ev === 'run.result') {
						outcome = e.why || '';
					}
					onEvent(e);
				} catch {
					// Not an event: the process said something else; stderr explains a failure.
				}
			}
		});
		child.stderr.on('data', (chunk) => { stderr += chunk; });
		child.on('error', (err) => resolve({ ok: false, outcome: '', output: err.message }));
		child.on('close', (code) => resolve({ ok: code === 0, outcome, output: stderr.trim() }));
	});
}

/**
 * Seals the contract the person approved at a review stop: `gofi spec approve
 * <context>` approves the context's spec and every open change that touches it
 * (decision 0003). The panel does it, never the agent; a document with an
 * error seals nothing, and the command's output says what to fix.
 *
 * @param {string} gofiPath
 * @param {string} root
 * @param {string} context
 * @returns {Promise<{ok: boolean, output: string}>}
 */
function sealContract(gofiPath, root, context) {
	return new Promise((resolve) => {
		execFile(gofiPath || 'gofi', ['spec', 'approve', context], { cwd: root, timeout: TIMEOUT_MS }, (err, stdout, stderr) => {
			const output = `${stdout || ''}${stderr || ''}`.trim();
			resolve({ ok: !err, output });
		});
	});
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
/**
 * The decisions a delivery's run waits on at its gate, as an elicitation —
 * null when none is open.
 *
 * @param {string} root
 * @param {string} run
 */
function readGate(root, run) {
	try {
		const q = JSON.parse(fs.readFileSync(path.join(root, '.gofi', 'runs', run, 'gate.json'), 'utf8'));
		return Array.isArray(q.questions) && q.questions.length > 0 ? { questions: q.questions, derived: [] } : null;
	} catch {
		return null;
	}
}

/**
 * Answers a run's gate as gofi answer does: a question left out takes its
 * recommendation, recorded as assumed.
 *
 * @param {string} gofiPath
 * @param {string} root
 * @param {string} run
 * @param {Record<string, string>} answers
 * @returns {Promise<{ok: boolean, output: string}>}
 */
function answerGate(gofiPath, root, run, answers) {
	const args = ['answer', run];
	for (const [id, value] of Object.entries(answers)) {
		args.push('--set', `${id}=${value}`);
	}
	return new Promise((resolve) => {
		execFile(gofiPath || 'gofi', args, { cwd: root }, (err, stdout, stderr) => {
			resolve({ ok: !err, output: String(stderr || stdout || (err && err.message) || '').trim() });
		});
	});
}

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
	SCHEMA, shouldPlan, intakeArgs, runIntake, runIntakeKept, stopIntakeServers, IntakeServer, phaseTurn, nextStep, deliveryFlow, deliverable, deliverContext,
	readElicitation, clearElicitation, elicitQuestions, withDecisions, sealContract, readGate, answerGate,
};
