'use strict';

/**
 * The panel conducts what `gofi intake` plans: free text is planned, its
 * questions answered by button, its phases run one turn each — a raised phase
 * on its tier's model — with a stop for review after a PRD or a spec.
 *
 * The intake and the engine are fakes: `intakeRunner` answers from a script,
 * `startTurn` records the turn, and each phase is ended by feeding the `done`
 * an engine would send. Run with `node test/conduct.test.js`.
 */

const assert = require('assert');
const { installVscodeStub, makeContext, runner } = require('./vscode-stub.js');

installVscodeStub();

const { Chat } = require('../src/chat.js');
const { shouldPlan, intakeArgs, nextStep, phaseTurn } = require('../src/intake.js');
const { test, run } = runner();

function phase(role, produces, extra = {}) {
	return { role, produces, tier: 'standard', base_tier: 'standard', turn: `/${role} pedido`, ...extra };
}

function threePhases() {
	return {
		schema: 'gofi.intake/v1', request: 'altere o pricing', intent: 'change', artifact: 'code', context: { name: 'pricing' },
		plan: [phase('gofi-spec', 'spec', { review_after: true }), phase('gofi-eng', 'code'), phase('gofi-qa', 'audit')],
		questions: [], assumptions: [], ask: false,
	};
}

function makeChat(script) {
	const chat = new Chat(makeContext(), 1);
	// The chat reads its project root off the workspace; the test gives it one.
	Object.defineProperty(chat, 'projectRoot', { value: '/projeto', writable: true, configurable: true });
	Object.defineProperty(chat, 'cwd', { value: '/projeto', writable: true, configurable: true });
	// The conducted flow is opt-in (`gofiAI.intake`): these tests turn it on.
	Object.defineProperty(chat, 'config', { value: { get: (k) => (k === 'intake' ? true : undefined) }, writable: true, configurable: true });
	const posted = [];
	chat.surfaces.add({ postMessage: (m) => posted.push(m) });
	const turns = [];
	chat.startTurn = (prompt) => turns.push({ prompt, model: chat.modelOverride });
	// No real engine in a test: warming records the call instead of spawning.
	chat.warmed = 0;
	chat.warmEngine = () => { chat.warmed++; };
	const calls = [];
	chat.intakeRunner = async (gofi, root, text, opts) => {
		calls.push({ text, opts });
		return script.length > 1 ? script.shift() : script[0];
	};
	// The contract is sealed and the delivery is the loop's — gofi spec
	// approve and gofi run --context — so both are recorded, not run.
	const deliveries = [];
	chat.sealRunner = async () => ({ ok: true, output: '' });
	chat.deliveryRunner = async (gofi, root, flow, context, onEvent) => {
		deliveries.push(`${flow} ${context}`);
		onEvent({ ev: 'node.start', node: 'implement' });
		return { ok: true, outcome: chat.deliveryOutcome || 'accepted', output: '' };
	};
	return { chat, posted, turns, calls, deliveries };
}

const tick = () => new Promise((resolve) => setImmediate(resolve));
const done = (chat, isError = false) => chat.onProviderEvent({ type: 'done', isError, costUsd: 0, durationMs: 1 });

test('shouldPlan: só texto livre, sem anexos, passa pelo plano', () => {
	assert.ok(shouldPlan('altere o pricing', 0));
	assert.ok(!shouldPlan('/gofi-eng faça', 0));
	assert.ok(!shouldPlan('!git status', 0));
	assert.ok(!shouldPlan('veja este arquivo', 1));
	assert.deepStrictEqual(intakeArgs('x', { answers: { context: 'order' }, proceed: true }),
		['intake', 'x', '--json', '--answer', 'context=order', '--proceed']);
	assert.strictEqual(nextStep({ ask: true, plan: [] }, false), 'answer');
	assert.strictEqual(nextStep({ plan: [phase('a', 'b')], assumptions: ['x'] }, false), 'confirm');
	assert.deepStrictEqual(phaseTurn(phase('gofi-eng', 'code', { role_turn: 'Siga', model: 'opus' })), { text: 'Siga', model: 'opus' });
});

