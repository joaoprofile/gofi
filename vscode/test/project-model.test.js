'use strict';

// The graph page's three views, over a project written to disk the way
// `gofi index` leaves one: the overview of how each context stands, the graph
// of the whole project — its tree and the links across it — and the trace of
// one context.
const assert = require('assert');
const fs = require('fs');
const os = require('os');
const path = require('path');
const { runner } = require('./vscode-stub.js');
const model = require('../src/projectModel.js');

const { test, run } = runner();

function write(root, rel, value) {
	const file = path.join(root, rel);
	fs.mkdirSync(path.dirname(file), { recursive: true });
	fs.writeFileSync(file, typeof value === 'string' ? value : JSON.stringify(value));
}

/** A project with two contexts, a library, and two trees of code. */
function project() {
	const root = fs.mkdtempSync(path.join(os.tmpdir(), 'gofi-model-'));
	write(root, '.gofi/index/docs/graph.json', {
		schema: 'gofi-docgraph/v1',
		nos: {
			'ctx:user': { tipo: 'contexto' },
			'ctx:billing': { tipo: 'contexto' },
			'ctx:{contexto}': { tipo: 'contexto' },
			'prd/user/prd-user.md': { tipo: 'doc', ctx: 'user', corpus: 'prd', status: 'proposto' },
			'specs/user/spec.md': { tipo: 'doc', ctx: 'user', corpus: 'specs', status: 'aprovado', versao: '1.1' },
			'specs/user/changes/archive/2026-01-01-x/design.md': { tipo: 'doc', ctx: 'user', corpus: 'specs' },
			'specs/billing/proposal.md': { tipo: 'doc', ctx: 'billing', corpus: 'specs' },
			'specs/billing/deltas/billing.md': { tipo: 'doc', ctx: 'billing', corpus: 'specs' },
			'.claude/sdk/go/api/http.md': { tipo: 'doc', corpus: '.claude/sdk' },
			'.claude/expertise/ddd/layers.md': { tipo: 'doc', corpus: '.claude/expertise' },
			'req:user/REQ-01': { tipo: 'requisito', ctx: 'user', status: 'vivo' },
			'req:user/REQ-02': { tipo: 'requisito', ctx: 'user', status: 'proposto' },
			'req:billing/REQ-01': { tipo: 'requisito', ctx: 'billing', status: 'proposto' },
			'cen:user/REQ-01.1': { tipo: 'cenario', ctx: 'user' },
			'cen:user/REQ-01.2': { tipo: 'cenario', ctx: 'user' },
			'ent:"user"': { tipo: 'entidade' },
			'ent:membership': { tipo: 'entidade' },
			'ent:{tabelas}': { tipo: 'entidade' },
			'code:backend/domain/user/service/user.go': { tipo: 'codigo' },
			'code:backend/domain/user/model/entity.go': { tipo: 'codigo' },
			'code:backend/.migrations/0001_user.up.sql': { tipo: 'codigo' },
		},
		arestas: [
			['specs/user/spec.md', 'req:user/REQ-01', 'especifica'],
			['specs/user/spec.md', 'req:user/REQ-02', 'especifica'],
			['req:user/REQ-01', 'cen:user/REQ-01.1', 'especifica'],
			['req:user/REQ-01', 'cen:user/REQ-01.2', 'especifica'],
			['specs/user/spec.md', 'ent:"user"', 'toca'],
			['specs/user/spec.md', 'ent:membership', 'toca'],
			['specs/user/spec.md', 'ent:{tabelas}', 'toca'],
			['specs/user/spec.md', 'prd/user/prd-user.md', 'cita'],
			['specs/billing/deltas/billing.md', 'req:billing/REQ-01', 'adiciona'],
			['specs/billing/proposal.md', 'ctx:user', 'depende'],
			['specs/billing/proposal.md', 'ctx:{contexto}', 'depende'],
			['code:backend/domain/user/service/user.go', 'ctx:user', 'pertence'],
			['code:backend/domain/user/model/entity.go', 'ctx:user', 'pertence'],
			['ent:"user"', 'code:backend/domain/user/model/entity.go', 'implementa'],
			['ent:"user"', 'code:backend/.migrations/0001_user.up.sql', 'implementa'],
		],
	});
	write(root, '.gofi/index/docs/index.json', {
		docs: [
			{ path: 'specs/user/spec.md', ctx: 'user', tipo: 'spec', titulo: 'Spec — user', secs: [
				{ h: 'Propósito', ini: 5, fim: 10 },
				{ h: 'Requisito: REQ-01 — Pedir o código de acesso', ini: 12, fim: 40 },
				{ h: 'Cenário: REQ-01.1 — Celular conhecido', ini: 20, fim: 25 },
				{ h: 'Requisito: REQ-02 — Regras do código', ini: 41, fim: 300 },
			] },
			{ path: 'prd/user/prd-user.md', ctx: 'user', tipo: 'prd', titulo: 'PRD — user', secs: [{ h: 'Problema', ini: 3, fim: 80 }] },
			{ path: 'specs/user/changes/archive/2026-01-01-x/design.md', ctx: 'user', tipo: 'design', secs: [{ h: 'Tabelas', ini: 1, fim: 50 }] },
			{ path: 'specs/billing/proposal.md', ctx: 'billing', tipo: 'mudanca', secs: [{ h: 'Por quê', ini: 1, fim: 30 }] },
			{ path: 'specs/billing/deltas/billing.md', ctx: 'billing', tipo: 'delta', secs: [{ h: 'ADICIONADO', ini: 1, fim: 60 }] },
			{ path: '.claude/sdk/go/api/http.md', ctx: '', tipo: 'sdk', titulo: 'HTTP', secs: [{ h: 'Uso', ini: 1, fim: 120 }] },
			{ path: '.claude/expertise/ddd/layers.md', ctx: '', tipo: 'expertise', secs: [] },
		],
	});
	write(root, '.gofi/index/code/gofi_graph_index.json', {
		scopes: [
			{ name: 'project', dir: '.', root: 'backend', language: 'go' },
			{ name: 'frontend', dir: 'frontend', root: 'frontend', language: 'typescript', framework: 'react' },
		],
	});
	write(root, '.gofi/index/code/gofi_graph.json', {
		language: 'go',
		stats: { packages: 2, types: 3, funcs: 4, methods: 6, files: 3, loc: 700 },
		files: {
			'domain/user/service/user.go': { unit: 'x/domain/user/service', lines: 400 },
			'domain/user/model/entity.go': { unit: 'x/domain/user/model', lines: 200 },
			'domain/billing/service/charge.go': { unit: 'x/domain/billing/service', lines: 100 },
		},
		nodes: [
			{ id: 'func:x.Login', kind: 'func', name: 'Login', file: 'domain/user/service/user.go', line: 30, score: 0.9 },
			{ id: 'func:x.Logout', kind: 'func', name: 'Logout', file: 'domain/user/service/user.go', line: 90, score: 0.2 },
			{ id: 'struct:x.User', kind: 'struct', name: 'User', file: 'domain/user/model/entity.go', line: 8, score: 0.7 },
			{ id: 'pkg:x', kind: 'package', name: 'service', file: 'domain/user/service/user.go' },
		],
		edges: [
			{ from: 'pkg:x', to: 'func:x.Login', rel: 'contains' },
			{ from: 'func:x.Login', to: 'struct:x.User', rel: 'uses' },
			{ from: 'func:x.Logout', to: 'struct:x.User', rel: 'calls' },
			{ from: 'func:x.Login', to: 'func:x.Logout', rel: 'calls' }, // the same file: no link
			{ from: 'pkg:x/domain/billing/service', to: 'pkg:x/domain/user/model', rel: 'imports' },
			{ from: 'pkg:x/domain/user/service', to: 'pkg:x/domain/user/model', rel: 'imports' }, // already tied by its symbols
			{ from: 'pkg:x/domain/billing/service', to: 'pkg:fmt', rel: 'imports' }, // not of the project
		],
	});
	write(root, '.gofi/index/code/frontend/gofi_graph.json', {
		language: 'typescript',
		framework: 'react',
		stats: { packages: 1, files: 1, loc: 40 },
		files: { 'src/App.tsx': { unit: 'frontend/src/App', lines: 40 } },
		nodes: [],
		edges: [],
	});
	return root;
}

