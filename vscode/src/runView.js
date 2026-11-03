'use strict';

const fs = require('fs');
const path = require('path');

/**
 * What a delivery run is, drawn (gofi decision 0002, §M16): the flow it
 * carries out as a graph — the nodes it went through, how many times, the one
 * it is at — and the trace matrix of what it delivered, each requirement with
 * the code that implements it and each scenario with its test.
 *
 * Everything is read from the run's folder as gofi writes it — flow.json,
 * state.json, events.ndjson, trace.json —; nothing here decides anything. The
 * functions are pure over that folder, so the picture is tested without an
 * editor; the panel only shows it and redraws when the folder changes.
 */

const RUNS = path.join('.gofi', 'runs');

function readJSON(file) {
	try {
		return JSON.parse(fs.readFileSync(file, 'utf8'));
	} catch {
		return null;
	}
}

/**
 * The runs of a project, newest first: the folders with a state.
 *
 * @param {string} root
 * @returns {{run: string, change: string, outcome: string, node: string}[]}
 */
function listRuns(root) {
	let names = [];
	try {
		names = fs.readdirSync(path.join(root, RUNS));
	} catch {
		return [];
	}
	const out = [];
	for (const run of names) {
		const state = readJSON(path.join(root, RUNS, run, 'state.json'));
		if (state && state.run) {
			out.push({ run, change: state.mudanca || '', outcome: state.outcome || '', node: state.node || '' });
		}
	}
	return out.sort((a, b) => (a.run < b.run ? 1 : -1));
}

/**
 * A run as drawn: its flow, its state, how many times each node ran and
 * the edges it took, and its trace.
 *
 * @param {string} root
 * @param {string} run
 */
function readRun(root, run) {
	const dir = path.join(root, RUNS, run);
	const flow = readJSON(path.join(dir, 'flow.json'));
	const state = readJSON(path.join(dir, 'state.json'));
	const trace = readJSON(path.join(dir, 'trace.json'));
	const visits = {};
	const taken = {};
	let text = '';
	try {
		text = fs.readFileSync(path.join(dir, 'events.ndjson'), 'utf8');
	} catch {
		// A run that wrote no event yet is drawn as its flow alone.
	}
	for (const line of text.split('\n')) {
		if (!line.trim()) {
			continue;
		}
		let e;
		try {
			e = JSON.parse(line);
		} catch {
			continue;
		}
		if (e.ev === 'node.start') {
			visits[e.node] = (visits[e.node] || 0) + 1;
		} else if (e.ev === 'edge' && e.to) {
			const key = `${e.node}>${e.to}`;
			taken[key] = (taken[key] || 0) + 1;
		}
	}
	return { run, flow, state, trace, visits, taken };
}

/**
 * Places the flow's nodes in layers: a node is one layer below the deepest
 * node that reaches it forward from the start; an edge back to an earlier
 * layer is a cycle of the flow — a rework — and drawn as one.
 *
 * @param {{start: string, nodes: {name: string}[], edges: {from: string, to: string}[]}} flow
 * @returns {{rank: Record<string, number>, layers: string[][]}}
 */
function layout(flow) {
	const rank = {};
	const out = {};
	for (const e of flow.edges) {
		(out[e.from] = out[e.from] || []).push(e.to);
	}
	// Breadth first from the start: the first time a node is reached is its
	// layer, so a cycle never pushes the nodes it returns to further down.
	rank[flow.start] = 0;
	const queue = [flow.start];
	while (queue.length) {
		const n = queue.shift();
		for (const to of out[n] || []) {
			if (rank[to] === undefined) {
				rank[to] = rank[n] + 1;
				queue.push(to);
			}
		}
	}
	let last = Math.max(0, ...Object.values(rank));
	for (const node of flow.nodes) {
		if (rank[node.name] === undefined) {
			rank[node.name] = ++last;
		}
	}
	const layers = [];
	for (const node of flow.nodes) {
		(layers[rank[node.name]] = layers[rank[node.name]] || []).push(node.name);
	}
	return { rank, layers: layers.filter(Boolean) };
}