test('o plano é conduzido: spec, parada para revisão, depois a entrega é do loop', async () => {
	const { chat, posted, turns, deliveries } = makeChat([threePhases()]);
	await chat.send('altere o pricing');
	await tick();
	assert.deepStrictEqual(turns.map((t) => t.prompt), ['/gofi-spec pedido']);
	done(chat);
	assert.ok(posted.some((m) => m.type === 'planReview' && m.next === 'gofi-eng'), 'depois da spec, o painel pede revisão');
	assert.strictEqual(turns.length, 1, 'nada roda antes da revisão');
	chat.onWebviewMessage({ type: 'planContinue' });
	await tick();
	await tick();
	assert.deepStrictEqual(turns.map((t) => t.prompt), ['/gofi-spec pedido'], 'implementação e auditoria não são turnos do painel');
	assert.deepStrictEqual(deliveries, ['delivery pricing']);
	assert.strictEqual(chat.plan, null);
	assert.ok(posted.some((m) => m.type === 'planEnded' && /entregue/.test(m.text)));
});

test('uma entrega não aceita para o plano com o resultado', async () => {
	const { chat, posted, turns, deliveries } = makeChat([threePhases()]);
	chat.deliveryOutcome = 'impasse';
	await chat.send('altere o pricing');
	await tick();
	done(chat);
	chat.onWebviewMessage({ type: 'planContinue' });
	await tick();
	await tick();
	assert.strictEqual(deliveries.length, 1);
	assert.strictEqual(turns.length, 1);
	assert.ok(posted.some((m) => m.type === 'planEnded' && /impasse/.test(m.text)));
});

test('passada a revisão do contrato, o painel sela o que a pessoa aprovou antes de seguir', async () => {
	const plan = { ...threePhases(), context: { name: 'pricing' } };
	const { chat, posted, turns } = makeChat([plan]);
	const sealed = [];
	chat.sealRunner = async (gofi, root, context) => {
		sealed.push(context);
		return { ok: true, output: 'aprovada: specs/changes/x/proposal.md' };
	};
	await chat.send('altere o pricing');
	await tick();
	done(chat);
	chat.onWebviewMessage({ type: 'planContinue' });
	await tick();
	assert.deepStrictEqual(sealed, ['pricing'], 'o contexto revisado é selado, uma vez');
	assert.deepStrictEqual(turns.map((t) => t.prompt), ['/gofi-spec pedido'], 'a entrega é do loop, não um turno');
	assert.ok(posted.some((m) => m.type === 'notice' && /aprovada/.test(m.text)));
});

test('um contrato que não passa no check não é selado e o plano para', async () => {
	const plan = { ...threePhases(), context: { name: 'pricing' } };
	const { chat, posted, turns } = makeChat([plan]);
	chat.sealRunner = async () => ({ ok: false, output: 'ERROR specs/changes/x/deltas/pricing.md: REQ-07 sem cenário' });
	await chat.send('altere o pricing');
	await tick();
	done(chat);
	chat.onWebviewMessage({ type: 'planContinue' });
	await tick();
	assert.strictEqual(turns.length, 1, 'nada implementa um contrato não aprovado');
	assert.ok(posted.some((m) => m.type === 'planEnded' && /REQ-07 sem cenário/.test(m.text)));
});

test('um plano travado não roda nada e diz o que destrava', async () => {
	const blocked = { ...threePhases(), plan: [], blocked: 'order tem spec no formato 2: migre com `gofi spec migrate order`' };
	const { chat, posted, turns } = makeChat([blocked]);
	await chat.send('corrija o cancelamento');
	await tick();
	assert.strictEqual(turns.length, 0, 'nada roda');
	assert.ok(posted.some((m) => m.type === 'planEnded' && /gofi spec migrate order/.test(m.text)));
});