const leaves = (node) => (node.children ? node.children.flatMap(leaves) : [node]);
const group = (node, id) => (node.id === id ? node : (node.children || []).map((c) => group(c, id)).find(Boolean));

test('a project never indexed has nothing to draw, and says so by returning nothing', () => {
	const root = fs.mkdtempSync(path.join(os.tmpdir(), 'gofi-model-empty-'));
	assert.strictEqual(model.hasGraph(root), false);
	assert.strictEqual(model.overview(root), null);
	assert.strictEqual(model.graph(root), null);
	assert.strictEqual(model.trace(root, 'user'), null);
});

test('the overview says how each context stands, and leaves the templates\' placeholders out', () => {
	const o = model.overview(project());
	assert.deepStrictEqual(o.contexts.map((c) => c.name), ['billing', 'user']);
	assert.deepStrictEqual(o.totals, { contexts: 2, docs: 7, requirements: 3, live: 1, scenarios: 2, entities: 2, files: 4, lines: 740 });

	const user = o.contexts.find((c) => c.name === 'user');
	assert.strictEqual(user.stage, 'implementado', 'a live requirement and code of its own');
	assert.deepStrictEqual(user.docs, { prd: 1, spec: 1 }, 'what was archived is counted apart');
	assert.strictEqual(user.archived, 1);
	assert.strictEqual(user.requirements, 2);
	assert.strictEqual(user.live, 1);
	assert.strictEqual(user.scenarios, 2);
	assert.deepStrictEqual(user.entities, ['membership', 'user']);
	assert.strictEqual(user.files, 2);
	assert.deepStrictEqual(user.usedBy, ['billing']);

	const billing = o.contexts.find((c) => c.name === 'billing');
	assert.strictEqual(billing.stage, 'especificado');
	assert.deepStrictEqual(billing.docs, { proposal: 1, delta: 1 });
	assert.deepStrictEqual(billing.dependsOn, ['user']);
});

