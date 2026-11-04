'use strict';

/**
 * The run panel's picture, pinned against a run folder as gofi writes it:
 * the flow laid out in layers with a rework drawn as a cycle, the nodes the
 * run went through and the one it is at marked, the edges it took solid, and
 * the trace matrix with its gaps.
 *
 * Run with `node test/run-view.test.js`.
 */

const assert = require('assert');
const fs = require('fs');
const os = require('os');
const path = require('path');

const { listRuns, readRun, layout, runMarkup } = require('../src/runView.js');

const root = fs.mkdtempSync(path.join(os.tmpdir(), 'run-view-'));
const dir = path.join(root, '.gofi', 'runs', 'r-1');
fs.mkdirSync(dir, { recursive: true });
const flow = {
	schema: 'gofi-flow-view/v1', name: 'delivery', start: 'implement',
	nodes: [
		{ name: 'accepted', kind: 'terminal', outcome: 'accepted' },
		{ name: 'audit', kind: 'agent', stage: 'audit', parallel: true },
		{ name: 'implement', kind: 'agent', stage: 'implement' },
		{ name: 'policy', kind: 'policy' },
		{ name: 'sense', kind: 'sensor', stages: ['build', 'test'] },
	],
	edges: [
		{ from: 'implement', to: 'sense' },
		{ from: 'sense', to: 'implement', when: 'sensor_failed: []', max: 3 },
		{ from: 'sense', to: 'audit' },
		{ from: 'audit', to: 'policy' },
		{ from: 'policy', to: 'accepted', when: 'accept: true' },
	],
};
fs.writeFileSync(path.join(dir, 'flow.json'), JSON.stringify(flow));
fs.writeFileSync(path.join(dir, 'state.json'), JSON.stringify({ run: 'r-1', mudanca: 'criar-pedidos', contexto: 'pedidos', node: 'audit', cost_usd: 1.5,
	sensor: { failed: ['build'], findings: [{ file: 'backend/x.go', line: 3, message: 'undefined: Total' }], output: 'security: não verificado — sem comando no pack nem em loop.sensors\nbuild: go build' },
	verdict: 'rejected', open: [{ severity: 'major', rule: 'pedidos/REQ-01', message: 'sem dono' }], proposal: '.gofi/proposals/pedidos-1.md', branch: 'gofi/criar-pedidos' }));
fs.writeFileSync(path.join(dir, 'events.ndjson'), [
	{ ev: 'node.start', node: 'implement' }, { ev: 'edge', node: 'implement', to: 'sense' },
	{ ev: 'node.start', node: 'sense' }, { ev: 'edge', node: 'sense', to: 'implement' },
	{ ev: 'node.start', node: 'implement' }, { ev: 'edge', node: 'implement', to: 'sense' },
	{ ev: 'node.start', node: 'sense' }, { ev: 'edge', node: 'sense', to: 'audit' },
	{ ev: 'node.start', node: 'audit' },
].map((e) => JSON.stringify(e)).join('\n'));
fs.writeFileSync(path.join(dir, 'trace.json'), JSON.stringify({ rows: [
	{ context: 'pedidos', id: 'REQ-01', title: 'Total', marks: [{ file: 'backend/x.go', line: 3 }] },
	{ context: 'pedidos', id: 'REQ-01.1', title: 'Zero', scenario: true, marks: [] },
] }));

const runs = listRuns(root);
assert.deepStrictEqual(runs.map((r) => r.run), ['r-1']);

const model = readRun(root, 'r-1');
assert.strictEqual(model.visits.implement, 2);
assert.strictEqual(model.taken['sense>implement'], 1);

// The rework goes back up: it never pushes implement below sense.
const { rank } = layout(flow);
assert.ok(rank.implement < rank.sense && rank.sense < rank.audit && rank.audit < rank.policy && rank.policy < rank.accepted, JSON.stringify(rank));

const html = runMarkup(model, 'run.css');
assert.ok(html.includes('class="node agent visited current"'), 'the node the run is at is marked');
assert.ok(html.includes('implement ×2'), 'how many times a node ran is shown');
assert.ok(/class="edge taken back"/.test(html), 'the rework taken is drawn as a cycle');
assert.ok(html.includes('1 de 2 com resposta no código'), 'the matrix counts its gaps');
assert.ok(html.includes('class="scenario gap"'), 'a scenario with no test is marked');
assert.ok(!html.includes('<script'), 'the page runs no script');
assert.ok(html.includes('Falhou em build') && html.includes('backend/x.go:3'), 'what the sensors found is shown');
assert.ok(html.includes('class="warn">security: não verificado'), 'a stage not checked is not shown as passed');
assert.ok(html.includes('Veredicto: <b>rejected</b>') && html.includes('pedidos/REQ-01'), 'the verdict and what is open are shown');
assert.ok(html.includes('.gofi/proposals/pedidos-1.md') && html.includes('gofi/criar-pedidos'), 'the amendment and the branch are shown');

// Only what was added is read: a new event moves the counts on.
fs.appendFileSync(path.join(dir, 'events.ndjson'), '\n' + JSON.stringify({ ev: 'node.start', node: 'policy' }) + '\n');
assert.strictEqual(readRun(root, 'r-1').visits.policy, 1);
assert.strictEqual(readRun(root, 'r-1').visits.implement, 2, 'nothing is counted twice');

// A batch of gofi run --all: what it delivered, how, and what it spent.
const { listBatches } = require('../src/runView.js');
fs.mkdirSync(path.join(root, '.gofi', 'batches'), { recursive: true });
fs.writeFileSync(path.join(root, '.gofi', 'batches', 'b-1.json'), JSON.stringify({ schema: 'gofi-batch/v1', id: 'b-1',
	done: [{ change: 'a', outcome: 'accepted' }, { change: 'b', outcome: 'stopped' }], spent: { 'r-1': 1.5, 'r-2': 0.25 } }));
const [batch] = listBatches(root);
assert.deepStrictEqual(batch.counts, { accepted: 1, stopped: 1 });
assert.strictEqual(batch.spent, 1.75);

fs.rmSync(root, { recursive: true, force: true });
console.log('run-view: ok');
