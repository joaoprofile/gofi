'use strict';

const {
	runIntakeKept, phaseTurn, nextStep, deliveryFlow, deliverable, deliverContext,
	readElicitation, clearElicitation, elicitQuestions, withDecisions, sealContract, readGate, answerGate, reviewContext,
} = require('./intake.js');

/** The decisions a delivery's gate waits on, asked as an elicitation's are. */
const GATE = '__gate';
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

	/**
	 * Whether free text goes through the intake: only when `gofiAI.intake` is
	 * on. Off by default — a message goes to the model as typed, and the
	 * person starts a conducted flow by choice.
	 */
	intakeEnabled() {
		return this.config.get('intake') === true && Boolean(this.projectRoot);
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
		this.plan = { text, result, next: 0, answers: answers || {}, stash: null, run: null, outcome: '', verdict: '', decisions: {}, asking: [], askingFor: '' };
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
			case 'blocked':
				this.endPlan(`O plano não pode rodar: ${result.blocked}`);
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
				this.conductNext();
				break;
			case 'planContinue':
				this.continuePastReview();
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
		const flow = deliveryFlow(phase.produces);
		if (flow && !phase.elicit) {
			this.conductDelivery(flow);
			return;
		}
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
	 * Hands what the plan builds to the loop — every sealed change of the
	 * context through the flow — and skips the build and audit phases the flow
	 * holds. Accepted, the plan goes on; otherwise it ends with the outcome.
	 *
	 * @param {string} flow
	 */
	async conductDelivery(flow) {
		const plan = this.plan;
		const phases = plan.result.plan;
		const context = plan.result.context && plan.result.context.name;
		while (plan.next < phases.length && deliverable(phases[plan.next])) {
			plan.next++;
		}
		if (!context) {
			this.endPlan('Sem contexto, nada pode ser entregue: o plano para aqui.');
			return;
		}
		this.running = true;
		this.setRunning(true);
		this.post({ type: 'notice', text: `Entrega: o loop executa as mudanças seladas de ${context} (fluxo ${flow}).` });
		// What each run spent, as its events say; the gate being answered.
		const delivery = { control: {}, spent: {}, total() { return Object.values(this.spent).reduce((a, b) => a + b, 0); } };
		this.delivery = delivery;
		const done = await this.deliveryRunner(this.gofiPath(), this.projectRoot, flow, context, (e) => {
			if (e.run && typeof e.cost_usd === 'number' && e.cost_usd > (delivery.spent[e.run] || 0)) {
				delivery.spent[e.run] = e.cost_usd;
			}
			if (e.ev === 'node.start') {
				this.post({ type: 'notice', text: `● ${e.node}` });
			} else if (e.ev === 'edge') {
				this.post({ type: 'notice', text: `→ ${e.to}  (${e.why})` });
			} else if (e.ev === 'node.failed') {
				this.post({ type: 'notice', text: `${e.node}: ${e.why}` });
			} else if (e.ev === 'gate.open') {
				// Asked here as cards, answered into the run's gate — any
				// terminal could answer it too, with gofi answer.
				const gate = e.run && plan.asking.length === 0 ? this.gateReader(this.projectRoot, e.run) : null;
				if (gate) {
					plan.decisions[GATE] = { e: gate, answers: {}, run: e.run };
					plan.asking = elicitQuestions(gate);
					plan.askingFor = GATE;
					this.post({ type: 'notice', text: `A execução ${e.run} espera ${gate.questions.length === 1 ? 'uma decisão' : `${gate.questions.length} decisões`} que a mudança não cobre.` });
					this.askNextDecision();
				} else {
					this.post({ type: 'notice', text: 'A entrega espera uma decisão que a mudança não cobre.', hint: `responda num terminal — ${e.why}` });
				}
			} else if (e.ev === 'gate.answered') {
				this.post({ type: 'notice', text: 'Decisão respondida: a execução segue.' });
			} else if (e.ev === 'run.cancelled') {
				this.post({ type: 'notice', text: `Execução parada em ${e.node}: continua com gofi run --resume ${e.run || ''}`.trim() });
			} else if (e.ev === 'land.conflict') {
				this.post({ type: 'notice', text: `Conflito ao integrar: ${e.why}`, hint: 'a branch ficou para você' });
			} else if (e.ev === 'dispatch' || e.ev === 'skip' || e.ev === 'plan') {
				this.post({ type: 'notice', text: `${e.ev} ${[e.to, e.why].filter(Boolean).join(' — ')}` });
			} else if (e.ev === 'run.result') {
				this.post({ type: 'notice', text: `${e.node}: ${e.why || 'sem resultado'} · US$ ${(delivery.spent[e.node] || 0).toFixed(2)}` });
			} else if (e.ev === 'escalate') {
				this.post({ type: 'notice', text: `${e.to} sobe de tier: ${e.why}` });
			} else if (e.ev === 'proposal') {
				this.post({ type: 'notice', text: 'A entrega não pode seguir: há uma proposta de emenda para você decidir.', hint: e.to });
			} else if (e.ev === 'land.done') {
				this.post({ type: 'notice', text: `Mudança integrada em ${e.why}.` });
			} else if (e.ev === 'merge') {
				this.post({ type: 'notice', text: `Auditoria em lentes: ${e.why}.` });
			} else if (e.ev === 'guard') {
				this.post({ type: 'notice', text: `${e.to}: ${e.why}` });
			} else if (e.ev === 'lessons') {
				this.post({ type: 'notice', text: `A entrega propôs lições ao knowledge do projeto: ${e.why}.`, hint: 'gofi learn accept|reject' });
			} else if (e.ev === 'work.kept') {
				this.post({ type: 'notice', text: `O trabalho não aceito ficou na branch ${e.to}; a árvore voltou ao início.` });
			}
		}, delivery.control);
		this.delivery = null;
		if (plan.run) {
			plan.run.phase({ role: 'gofi run', tier: '', why: flow, tier_why: '', seconds: 0, cost_usd: delivery.total(), ok: done.ok && done.outcome === 'accepted' });
		}
		plan.verdict = done.outcome;
		if (!done.ok || done.outcome !== 'accepted') {
			this.running = false;
			this.setRunning(false);
			plan.outcome = done.outcome === 'accepted' ? 'failed' : 'escalated';
			this.endPlan(`A entrega não foi aceita (${done.outcome || 'falhou'}): o plano para aqui.${done.output ? `\n${done.output}` : ''}`);
			return;
		}
		if (plan.next >= phases.length) {
			this.running = false;
			this.setRunning(false);
			plan.outcome = 'done';
			this.endPlan('Plano concluído: a mudança foi entregue e arquivada.');
			return;
		}
		this.conductNext();
	},

	/**
	 * Takes the end of a phase's turn: stop on a failure, ask what an
	 * elicitation left, wait for review after a PRD or a spec, run the next
	 * phase, or end the plan.
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
		if (done.elicit && done.elicit_file && !event.isError) {
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
			const context = plan.result.context && plan.result.context.name;
			if (done.produces === 'spec' && context) {
				// The reviewer reads the change before the person approves it,
				// as gofi chat does: what it found comes first, then the stop.
				this.post({ type: 'notice', text: `Revisando as mudanças de ${context} antes da aprovação…` });
				this.reviewRunner(this.gofiPath(), this.projectRoot, context, (line) => this.post({ type: 'notice', text: line })).then((r) => {
					if (r.output) {
						this.post({ type: 'notice', text: r.ok ? 'Revisão da mudança:' : 'A revisão achou bloqueios — corrija, ou aprove como está se o achado não procede:', hint: r.output });
					}
					if (this.plan === plan) {
						this.post({ type: 'planReview', produces: done.produces, next: phases[plan.next].role });
					}
				});
				return;
			}
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
			if (plan.askingFor === GATE) {
				this.answerGateDecisions();
				return;
			}
			this.conductNext();
			return;
		}
		const d = plan.decisions[plan.askingFor];
		this.post({
			type: 'planDecision', role: plan.askingFor, question: plan.asking[0],
			index: d.e.questions.length - plan.asking.length, total: d.e.questions.length,
		});
	},

	/**
	 * Writes what was answered of a delivery's gate into the run: what was
	 * left out takes its recommendation. The delivery goes on by itself.
	 */
	async answerGateDecisions() {
		const plan = this.plan;
		const d = plan.decisions[GATE];
		delete plan.decisions[GATE];
		plan.askingFor = '';
		const res = await this.gateAnswerer(this.gofiPath(), this.projectRoot, d.run, d.answers);
		if (!res.ok) {
			this.post({ type: 'notice', text: `A resposta não foi gravada: ${res.output}`, hint: `gofi answer ${d.run}` });
		}
	},

	/** Stops the delivery under way at its next checkpoint; it goes on with gofi run --resume. */
	stopDelivery() {
		const d = this.delivery;
		if (d && d.control.stop && !d.stopping) {
			d.stopping = true;
			d.control.stop();
			this.post({ type: 'notice', text: 'Parando a entrega no próximo checkpoint — gofi run --resume continua.' });
		}
	},

	/** Ends the plan, closing its record and giving the chat its own session back. */
	/**
	 * Goes on past a review stop. When the phase reviewed wrote the contract,
	 * what the person approved is sealed first; one that does not pass the
	 * check stops the plan, with what to fix.
	 */
	async continuePastReview() {
		const plan = this.plan;
		const reviewed = plan && plan.result.plan[plan.next - 1];
		const context = plan && plan.result.context && plan.result.context.name;
		if (reviewed && reviewed.produces === 'spec' && context) {
			const sealed = await this.sealRunner(this.gofiPath(), this.projectRoot, context);
			if (!sealed.ok) {
				this.endPlan(`O contrato de ${context} não pode ser aprovado como está: o plano para aqui.\n${sealed.output}`);
				return;
			}
			if (sealed.output) {
				this.post({ type: 'notice', text: sealed.output });
			}
		}
		this.conductNext();
	},

	endPlan(message) {
		this.stopDelivery();
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
	ChatClass.prototype.sealRunner = sealContract;
	ChatClass.prototype.deliveryRunner = deliverContext;
	ChatClass.prototype.gateReader = readGate;
	ChatClass.prototype.reviewRunner = reviewContext;
	ChatClass.prototype.gateAnswerer = answerGate;
}

module.exports = { installConduct };