test('the overview measures each tree of code and names the libraries', () => {
	const o = model.overview(project());
	assert.deepStrictEqual(o.trees.map((t) => [t.name, t.language, t.framework, t.files, t.lines]), [
		['backend', 'go', '', 3, 700],
		['frontend', 'typescript', 'react', 1, 40],
	]);
	const backend = o.trees[0];
	assert.strictEqual(backend.funcs, 10, 'functions and methods together');
	assert.deepStrictEqual(backend.top.map((p) => [p.name, p.lines]), [['user/service', 400], ['user/model', 200], ['billing/service', 100]]);
	assert.deepStrictEqual(o.libraries.map((l) => [l.name, l.docs]).sort(), [['Expertise', 1], ['SDK', 1]]);
});

test('the graph holds every document and every source file once, in the tree the project is kept in', () => {
	const root = project();
	const { tree } = model.graph(root);
	assert.deepStrictEqual(tree.children.map((a) => a.name), ['Contextos', 'Bibliotecas', 'backend', 'frontend']);
	const all = leaves(tree);
	assert.strictEqual(all.length, 11, '7 documents and 4 source files');
	assert.strictEqual(new Set(all.map((l) => l.id)).size, all.length, 'nothing is drawn twice');

	const spec = all.find((l) => l.id === 'specs/user/spec.md');
	assert.deepStrictEqual([spec.kind, spec.size, spec.title, spec.ctx], ['spec', 300, 'Spec — user', ['user']]);
	assert.strictEqual(all.find((l) => l.path.includes('/archive/')).archived, true);
	assert.deepStrictEqual(leaves(group(tree, 'ctx:billing')).map((l) => l.kind).sort(), ['delta', 'proposal']);
	assert.deepStrictEqual(leaves(group(tree, 'lib:SDK')).map((l) => l.path), ['.claude/sdk/go/api/http.md']);
	assert.strictEqual(group(tree, 'ctx:{contexto}'), undefined, 'a template\'s placeholder is not a context');

	// Code is a tree of folders, at its path in the project; a folder that only
	// holds another is read as one. A file carries the contexts it belongs to.
	const backend = group(tree, 'scope:backend');
	assert.deepStrictEqual([backend.kind, backend.language, backend.children.map((c) => c.id)], ['scope', 'go', ['dir:backend:backend/domain']]);
	const domain = backend.children[0];
	assert.deepStrictEqual(domain.children.map((c) => [c.name, c.path]), [['billing/service', 'backend/domain/billing/service'], ['user', 'backend/domain/user']]);
	assert.deepStrictEqual(group(tree, 'dir:backend:backend/domain/user').children.map((c) => c.name), ['model', 'service']);
	const file = all.find((l) => l.id === 'code:backend/domain/user/service/user.go');
	assert.deepStrictEqual([file.kind, file.size, file.ctx, file.symbols], ['file', 400, ['user'], 2]);
	assert.deepStrictEqual(all.find((l) => l.id === 'code:backend/domain/billing/service/charge.go').ctx, []);
	// A file at the root of its tree hangs from the tree itself.
	assert.deepStrictEqual(group(tree, 'scope:frontend').children.map((c) => c.id), ['dir:frontend:frontend/src']);
	assert.strictEqual(all.find((l) => l.id === 'code:frontend/src/App.tsx').size, 40);
});

