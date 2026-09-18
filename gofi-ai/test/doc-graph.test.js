'use strict';

/**
 * The two views the graph panel draws, pinned against a synthetic corpus.
 *
 * `docGraph.js` is the only place in the extension that decides what the
 * picture omits, and every one of those decisions is a judgement that would
 * regress invisibly: a hairball still renders, it just stops answering
 * anything. So what is pinned here is the omissions — source files absent from
 * the repository view, packages rolled up per directory, ubiquitous tables left
 * unexpanded, the package cap reported rather than swallowed — plus the two
 * shapes of version history the templates produce.
 *
 * Run with `node test/doc-graph.test.js`.
 */

const assert = require('assert');
const fs = require('fs');
const os = require('os');
const path = require('path');

const docGraph = require('../src/docGraph.js');
const { runner } = require('./vscode-stub.js');

const { test, run } = runner();

/**
 * A corpus with three contexts.
 *
 * `pagamento` and `cobranca` both describe `fatura`; all three describe
 * `comum`. Small on purpose — this is the size at which the ubiquity floor
 * applies and the package cap, not the ubiquity rule, is what holds.
 */
function makeProject() {
	const root = fs.mkdtempSync(path.join(os.tmpdir(), 'gofi-graph-'));
	fs.mkdirSync(path.join(root, '.gofi', 'docs'), { recursive: true });
	fs.mkdirSync(path.join(root, '.claude', 'memory', 'contexts'), { recursive: true });

	const nos = {
		'ctx:pagamento': { tipo: 'contexto' },
		'ctx:cobranca': { tipo: 'contexto' },
		'ctx:relatorio': { tipo: 'contexto' },
		'specs/pagamento/sdd-pagamento.md': {
			tipo: 'doc', ctx: 'pagamento', corpus: 'specs', status: 'implementado', versao: '2.1',
		},
		'prd/pagamento/prd-pagamento.md': {
			tipo: 'doc', ctx: 'pagamento', corpus: 'prd', status: '', versao: '1.0',
		},
		'.claude/memory/contexts/pagamento.md': {
			tipo: 'doc', ctx: 'pagamento', corpus: '.claude/memory/contexts', status: 'implementado', versao: '2.1',
		},
		'specs/cobranca/sdd-cobranca.md': {
			tipo: 'doc', ctx: 'cobranca', corpus: 'specs', status: 'aprovado', versao: '1.3',
		},
		'specs/relatorio/sdd-relatorio.md': {
			tipo: 'doc', ctx: 'relatorio', corpus: 'specs', status: 'aprovado', versao: '1.0',
		},
		'ent:fatura': { tipo: 'entidade' },
		'ent:comum': { tipo: 'entidade' },
	};
	const arestas = [
		['specs/pagamento/sdd-pagamento.md', 'ctx:pagamento', 'pertence'],
		['prd/pagamento/prd-pagamento.md', 'ctx:pagamento', 'pertence'],
		['.claude/memory/contexts/pagamento.md', 'ctx:pagamento', 'pertence'],
		['specs/cobranca/sdd-cobranca.md', 'ctx:cobranca', 'pertence'],
		['specs/relatorio/sdd-relatorio.md', 'ctx:relatorio', 'pertence'],

		['.claude/memory/contexts/pagamento.md', 'specs/pagamento/sdd-pagamento.md', 'declara'],
		['.claude/memory/contexts/pagamento.md', 'specs/pagamento/sdd-pagamento.md', 'cita'],
		['specs/pagamento/sdd-pagamento.md', 'specs/cobranca/sdd-cobranca.md', 'cita'],

		['specs/pagamento/sdd-pagamento.md', 'ent:fatura', 'toca'],
		['specs/cobranca/sdd-cobranca.md', 'ent:fatura', 'toca'],
		['specs/pagamento/sdd-pagamento.md', 'ent:comum', 'toca'],
		['specs/cobranca/sdd-cobranca.md', 'ent:comum', 'toca'],
		['specs/relatorio/sdd-relatorio.md', 'ent:comum', 'toca'],
	];

	// `fatura` is implemented in two packages; `comum` in thirty, spread over
	// thirty directories — the shape that makes a ubiquitous table swamp a view.
	for (const file of ['backend/domain/pagamento/repository/fatura.go', 'backend/domain/pagamento/repository/linha.go', 'backend/domain/pagamento/service/cobrar.go']) {
		nos['code:' + file] = { tipo: 'codigo' };
		arestas.push(['ent:fatura', 'code:' + file, 'implementa']);
	}
	for (let i = 0; i < 30; i++) {
		const file = `backend/domain/pacote${i}/repository/comum.go`;
		nos['code:' + file] = { tipo: 'codigo' };
		arestas.push(['ent:comum', 'code:' + file, 'implementa']);
	}
	// One package declares the context outright.
	nos['code:backend/domain/pagamento/application/handler.go'] = { tipo: 'codigo' };
	arestas.push(['code:backend/domain/pagamento/application/handler.go', 'ctx:pagamento', 'pertence']);

	fs.writeFileSync(
		path.join(root, '.gofi', 'docs', 'graph.json'),
		JSON.stringify({ schema: 'gofi-docgraph/v1', nos, arestas }),
	);
	return root;
}