test('uma pergunta respondida por botão volta ao intake e espera confirmação', async () => {
	const first = { schema: 'gofi.intake/v1', request: 'altere as regras', plan: [], ask: true,
		questions: [{ id: 'context', text: 'Em qual contexto?', options: ['invoice', 'shipping'] }], assumptions: [] };
	const second = { ...threePhases(), plan: [phase('gofi-eng', 'code')] };
	const { chat, turns, calls } = makeChat([first, second]);
	await chat.send('altere as regras');
	await tick();
	assert.strictEqual(turns.length, 0, 'com pergunta, nada roda');
	chat.onWebviewMessage({ type: 'planAnswer', id: 'context', value: 'shipping' });
	await tick();
	assert.deepStrictEqual(calls[1].opts.answers, { context: 'shipping' });
	assert.strictEqual(turns.length, 0, 'plano respondido espera confirmação');
	chat.onWebviewMessage({ type: 'planConduct' });
	await tick();
	assert.deepStrictEqual(turns.map((t) => t.prompt), [], 'confirmado, o que o plano constrói vai ao loop');
});

test('uma fase de nível subido roda no modelo dela, e o modelo volta no fim', async () => {
	const raised = { ...threePhases(), plan: [phase('gofi-doc', 'docs', { tier: 'standard', role_turn: 'Siga o papel', model: 'claude-sonnet-5-5' })] };
	const { chat, turns } = makeChat([raised]);
	await chat.send('documente o cancelamento');
	await tick();
	assert.deepStrictEqual(turns[0], { prompt: 'Siga o papel', model: 'claude-sonnet-5-5' });
	done(chat);
	assert.strictEqual(chat.modelOverride, null, 'o modelo da sessão volta ao fim do plano');
});

test('uma skill invocada direto não passa pelo plano', async () => {
	const { chat, turns, calls } = makeChat([threePhases()]);
	await chat.send('/gofi-eng implemente');
	assert.strictEqual(calls.length, 0);
	assert.strictEqual(turns[0].prompt, '/gofi-eng implemente');
});

test('sem gofi no PATH, a mensagem vai como foi escrita', async () => {
	const { chat, turns, posted } = makeChat([threePhases()]);
	chat.intakeRunner = async () => { throw new Error('spawn gofi ENOENT'); };
	await chat.send('altere o pricing');
	await tick();
	assert.deepStrictEqual(turns.map((t) => t.prompt), ['altere o pricing']);
	assert.ok(posted.some((m) => m.type === 'notice' && /Sem plano/.test(m.text)));
});

test('uma fase que falha para o plano', async () => {
	const { chat, turns } = makeChat([threePhases()]);
	await chat.send('altere o pricing');
	await tick();
	done(chat, true);
	assert.strictEqual(turns.length, 1);
	assert.strictEqual(chat.plan, null);
});

test('cada fase roda numa sessão nova; a da conversa fica guardada e volta no fim', async () => {
	const plan = {
		schema: 'gofi.intake/v1', request: 'corrija o pricing', intent: 'fix', artifact: 'code',
		plan: [phase('gofi-spec', 'spec'), phase('gofi-doc', 'docs')], questions: [], assumptions: [], ask: false,
	};
	const { chat, turns } = makeChat([plan]);
	const disposed = [];
	const fake = (name) => ({ name, dispose: () => disposed.push(name), cancel() {}, send() {} });
	const own = fake('conversa');
	chat.session = own;
	chat.modelOverride = 'claude-sonnet-5-5';
	chat.record.engineSessionId = 'conversa-id';
	// startTurn is the stub: it stands for the engine opening a session.
	const start = chat.startTurn;
	chat.startTurn = (prompt, images) => {
		assert.strictEqual(chat.session, null, 'a fase abriria em cima da sessão da conversa');
		assert.ok(chat.freshSession);
		chat.session = fake(`fase-${turns.length + 1}`);
		start(prompt, images);
	};
	await chat.planRequest('corrija o pricing');
	chat.onProviderEvent({ type: 'meta', sessionId: 'fase-1-id', model: 'x' });
	assert.strictEqual(chat.record.engineSessionId, 'conversa-id', 'o id da fase substituiu o da conversa');
	done(chat);
	await tick();
	assert.deepStrictEqual(disposed, ['fase-1']);
	done(chat);
	await tick();
	assert.strictEqual(turns.length, 2);
	assert.deepStrictEqual(disposed, ['fase-1', 'fase-2']);
	assert.strictEqual(chat.session, own);
	assert.strictEqual(chat.modelOverride, 'claude-sonnet-5-5');
	assert.ok(!chat.freshSession);
});