test('the graph\'s links are what the two indexes say, between things of the tree', () => {
	const root = project();
	const { tree, links } = model.graph(root);
	const ids = new Set();
	const walk = (n) => {
		ids.add(n.id);
		(n.children || []).forEach(walk);
	};
	walk(tree);
	assert.ok(links.every((l) => ids.has(l.from) && ids.has(l.to) && l.from !== l.to), 'every link has both ends in the tree');
	const said = links.map((l) => `${l.from} -${l.kind}${l.weight > 1 ? `×${l.weight}` : ''}-> ${l.to}`).sort();
	assert.deepStrictEqual(said, [
		// Two symbols of one file reaching into another are one link, twice as heavy.
		'code:backend/domain/user/service/user.go -code×2-> code:backend/domain/user/model/entity.go',
		'code:backend/domain/user/model/entity.go -pertence-> ctx:user',
		'code:backend/domain/user/service/user.go -pertence-> ctx:user',
		// Two packages that only import each other are tied folder to folder.
		'dir:backend:backend/domain/billing/service -code-> dir:backend:backend/domain/user/model',
		'specs/billing/proposal.md -depende-> ctx:user',
		'specs/user/spec.md -cita-> prd/user/prd-user.md',
	].sort());
});

test('the trace of a context goes from its documents to its requirements, and from its tables to the code', () => {
	const t = model.trace(project(), 'user');
	assert.strictEqual(t.stage, 'implementado');
	assert.strictEqual(t.archived, 1);
	const [docs, reqs, ents, code] = t.columns;
	assert.deepStrictEqual(docs.nodes.map((d) => d.kind), ['prd', 'spec'], 'from the problem to the state, the archive left out');
	assert.deepStrictEqual(reqs.nodes.map((r) => [r.name, r.title, r.status, r.scenarios, r.line]), [
		['REQ-01', 'Pedir o código de acesso', 'vivo', 2, 12],
		['REQ-02', 'Regras do código', 'proposto', 0, 41],
	]);
	assert.deepStrictEqual(ents.nodes.map((e) => e.name), ['membership', 'user']);
	assert.deepStrictEqual(code.nodes.map((p) => [p.path, p.own, p.files.length]), [
		['backend/domain/user/model', true, 1],
		['backend/domain/user/service', true, 1],
		['backend/.migrations', false, 1],
	], 'the context\'s own packages first, then the ones that only implement a table of it');

	// Every line is an edge of the graph between two things that are drawn.
	const ids = new Set(t.columns.flatMap((c) => c.nodes.map((n) => n.id)));
	for (const l of t.links) {
		assert.ok(ids.has(l.from) && ids.has(l.to), `${l.from} → ${l.to} joins what is drawn`);
	}
	const kinds = t.links.map((l) => l.kind).sort();
	assert.deepStrictEqual(kinds, ['especifica', 'especifica', 'implementa', 'implementa', 'toca', 'toca']);
	assert.strictEqual(model.trace(project(), 'nobody'), null);
});

test('the details of a document and of a source file say what they are tied to', () => {
	const root = project();
	const doc = model.details(root, 'specs/user/spec.md');
	assert.strictEqual(doc.title, 'Spec — user');
	assert.deepStrictEqual(doc.facts, ['contexto user', 'aprovado', 'v1.1', '300 linhas', '4 seções']);
	assert.deepStrictEqual(doc.groups.map((g) => [g.title, g.items.length]), [['Requisitos', 2], ['Tabelas', 2], ['Cita', 1]]);
	assert.deepStrictEqual(doc.groups[0].items[0], { id: 'req:user/REQ-01', name: 'REQ-01', sub: 'Pedir o código de acesso', path: 'specs/user/spec.md', line: 12 });
	assert.strictEqual(model.details(root, 'prd/user/prd-user.md').groups[0].title, 'Citado por');

	const file = model.details(root, 'code:backend/domain/user/service/user.go');
	assert.deepStrictEqual(file.facts, ['backend', '400 linhas', '2 símbolos']);
	assert.deepStrictEqual(file.groups.map((g) => g.title), ['Contextos', 'Símbolos']);
	assert.deepStrictEqual(file.groups[1].items.map((s) => [s.name, s.line]), [['Login', 30], ['Logout', 90]], 'the symbols that matter most first');
	assert.strictEqual(model.details(root, 'code:backend/domain/user/model/entity.go').groups[1].title, 'Implementa as tabelas');
	assert.strictEqual(model.details(root, 'nothing'), null);
});

test('a rebuild of the index is read on the next ask', () => {
	const root = project();
	assert.strictEqual(model.overview(root).totals.contexts, 2);
	const file = path.join(root, '.gofi/index/docs/graph.json');
	const graph = JSON.parse(fs.readFileSync(file, 'utf8'));
	graph.nos['ctx:order'] = { tipo: 'contexto' };
	fs.writeFileSync(file, JSON.stringify(graph));
	fs.utimesSync(file, new Date(), new Date(Date.now() + 5000));
	assert.strictEqual(model.overview(root).totals.contexts, 3);
});

run();
