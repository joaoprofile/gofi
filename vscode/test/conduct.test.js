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
		schema: 'gofi.intake/v1', request: 'altere o pricing', intent: 'change', artifact: 'code',
		plan: [phase('gofi-spec', 'spec', { review_after: true }), phase('gofi-eng', 'code'), phase('gofi-qa', 'audit')],
		questions: [], assumptions: [], ask: false,
	};
}

function makeChat(script) {
	const chat = new Chat(makeContext(), 1);
	// The chat reads its project root off the workspace; the test gives it one.
	Object.defineProperty(chat, 'projectRoot', { value: '/projeto', writable: true, configurable: true });
	Object.defineProperty(chat, 'cwd', { value: '/projeto', writable: true, configurable: true });
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
	return { chat, posted, turns, calls };
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

test('o plano é conduzido: spec, parada para revisão, depois eng e qa sozinhos', async () => {
	const { chat, posted, turns } = makeChat([threePhases()]);
	await chat.send('altere o pricing');
	await tick();
	assert.deepStrictEqual(turns.map((t) => t.prompt), ['/gofi-spec pedido']);
	done(chat);
	assert.ok(posted.some((m) => m.type === 'planReview' && m.next === 'gofi-eng'), 'depois da spec, o painel pede revisão');
	assert.strictEqual(turns.length, 1, 'nada roda antes da revisão');
	chat.onWebviewMessage({ type: 'planContinue' });
	done(chat);
	await tick();
	done(chat);
	assert.deepStrictEqual(turns.map((t) => t.prompt), ['/gofi-spec pedido', '/gofi-eng pedido', '/gofi-qa pedido']);
	assert.strictEqual(chat.plan, null);
	assert.ok(posted.some((m) => m.type === 'planEnded' && /concluído/.test(m.text)));
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
	assert.deepStrictEqual(turns.map((t) => t.prompt), ['/gofi-eng pedido']);
});

test('uma fase de nível subido roda no modelo dela, e o modelo volta no fim', async () => {
	const raised = { ...threePhases(), plan: [phase('gofi-eng', 'code', { tier: 'deep', role_turn: 'Siga o papel', model: 'claude-opus-5-5' })] };
	const { chat, turns } = makeChat([raised]);
	await chat.send('corrija o cancelamento');
	await tick();
	assert.deepStrictEqual(turns[0], { prompt: 'Siga o papel', model: 'claude-opus-5-5' });
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

test('uma auditoria reprovada roda a nova tentativa um nível acima e o run fica registrado', async () => {
	const fs = require('fs');
	const os = require('os');
	const path = require('path');
	const root = fs.mkdtempSync(path.join(os.tmpdir(), 'gofi-conduct-'));
	const mem = path.join(root, 'mem', 'pricing.md');
	fs.mkdirSync(path.dirname(mem), { recursive: true });
	const qa = phase('gofi-qa', 'audit', {
		on_reject: [
			phase('gofi-eng', 'code', { tier: 'deep', turn: '/gofi-eng refaça', role_turn: 'Siga o papel: refaça', model: 'claude-opus-5-5' }),
			phase('gofi-qa', 'audit', { turn: '/gofi-qa de novo' }),
		],
	});
	const plan = {
		schema: 'gofi.intake/v1', request: 'corrija o pricing', intent: 'fix', artifact: 'code', context: { name: 'pricing' },
		verdict_file: 'mem/pricing.md', plan: [phase('gofi-eng', 'code'), qa], questions: [], assumptions: [], ask: false,
	};
	const { chat, turns } = makeChat([plan]);
	chat.projectRoot = root;
	const verdicts = ['reprovado', 'aprovado'];
	const audit = () => fs.writeFileSync(mem, `---\nstatus: ${verdicts.shift()}\n---\n`);
	await chat.planRequest('corrija o pricing');
	done(chat);
	await tick();
	audit();
	done(chat);
	await tick();
	assert.strictEqual(turns[2].prompt, 'Siga o papel: refaça');
	assert.strictEqual(turns[2].model, 'claude-opus-5-5');
	done(chat);
	await tick();
	assert.strictEqual(turns[3].prompt, '/gofi-qa de novo');
	audit();
	done(chat);
	await tick();
	assert.strictEqual(turns.length, 4);
	assert.strictEqual(chat.plan, null);
	const dir = path.join(root, '.gofi', 'runs');
	const files = fs.readdirSync(dir).filter((f) => f.endsWith('.json'));
	assert.strictEqual(files.length, 1);
	const rec = JSON.parse(fs.readFileSync(path.join(dir, files[0]), 'utf8'));
	assert.strictEqual(rec.schema, 'gofi.run/v1');
	assert.strictEqual(rec.surface, 'panel');
	assert.strictEqual(rec.outcome, 'done');
	assert.strictEqual(rec.verdict, 'aprovado');
	assert.strictEqual(rec.phases.length, 4);
	assert.strictEqual(rec.phases[2].model, 'claude-opus-5-5');
	fs.rmSync(root, { recursive: true, force: true });
});

test('cada fase roda numa sessão nova; a da conversa fica guardada e volta no fim', async () => {
	const plan = {
		schema: 'gofi.intake/v1', request: 'corrija o pricing', intent: 'fix', artifact: 'code',
		plan: [phase('gofi-eng', 'code'), phase('gofi-qa', 'audit')], questions: [], assumptions: [], ask: false,
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

test('uma implementação que para por decisão: pergunta, registro na spec e nova rodada', async () => {
	const fs = require('fs');
	const os = require('os');
	const path = require('path');
	const root = fs.mkdtempSync(path.join(os.tmpdir(), 'gofi-block-'));
	const file = '.gofi/elicit/order-gofi-eng.json';
	const eng = phase('gofi-eng', 'code', { turn: '/gofi-eng corrija', elicit_file: file });
	eng.on_block = [
		phase('gofi-spec', '', { tier: 'standard', base_tier: 'deep', turn: 'registre', role_turn: 'registre', model: 'claude-sonnet-5-5' }),
		phase('gofi-eng', 'code', { turn: '/gofi-eng retome', elicit_file: file }),
	];
	const plan = {
		schema: 'gofi.intake/v1', request: 'corrija o cancelamento', intent: 'fix', artifact: 'code', context: { name: 'order' },
		plan: [eng], questions: [], assumptions: [], ask: false,
	};
	const { chat, posted, turns } = makeChat([plan]);
	chat.projectRoot = root;
	const block = () => {
		fs.mkdirSync(path.join(root, '.gofi', 'elicit'), { recursive: true });
		fs.writeFileSync(path.join(root, file), JSON.stringify({
			schema: 'gofi.elicit/v1',
			questions: [{ id: 'idx', text: 'Perfil de acesso?', options: ['append-only', 'hot UPDATE'], recommended: 'hot UPDATE', why: 'atualiza' }],
		}));
	};
	await chat.planRequest('corrija o cancelamento');
	block();
	done(chat);
	await tick();
	assert.strictEqual(posted.filter((m) => m.type === 'planDecision').length, 1);
	chat.onPlanMessage({ type: 'planElicitAnswer', id: 'idx', value: 'append-only' });
	assert.strictEqual(turns[1].prompt.split('\n')[0], 'registre');
	assert.ok(turns[1].prompt.includes('Perfil de acesso? → append-only'));
	assert.strictEqual(turns[1].model, 'claude-sonnet-5-5');
	done(chat);
	await tick();
	assert.strictEqual(turns[2].prompt, '/gofi-eng retome');
	block();
	done(chat);
	await tick();
	assert.strictEqual(chat.plan, null, 'parado de novo, o plano devia acabar');
	assert.ok(posted.some((m) => m.type === 'planEnded' && m.text.includes('parou de novo')));
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

test('com o painel planejando, o hook do gofi não planeja a mesma mensagem de novo', () => {
	const { engineEnv } = require('../src/providers/claudeCode.js');
	assert.strictEqual(engineEnv({ get: () => undefined }).GOFI_CONDUCTING, '1');
	assert.strictEqual(engineEnv({ get: (k) => (k === 'intake' ? false : undefined) }).GOFI_CONDUCTING, process.env.GOFI_CONDUCTING);
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

run();
