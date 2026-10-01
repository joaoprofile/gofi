'use strict';

const {
	runIntakeKept, phaseTurn, nextStep, readVerdict, statusOf, rejected,
	readElicitation, clearElicitation, elicitQuestions, withDecisions,
} = require('./intake.js');
const { RunRecord } = require('./runs.js');

/**
 * The panel's side of the intake (gofi decision 0001): a request in free text
 * is planned by `gofi intake --json`, its questions offered as buttons, and its
 * phases conducted one turn each on the same engine — each role at its tier,
 * with a stop for review after a PRD or a spec.
 *
 * Nothing here decides a plan: gofi does, and hands each phase's turn, model
 * and review stop ready. These are methods of Chat, installed on it by
 * `installConduct`, so they share its engine, its queue and its posts.
 */
const conduct = {
	/** The gofi binary: `gofiAI.gofiPath`, or `gofi` on PATH. */
	gofiPath() {
		return String(this.config.get('gofiPath') || 'gofi').trim() || 'gofi';
	},

	/** Whether free text goes through the intake: on unless `gofiAI.intake` is off. */
	intakeEnabled() {
		return this.config.get('intake') !== false && Boolean(this.projectRoot);
	},

	/**
	 * Plans a request and takes the next step. When the intake cannot run —
	 * no gofi on PATH, a project without an index — the message goes as it
	 * was written: the panel worked without a plan before, and still does.
	 *
	 * @param {string} text
	 * @param {Record<string, string>} [answers]
	 * @param {boolean} [proceed]
	 */
	async planRequest(text, answers, proceed) {
		this.post({ type: 'planning' });
		// The engine starts while the request is planned. Most requests go to
		// it as typed; one that is conducted keeps it for the conversation.
		if (!this.plan) {
			try {
				this.warmEngine();
			} catch {
				// Starting early is an optimisation; the turn starts it anyway.
			}
		}
		let result;
		try {
			result = await this.intakeRunner(this.gofiPath(), this.projectRoot, text, { answers, proceed });
		} catch (err) {
			this.plan = null;
			this.post({ type: 'notice', text: 'Sem plano: a mensagem vai como foi escrita.', hint: err.message });
			this.startTurn(text, []);
			return;
		}
		const answered = Boolean(answers && Object.keys(answers).length > 0);
		const step = nextStep(result, answered);
		this.plan = { text, result, next: 0, answers: answers || {}, stash: null, run: null, retried: false, outcome: '', verdict: '', decisions: {}, asking: [], askingFor: '' };
		// A question about the project goes with the sections that answer it.
		const turn = step === 'raw' && result.turn ? result.turn : text;
		if (step !== 'direct' && turn === text) {
			// A message that is no task goes as typed, with no card for a plan
			// it does not have.
			this.post({ type: 'plan', result, step });
		}
		switch (step) {
			case 'direct':
			case 'raw':
				this.plan = null;
				this.startTurn(turn, []);
				break;
			case 'answerOnly':
				this.plan = null;
				this.running = false;
				this.setRunning(false);
				break;
			case 'answer':
			case 'confirm':
				this.running = false;
				this.setRunning(false);
				break;
			default:
				this.conductNext();
		}
	},

	/**
	 * A button of the plan card: an answer, going on, conducting, sending as
	 * typed, or dropping the plan.
	 *
	 * @param {{type: string, id?: string, value?: string}} message
	 */
	onPlanMessage(message) {
		const plan = this.plan;
		if (!plan) {
			return;
		}
		switch (message.type) {
			case 'planAnswer':
				this.planRequest(plan.text, { ...plan.answers, [String(message.id)]: String(message.value) }, false);
				break;
			case 'planProceed':
				this.planRequest(plan.text, plan.answers, true);
				break;
			case 'planConduct':
			case 'planContinue':
				this.conductNext();
				break;
			case 'planRaw':
				this.plan = null;
				this.startTurn(plan.text, []);
				break;
			case 'planElicitAnswer':
				plan.decisions[plan.askingFor].answers[String(message.id)] = String(message.value);
				plan.asking.shift();
				this.askNextDecision();
				break;
			case 'planElicitProceed':
				// The rest go on their recommendation, written down as assumed.
				plan.asking = [];
				this.askNextDecision();
				break;
			case 'planCancel':
			case 'planStop':
				this.endPlan(message.type === 'planCancel' ? 'Plano descartado.' : 'Plano parado; a conversa continua.');
				break;
			default:
				break;
		}
	},

	/** Runs the plan's next phase as one turn. */
	conductNext() {
		const plan = this.plan;
		const phases = plan.result.plan;
		const i = plan.next;
		const phase = phases[i];
		if (!plan.run && i === 0) {
			plan.run = new RunRecord(this.projectRoot, plan.result);
		}
		plan.before = readVerdict(this.projectRoot, plan.result);
		let { text, model } = phaseTurn(phase);
		plan.model = model;
		if (phase.elicit_file) {
			clearElicitation(this.projectRoot, phase.elicit_file);
		}
		if (!phase.elicit && plan.decisions[phase.role]) {
			const d = plan.decisions[phase.role];
			text = withDecisions(text, d.e, d.answers);
			delete plan.decisions[phase.role];
		}
		this.openPhaseSession(model);
		plan.next = i + 1;
		this.conducting = true;
		this.running = true;
		this.setRunning(true);
		this.post({ type: 'planPhase', index: i, total: phases.length, role: phase.role, tier: phase.tier, raised: Boolean(model) && !phase.elicit, elicit: Boolean(phase.elicit) });
		this.startTurn(text, []);
	},

	/**
	 * Takes the end of a phase's turn: stop on a failure, run the retry a
	 * failed audit carries, wait for review after a PRD or a spec, run the
	 * next phase, or end the plan.
	 *
	 * @param {{isError?: boolean, costUsd?: number, durationMs?: number, error?: string}} event
	 */
	afterPhase(event) {
		this.conducting = false;
		const plan = this.plan;
		const phases = plan.result.plan;
		const done = phases[plan.next - 1];
		let elicited = null;
		let elicitError = null;
		if (done.elicit_file && !event.isError) {
			try {
				elicited = readElicitation(this.projectRoot, done.elicit_file);
			} catch (err) {
				elicitError = err;
			}
		}
		if (plan.run) {
			plan.run.phase({
				role: done.role, tier: done.tier, model: plan.model || undefined, why: done.why, tier_why: done.tier_why,
				seconds: (event.durationMs || 0) / 1000, cost_usd: event.costUsd || 0, ok: !event.isError && !elicitError,
				error: event.isError ? String(event.error || 'erro') : (elicitError ? elicitError.message : undefined),
				asked: elicited ? elicited.questions.length : undefined,
			});
		}
		if (event.isError || elicitError) {
			this.running = false;
			this.setRunning(false);
			plan.outcome = 'failed';
			this.endPlan(elicitError ? `${elicitError.message}: o plano para aqui.` : 'A fase não terminou: o plano para aqui.');
			return;
		}
		if (done.elicit) {
			if (!elicited) {
				this.post({ type: 'notice', text: `/${done.role}: a elicitação não deixou perguntas — o papel segue com o pedido montado.` });
				setImmediate(() => this.conductNext());
				return;
			}
			plan.decisions[done.role] = { e: elicited, answers: {} };
			plan.asking = elicitQuestions(elicited);
			plan.askingFor = done.role;
			this.running = false;
			this.setRunning(false);
			this.askNextDecision();
			return;
		}
		if (elicited && elicited.questions.length > 0) {
			this.running = false;
			this.setRunning(false);
			if (!done.on_block || done.on_block.length === 0) {
				// Stopped again after the spec was completed: the person's to look at.
				plan.outcome = 'stopped';
				this.endPlan(`/${done.role} parou de novo por decisões que a spec não cobre (em ${done.elicit_file}): o plano para aqui.`);
				return;
			}
			// The implementation stopped on decisions the spec does not cover:
			// asked here, recorded in the spec, and the implementation runs again.
			const recorder = done.on_block[0].role;
			plan.decisions[recorder] = { e: elicited, answers: {} };
			phases.splice(plan.next, 0, ...done.on_block);
			plan.asking = elicitQuestions(elicited);
			plan.askingFor = recorder;
			this.post({
				type: 'notice',
				text: recorder === done.role
					? `/${done.role} parou por decisões que o projeto não responde: responda, e ela roda de novo com as respostas.`
					: `/${done.role} parou por decisões que a spec não cobre: /${recorder} as registra na spec, e a implementação roda de novo.`,
			});
			this.askNextDecision();
			return;
		}
		if (done.produces === 'audit') {
			const now = readVerdict(this.projectRoot, plan.result);
			plan.verdict = statusOf(now);
			if (rejected(plan.before, now)) {
				if (done.on_reject && done.on_reject.length > 0) {
					// The retry the plan carries: the implementation one tier up,
					// then the audit again.
					phases.splice(plan.next, 0, ...done.on_reject);
					plan.retried = true;
					const fix = done.on_reject[0];
					this.post({ type: 'notice', text: `O QA reprovou a entrega: /${fix.role} de novo, no nível ${fix.tier}, e a auditoria de novo.` });
					setImmediate(() => this.conductNext());
					return;
				}
				if (plan.retried) {
					this.running = false;
					this.setRunning(false);
					plan.outcome = 'escalated';
					this.endPlan('Reprovado de novo depois da correção: o plano para aqui para você ver os achados.');
					return;
				}
			}
		}
		if (plan.next >= phases.length) {
			this.running = false;
			this.setRunning(false);
			plan.outcome = 'done';
			const next = plan.result.next ? ` Depois, se quiser: /${plan.result.next}.` : '';
			this.endPlan(`Plano concluído.${next}`);
			return;
		}
		if (done.review_after) {
			this.running = false;
			this.setRunning(false);
			this.post({ type: 'planReview', produces: done.produces, next: phases[plan.next].role });
			return;
		}
		setImmediate(() => this.conductNext());
	},

	/**
	 * Offers the next decision an elicitation left, as a card with its
	 * options — answered here, where it costs nothing, before the role runs on
	 * its own tier. With none left, the role runs.
	 */
	askNextDecision() {
		const plan = this.plan;
		if (plan.asking.length === 0) {
			this.conductNext();
			return;
		}
		const d = plan.decisions[plan.askingFor];
		this.post({
			type: 'planDecision', role: plan.askingFor, question: plan.asking[0],
			index: d.e.questions.length - plan.asking.length, total: d.e.questions.length,
		});
	},

	/** Ends the plan, closing its record and giving the chat its own session back. */
	endPlan(message) {
		const plan = this.plan;
		if (plan && plan.run) {
			plan.run.finish(plan.outcome || 'stopped', plan.verdict);
			plan.run = null;
		}
		if (plan && plan.stash) {
			this.closePhaseSession();
			this.session = plan.stash.session;
			this.approvals = plan.stash.approvals;
			this.modelOverride = plan.stash.model;
			plan.stash = null;
			this.postModel();
		}
		this.freshSession = false;
		this.plan = null;
		this.conducting = false;
		this.post({ type: 'planEnded', text: message });
	},

	/**
	 * Gives the next phase a session of its own: a new engine process, not
	 * resuming anything, on the phase's model or the chat's. The phase reads
	 * what the ones before it left on disk — the spec, the context's memory,
	 * the code — never their conversation, nor the chat's. The chat's session
	 * is set aside, untouched, until the plan ends.
	 *
	 * @param {string|null} model
	 */
	openPhaseSession(model) {
		const plan = this.plan;
		if (!plan.stash) {
			plan.stash = { session: this.session, approvals: this.approvals, model: this.modelOverride || null };
		} else {
			this.closePhaseSession();
		}
		this.session = null;
		this.approvals = null;
		this.modelOverride = model || plan.stash.model;
		this.freshSession = true;
		this.postModel();
	},

	/** Ends the running phase's session and its approval channel. */
	closePhaseSession() {
		if (this.session) {
			this.session.dispose();
			this.session = null;
		}
		if (this.approvals) {
			this.approvals.dispose();
			this.approvals = null;
		}
		this.pendingApprovals.clear();
	},

	/**
	 * Drops a plan the chat is leaving — a new conversation, another one
	 * loaded, the panel closing: the session set aside for the chat is
	 * released with it. The phase's own session is the caller's to release.
	 */
	abandonPlan() {
		const plan = this.plan;
		if (plan && plan.stash) {
			if (plan.stash.session) {
				plan.stash.session.dispose();
			}
			if (plan.stash.approvals) {
				plan.stash.approvals.dispose();
			}
			this.modelOverride = plan.stash.model;
			plan.stash = null;
		}
		if (plan && plan.run) {
			plan.run.finish('stopped', plan.verdict);
			plan.run = null;
		}
		this.freshSession = false;
		this.plan = null;
		this.conducting = false;
	},
};

/** Installs the conduct methods on Chat. */
function installConduct(ChatClass) {
	Object.assign(ChatClass.prototype, conduct);
	ChatClass.prototype.intakeRunner = runIntakeKept;
}

module.exports = { installConduct };
