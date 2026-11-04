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
	const { visits, taken } = readEvents(path.join(dir, 'events.ndjson'));
	const report = fs.existsSync(path.join(dir, 'report.md'));
	return { run, flow, state, trace, visits, taken, report };
}

/** What each events file was read up to, and what it counted so far. */
const tails = new Map();

/**
 * Counts a run's node visits and edges taken, reading only what was added
 * to its events since the last read: a run's events only grow. A file that
 * shrank — another run under the same name — is read again from the start.
 *
 * @param {string} file
 */
function readEvents(file) {
	let t = tails.get(file);
	let size = 0;
	try {
		size = fs.statSync(file).size;
	} catch {
		// A run that wrote no event yet is drawn as its flow alone.
		tails.delete(file);
		return { visits: {}, taken: {} };
	}
	if (!t || size < t.offset) {
		t = { offset: 0, rest: '', visits: {}, taken: {} };
		tails.set(file, t);
	}
	if (size > t.offset) {
		const fd = fs.openSync(file, 'r');
		try {
			const buf = Buffer.alloc(size - t.offset);
			fs.readSync(fd, buf, 0, buf.length, t.offset);
			t.offset = size;
			const text = t.rest + buf.toString('utf8');
			const lines = text.split('\n');
			// A line still being written waits for the next read; one that
			// is whole already — an object, the last line of the file — is
			// counted now: a part of an object never parses.
			t.rest = lines.pop();
			for (const line of lines) {
				count(t, line);
			}
			if (t.rest.trim() && isJSON(t.rest)) {
				count(t, t.rest);
				t.rest = '';
			}
		} finally {
			fs.closeSync(fd);
		}
	}
	return { visits: { ...t.visits }, taken: { ...t.taken } };
}

function isJSON(text) {
	try {
		JSON.parse(text);
		return true;
	} catch {
		return false;
	}
}

function count(t, line) {
	if (!line.trim()) {
		return;
	}
	let e;
	try {
		e = JSON.parse(line);
	} catch {
		return;
	}
	if (e.ev === 'node.start') {
		t.visits[e.node] = (t.visits[e.node] || 0) + 1;
	} else if (e.ev === 'edge' && e.to) {
		const key = `${e.node}>${e.to}`;
		t.taken[key] = (t.taken[key] || 0) + 1;
	}
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
		+ detailsHTML(model)
		+ `<section><h2>Rastreio</h2>${matrixHTML(model.trace)}</section></body></html>`;
}

const MAX_SHOWN = 20;

/**
 * What the run knows beyond its flow: the sensors — a stage not checked
 * is not a stage that passed —, the audit, the amendment it proposed, where
 * its work went, and its report.
 *
 * @param {ReturnType<typeof readRun>} model
 */
function detailsHTML(model) {
	const s = model.state || {};
	const out = [];
	const sensor = s.sensor || {};
	const notes = String(sensor.output || '').split('\n').filter((l) => /não verificado|verde na mesma árvore/.test(l));
	if ((sensor.failed || []).length > 0 || notes.length > 0) {
		let h = '<section><h2>Sensores</h2>';
		if ((sensor.failed || []).length > 0) {
			h += `<p class="bad">Falhou em ${esc(sensor.failed.join(', '))}.</p>`;
			const fs_ = sensor.findings || [];
			if (fs_.length > 0) {
				h += '<ul class="findings">' + fs_.slice(0, MAX_SHOWN).map((f) => `<li><code>${esc(f.file || f.rule || '')}${f.line ? `:${f.line}` : ''}</code> ${esc(f.message || '')}</li>`).join('') + '</ul>';
				if (fs_.length > MAX_SHOWN) {
					h += `<p class="muted">… +${fs_.length - MAX_SHOWN}</p>`;
				}
			}
		}
		for (const n of notes) {
			h += `<p class="${/não verificado/.test(n) ? 'warn' : 'muted'}">${esc(n)}</p>`;
		}
		out.push(h + '</section>');
	}
	const open = s.open || [];
	if (s.verdict || open.length > 0 || (s.notes || []).length > 0) {
		let h = `<section><h2>Auditoria</h2><p>Veredicto: <b>${esc(s.verdict || '—')}</b>${(s.lenses || []).length ? ` · lentes ${esc(s.lenses.join(', '))}` : ''}</p>`;
		const list = [...open, ...(s.notes || [])];
		if (list.length > 0) {
			h += '<ul class="findings">' + list.slice(0, MAX_SHOWN).map((f) => `<li><span class="sev ${esc(f.severity)}">${esc(f.severity)}</span> <code>${esc(f.rule || '')}</code> ${esc(f.message || '')}</li>`).join('') + '</ul>';
		}
		if ((s.ambiguous || []).length > 0) {
			h += `<p class="warn">As lentes divergiram sobre o dono de ${esc(s.ambiguous.join(', '))}.</p>`;
		}
		out.push(h + '</section>');
	}
	if (s.proposal) {
		out.push(`<section><h2>Proposta de emenda</h2><p>A execução parou para você decidir: <code>${esc(s.proposal)}</code></p></section>`);
	}
	if (s.branch || s.landing || s.worktree) {
		out.push(`<section><h2>Integração</h2><p>${s.branch ? `branch <code>${esc(s.branch)}</code>` : ''}${s.landing ? ` · ${esc(s.landing)}` : ''}</p></section>`);
	}
	if (model.report) {
		out.push(`<section><h2>Relatório</h2><p><code>gofi report ${esc(model.run)}</code> — em <code>.gofi/runs/${esc(model.run)}/report.md</code></p></section>`);
	}
	return out.join('');
}

/** Where gofi run --all keeps each batch: its state and its summary. */
const BATCHES = path.join('.gofi', 'batches');

/**
 * The batches of a project, newest first: what each delivered and how.
 *
 * @param {string} root
 * @returns {{id: string, summary: string, counts: Record<string, number>, spent: number}[]}
 */
function listBatches(root) {
	let names = [];
	try {
		names = fs.readdirSync(path.join(root, BATCHES)).filter((n) => n.endsWith('.json'));
	} catch {
		return [];
	}
	const out = [];
	for (const name of names) {
		const b = readJSON(path.join(root, BATCHES, name));
		if (!b || b.schema !== 'gofi-batch/v1') {
			continue;
		}
		const counts = {};
		for (const d of b.done || []) {
			counts[d.outcome] = (counts[d.outcome] || 0) + 1;
		}
		const spent = Object.values(b.spent || {}).reduce((a, x) => a + x, 0);
		out.push({ id: b.id, summary: path.join(root, BATCHES, `${b.id}.md`), counts, spent });
	}
	return out.sort((a, b) => (a.id < b.id ? 1 : -1));
}

module.exports = { listRuns, listBatches, readRun, layout, flowSVG, matrixHTML, runMarkup, RUNS, BATCHES };