const root = makeProject();

/**
 * A corpus wide enough for ubiquity to mean something.
 *
 * The threshold has a floor, deliberately: in a three-context project every
 * table looks common, and filtering there would leave the view empty. So the
 * behaviour only exists above that floor, and only a corpus this size can pin
 * it.
 */
function makeWideProject() {
	const wide = fs.mkdtempSync(path.join(os.tmpdir(), 'gofi-wide-'));
	fs.mkdirSync(path.join(wide, '.gofi', 'docs'), { recursive: true });

	const nos = { 'ent:comum': { tipo: 'entidade' }, 'ent:fatura': { tipo: 'entidade' } };
	const arestas = [];
	for (let i = 0; i < 24; i++) {
		const name = `ctx${i}`;
		const spec = `specs/${name}/sdd-${name}.md`;
		nos['ctx:' + name] = { tipo: 'contexto' };
		nos[spec] = { tipo: 'doc', ctx: name, corpus: 'specs', status: 'aprovado', versao: '1.0' };
		arestas.push([spec, 'ctx:' + name, 'pertence'], [spec, 'ent:comum', 'toca']);
		// Only the first two also describe `fatura`, which keeps it rare.
		if (i < 2) {
			arestas.push([spec, 'ent:fatura', 'toca']);
		}
	}
	for (let i = 0; i < 30; i++) {
		const file = `backend/domain/pacote${i}/repository/comum.go`;
		nos['code:' + file] = { tipo: 'codigo' };
		arestas.push(['ent:comum', 'code:' + file, 'implementa']);
	}
	for (const file of ['backend/domain/fatura/repository/fatura.go', 'backend/domain/fatura/service/emitir.go']) {
		nos['code:' + file] = { tipo: 'codigo' };
		arestas.push(['ent:fatura', 'code:' + file, 'implementa']);
	}
	fs.writeFileSync(
		path.join(wide, '.gofi', 'docs', 'graph.json'),
		JSON.stringify({ schema: 'gofi-docgraph/v1', nos, arestas }),
	);
	return wide;
}

const wideRoot = makeWideProject();

test('a visão do repositório mostra contextos e documentos, nunca código', () => {
	const view = docGraph.repositoryView(root);
	assert.strictEqual(view.view, 'repositorio');
	assert.ok(
		view.nodes.every((n) => n.kind !== 'codigo' && !n.id.startsWith('code:')),
		'arquivo de código não pertence à visão do projeto',
	);
	assert.strictEqual(view.counts.contexto, 3);
	assert.strictEqual(view.counts.specs, 3);
	assert.strictEqual(view.counts.prd, 1);
	assert.strictEqual(view.counts.memory, 1);
	assert.strictEqual(view.counts.entidade, undefined, 'tabelas só entram quando pedidas');
});

test('as tabelas entram na visão do repositório sob demanda', () => {
	const view = docGraph.repositoryView(root, { entities: true });
	assert.strictEqual(view.counts.entidade, 2);
});

test('arestas paralelas viram um link só, carregando os dois tipos', () => {
	const view = docGraph.repositoryView(root);
	const pair = view.links.filter(
		(l) =>
			[l.source, l.target].includes('.claude/memory/contexts/pagamento.md') &&
			[l.source, l.target].includes('specs/pagamento/sdd-pagamento.md'),
	);
	assert.strictEqual(pair.length, 1, 'declara + cita no mesmo par não podem virar duas molas');
	assert.deepStrictEqual(pair[0].kinds.sort(), ['cita', 'declara']);
});

test('a visão de um contexto agrupa o código por pacote', () => {
	const view = docGraph.contextView(root, 'pagamento');
	const packages = view.nodes.filter((n) => n.kind === 'pacote');
	const repository = packages.find((p) => p.dir === 'backend/domain/pagamento/repository');
	assert.ok(repository, 'os dois arquivos do repository viram um pacote');
	assert.strictEqual(repository.files, 2);
	assert.ok(
		view.nodes.every((n) => !n.id.startsWith('code:')),
		'nenhum arquivo solto sobrevive ao rollup',
	);
});

test('o pacote que declara //gofi:context entra ligado direto ao contexto', () => {
	const view = docGraph.contextView(root, 'pagamento');
	const declared = view.nodes.find((n) => n.kind === 'pacote' && n.declared);
	assert.ok(declared, 'o pacote que se declarou do contexto precisa aparecer');
	assert.strictEqual(declared.dir, 'backend/domain/pagamento/application');
	assert.ok(
		view.links.some((l) => l.source === declared.id && l.target === 'ctx:pagamento'),
		'o elo declarado é com o contexto, não com uma tabela',
	);
});

