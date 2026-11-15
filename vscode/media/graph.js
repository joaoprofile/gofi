// The graph page: the project gofi indexed, in three views.
//
//   Visão geral — how each context stands and how large each tree of code is.
//   Mapa        — every document and every source file, a cell sized by its
//                 lines, grouped the way the project is; a context lights up
//                 everything that is its own, documents and code alike.
//   Rastreio    — one context from its documents to its requirements, and from
//                 its tables to the code that implements them.
//
// Nothing is laid out by a simulation: each view has the layout its question
// has, computed once per size, so nine hundred nodes cost a frame and never
// settle into a hairball. The extension side (projectModel.js) reads the disk
// and hands the views over; this side only draws and asks.
(function () {
	'use strict';

	const vscode = acquireVsCodeApi();
	const $ = (id) => document.getElementById(id);

	/** Whatever the last view was, so a reload of the page comes back to it. */
	const saved = vscode.getState() || {};
	const state = {
		tab: saved.tab || 'overview',
		model: null, // { overview, map }
		trace: null, // the context being traced
		context: saved.context || '',
		zoom: saved.zoom || 'root', // the group the map is zoomed into
		lit: '', // the context whose things the map lights up
		filter: '',
		selected: '',
	};
	const remember = () => vscode.setState({ tab: state.tab, context: state.context, zoom: state.zoom });

	// ── small helpers ────────────────────────────────────────────────────

	/** Builds an element: h('div', {class: 'x', onclick}, child, 'text', …). */
	function h(tag, attrs, ...children) {
		const el = document.createElement(tag);
		for (const [key, value] of Object.entries(attrs || {})) {
			if (value === null || value === undefined || value === false) {
				continue;
			}
			if (key.startsWith('on')) {
				el.addEventListener(key.slice(2), value);
			} else if (key === 'style') {
				Object.assign(el.style, value);
			} else if (key === 'dataset') {
				Object.assign(el.dataset, value);
			} else {
				el.setAttribute(key, value === true ? '' : String(value));
			}
		}
		for (const child of children.flat()) {
			if (child !== null && child !== undefined && child !== false) {
				el.append(child);
			}
		}
		return el;
	}

	const number = (n) => Number(n || 0).toLocaleString('pt-BR');

	/** A count at a glance: 950, 13,4 mil. */
	function compact(n) {
		if (n >= 10000) {
			return `${Math.round(n / 1000)} mil`;
		}
		if (n >= 1000) {
			return `${(n / 1000).toFixed(1).replace('.', ',')} mil`;
		}
		return String(n || 0);
	}

	const plural = (n, one, many) => `${number(n)} ${n === 1 ? one : many}`;

	const KIND_LABEL = {
		prd: 'PRD',
		spec: 'spec',
		design: 'design',
		proposal: 'proposta',
		delta: 'delta',
		memory: 'memória',
		doc: 'documento',
		file: 'código',
		package: 'pacote',
		requirement: 'requisito',
		entity: 'tabela',
	};
	const STAGE_LABEL = { implementado: 'implementado', especificado: 'especificado', descoberta: 'em descoberta', vazio: 'sem documento' };

	// ── the page's frame ─────────────────────────────────────────────────

	const view = $('view');
	const TABS = [
		['overview', 'Visão geral'],
		['map', 'Mapa'],
		['trace', 'Rastreio'],
	];

	function drawTabs() {
		$('tabs').replaceChildren(
			...TABS.map(([key, label]) =>
				h('button', { class: `tab${state.tab === key ? ' on' : ''}`, type: 'button', role: 'tab', 'aria-selected': state.tab === key, onclick: () => show(key) }, label),
			),
		);
	}

	function show(tab, options) {
		state.tab = tab;
		Object.assign(state, options || {});
		remember();
		render();
	}

	function render() {
		drawTabs();
		$('empty').hidden = true;
		view.hidden = false;
		view.className = `view-${state.tab}`;
		if (!state.model) {
			return;
		}
		$('project').textContent = state.model.overview.project;
		const t = state.model.overview.totals;
		$('totals').textContent = `${plural(t.contexts, 'contexto', 'contextos')} · ${plural(t.docs, 'documento', 'documentos')} · ${plural(t.files, 'arquivo', 'arquivos')} de código`;
		$('search').hidden = state.tab === 'overview';
		if (state.tab === 'overview') {
			drawOverview();
		} else if (state.tab === 'map') {
			drawMap();
		} else {
			drawTrace();
		}
	}

	// ── Visão geral ──────────────────────────────────────────────────────

	function drawOverview() {
		const o = state.model.overview;
		const t = o.totals;
		const tile = (value, label, sub) => h('div', { class: 'tile' }, h('div', { class: 'tile-value' }, value), h('div', { class: 'tile-label' }, label), sub && h('div', { class: 'tile-sub' }, sub));

		const tiles = h('section', { class: 'tiles' },
			tile(number(t.contexts), 'contextos'),
			tile(number(t.docs), 'documentos'),
			tile(number(t.requirements), 'requisitos', t.requirements ? `${number(t.live)} vivos · ${Math.round((t.live / t.requirements) * 100)}%` : ''),
			tile(number(t.scenarios), 'cenários'),
			tile(number(t.entities), 'tabelas'),
			tile(number(t.files), 'arquivos de código'),
			tile(compact(t.lines), 'linhas de código'),
		);

		const cards = o.contexts.map((c) => {
			const share = c.requirements ? c.live / c.requirements : 0;
			const docs = Object.entries(c.docs).map(([kind, n]) => h('span', { class: `pill k-${kind}` }, `${KIND_LABEL[kind] || kind} ${n}`));
			const rel = (label, names) => names.length > 0 && h('div', { class: 'card-rel' }, h('span', { class: 'muted' }, label),
				names.map((n) => h('button', { class: 'link', type: 'button', onclick: (e) => { e.stopPropagation(); show('trace', { context: n }); } }, n)));
			return h('article', { class: `card stage-${c.stage}`, tabindex: 0, role: 'button', onclick: () => show('trace', { context: c.name }), onkeydown: (e) => e.key === 'Enter' && show('trace', { context: c.name }) },
				h('header', { class: 'card-head' }, h('h3', {}, c.name), h('span', { class: `stage stage-${c.stage}` }, STAGE_LABEL[c.stage] || c.stage)),
				h('div', { class: 'meter', title: `${c.live} de ${c.requirements} requisitos vivos` }, h('i', { style: { width: `${Math.round(share * 100)}%` } })),
				h('div', { class: 'card-line' }, c.requirements ? `${number(c.live)} de ${plural(c.requirements, 'requisito vivo', 'requisitos vivos')}` : 'sem requisitos ainda', c.scenarios > 0 && h('span', { class: 'muted' }, ` · ${plural(c.scenarios, 'cenário', 'cenários')}`)),
				h('div', { class: 'pills' }, docs, c.archived > 0 && h('span', { class: 'pill ghost' }, `+${c.archived} arquivados`)),
				h('div', { class: 'card-line muted' }, `${plural(c.entities.length, 'tabela', 'tabelas')} · ${plural(c.files, 'arquivo', 'arquivos')} de código`),
				rel('depende de', c.dependsOn),
				rel('usado por', c.usedBy),
			);
		});

		const trees = o.trees.map((s) => {
			const most = Math.max(1, ...s.top.map((p) => p.lines));
			return h('article', { class: 'card tree', tabindex: 0, role: 'button', onclick: () => show('map', { zoom: `scope:${s.name}` }), onkeydown: (e) => e.key === 'Enter' && show('map', { zoom: `scope:${s.name}` }) },
				h('header', { class: 'card-head' }, h('h3', {}, s.name), h('span', { class: 'stage' }, [s.language, s.framework].filter(Boolean).join(' · '))),
				h('div', { class: 'card-line' }, `${plural(s.files, 'arquivo', 'arquivos')} · ${compact(s.lines)} linhas`),
				h('div', { class: 'card-line muted' }, `${plural(s.packages, 'pacote', 'pacotes')} · ${plural(s.types, 'tipo', 'tipos')} · ${plural(s.funcs, 'função', 'funções')}`),
				h('div', { class: 'bars' }, s.top.map((p) => h('div', { class: 'bar', title: `${p.dir} — ${number(p.lines)} linhas em ${plural(p.files, 'arquivo', 'arquivos')}` },
					h('span', { class: 'bar-name' }, p.name), h('span', { class: 'bar-track' }, h('i', { style: { width: `${Math.max(3, Math.round((p.lines / most) * 100))}%` } })), h('span', { class: 'bar-value' }, compact(p.lines))))),
			);
		});

		const libraries = o.libraries.map((l) => h('button', { class: 'chip', type: 'button', onclick: () => show('map', { zoom: `lib:${l.name}` }) }, l.name, h('b', {}, number(l.docs))));

		view.replaceChildren(
			tiles,
			o.contexts.length > 0 && h('section', {}, h('h2', {}, 'Contextos', h('span', { class: 'muted' }, 'do problema ao código — clique para rastrear')), drawOrder(o.contexts), h('div', { class: 'cards' }, cards)),
			trees.length > 0 && h('section', {}, h('h2', {}, 'Código', h('span', { class: 'muted' }, 'cada árvore que o gofi leu — clique para abrir no mapa')), h('div', { class: 'cards' }, trees)),
			libraries.length > 0 && h('section', {}, h('h2', {}, 'Bibliotecas', h('span', { class: 'muted' }, 'o que os agentes consultam')), h('div', { class: 'chips' }, libraries)),
		);
	}

	/** The contexts in the order their dependencies allow: who stands on whom. */
	function drawOrder(contexts) {
		const level = new Map();
		const depth = (name, seen) => {
			if (level.has(name)) {
				return level.get(name);
			}
			if (seen.has(name)) {
				return 0; // a cycle: it is drawn where it is first met
			}
			seen.add(name);
			const c = contexts.find((x) => x.name === name);
			const d = c && c.dependsOn.length > 0 ? 1 + Math.max(...c.dependsOn.map((n) => depth(n, seen))) : 0;
			level.set(name, d);
			return d;
		};
		contexts.forEach((c) => depth(c.name, new Set()));
		const levels = [];
		for (const c of contexts) {
			(levels[level.get(c.name)] = levels[level.get(c.name)] || []).push(c);
		}
		if (levels.length < 2) {
			return null;
		}
		return h('div', { class: 'order', title: 'A ordem que as dependências entre os contextos permitem' },
			levels.flatMap((group, i) => [
				i > 0 && h('span', { class: 'arrow' }, '→'),
				h('span', { class: 'order-level' }, group.map((c) => h('button', { class: `chip stage-${c.stage}`, type: 'button', onclick: () => show('trace', { context: c.name }) }, c.name))),
			]),
		);
	}

	// ── Mapa ─────────────────────────────────────────────────────────────

	/** A node of the map by its id, and the path of groups that leads to it. */
	function find(node, id, trail) {
		const here = [...(trail || []), node];
		if (node.id === id) {
			return here;
		}
		for (const child of node.children || []) {
			const hit = find(child, id, here);
			if (hit) {
				return hit;
			}
		}
		return null;
	}

	function weigh(node) {
		if (!node.children) {
			node.value = Math.max(node.size || 1, 12); // a file of three lines is still a file to see
			node.leaves = 1;
			return;
		}
		node.value = 0;
		node.leaves = 0;
		for (const child of node.children) {
			weigh(child);
			node.value += child.value;
			node.leaves += child.leaves;
		}
		node.children.sort((a, b) => b.value - a.value);
	}

	/**
	 * Squarified treemap: cuts a rectangle into cells whose areas are the
	 * values, each as close to a square as the row it falls in allows.
	 */
	function squarify(items, x, y, w, h0) {
		const out = [];
		const total = items.reduce((sum, it) => sum + it.value, 0);
		if (total <= 0 || w <= 0 || h0 <= 0) {
			return out;
		}
		const scale = (w * h0) / total;
		const rest = items.map((it) => ({ it, area: it.value * scale }));
		let row = [];
		const worst = (cells, side) => {
			const sum = cells.reduce((s, c) => s + c.area, 0);
			const big = Math.max(...cells.map((c) => c.area));
			const small = Math.min(...cells.map((c) => c.area));
			return Math.max((side * side * big) / (sum * sum), (sum * sum) / (side * side * small));
		};
		const place = (cells) => {
			const sum = cells.reduce((s, c) => s + c.area, 0);
			if (w >= h0) {
				const width = sum / h0;
				let cy = y;
				for (const c of cells) {
					const height = c.area / width;
					out.push({ node: c.it, x, y: cy, w: width, h: height });
					cy += height;
				}
				x += width;
				w -= width;
			} else {
				const height = sum / w;
				let cx = x;
				for (const c of cells) {
					const width = c.area / height;
					out.push({ node: c.it, x: cx, y, w: width, h: height });
					cx += width;
				}
				y += height;
				h0 -= height;
			}
		};
		while (rest.length > 0) {
			const side = Math.min(w, h0);
			const next = [...row, rest[0]];
			if (row.length === 0 || worst(next, side) <= worst(row, side)) {
				row = next;
				rest.shift();
			} else {
				place(row);
				row = [];
			}
		}
		if (row.length > 0) {
			place(row);
		}
		return out;
	}

	const HEAD = 18; // a group's name
	const PAD = 2;

	/** Lays a group out, its children inside it, into flat lists of boxes. */
	function layout(node, x, y, w, h0, depth, boxes) {
		if (!node.children) {
			boxes.leaves.push({ node, x, y, w, h: h0 });
			return;
		}
		const titled = depth > 0 && w > 54 && h0 > HEAD + 14;
		if (depth > 0) {
			boxes.groups.push({ node, x, y, w, h: h0, titled, depth });
		}
		const top = titled ? HEAD : 0;
		const pad = depth > 0 ? PAD : 0;
		for (const cell of squarify(node.children, x + pad, y + top + pad, w - 2 * pad, h0 - top - 2 * pad)) {
			layout(cell.node, cell.x, cell.y, cell.w, cell.h, depth + 1, boxes);
		}
	}

	function matches(node, filter) {
		if (!filter) {
			return true;
		}
		return `${node.name} ${node.path || ''} ${node.title || ''}`.toLowerCase().includes(filter);
	}

	function drawMap() {
		const root = state.model.map;
		let trail = find(root, state.zoom) || [root];
		let top = trail[trail.length - 1];
		if (!top.children) {
			trail = trail.slice(0, -1);
			top = trail[trail.length - 1];
		}

		const crumbs = h('nav', { class: 'crumbs', 'aria-label': 'Onde o mapa está' },
			trail.flatMap((n, i) => [i > 0 && h('span', { class: 'muted' }, '›'), h('button', { class: `crumb${i === trail.length - 1 ? ' on' : ''}`, type: 'button', onclick: () => show('map', { zoom: n.id }) }, i === 0 ? 'projeto' : n.name)]),
			h('span', { class: 'muted crumb-facts' }, `${plural(top.leaves, 'arquivo', 'arquivos')} · ${compact(top.value)} linhas`),
		);

		const contexts = state.model.overview.contexts;
		const lights = h('div', { class: 'lights', title: 'Acende tudo o que é de um contexto: seus documentos e o seu código' },
			h('span', { class: 'muted' }, 'acender contexto'),
			contexts.map((c) => h('button', { class: `chip${state.lit === c.name ? ' on' : ''}`, type: 'button', onclick: () => { state.lit = state.lit === c.name ? '' : c.name; drawMap(); } }, c.name)),
		);

		const legend = h('div', { class: 'legend' },
			['prd', 'proposal', 'design', 'delta', 'spec', 'memory', 'doc'].map((k) => h('span', { class: 'key' }, h('i', { class: `k-${k}` }), KIND_LABEL[k])),
			h('span', { class: 'key' }, h('i', { class: 'k-file owned' }), 'código de um contexto'),
			h('span', { class: 'key' }, h('i', { class: 'k-file' }), 'outro código'),
		);
		const stage = h('div', { class: 'map', role: 'img', 'aria-label': 'Mapa do projeto' });
		view.replaceChildren(h('div', { class: 'map-bar' }, crumbs, h('span', { class: 'grow' }), legend), lights, stage);

		const paint = () => {
			const w = stage.clientWidth;
			const h0 = stage.clientHeight;
			if (w < 40 || h0 < 40) {
				return;
			}
			const boxes = { groups: [], leaves: [] };
			layout(top, 0, 0, w, h0, 0, boxes);
			const filter = state.filter;
			const dimming = Boolean(filter || state.lit);
			let shown = 0;
			const frag = document.createDocumentFragment();
			for (const g of boxes.groups) {
				frag.append(h('div', {
					class: `group d${Math.min(g.depth, 3)} g-${g.node.kind}${g.titled ? ' titled' : ''}`,
					style: { left: `${g.x}px`, top: `${g.y}px`, width: `${g.w}px`, height: `${g.h}px` },
				}, g.titled && h('button', { class: 'group-name', type: 'button', title: `${g.node.name} — ${plural(g.node.leaves, 'arquivo', 'arquivos')}, ${number(g.node.value)} linhas · clique para aproximar`, onclick: () => show('map', { zoom: g.node.id }) },
					h('span', {}, g.node.name), g.w > 130 && h('span', { class: 'muted' }, ` ${compact(g.node.value)}`))));
			}
			for (const leaf of boxes.leaves) {
				const n = leaf.node;
				const hit = matches(n, filter) && (!state.lit || (n.ctx || []).includes(state.lit));
				if (hit) {
					shown += 1;
				}
				const owned = n.kind === 'file' && (n.ctx || []).length > 0;
				frag.append(h('button', {
					class: `leaf k-${n.kind}${owned ? ' owned' : ''}${n.archived ? ' archived' : ''}${dimming && !hit ? ' dim' : ''}${state.selected === n.id ? ' sel' : ''}`,
					type: 'button',
					style: { left: `${leaf.x}px`, top: `${leaf.y}px`, width: `${Math.max(leaf.w - 1, 1)}px`, height: `${Math.max(leaf.h - 1, 1)}px` },
					dataset: { id: n.id },
					'aria-label': n.path,
				}, leaf.w > 46 && leaf.h > 15 && h('span', {}, n.name)));
			}
			stage.replaceChildren(frag);
			$('search').placeholder = dimming ? `${number(shown)} de ${number(top.leaves)}` : 'filtrar…';
		};
		paint();
		drawMap.paint = paint;

		// One listener for nine hundred cells: what is under the pointer says who it is.
		const leafAt = (event) => {
			const el = event.target.closest('.leaf');
			return el ? (find(top, el.dataset.id) || []).pop() : null;
		};
		stage.addEventListener('mousemove', (event) => {
			const n = leafAt(event);
			if (!n) {
				tip.hidden = true;
				return;
			}
			tip.replaceChildren(h('b', {}, n.title || n.name), h('div', { class: 'muted' }, n.path), h('div', {}, `${KIND_LABEL[n.kind] || n.kind} · ${number(n.size)} linhas${(n.ctx || []).length ? ` · ${n.ctx.join(', ')}` : ''}${n.archived ? ' · arquivado' : ''}`));
			tip.hidden = false;
			const box = view.getBoundingClientRect();
			tip.style.left = `${Math.min(event.clientX - box.left + 14, box.width - tip.offsetWidth - 8)}px`;
			tip.style.top = `${Math.min(event.clientY - box.top + 16, box.height - tip.offsetHeight - 8)}px`;
		});
		stage.addEventListener('mouseleave', () => { tip.hidden = true; });
		stage.addEventListener('click', (event) => {
			const n = leafAt(event);
			if (n) {
				select(n.id);
			}
		});
		stage.addEventListener('dblclick', (event) => {
			const n = leafAt(event);
			if (n) {
				vscode.postMessage({ type: 'open', path: n.path });
			}
		});
		const tip = h('div', { class: 'tip', hidden: true });
		view.append(tip);
	}

	// ── Rastreio ─────────────────────────────────────────────────────────

	const COLUMN_TITLE = { docs: 'Documentos', requirements: 'Requisitos', entities: 'Tabelas', code: 'Código' };
	const LINK_LABEL = { adiciona: 'adiciona', especifica: 'especifica', modifica: 'modifica', toca: 'toca', implementa: 'implementa' };

	function drawTrace() {
		const contexts = state.model.overview.contexts;
		if (!state.context || !contexts.some((c) => c.name === state.context)) {
			state.context = contexts.length > 0 ? contexts[0].name : '';
		}
		const picker = h('div', { class: 'trace-bar' },
			h('span', { class: 'muted' }, 'contexto'),
			contexts.map((c) => h('button', { class: `chip stage-${c.stage}${state.context === c.name ? ' on' : ''}`, type: 'button', onclick: () => show('trace', { context: c.name }) }, c.name)),
		);
		if (!state.context) {
			view.replaceChildren(picker, h('p', { class: 'muted pad' }, 'Este projeto ainda não tem contextos.'));
			return;
		}
		const t = state.trace;
		if (!t || t.context !== state.context) {
			view.replaceChildren(picker, h('p', { class: 'muted pad' }, 'lendo o contexto…'));
			vscode.postMessage({ type: 'trace', context: state.context });
			return;
		}

		const filter = state.filter;
		const card = (n) => {
			const text = `${n.name} ${n.title || ''} ${n.path || ''}`.toLowerCase();
			return h('button', {
				class: `node k-${n.kind}${n.status ? ` s-${n.status}` : ''}${n.own === false ? ' borrowed' : ''}${filter && !text.includes(filter) ? ' dim' : ''}${state.selected === n.id ? ' sel' : ''}`,
				type: 'button',
				dataset: { id: n.id },
				title: n.path || n.name,
				onclick: () => selectTraced(n),
				ondblclick: () => n.path && n.kind !== 'package' && vscode.postMessage({ type: 'open', path: n.path, line: n.line || 0 }),
			},
				h('span', { class: 'node-name' }, n.name),
				n.title && h('span', { class: 'node-title' }, n.title),
				n.kind === 'requirement' && n.scenarios > 0 && h('span', { class: 'node-count', title: plural(n.scenarios, 'cenário', 'cenários') }, n.scenarios),
				n.kind === 'package' && h('span', { class: 'node-count', title: plural(n.files.length, 'arquivo', 'arquivos') }, n.files.length),
				n.status && n.kind !== 'requirement' && h('span', { class: 'node-status' }, n.status),
			);
		};
		const columns = t.columns.map((col) => h('section', { class: `column c-${col.key}` },
			h('h3', {}, COLUMN_TITLE[col.key], h('span', { class: 'muted' }, number(col.nodes.length))),
			col.nodes.length > 0 ? col.nodes.map(card) : h('p', { class: 'muted none' }, col.key === 'code' ? 'nenhum código ligado ainda' : 'nada ainda'),
		));

		const facts = h('div', { class: 'trace-facts' },
			h('span', { class: `stage stage-${t.stage}` }, STAGE_LABEL[t.stage] || t.stage),
			t.dependsOn.length > 0 && h('span', {}, h('span', { class: 'muted' }, 'depende de '), t.dependsOn.map((n) => h('button', { class: 'link', type: 'button', onclick: () => show('trace', { context: n }) }, n))),
			t.archived > 0 && h('span', { class: 'muted' }, `${plural(t.archived, 'documento arquivado', 'documentos arquivados')} fora do desenho`),
			h('span', { class: 'grow' }),
			h('span', { class: 'muted' }, 'passe o mouse para seguir uma linha · dois cliques abrem o arquivo'),
		);
		const wires = document.createElementNS('http://www.w3.org/2000/svg', 'svg');
		wires.setAttribute('class', 'wires');
		const board = h('div', { class: 'board' }, wires, columns);
		view.replaceChildren(picker, facts, board);

		// The lines are drawn from where the cards ended up, once they are laid out.
		const paint = () => {
			const origin = board.getBoundingClientRect();
			wires.setAttribute('width', board.scrollWidth);
			wires.setAttribute('height', board.scrollHeight);
			const box = new Map();
			for (const el of board.querySelectorAll('.node')) {
				const r = el.getBoundingClientRect();
				box.set(el.dataset.id, { left: r.left - origin.left + board.scrollLeft, right: r.right - origin.left + board.scrollLeft, mid: r.top - origin.top + board.scrollTop + r.height / 2 });
			}
			wires.replaceChildren(...t.links.map((l) => {
				const a = box.get(l.from);
				const b = box.get(l.to);
				if (!a || !b) {
					return null;
				}
				const bend = Math.max(24, (b.left - a.right) / 2);
				const path = document.createElementNS('http://www.w3.org/2000/svg', 'path');
				path.setAttribute('d', `M${a.right},${a.mid} C${a.right + bend},${a.mid} ${b.left - bend},${b.mid} ${b.left},${b.mid}`);
				path.setAttribute('class', `wire w-${l.kind}`);
				path.dataset.from = l.from;
				path.dataset.to = l.to;
				const title = document.createElementNS('http://www.w3.org/2000/svg', 'title');
				title.textContent = LINK_LABEL[l.kind] || l.kind;
				path.append(title);
				return path;
			}).filter(Boolean));
			// The lines were drawn anew: the one being followed stays lit.
			follow(followed);
		};

		// Following a line: what a card reaches, and what reaches it, stays lit.
		let followed = '';
		const follow = (id) => {
			followed = id;
			const lit = new Set();
			if (id) {
				lit.add(id);
				const walk = (from, key, other) => {
					for (const l of t.links) {
						if (l[key] === from && !lit.has(l[other])) {
							lit.add(l[other]);
							walk(l[other], key, other);
						}
					}
				};
				walk(id, 'from', 'to');
				walk(id, 'to', 'from');
			}
			board.classList.toggle('following', lit.size > 1);
			for (const el of board.querySelectorAll('.node')) {
				el.classList.toggle('lit', lit.has(el.dataset.id));
			}
			for (const el of wires.querySelectorAll('.wire')) {
				el.classList.toggle('lit', lit.has(el.dataset.from) && lit.has(el.dataset.to));
			}
		};
		board.addEventListener('mouseover', (event) => {
			const el = event.target.closest('.node');
			follow(el ? el.dataset.id : '');
		});
		board.addEventListener('mouseleave', () => follow(''));
		paint();
		drawTrace.paint = paint;
	}

	// ── what is selected ─────────────────────────────────────────────────

	function select(id) {
		state.selected = id;
		vscode.postMessage({ type: 'details', id });
		for (const el of view.querySelectorAll('.sel')) {
			el.classList.remove('sel');
		}
		for (const el of view.querySelectorAll(`[data-id="${CSS.escape(id)}"]`)) {
			el.classList.add('sel');
		}
	}

	/** A card of the trace: what the page already knows of it is shown at once. */
	function selectTraced(n) {
		if (n.kind === 'requirement') {
			state.selected = n.id;
			showDetails({ id: n.id, kind: 'requirement', name: n.name, title: n.title, path: n.path, line: n.line, facts: [n.status, plural(n.scenarios, 'cenário', 'cenários')].filter(Boolean), groups: [] });
		} else if (n.kind === 'package') {
			state.selected = n.id;
			showDetails({
				id: n.id, kind: 'package', name: n.name, title: n.path, path: '', dir: n.path,
				facts: [plural(n.files.length, 'arquivo', 'arquivos'), n.own ? 'código do contexto' : 'implementa uma tabela do contexto'],
				groups: [{ title: 'Arquivos', items: n.files.map((f) => ({ id: `code:${f}`, name: f.slice(f.lastIndexOf('/') + 1), sub: f, path: f })) }],
			});
		} else if (n.kind === 'entity') {
			state.selected = n.id;
			const t = state.trace;
			const names = (ids, col) => ids.map((id) => t.columns[col].nodes.find((x) => x.id === id)).filter(Boolean);
			showDetails({
				id: n.id, kind: 'entity', name: n.name, title: '', path: '', facts: ['tabela'],
				groups: [
					{ title: 'Tocada por', items: names(t.links.filter((l) => l.to === n.id).map((l) => l.from), 0).map((d) => ({ id: d.id, name: d.title || d.name, sub: d.path, path: d.path })) },
					{ title: 'Implementada em', items: names(t.links.filter((l) => l.from === n.id).map((l) => l.to), 3).map((p) => ({ id: p.id, name: p.name, sub: p.path, dir: p.path })) },
				].filter((g) => g.items.length > 0),
			});
		} else {
			select(n.id);
			return;
		}
		for (const el of view.querySelectorAll('.sel')) {
			el.classList.remove('sel');
		}
		for (const el of view.querySelectorAll(`[data-id="${CSS.escape(n.id)}"]`)) {
			el.classList.add('sel');
		}
	}

	function showDetails(d) {
		const panel = $('details');
		if (!d) {
			panel.hidden = true;
			return;
		}
		const open = (item) => {
			if (item.path) {
				vscode.postMessage({ type: 'open', path: item.path, line: item.line || 0 });
			} else if (item.dir) {
				vscode.postMessage({ type: 'reveal', dir: item.dir });
			} else if (item.id && item.id.startsWith('ctx:')) {
				show('trace', { context: item.id.slice(4) });
			}
		};
		$('detailsKind').textContent = KIND_LABEL[d.kind] || d.kind;
		$('detailsKind').className = `badge k-${d.kind}`;
		$('detailsTitle').textContent = d.title || d.name;
		$('detailsPath').textContent = d.path || d.dir || '';
		$('detailsFacts').replaceChildren(...(d.facts || []).map((f) => h('span', {}, f)));
		$('detailsOpen').hidden = !(d.path || d.dir);
		$('detailsOpen').textContent = d.path ? 'abrir' : 'mostrar no explorador';
		$('detailsOpen').onclick = () => open(d);
		$('detailsBody').replaceChildren(...(d.groups || []).map((g) => h('section', {},
			h('h4', {}, g.title, h('span', { class: 'muted' }, number(g.items.length))),
			h('ul', {}, g.items.map((item) => h('li', {}, h('button', { class: 'item', type: 'button', disabled: !(item.path || item.dir || (item.id || '').startsWith('ctx:')), onclick: () => open(item) },
				h('span', {}, item.name), item.sub && h('small', {}, item.sub)))))),
		));
		panel.hidden = false;
		repaint();
	}

	function repaint() {
		const paint = state.tab === 'map' ? drawMap.paint : state.tab === 'trace' ? drawTrace.paint : null;
		if (paint) {
			requestAnimationFrame(paint);
		}
	}

	// ── the frame's own controls and the extension's messages ────────────

	function showEmpty(message) {
		view.hidden = true;
		$('details').hidden = true;
		$('empty').hidden = false;
		const reasons = {
			'sem-pasta': 'Abra a pasta de um projeto para ver o que o gofi indexou.',
			'sem-grafo': 'Este projeto ainda não foi indexado: não há o que desenhar.',
		};
		$('emptyText').textContent = reasons[message.reason] || 'Nada para mostrar.';
		$('emptyFile').textContent = message.file || '';
		$('buildBtn').hidden = message.reason !== 'sem-grafo';
	}

	window.addEventListener('message', (event) => {
		const message = event.data || {};
		switch (message.type) {
			case 'model':
				state.model = { overview: message.overview, map: message.map };
				weigh(state.model.map);
				state.trace = null;
				if (message.context) {
					state.tab = 'trace';
					state.context = message.context;
				}
				render();
				return;
			case 'trace':
				state.trace = message.data;
				if (state.tab === 'trace') {
					drawTrace();
				}
				return;
			case 'details':
				showDetails(message.data);
				return;
			case 'empty':
				state.model = null;
				drawTabs();
				showEmpty(message);
				return;
			default:
		}
	});

	$('search').addEventListener('input', () => {
		state.filter = $('search').value.trim().toLowerCase();
		if (state.tab === 'map') {
			drawMap.paint();
		} else if (state.tab === 'trace') {
			drawTrace();
		}
	});
	$('refresh').addEventListener('click', () => vscode.postMessage({ type: 'ready' }));
	$('buildBtn').addEventListener('click', () => vscode.postMessage({ type: 'build' }));
	$('closeDetails').addEventListener('click', () => {
		state.selected = '';
		$('details').hidden = true;
		for (const el of view.querySelectorAll('.sel')) {
			el.classList.remove('sel');
		}
		repaint();
	});
	window.addEventListener('keydown', (event) => {
		if (event.key === 'Escape') {
			if (!$('details').hidden) {
				$('closeDetails').click();
			} else if (state.tab === 'map' && state.zoom !== 'root') {
				const trail = find(state.model.map, state.zoom) || [];
				show('map', { zoom: trail.length > 1 ? trail[trail.length - 2].id : 'root' });
			}
		}
	});
	let resizing = 0;
	window.addEventListener('resize', () => {
		cancelAnimationFrame(resizing);
		resizing = requestAnimationFrame(repaint);
	});

	drawTabs();
	vscode.postMessage({ type: 'ready' });
})();