test('uma conversa nova no meio do plano solta a sessão guardada', async () => {
	const { chat } = makeChat([threePhases()]);
	const disposed = [];
	chat.session = { dispose: () => disposed.push('conversa'), cancel() {}, send() {} };
	await chat.planRequest('altere o pricing');
	chat.newSession();
	assert.ok(disposed.includes('conversa'));
	assert.strictEqual(chat.plan, null);
	assert.ok(!chat.freshSession);
});

test('a elicitação roda antes; as decisões viram cartões e chegam ao papel no fim do turno', async () => {
	const fs = require('fs');
	const os = require('os');
	const path = require('path');
	const root = fs.mkdtempSync(path.join(os.tmpdir(), 'gofi-elicit-'));
	const file = '.gofi/elicit/catalog-gofi-spec.json';
	const plan = {
		schema: 'gofi.intake/v1', request: 'especifique o catálogo', intent: 'create', artifact: 'spec', context: { name: 'catalog' },
		plan: [
			phase('gofi-spec', '', { elicit: true, elicit_file: file, tier: 'standard', base_tier: 'deep', turn: 'elicite', role_turn: 'elicite', model: 'claude-sonnet-5-5' }),
			phase('gofi-spec', 'spec', { tier: 'deep', base_tier: 'deep', turn: '/gofi-spec escreva' }),
		],
		questions: [], assumptions: [], ask: false,
	};
	const { chat, posted, turns } = makeChat([plan]);
	chat.projectRoot = root;
	await chat.planRequest('especifique o catálogo');
	assert.strictEqual(turns[0].prompt, 'elicite');
	assert.strictEqual(turns[0].model, 'claude-sonnet-5-5');
	fs.mkdirSync(path.join(root, '.gofi', 'elicit'), { recursive: true });
	fs.writeFileSync(path.join(root, file), JSON.stringify({
		schema: 'gofi.elicit/v1',
		questions: [
			{ id: 'list', text: 'Listagem paginada?', options: ['simples', 'paginada'], recommended: 'paginada', why: 'sem limite' },
			{ id: 'cache', text: 'Cache?', options: ['não', 'sim'], recommended: 'não', why: 'muda sempre' },
		],
		derived: ['banco: PostgreSQL (.gofi.yaml)'],
	}));
	done(chat);
	await tick();
	const cards = () => posted.filter((m) => m.type === 'planDecision');
	assert.strictEqual(cards().length, 1);
	assert.strictEqual(cards()[0].question.options[0], 'paginada');
	assert.strictEqual(turns.length, 1, 'o papel rodou antes das respostas');
	chat.onPlanMessage({ type: 'planElicitAnswer', id: 'list', value: 'simples' });
	assert.strictEqual(cards().length, 2);
	chat.onPlanMessage({ type: 'planElicitProceed' });
	assert.strictEqual(turns.length, 2);
	assert.ok(turns[1].prompt.startsWith('/gofi-spec escreva'));
	assert.ok(turns[1].prompt.includes('Listagem paginada? → simples'));
	assert.ok(turns[1].prompt.includes('Cache? → não (assumido'));
	assert.ok(turns[1].prompt.includes('banco: PostgreSQL'));
	fs.rmSync(root, { recursive: true, force: true });
});

test('mensagem que não é tarefa vai como foi escrita, sem cartão de plano', async () => {
	const { chat, posted, turns } = makeChat([{ schema: 'gofi.intake/v1', request: 'oi', chat: true, plan: [], questions: [], assumptions: [], ask: false }]);
	await chat.planRequest('oi');
	assert.deepStrictEqual(turns.map((t) => t.prompt), ['oi']);
	assert.ok(!posted.some((m) => m.type === 'plan'), 'um cartão de plano apareceu para um cumprimento');
});

test('o motor sobe enquanto o pedido é planejado', async () => {
	const { chat } = makeChat([threePhases()]);
	await chat.planRequest('altere o pricing');
	assert.strictEqual(chat.warmed, 1);
});