test('a tabela comum a boa parte do corpus é marcada e não expande pacotes', () => {
	const view = docGraph.contextView(wideRoot, 'ctx0');
	const comum = view.nodes.find((n) => n.id === 'ent:comum');
	const fatura = view.nodes.find((n) => n.id === 'ent:fatura');

	assert.strictEqual(comum.common, true, '24 de 24 contextos é ubíqua');
	assert.strictEqual(comum.contexts, 24);
	assert.strictEqual(fatura.common, false, '2 de 24 é justamente o que carrega sinal');
	assert.strictEqual(fatura.contexts, 2);

	const dirs = view.nodes.filter((n) => n.kind === 'pacote').map((n) => n.dir);
	assert.ok(
		!dirs.some((d) => d.startsWith('backend/domain/pacote')),
		'os trinta pacotes da tabela ubíqua não podem entrar',
	);
	assert.deepStrictEqual(
		dirs.sort(),
		['backend/domain/fatura/repository', 'backend/domain/fatura/service'],
		'sobra o código da tabela que só este contexto e mais um descrevem',
	);
	assert.strictEqual(view.omitted, null, 'nada foi cortado por teto aqui');
});

test('num corpus pequeno o piso do limiar impede filtrar até esvaziar', () => {
	const view = docGraph.contextView(root, 'pagamento');
	const comum = view.nodes.find((n) => n.id === 'ent:comum');
	assert.strictEqual(comum.contexts, 3);
	assert.strictEqual(comum.common, false, 'três contextos ainda não fazem uma tabela ubíqua');
});

test('um documento de outro contexto entra como o contexto, não como o arquivo', () => {
	const view = docGraph.contextView(root, 'pagamento');
	assert.ok(
		!view.nodes.some((n) => n.id === 'specs/cobranca/sdd-cobranca.md'),
		'a spec vizinha não é arrastada inteira para dentro',
	);
	const neighbour = view.nodes.find((n) => n.id === 'ctx:cobranca');
	assert.ok(neighbour, 'o vizinho aparece como contexto clicável');
	assert.strictEqual(neighbour.focus, undefined);
	assert.ok(
		view.links.some(
			(l) => l.source === 'specs/pagamento/sdd-pagamento.md' && l.target === 'ctx:cobranca',
		),
	);
});

test('o corte de pacotes é reportado, não silencioso', () => {
	// `relatorio` reaches all thirty directories through `comum`, which in a
	// corpus this small is not ubiquitous — so the cap is what has to hold.
	const view = docGraph.contextView(root, 'relatorio');
	const packages = view.nodes.filter((n) => n.kind === 'pacote');
	assert.strictEqual(packages.length, 24, 'o teto de pacotes vale');
	assert.deepStrictEqual(view.omitted, { packages: 6 }, 'o que ficou fora tem de ser dito');
});

test('contexto inexistente responde nulo em vez de uma visão vazia', () => {
	assert.strictEqual(docGraph.contextView(root, 'nao-existe'), null);
});

test('projeto sem grafo não é erro, é ausência', () => {
	const bare = fs.mkdtempSync(path.join(os.tmpdir(), 'gofi-bare-'));
	assert.strictEqual(docGraph.hasGraph(bare), false);
	assert.strictEqual(docGraph.repositoryView(bare), null);
	assert.deepStrictEqual(docGraph.listContexts(bare), []);
});

test('o histórico de versões é lido do memory, em lista ou em tabela', () => {
	const file = path.join(root, '.claude', 'memory', 'contexts', 'pagamento.md');
	fs.writeFileSync(
		file,
		[
			'---',
			'versao: "2.1"',
			'---',
			'',
			'## Estado atual',
			'',
			'- isto não é histórico',
			'',
			'## Histórico de versões',
			'',
			'- v2.1 — 2026-08-01 — split do serviço de cobrança',
			'- v1.0 — 2026-05-02 — primeira versão',
			'',
			'## Outra seção',
			'',
			'- nem isto',
			'',
		].join('\n'),
	);
	const list = docGraph.readHistory(root, 'pagamento');
	assert.strictEqual(list.length, 2, 'só as linhas dentro da seção de histórico');
	assert.ok(list[0].text.startsWith('v2.1'));

	fs.writeFileSync(
		file,
		[
			'## Histórico de versões',
			'',
			'| Versão | Data | Mudança |',
			'| --- | --- | --- |',
			'| 1.1 | 2026-06-01 | ajuste de contrato |',
			'| 1.0 | 2026-05-02 | primeira versão |',
			'',
		].join('\n'),
	);
	const table = docGraph.readHistory(root, 'pagamento');
	assert.strictEqual(table.length, 2, 'cabeçalho e separador não são versões');
	assert.strictEqual(table[0].text, '1.1 — 2026-06-01 — ajuste de contrato');

	assert.deepStrictEqual(docGraph.readHistory(root, 'sem-memoria'), []);
});

run();