function esc(s) {
	return String(s == null ? '' : s).replace(/[&<>"']/g, (c) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' })[c]);
}

const W = 150;
const H = 40;
const GX = 30;
const GY = 46;

/**
 * The flow as SVG: each node a box — the one the run is at outlined, the
 * ones it went through filled, with how many times —, each edge an arrow,
 * the ones taken drawn solid and the rest faint.
 *
 * @param {ReturnType<typeof readRun>} model
 */
function flowSVG(model) {
	const { flow, state, visits, taken } = model;
	const { rank, layers } = layout(flow);
	const width = Math.max(...layers.map((l) => l.length)) * (W + GX) + GX;
	const height = layers.length * (H + GY) + GY;
	const pos = {};
	layers.forEach((names, r) => {
		const offset = (width - names.length * (W + GX) + GX) / 2;
		names.forEach((name, i) => {
			pos[name] = { x: offset + i * (W + GX), y: GY / 2 + r * (H + GY) };
		});
	});
	const kinds = {};
	for (const n of flow.nodes) {
		kinds[n.name] = n;
	}
	const current = state && state.node;
	const outcome = state && state.outcome;
	let edges = '';
	for (const e of flow.edges) {
		const a = pos[e.from];
		const b = pos[e.to];
		if (!a || !b) {
			continue;
		}
		const back = rank[e.to] <= rank[e.from];
		const used = taken[`${e.from}>${e.to}`];
		const x1 = a.x + W / 2;
		const y1 = a.y + H;
		const x2 = b.x + W / 2;
		const y2 = back ? b.y + H : b.y;
		const bend = back ? `C ${x1 + 60} ${y1 + 40}, ${x2 + 60} ${y2 + 40}, ${x2} ${y2}` : `L ${x2} ${y2}`;
		const title = `${e.from} → ${e.to}${e.when ? `: ${e.when}` : ''}${e.max ? ` (até ${e.max})` : ''}${used ? ` — tomada ${used}×` : ''}`;
		edges += `<path class="edge${used ? ' taken' : ''}${back ? ' back' : ''}" d="M ${x1} ${y1} ${bend}" marker-end="url(#arrow)"><title>${esc(title)}</title></path>`;
	}
	let nodes = '';
	for (const name of Object.keys(pos)) {
		const { x, y } = pos[name];
		const n = kinds[name] || {};
		const count = visits[name] || 0;
		const classes = ['node', n.kind || ''];
		if (count) {
			classes.push('visited');
		}
		if (name === current) {
			classes.push('current');
		}
		if (n.kind === 'terminal' && n.outcome && n.outcome === outcome) {
			classes.push('ended');
		}
		const what = n.kind === 'agent' ? (n.stage || '') : n.kind === 'sensor' ? (n.stages || []).join(' · ') : n.kind;
		nodes += `<g class="${classes.join(' ')}"><title>${esc(`${name} — ${n.kind}${what ? `: ${what}` : ''}${count ? ` · ${count}×` : ''}`)}</title>`
			+ `<rect x="${x}" y="${y}" width="${W}" height="${H}" rx="6"/>`
			+ `<text x="${x + W / 2}" y="${y + 17}">${esc(name)}${count > 1 ? ` ×${count}` : ''}</text>`
			+ `<text class="sub" x="${x + W / 2}" y="${y + 32}">${esc(what)}</text></g>`;
	}
	return `<svg viewBox="0 0 ${width} ${height}" width="${width}" height="${height}" role="img" aria-label="Fluxo da execução">`
		+ '<defs><marker id="arrow" viewBox="0 0 10 10" refX="9" refY="5" markerWidth="7" markerHeight="7" orient="auto-start-reverse"><path d="M 0 0 L 10 5 L 0 10 z"/></marker></defs>'
		+ edges + nodes + '</svg>';
}

/**
 * The trace matrix: each requirement with what implements it, each scenario
 * with what tests it, a gap marked.
 *
 * @param {any} trace
 */
function matrixHTML(trace) {
	if (!trace || !Array.isArray(trace.rows)) {
		return '<p class="muted">Sem rastreio ainda: a matriz é escrita quando a execução é aceita (<code>gofi trace &lt;mudança&gt;</code> a mostra antes).</p>';
	}
	let gaps = 0;
	const rows = trace.rows.map((r) => {
		const marks = (r.marks || []).map((m) => `<code>${esc(m.file)}:${esc(m.line)}</code>`).join(' ');
		if (!marks) {
			gaps++;
		}
		return `<tr class="${r.scenario ? 'scenario' : 'requirement'}${marks ? '' : ' gap'}"><td>${esc(r.context)}</td><td>${r.scenario ? '↳ ' : ''}${esc(r.id)} — ${esc(r.title)}</td>`
			+ `<td>${marks || (r.scenario ? 'nenhum teste' : 'nada o implementa')}</td></tr>`;
	}).join('');
	return `<p>${trace.rows.length - gaps} de ${trace.rows.length} com resposta no código.</p>`
		+ `<table class="matrix"><thead><tr><th>Contexto</th><th>Requisito / cenário</th><th>Implementa / testa</th></tr></thead><tbody>${rows}</tbody></table>`;
}

/**
 * The page of a run: what it is, its flow and its matrix.
 *
 * @param {ReturnType<typeof readRun>} model
 * @param {string} css The stylesheet's URI.
 */
function runMarkup(model, css) {
	const s = model.state || {};
	const where = s.node ? `em <b>${esc(s.node)}</b>` : `terminou <b>${esc(s.outcome || '')}</b>`;
	const head = `<header><h1>${esc(s.mudanca || model.run)}</h1>`
		+ `<p><code>${esc(model.run)}</code> · contexto ${esc(s.contexto || '')} · ${where} · US$ ${(s.cost_usd || 0).toFixed(2)}${s.why ? ` · ${esc(s.why)}` : ''}</p></header>`;
	const graph = model.flow ? flowSVG(model) : '<p class="muted">Esta execução não gravou o fluxo (gofi anterior ao flow.json).</p>';
	return `<!DOCTYPE html><html lang="pt-BR"><head><meta charset="UTF-8"><link rel="stylesheet" href="${css}"></head>`
		+ `<body id="runView">${head}<section><h2>Fluxo</h2><div class="flow">${graph}</div></section>`
		+ `<section><h2>Rastreio</h2>${matrixHTML(model.trace)}</section></body></html>`;
}

module.exports = { listRuns, readRun, layout, flowSVG, matrixHTML, runMarkup, RUNS };