test('o init do motor aquecido chega ao primeiro turno, sem mensagem enviada antes', () => {
	const provider = require('../src/providers/claudeCode.js');
	const p = provider.claudeCodeProvider;
	const session = p.createSession({ cwd: '/tmp', config: { get: () => undefined }, project: null });
	const written = [];
	session.ensureChild = function () {
		this.child = { killed: false, stdin: { write: (l) => written.push(l), end() {} }, kill() {} };
		return true;
	};
	session.warm();
	session.translate(JSON.stringify({ type: 'system', subtype: 'init', session_id: 's-1', model: 'claude-sonnet-5-5' }));
	assert.strictEqual(written.length, 0, 'o aquecimento escreveu no motor');
	const events = [];
	session.send({ prompt: 'oi', allowWrites: true }, (e) => events.push(e));
	assert.strictEqual(events[0].type, 'meta');
	assert.strictEqual(events[0].sessionId, 's-1');
	assert.strictEqual(written.length, 1);
});

test('o intake fica aberto entre mensagens e volta à chamada comum quando falha', async () => {
	const fs = require('fs');
	const os = require('os');
	const path = require('path');
	const { runIntakeKept, stopIntakeServers } = require('../src/intake.js');
	const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'gofi-serve-'));
	const fake = path.join(dir, 'gofi');
	fs.writeFileSync(fake, [
		'#!/bin/sh',
		'if [ "$2" = "--serve" ]; then',
		'  n=0',
		'  while read line; do',
		'    n=$((n+1))',
		'    case "$line" in',
		'      *boom*) echo \'{"error":"boom"}\' ;;',
		'      *) echo "{\\"schema\\":\\"gofi.intake/v1\\",\\"request\\":\\"r$n\\",\\"chat\\":true,\\"plan\\":[]}" ;;',
		'    esac',
		'  done',
		'  exit 0',
		'fi',
		'echo \'{"schema":"gofi.intake/v1","request":"oneshot","plan":[]}\'',
	].join('\n'), { mode: 0o755 });
	const first = await runIntakeKept(fake, dir, 'oi');
	const second = await runIntakeKept(fake, dir, 'oi de novo');
	assert.strictEqual(first.request, 'r1');
	assert.strictEqual(second.request, 'r2', 'o segundo pedido abriu outro processo');
	const failed = await runIntakeKept(fake, dir, 'boom');
	assert.strictEqual(failed.request, 'oneshot', 'o erro do servidor não voltou à chamada comum');

	// A gofi without --serve: noted once, then every request is a plain call.
	const old = path.join(dir, 'gofi-old');
	fs.writeFileSync(old, [
		'#!/bin/sh',
		'if [ "$2" = "--serve" ]; then echo "Error: unknown flag: --serve" >&2; exit 1; fi',
		'echo \'{"schema":"gofi.intake/v1","request":"oneshot","plan":[]}\'',
	].join('\n'), { mode: 0o755 });
	assert.strictEqual((await runIntakeKept(old, dir, 'oi')).request, 'oneshot');
	assert.strictEqual((await runIntakeKept(old, dir, 'oi')).request, 'oneshot');
	stopIntakeServers();
	fs.rmSync(dir, { recursive: true, force: true });
});

test('sem gofiAI.intake ligado, o texto livre vai direto ao modelo', async () => {
	const { chat, turns, calls } = makeChat([threePhases()]);
	Object.defineProperty(chat, 'config', { value: { get: () => undefined }, configurable: true });
	await chat.send('altere o pricing');
	await tick();
	assert.strictEqual(calls.length, 0, 'o intake não roda');
	assert.deepStrictEqual(turns.map((t) => t.prompt), ['altere o pricing']);
});

test('pergunta sobre o projeto vai com os ponteiros, sem cartão de plano', async () => {
	const { chat, posted, turns } = makeChat([{ schema: 'gofi.intake/v1', request: 'explique o pricing', intent: 'explain', plan: [], questions: [], assumptions: [], ask: false, turn: 'explique o pricing\n\nComece por estes trechos: - specs/pricing/sdd-pricing.md L1-9' }]);
	await chat.planRequest('explique o pricing');
	assert.strictEqual(turns.length, 1);
	assert.ok(turns[0].prompt.includes('specs/pricing/sdd-pricing.md'));
	assert.ok(!posted.some((m) => m.type === 'plan'));
});

test('o arquivo em foco continua enquanto está aberto e vai como referência na mensagem', () => {
	const vscode = require('vscode');
	const { chat, posted } = makeChat([threePhases()]);
	const uri = { scheme: 'file', toString: () => 'file:///p/docs/releases/v0.3.4.md' };
	const editor = { document: { uri }, selection: { isEmpty: false, start: { line: 9 }, end: { line: 19 } } };
	vscode.workspace.asRelativePath = () => 'docs/releases/v0.3.4.md';
	vscode.window.activeTextEditor = editor;
	vscode.window.tabGroups.all = [{ tabs: [{ input: { uri } }] }];
	chat.postActiveFile();
	// The chat takes the focus: the editor reports no text editor at all.
	vscode.window.activeTextEditor = undefined;
	chat.postActiveFile();
	const last = posted.filter((m) => m.type === 'activeFile').pop();
	assert.strictEqual(last.file && last.file.path, 'docs/releases/v0.3.4.md', 'o foco sumiu quando o chat ganhou o foco');
	assert.strictEqual(last.use, true);
	assert.strictEqual(chat.focusNote('explique isto'), '\n\n[Arquivo em foco no editor: docs/releases/v0.3.4.md, linhas 10–20 selecionadas]');
	assert.strictEqual(chat.focusNote('!ls'), '', 'um comando de shell levou a referência');
	// Turned off, the chip stays and the message goes without it.
	chat.onWebviewMessage({ type: 'focusToggle' });
	assert.strictEqual(chat.focusNote('explique isto'), '');
	assert.strictEqual(posted.filter((m) => m.type === 'activeFile').pop().use, false);
	chat.onWebviewMessage({ type: 'focusToggle' });
	// Closed, it is gone.
	vscode.window.tabGroups.all = [];
	chat.postActiveFile();
	assert.strictEqual(posted.filter((m) => m.type === 'activeFile').pop().file, null);
	vscode.window.tabGroups.all = [];
	vscode.window.activeTextEditor = undefined;
});

test('a entrega para pelo painel, pergunta o portão em cartões e diz o que cada execução gastou', async () => {
	const { chat, posted } = makeChat([threePhases()]);
	let release;
	const finished = new Promise((resolve) => { release = resolve; });
	let stops = 0;
	const answered = [];
	chat.gateReader = (root, run) => (run === 'r-1' ? { questions: [
		{ id: 'q1', text: 'Paginar?', options: ['sim', 'não'], recommended: 'sim', why: 'padrão' },
		{ id: 'q2', text: 'Ordem?', options: ['data', 'valor'], recommended: 'data' }], derived: [] } : null);
	chat.gateAnswerer = async (gofi, root, run, answers) => { answered.push({ run, answers }); return { ok: true, output: '' }; };
	chat.deliveryRunner = async (gofi, root, flow, context, onEvent, control) => {
		control.stop = () => { stops++; };
		onEvent({ ev: 'node.start', node: 'implement', run: 'r-1', cost_usd: 0.5 });
		onEvent({ ev: 'gate.open', node: 'implement', run: 'r-1', why: 'gofi answer r-1', cost_usd: 0.8 });
		await finished;
		onEvent({ ev: 'run.result', node: 'r-1', why: 'accepted', run: 'r-1', cost_usd: 1.5 });
		return { ok: true, outcome: 'accepted', output: '' };
	};
	await chat.send('altere o pricing');
	await tick();
	done(chat);
	chat.onWebviewMessage({ type: 'planContinue' });
	await tick();
	await tick();
	const card = posted.find((m) => m.type === 'planDecision');
	assert.ok(card && card.question.id === 'q1', 'o portão vira cartão');
	chat.onWebviewMessage({ type: 'planElicitAnswer', id: 'q1', value: 'não' });
	chat.onWebviewMessage({ type: 'planElicitProceed' });
	await tick();
	assert.deepStrictEqual(answered, [{ run: 'r-1', answers: { q1: 'não' } }], 'o que ficou de fora segue a recomendação, como gofi answer');
	chat.stop();
	chat.stop();
	assert.strictEqual(stops, 1, 'parar para a entrega uma vez, no próximo checkpoint');
	release();
	await tick();
	await tick();
	assert.ok(posted.some((m) => m.type === 'notice' && /r-1: accepted · US\$ 1\.50/.test(m.text)), 'o custo da execução aparece');
});

run();
