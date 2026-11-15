// The graph page: the project gofi indexed, in three views.
//
//   Visão geral — how each context stands and how large each tree of code is.
//   Grafo       — the project as a graph beside its tree: contexts, libraries
//                 and each tree of code as groups that open into what they
//                 hold, and the links that cross between them. It is drawn
//                 folded and opened a branch at a time, so it is a picture to
//                 read and never nine hundred dots at once; the nodes are
//                 laid out by a simulation (graphLayout.js), dragged, pinned.
//   Rastreio    — one context from its documents to its requirements, and from
//                 its tables to the code that implements them.
//
// The extension side (projectModel.js) reads the disk and hands the views
// over; this side only draws and asks.
(function () {
	'use strict';

	const vscode = acquireVsCodeApi();
	const $ = (id) => document.getElementById(id);

	/** Whatever the last view was, so a reload of the page comes back to it. */
	const saved = vscode.getState() || {};
	const state = {
		tab: saved.tab === 'map' ? 'graph' : saved.tab || 'overview',
		model: null, // { overview }
		trace: null, // the context being traced
		context: saved.context || '',
		focus: '', // a node the graph is asked to open on
		filter: '',
		selected: '',
	};
	const remember = () => vscode.setState({ tab: state.tab, context: state.context, open: [...graph.open], off: [...graph.off] });

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
		context: 'contexto',
		library: 'biblioteca',
		scope: 'árvore de código',
		folder: 'pasta',
		section: 'seção',
		root: 'projeto',
		package: 'pacote',
		requirement: 'requisito',
		entity: 'tabela',
	};
	const STAGE_LABEL = { implementado: 'implementado', especificado: 'especificado', descoberta: 'em descoberta', vazio: 'sem documento' };

	// ── the page's frame ─────────────────────────────────────────────────

	const view = $('view');
	const TABS = [
		['overview', 'Visão geral'],
		['graph', 'Grafo'],
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
		leaveGraph();
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
		} else if (state.tab === 'graph') {
			drawGraph();
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
			return h('article', { class: 'card tree', tabindex: 0, role: 'button', onclick: () => show('graph', { focus: `scope:${s.name}` }), onkeydown: (e) => e.key === 'Enter' && show('graph', { focus: `scope:${s.name}` }) },
				h('header', { class: 'card-head' }, h('h3', {}, s.name), h('span', { class: 'stage' }, [s.language, s.framework].filter(Boolean).join(' · '))),
				h('div', { class: 'card-line' }, `${plural(s.files, 'arquivo', 'arquivos')} · ${compact(s.lines)} linhas`),
				h('div', { class: 'card-line muted' }, `${plural(s.packages, 'pacote', 'pacotes')} · ${plural(s.types, 'tipo', 'tipos')} · ${plural(s.funcs, 'função', 'funções')}`),
				h('div', { class: 'bars' }, s.top.map((p) => h('div', { class: 'bar', title: `${p.dir} — ${number(p.lines)} linhas em ${plural(p.files, 'arquivo', 'arquivos')}` },
					h('span', { class: 'bar-name' }, p.name), h('span', { class: 'bar-track' }, h('i', { style: { width: `${Math.max(3, Math.round((p.lines / most) * 100))}%` } })), h('span', { class: 'bar-value' }, compact(p.lines))))),
			);
		});

		const libraries = o.libraries.map((l) => h('button', { class: 'chip', type: 'button', onclick: () => show('graph', { focus: `lib:${l.name}` }) }, l.name, h('b', {}, number(l.docs))));

		view.replaceChildren(
			tiles,
			o.contexts.length > 0 && h('section', {}, h('h2', {}, 'Contextos', h('span', { class: 'muted' }, 'do problema ao código — clique para rastrear')), drawOrder(o.contexts), h('div', { class: 'cards' }, cards)),
			trees.length > 0 && h('section', {}, h('h2', {}, 'Código', h('span', { class: 'muted' }, 'cada árvore que o gofi leu — clique para abrir no grafo')), h('div', { class: 'cards' }, trees)),
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

	// ── Grafo ────────────────────────────────────────────────────────────

	const G = window.GofiGraph;

	/** The links by what they say, each family drawn in its colour and hidden on its own. */
	const FAMILIES = [
		['code', 'código', 'Um arquivo cujo código chama ou usa o de outro'],
		['docs', 'documentos', 'Um documento que cita outro ou depende de outro contexto'],
		['bridge', 'contexto ↔ código', 'O código que é de um contexto'],
	];
	const FAMILY_OF = { code: 'code', cita: 'docs', depende: 'docs', pertence: 'bridge' };
	const LINK_SAYS = { code: 'usa', cita: 'cita', depende: 'depende de', pertence: 'pertence a' };

	/** What survives a redraw: what is open, where each node is, how it is looked at. */
	const graph = {
		nodes: new Map(), // the project's tree, by id
		links: [], // between its leaves and groups, as the indexes say
		open: new Set(saved.open || []),
		off: new Set(saved.off || []),
		bodies: new Map(), // where each node on screen is
		shown: [],
		shownSet: new Set(),
		edges: [], // the links between what is on screen
		near: new Map(), // who each node on screen is tied to
		hit: null, // what the filter keeps lit, the groups over it included
		sim: new G.Simulation(),
		cam: { x: 0, y: 0, k: 1 },
		to: null, // where the camera is going
		follow: null,
		autofit: false,
		hover: '',
		drag: null,
		fadeUntil: 0,
		raf: 0,
		view: null, // the page's elements while this view is up
	};
	const still = window.matchMedia && window.matchMedia('(prefers-reduced-motion: reduce)').matches;

	/** The name of the colour a node is drawn in, the same in the tree and in the picture. */
	function colourOf(n) {
		if (n.group) {
			return n.kind === 'root' ? 'section' : n.kind;
		}
		return n.kind === 'file' && (n.ctx || []).length > 0 ? 'owned' : n.kind;
	}

	function matches(node, filter) {
		if (!filter) {
			return true;
		}
		return `${node.name} ${node.path || ''} ${node.title || ''}`.toLowerCase().includes(filter);
	}

	/** Takes a new reading of the project, keeping what was open and where things were. */
	function setGraph(data) {
		graph.nodes = G.index(data.tree);
		graph.links = data.links || [];
		const kept = [...graph.open].filter((id) => graph.nodes.has(id));
		graph.open = kept.length > 0 ? new Set(kept) : G.opening(graph.nodes);
		for (const id of [...graph.bodies.keys()]) {
			if (!graph.nodes.has(id)) {
				graph.bodies.delete(id);
			}
		}
		graph.fresh = graph.bodies.size === 0;
	}

	/** Works out what is on screen and how it is tied, and lets the picture move to it. */
	function arrange(heat) {
		const { nodes, bodies, sim } = graph;
		graph.shown = G.visible(nodes, graph.open);
		graph.shownSet = new Set(graph.shown);
		graph.edges = G.aggregate(nodes, graph.shownSet, graph.links).filter((e) => !graph.off.has(FAMILY_OF[e.kind]));
		for (const id of [...bodies.keys()]) {
			if (!graph.shownSet.has(id)) {
				bodies.delete(id); // folded away: it comes back out of its group
			}
		}
		const now = performance.now();
		const { reach, ring } = G.room(nodes, graph.open, graph.shown);
		for (const id of graph.shown) {
			const n = nodes.get(id);
			const r = G.radius(n, graph.open.has(id));
			let b = bodies.get(id);
			if (!b) {
				const p = bodies.get(n.parent);
				const turn = G.hash(id) * Math.PI * 2;
				const far = p ? p.r + 14 + G.hash(`${id}.`) * 40 : 0;
				b = { id, x: (p ? p.x : 0) + Math.cos(turn) * far, y: (p ? p.y : 0) + Math.sin(turn) * far, vx: 0, vy: 0, r, fx: null, fy: null, born: graph.fresh || still ? 0 : now };
				bodies.set(id, b);
			}
			b.r = r;
			// A group pushes as what it is when folded, and keeps room for its name.
			const full = G.radius(n, false);
			b.q = -(70 + full * full * 0.9);
			b.pad = n.group ? 16 : 7;
		}
		const springs = [];
		const near = new Map(graph.shown.map((id) => [id, new Set()]));
		for (const id of graph.shown) {
			const n = nodes.get(id);
			const p = bodies.get(n.parent);
			if (!p) {
				continue;
			}
			const b = bodies.get(id);
			// Folded, on the ring around its group; open, as far out as its own branch asks.
			springs.push({ source: p, target: b, distance: p.r + (graph.open.has(id) ? 18 + Math.min(300, reach.get(id) * 0.62) : ring.get(n.parent)), strength: n.group ? 0.6 : 0.8 });
			near.get(id).add(n.parent);
			near.get(n.parent).add(id);
		}
		for (const e of graph.edges) {
			// A link only leans two nodes towards each other: the tree keeps the shape.
			springs.push({ source: bodies.get(e.from), target: bodies.get(e.to), distance: 140 + Math.min(260, reach.get(e.from)) + Math.min(260, reach.get(e.to)), strength: Math.min(0.018, 0.005 * (1 + Math.log2(e.weight))) });
			near.get(e.from).add(e.to);
			near.get(e.to).add(e.from);
		}
		graph.near = near;
		sim.set(graph.shown.map((id) => bodies.get(id)), springs);
		sim.aspect = graph.view && graph.view.h > 40 ? graph.view.w / graph.view.h : 1.6;
		sim.warm(heat);
		if (graph.fresh) {
			// The first picture opens already in shape, and settles from there.
			sim.alpha = 1;
			sim.settle(still ? 600 : 150, 120);
			G.level(sim.bodies, sim.aspect);
			sim.settle(still ? 600 : 30, 60);
			graph.fresh = false;
			graph.autofit = true;
			fit(true);
		} else if (still) {
			sim.settle(600);
		}
		graph.fadeUntil = now + 320;
		if (graph.view) {
			graph.view.count.textContent = `${plural(graph.shown.length, 'nó', 'nós')} · ${plural(graph.edges.length, 'ligação', 'ligações')}`;
		}
		wake();
	}

	/** Opens or folds a group. */
	function toggle(id) {
		const n = graph.nodes.get(id);
		if (!n || !n.group) {
			return;
		}
		if (graph.open.has(id)) {
			graph.open.delete(id);
		} else {
			graph.open.add(id);
		}
		remember();
		graph.autofit = false;
		arrange(0.55);
		drawTree();
	}

	/** Opens the groups over a node, so it is on screen. */
	function reveal(id) {
		let changed = false;
		for (const over of G.lineage(graph.nodes, id).slice(1)) {
			if (!graph.open.has(over)) {
				graph.open.add(over);
				changed = true;
			}
		}
		if (changed) {
			remember();
			arrange(0.55);
			drawTree();
		}
	}

	/** Shows a node: on screen, selected, and in the middle of the picture. */
	function focusNode(id) {
		if (!graph.nodes.has(id)) {
			return;
		}
		const hidden = !graph.shownSet.has(id);
		reveal(id);
		selectNode(id);
		// The camera goes to it only when it is not already there to see.
		const v = graph.view;
		const b = graph.bodies.get(id);
		const sx = (b.x - graph.cam.x) * graph.cam.k + v.w / 2;
		const sy = (b.y - graph.cam.y) * graph.cam.k + v.h / 2;
		if (hidden || graph.cam.k < 0.4 || sx < 50 || sy < 50 || sx > v.w - 50 || sy > v.h - 70) {
			graph.autofit = false;
			graph.follow = { id, until: performance.now() + 900 };
		}
		wake();
	}

	function selectNode(id) {
		const n = graph.nodes.get(id);
		if (!n) {
			return;
		}
		if (!n.group) {
			select(id);
		} else {
			state.selected = id;
			showDetails(groupDetails(n));
		}
		markTree();
		wake();
	}

	function deselect() {
		state.selected = '';
		$('details').hidden = true;
		markTree();
		wake();
	}

	/** What is known of a group without asking: what it holds and what it is tied to. */
	function groupDetails(n) {
		const node = (id) => graph.nodes.get(id);
		const item = (o, sub) => ({ id: o.id, name: o.title || o.name, sub, node: o.id });
		const inside = n.children.map(node).sort((a, b) => b.value - a.value).slice(0, 40)
			.map((c) => item(c, c.group ? `${plural(c.leaves, 'arquivo', 'arquivos')} · ${compact(c.value)} linhas` : `${KIND_LABEL[c.kind] || c.kind} · ${number(c.value)} linhas`));
		const tied = (key, other) => graph.edges.filter((e) => e[key] === n.id).sort((a, b) => b.weight - a.weight).slice(0, 30)
			.map((e) => item(node(e[other]), `${LINK_SAYS[e.kind]}${e.weight > 1 ? ` · ${number(e.weight)}×` : ''}`));
		return {
			id: n.id,
			kind: n.kind,
			name: n.name,
			title: n.name,
			path: '',
			dir: n.kind === 'folder' || n.kind === 'scope' ? n.path : '',
			context: n.kind === 'context' ? n.name : '',
			facts: [plural(n.leaves, n.kind === 'context' || n.kind === 'library' || n.kind === 'section' ? 'documento' : 'arquivo', n.kind === 'context' || n.kind === 'library' || n.kind === 'section' ? 'documentos' : 'arquivos'), `${compact(n.value)} linhas`, n.language].filter(Boolean),
			groups: [
				{ title: 'Dentro', items: inside },
				{ title: 'Aponta para', items: tied('from', 'to') },
				{ title: 'Apontado por', items: tied('to', 'from') },
			].filter((g) => g.items.length > 0),
		};
	}

	/** Adds to what the extension knows of a file the files it is tied to. */
	function withLinks(d) {
		if (!d || state.tab !== 'graph' || !graph.nodes.has(d.id)) {
			return d;
		}
		const tied = (key, other) => graph.links.filter((l) => l.kind === 'code' && l[key] === d.id).sort((a, b) => b.weight - a.weight).slice(0, 30)
			.map((l) => graph.nodes.get(l[other])).filter(Boolean).map((o) => ({ id: o.id, name: o.name, sub: o.path, node: o.id }));
		const more = [{ title: 'Usa', items: tied('from', 'to') }, { title: 'Usado por', items: tied('to', 'from') }].filter((g) => g.items.length > 0);
		return { ...d, groups: [...more, ...(d.groups || [])] };
	}

	// The camera: the point of the picture at the middle of the canvas, and how close.

	function fit(now) {
		const v = graph.view;
		if (!v || graph.shown.length === 0 || v.w < 40) {
			return;
		}
		let x0 = Infinity;
		let y0 = Infinity;
		let x1 = -Infinity;
		let y1 = -Infinity;
		for (const id of graph.shown) {
			const b = graph.bodies.get(id);
			x0 = Math.min(x0, b.x - b.r);
			y0 = Math.min(y0, b.y - b.r);
			x1 = Math.max(x1, b.x + b.r);
			y1 = Math.max(y1, b.y + b.r);
		}
		const k = Math.max(0.12, Math.min(1.5, (v.w - 90) / Math.max(x1 - x0, 1), (v.h - 110) / Math.max(y1 - y0, 1)));
		const to = { x: (x0 + x1) / 2, y: (y0 + y1) / 2 + 8 / k, k };
		if (now) {
			graph.cam = to;
			graph.to = null;
		} else {
			graph.to = to;
		}
		wake();
	}

	/** Lets go of every node that was put somewhere, and lays the picture out again. */
	function loosen() {
		for (const b of graph.bodies.values()) {
			b.fx = null;
			b.fy = null;
		}
		graph.autofit = true;
		graph.sim.warm(0.9);
		wake();
	}

	function foldAll() {
		graph.open = G.opening(graph.nodes);
		graph.bodies.clear();
		graph.fresh = true;
		remember();
		arrange(1);
		drawTree();
	}

	function wake() {
		if (!graph.raf && graph.view) {
			graph.raf = requestAnimationFrame(frame);
		}
	}

	function frame(now) {
		graph.raf = 0;
		const v = graph.view;
		if (!v) {
			return;
		}
		let moving = now < graph.fadeUntil;
		if (graph.sim.active) {
			graph.sim.tick();
			moving = true;
		} else if (graph.autofit) {
			graph.autofit = false;
		}
		if (graph.autofit && !graph.drag) {
			fit(false);
		}
		if (graph.follow) {
			const b = graph.bodies.get(graph.follow.id);
			if (b && now < graph.follow.until) {
				graph.to = { x: b.x, y: b.y, k: graph.cam.k < 0.4 ? 0.85 : graph.cam.k };
			} else {
				graph.follow = null;
			}
		}
		if (graph.to) {
			const c = graph.cam;
			const t = graph.to;
			c.x += (t.x - c.x) * 0.2;
			c.y += (t.y - c.y) * 0.2;
			c.k += (t.k - c.k) * 0.2;
			if (Math.abs(t.x - c.x) * c.k < 0.5 && Math.abs(t.y - c.y) * c.k < 0.5 && Math.abs(t.k - c.k) < 0.002) {
				graph.cam = { ...t };
				graph.to = null;
			}
			moving = true;
		}
		paintGraph(now);
		if (moving) {
			wake();
		}
	}

	/** The colours of the theme, read when the picture is drawn in them. */
	function palette() {
		const css = getComputedStyle(document.documentElement);
		const v = (name) => css.getPropertyValue(name).trim();
		const node = {};
		for (const k of ['prd', 'proposal', 'design', 'delta', 'spec', 'memory', 'doc', 'file']) {
			node[k] = v(`--k-${k}`);
		}
		node.owned = v('--k-file-owned');
		for (const k of ['context', 'library', 'scope', 'folder', 'section']) {
			node[k] = v(`--g-${k}`);
		}
		return {
			bg: v('--bg'), fg: v('--fg'), muted: v('--muted'), ink: v('--ink'), accent: v('--accent'), tree: v('--g-tree'), node,
			edge: { code: v('--e-code'), docs: v('--e-docs'), bridge: v('--e-bridge') },
			font: getComputedStyle(document.body).fontFamily || 'sans-serif',
		};
	}

	function paintGraph(now) {
		const v = graph.view;
		const { ctx, w, h: height } = v;
		if (w < 2 || height < 2) {
			return;
		}
		const pal = v.palette || (v.palette = palette());
		const cam = graph.cam;
		const k = cam.k;
		ctx.setTransform(v.dpr, 0, 0, v.dpr, 0, 0);
		ctx.clearRect(0, 0, w, height);

		const focus = graph.shownSet.has(graph.hover) ? graph.hover : graph.shownSet.has(state.selected) ? state.selected : '';
		const lit = focus ? graph.near.get(focus) : null;
		const bright = (id) => (!lit || id === focus || lit.has(id)) && (!graph.hit || graph.hit.has(id));
		// Under the pointer the rest steps well back; around what is only
		// selected it stays readable.
		const faint = focus && focus === graph.hover ? 0.14 : graph.hit ? 0.14 : 0.42;

		// Where each node is on the canvas, once.
		for (const id of graph.shown) {
			const b = graph.bodies.get(id);
			b.sx = (b.x - cam.x) * k + w / 2;
			b.sy = (b.y - cam.y) * k + height / 2;
			b.sr = Math.max(b.r * k, 2.2);
			b.a = b.born ? Math.min(1, (now - b.born) / 260) : 1;
			b.on = b.sx > -60 && b.sx < w + 60 && b.sy > -60 && b.sy < height + 60;
		}

		// The tree: each node to the group that holds it.
		ctx.lineWidth = 1;
		ctx.strokeStyle = pal.tree;
		for (const id of graph.shown) {
			const n = graph.nodes.get(id);
			const p = graph.bodies.get(n.parent);
			const b = graph.bodies.get(id);
			if (!p || (!p.on && !b.on)) {
				continue;
			}
			ctx.globalAlpha = (bright(id) && bright(n.parent) ? (lit ? 0.95 : 0.7) : faint * 0.8) * b.a;
			ctx.beginPath();
			ctx.moveTo(p.sx, p.sy);
			ctx.lineTo(b.sx, b.sy);
			ctx.stroke();
		}

		// The links across it, bent so that two ways between the same nodes are two lines.
		const heads = k >= 0.55;
		for (const e of graph.edges) {
			const a = graph.bodies.get(e.from);
			const b = graph.bodies.get(e.to);
			if (!a.on && !b.on) {
				continue;
			}
			const family = FAMILY_OF[e.kind];
			const mine = focus && (e.from === focus || e.to === focus);
			const dim = (focus && !mine) || (graph.hit && !(graph.hit.has(e.from) && graph.hit.has(e.to)));
			const dx = b.sx - a.sx;
			const dy = b.sy - a.sy;
			const d = Math.sqrt(dx * dx + dy * dy) || 1;
			const cx = (a.sx + b.sx) / 2 - (dy / d) * d * 0.14;
			const cy = (a.sy + b.sy) / 2 + (dx / d) * d * 0.14;
			ctx.globalAlpha = (dim ? faint * 0.36 : mine ? 0.95 : family === 'docs' ? 0.55 : 0.34) * Math.min(a.a, b.a);
			ctx.strokeStyle = pal.edge[family];
			ctx.lineWidth = Math.min(2.6, 0.8 + Math.log2(1 + e.weight) * 0.34);
			ctx.beginPath();
			ctx.moveTo(a.sx, a.sy);
			ctx.quadraticCurveTo(cx, cy, b.sx, b.sy);
			ctx.stroke();
			if ((heads || mine) && !dim) {
				// The arrow stops at the edge of what it points to.
				const tx = b.sx - cx;
				const ty = b.sy - cy;
				const tl = Math.sqrt(tx * tx + ty * ty) || 1;
				const px = b.sx - (tx / tl) * (b.sr + 2);
				const py = b.sy - (ty / tl) * (b.sr + 2);
				const s = 4 + ctx.lineWidth;
				ctx.fillStyle = pal.edge[family];
				ctx.beginPath();
				ctx.moveTo(px, py);
				ctx.lineTo(px - (tx / tl) * s - (ty / tl) * s * 0.5, py - (ty / tl) * s + (tx / tl) * s * 0.5);
				ctx.lineTo(px - (tx / tl) * s + (ty / tl) * s * 0.5, py - (ty / tl) * s - (tx / tl) * s * 0.5);
				ctx.closePath();
				ctx.fill();
			}
		}

		// The nodes: a group that is open is a ring around what it holds.
		const labels = [];
		for (const id of graph.shown) {
			const b = graph.bodies.get(id);
			if (!b.on) {
				continue;
			}
			const n = graph.nodes.get(id);
			const colour = pal.node[colourOf(n)] || pal.muted;
			const on = bright(id);
			const open = n.group && graph.open.has(id);
			ctx.globalAlpha = (on ? (n.archived ? 0.5 : 1) : faint) * b.a;
			ctx.beginPath();
			ctx.arc(b.sx, b.sy, b.sr, 0, Math.PI * 2);
			if (open) {
				ctx.fillStyle = pal.bg;
				ctx.fill();
				ctx.lineWidth = Math.max(1.5, Math.min(2.5, b.sr * 0.3));
				ctx.strokeStyle = colour;
				ctx.stroke();
				ctx.beginPath();
				ctx.arc(b.sx, b.sy, Math.max(1.2, b.sr * 0.3), 0, Math.PI * 2);
				ctx.fillStyle = colour;
				ctx.fill();
			} else {
				ctx.fillStyle = colour;
				ctx.fill();
				if (n.group && b.sr >= 9) {
					ctx.fillStyle = pal.ink;
					ctx.font = `700 ${Math.min(12, Math.max(9, b.sr * 0.72))}px ${pal.font}`;
					ctx.textAlign = 'center';
					ctx.textBaseline = 'middle';
					ctx.fillText(compact(n.leaves), b.sx, b.sy + 0.5);
				}
			}
			if (b.fx !== null && on) {
				// Put there by hand: a dot says it stays.
				ctx.fillStyle = pal.fg;
				ctx.beginPath();
				ctx.arc(b.sx + b.sr * 0.72, b.sy - b.sr * 0.72, 1.8, 0, Math.PI * 2);
				ctx.fill();
			}
			if (id === state.selected || id === graph.hover) {
				ctx.globalAlpha = b.a;
				ctx.lineWidth = id === state.selected ? 2 : 1.25;
				ctx.strokeStyle = id === state.selected ? pal.fg : pal.accent;
				ctx.beginPath();
				ctx.arc(b.sx, b.sy, b.sr + 3.5, 0, Math.PI * 2);
				ctx.stroke();
			}
			const wanted = id === focus || (lit && lit.has(id) && on);
			if (wanted || ((on || faint > 0.3) && (n.group ? b.sr >= 4 : b.sr >= 6.5))) {
				labels.push({ n, b, rank: (id === focus ? 1e9 : 0) + (wanted ? 1e7 : 0) + (n.group ? 1e5 + n.leaves : n.value), strong: n.group || id === focus });
			}
		}

		// The names, the ones that matter most first; one that would cover another is left out.
		labels.sort((p, q) => q.rank - p.rank);
		const placed = [];
		ctx.textAlign = 'center';
		ctx.textBaseline = 'top';
		ctx.lineJoin = 'round';
		for (const l of labels) {
			if (placed.length >= 260) {
				break;
			}
			const text = l.n.name.length > 30 ? `${l.n.name.slice(0, 29)}…` : l.n.name;
			ctx.font = `${l.strong ? 600 : 400} 11px ${pal.font}`;
			const half = ctx.measureText(text).width / 2 + 3;
			const box = { x0: l.b.sx - half, x1: l.b.sx + half, y0: l.b.sy + l.b.sr + 3, y1: l.b.sy + l.b.sr + 17 };
			if (l.rank < 1e9 && placed.some((p) => box.x0 < p.x1 && box.x1 > p.x0 && box.y0 < p.y1 && box.y1 > p.y0)) {
				continue;
			}
			placed.push(box);
			ctx.globalAlpha = (bright(l.n.id) ? 1 : 0.45) * l.b.a;
			ctx.lineWidth = 3;
			ctx.strokeStyle = pal.bg;
			ctx.strokeText(text, l.b.sx, box.y0);
			ctx.fillStyle = l.strong ? pal.fg : pal.muted;
			ctx.fillText(text, l.b.sx, box.y0);
		}
		ctx.globalAlpha = 1;
	}

	// The tree beside the picture: the same nodes, the same groups open.

	function drawTree() {
		const v = graph.view;
		if (!v) {
			return;
		}
		const rows = [];
		const walk = (id, depth) => {
			const n = graph.nodes.get(id);
			if (graph.hit && !graph.hit.has(id)) {
				return;
			}
			// Filtering, every branch that leads to a match is shown open.
			const open = n.group && (graph.hit ? true : graph.open.has(id));
			rows.push(h('div', {
				class: `row${n.group ? ' group' : ''}${state.selected === id ? ' sel' : ''}${graph.hit && !matches(n, state.filter) ? ' dim' : ''}${n.archived ? ' archived' : ''}`,
				role: 'treeitem',
				tabindex: -1,
				'aria-level': depth + 1,
				'aria-expanded': n.group ? String(open) : null,
				'aria-selected': String(state.selected === id),
				dataset: { id },
				style: { paddingLeft: `${8 + depth * 14}px` },
				title: n.path || n.title || n.name,
			},
				h('span', { class: `twist${n.group ? '' : ' none'}${open ? ' open' : ''}`, 'aria-hidden': 'true' }),
				h('i', { class: `dot c-${colourOf(n)}${open ? ' open' : ''}` }),
				h('span', { class: 'row-name' }, n.name),
				h('span', { class: 'row-meta' }, n.group ? number(n.leaves) : compact(n.value)),
			));
			if (open) {
				for (const c of n.children) {
					walk(c, depth + 1);
				}
			}
		};
		const first = graph.nodes.keys().next();
		if (!first.done) {
			walk(first.value, 0);
		}
		const held = v.tree.contains(document.activeElement) ? document.activeElement.dataset.id : '';
		v.tree.replaceChildren(...rows);
		const row = (held && rowOf(held)) || rowOf(state.selected) || rows[0];
		if (row) {
			row.tabIndex = 0;
			if (held) {
				row.focus();
			}
		}
	}

	function rowOf(id) {
		return id && graph.view ? graph.view.tree.querySelector(`.row[data-id="${CSS.escape(id)}"]`) : null;
	}

	/** Marks in the tree what is selected and what the pointer is over in the picture. */
	function markTree() {
		const v = graph.view;
		if (!v) {
			return;
		}
		for (const el of v.tree.querySelectorAll('.row.sel, .row.hov')) {
			el.classList.remove('sel', 'hov');
			el.setAttribute('aria-selected', 'false');
		}
		const sel = rowOf(state.selected);
		if (sel) {
			sel.classList.add('sel');
			sel.setAttribute('aria-selected', 'true');
			sel.scrollIntoView({ block: 'nearest' });
		}
		const hov = rowOf(graph.hover);
		if (hov) {
			hov.classList.add('hov');
		}
	}

	/** What the filter keeps: what matches, and the groups that lead to it. */
	function filterGraph() {
		graph.hit = null;
		let found = 0;
		if (state.filter) {
			graph.hit = new Set();
			for (const n of graph.nodes.values()) {
				if (matches(n, state.filter)) {
					found += 1;
					G.lineage(graph.nodes, n.id).forEach((id) => graph.hit.add(id));
				}
			}
		}
		if (graph.view) {
			graph.view.found.textContent = state.filter ? (found === 0 ? 'nada encontrado' : plural(found, 'encontrado', 'encontrados')) : '';
		}
		drawTree();
		wake();
	}

	function drawGraph() {
		const canvas = h('canvas', { class: 'graph-canvas', role: 'img', 'aria-label': 'Grafo do projeto: os nós da estrutura ao lado e as ligações entre eles' });
		const tree = h('div', { class: 'tree', role: 'tree', 'aria-label': 'Estrutura do projeto' });
		const tip = h('div', { class: 'tip', hidden: true });
		const count = h('span', { class: 'muted graph-count' });
		const found = h('span', { class: 'tree-found' });
		const families = FAMILIES.map(([key, label, says]) => h('button', {
			class: `chip fam f-${key}${graph.off.has(key) ? '' : ' on'}`, type: 'button', 'aria-pressed': String(!graph.off.has(key)), title: `${says} — clique para mostrar ou esconder`,
			onclick: (event) => {
				const on = graph.off.delete(key);
				if (!on) {
					graph.off.add(key);
				}
				event.currentTarget.classList.toggle('on', on);
				event.currentTarget.setAttribute('aria-pressed', String(on));
				remember();
				arrange(0.3);
			},
		}, h('i', {}), label));
		const wrap = h('div', { class: 'canvas-wrap' }, canvas, tip,
			h('div', { class: 'graph-hint muted' }, 'clique seleciona · dois cliques abrem um grupo ou o arquivo · arraste um nó para fixá-lo · role para aproximar'));
		view.replaceChildren(
			h('aside', { class: 'tree-pane' }, h('div', { class: 'tree-head' }, 'Estrutura', found), tree),
			h('section', { class: 'graph-stage' },
				h('div', { class: 'graph-bar' }, h('span', { class: 'muted' }, 'ligações'), families, h('span', { class: 'grow' }), count,
					h('button', { class: 'chip', type: 'button', title: 'Mostrar tudo o que está aberto', onclick: () => { graph.autofit = false; fit(false); } }, 'enquadrar'),
					h('button', { class: 'chip', type: 'button', title: 'Soltar os nós fixados e dispor de novo', onclick: loosen }, 'reorganizar'),
					h('button', { class: 'chip', type: 'button', title: 'Fechar os grupos, como ao abrir', onclick: foldAll }, 'recolher')),
				wrap),
		);
		const v = graph.view = { canvas, ctx: canvas.getContext('2d'), tree, tip, count, found, w: 0, h: 0, dpr: 1, palette: null, observer: null };

		const size = () => {
			const w = wrap.clientWidth;
			const height = wrap.clientHeight;
			if (v.w > 0 && w > 0) {
				// The picture stays where it is when the panel beside it opens or
				// closes: what was under the pointer is still under it.
				for (const cam of [graph.cam, graph.to]) {
					if (cam) {
						cam.x += (w - v.w) / 2 / cam.k;
						cam.y += (height - v.h) / 2 / cam.k;
					}
				}
			}
			v.w = w;
			v.h = height;
			v.dpr = window.devicePixelRatio || 1;
			canvas.width = Math.round(v.w * v.dpr);
			canvas.height = Math.round(v.h * v.dpr);
			wake();
		};
		size();
		v.observer = new ResizeObserver(size);
		v.observer.observe(wrap);

		const asked = state.focus;
		state.focus = '';
		if (asked && graph.nodes.has(asked)) {
			graph.open.add(asked);
			G.lineage(graph.nodes, asked).forEach((id) => graph.open.add(id));
		}
		filterGraph();
		arrange(graph.bodies.size === 0 ? 1 : 0.2);
		if (asked && graph.nodes.has(asked)) {
			focusNode(asked);
		}

		// The picture: what is under the pointer, by where things were drawn.
		const point = (event) => {
			const box = canvas.getBoundingClientRect();
			const sx = event.clientX - box.left;
			const sy = event.clientY - box.top;
			return { sx, sy, x: (sx - v.w / 2) / graph.cam.k + graph.cam.x, y: (sy - v.h / 2) / graph.cam.k + graph.cam.y };
		};
		const nodeAt = (p) => {
			for (let i = graph.shown.length - 1; i >= 0; i--) {
				const b = graph.bodies.get(graph.shown[i]);
				const reach = Math.max(b.r, 7 / graph.cam.k);
				if ((b.x - p.x) * (b.x - p.x) + (b.y - p.y) * (b.y - p.y) <= reach * reach) {
					return b.id;
				}
			}
			return '';
		};
		const hover = (id, p) => {
			if (id !== graph.hover) {
				graph.hover = id;
				canvas.classList.toggle('over', Boolean(id));
				markTree();
				wake();
			}
			const n = graph.nodes.get(id);
			if (!n || graph.drag) {
				tip.hidden = true;
				return;
			}
			tip.replaceChildren(
				h('b', {}, n.title || n.name),
				n.path && h('div', { class: 'muted' }, n.path),
				h('div', {}, n.group
					? `${KIND_LABEL[n.kind] || n.kind} · ${plural(n.leaves, 'arquivo', 'arquivos')} · ${compact(n.value)} linhas`
					: `${KIND_LABEL[n.kind] || n.kind} · ${number(n.value)} linhas${(n.ctx || []).length ? ` · ${n.ctx.join(', ')}` : ''}${n.archived ? ' · arquivado' : ''}`),
			);
			tip.hidden = false;
			tip.style.left = `${Math.max(8, Math.min(p.sx + 14, v.w - tip.offsetWidth - 8))}px`;
			tip.style.top = `${Math.max(8, Math.min(p.sy + 16, v.h - tip.offsetHeight - 8))}px`;
		};
		canvas.addEventListener('pointerdown', (event) => {
			if (event.button !== 0) {
				return;
			}
			const p = point(event);
			const id = nodeAt(p);
			const b = graph.bodies.get(id);
			graph.drag = { id, sx: p.sx, sy: p.sy, moved: false, cam: { ...graph.cam }, dx: b ? b.x - p.x : 0, dy: b ? b.y - p.y : 0 };
			canvas.setPointerCapture(event.pointerId);
			tip.hidden = true;
		});
		canvas.addEventListener('pointermove', (event) => {
			const p = point(event);
			const drag = graph.drag;
			if (!drag) {
				hover(nodeAt(p), p);
				return;
			}
			if (!drag.moved && Math.abs(p.sx - drag.sx) + Math.abs(p.sy - drag.sy) < 4) {
				return;
			}
			drag.moved = true;
			graph.autofit = false;
			graph.follow = null;
			canvas.classList.add('dragging');
			const b = graph.bodies.get(drag.id);
			if (b) {
				// The node goes with the pointer and stays where it is left.
				b.fx = p.x + drag.dx;
				b.fy = p.y + drag.dy;
				graph.sim.alphaTarget = 0.25;
			} else {
				graph.to = null;
				graph.cam = { x: drag.cam.x - (p.sx - drag.sx) / drag.cam.k, y: drag.cam.y - (p.sy - drag.sy) / drag.cam.k, k: drag.cam.k };
			}
			wake();
		});
		const drop = (event) => {
			const drag = graph.drag;
			if (!drag) {
				return;
			}
			graph.drag = null;
			graph.sim.alphaTarget = 0;
			canvas.classList.remove('dragging');
			if (!drag.moved && event.type === 'pointerup') {
				if (drag.id) {
					selectNode(drag.id);
				} else {
					deselect();
				}
			}
			wake();
		};
		canvas.addEventListener('pointerup', drop);
		canvas.addEventListener('pointercancel', drop);
		canvas.addEventListener('pointerleave', () => {
			if (!graph.drag) {
				hover('', null);
			}
		});
		canvas.addEventListener('dblclick', (event) => {
			const n = graph.nodes.get(nodeAt(point(event)));
			if (!n) {
				return;
			}
			if (n.group) {
				toggle(n.id);
			} else {
				vscode.postMessage({ type: 'open', path: n.path });
			}
		});
		canvas.addEventListener('wheel', (event) => {
			event.preventDefault();
			const p = point(event);
			const k = Math.max(0.1, Math.min(4, graph.cam.k * Math.exp(-event.deltaY * (event.ctrlKey ? 0.01 : 0.0016))));
			graph.autofit = false;
			graph.follow = null;
			graph.to = null;
			// What is under the pointer stays under it.
			graph.cam = { x: p.x - (p.sx - v.w / 2) / k, y: p.y - (p.sy - v.h / 2) / k, k };
			wake();
		}, { passive: false });

		// The tree: a row is a node; its arrow opens the group.
		const idOf = (event) => {
			const row = event.target.closest('.row');
			return row ? row.dataset.id : '';
		};
		tree.addEventListener('click', (event) => {
			const id = idOf(event);
			if (!id) {
				return;
			}
			if (event.target.closest('.twist') && !graph.hit) {
				toggle(id);
			} else {
				focusNode(id);
			}
		});
		tree.addEventListener('dblclick', (event) => {
			const n = graph.nodes.get(idOf(event));
			if (!n || event.target.closest('.twist')) {
				return;
			}
			if (n.group) {
				toggle(n.id);
			} else {
				vscode.postMessage({ type: 'open', path: n.path });
			}
		});
		tree.addEventListener('mouseover', (event) => {
			const id = idOf(event);
			if (id !== graph.hover) {
				graph.hover = graph.shownSet.has(id) ? id : '';
				wake();
			}
		});
		tree.addEventListener('mouseleave', () => {
			graph.hover = '';
			wake();
		});
		tree.addEventListener('keydown', (event) => {
			const row = event.target.closest('.row');
			if (!row) {
				return;
			}
			const n = graph.nodes.get(row.dataset.id);
			const rows = [...tree.querySelectorAll('.row')];
			const go = (el) => {
				if (el) {
					rows.forEach((r) => { r.tabIndex = -1; });
					el.tabIndex = 0;
					el.focus();
				}
			};
			const open = n.group && graph.open.has(n.id);
			if (event.key === 'ArrowDown') {
				go(rows[rows.indexOf(row) + 1]);
			} else if (event.key === 'ArrowUp') {
				go(rows[rows.indexOf(row) - 1]);
			} else if (event.key === 'ArrowRight' && n.group && !graph.hit) {
				if (open) {
					go(rows[rows.indexOf(row) + 1]);
				} else {
					toggle(n.id);
				}
			} else if (event.key === 'ArrowLeft' && !graph.hit) {
				if (open) {
					toggle(n.id);
				} else {
					go(rowOf(n.parent));
				}
			} else if (event.key === 'Enter' || event.key === ' ') {
				focusNode(n.id);
			} else {
				return;
			}
			event.preventDefault();
		});
	}

	/** Leaves the view: nothing keeps drawing a picture that is not up. */
	function leaveGraph() {
		if (graph.view) {
			graph.view.observer.disconnect();
			graph.view = null;
		}
		cancelAnimationFrame(graph.raf);
		graph.raf = 0;
		graph.hover = '';
		graph.drag = null;
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
			if (item.node && state.tab === 'graph') {
				focusNode(item.node);
			} else if (item.context) {
				show('trace', { context: item.context });
			} else if (item.path) {
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
		$('detailsOpen').hidden = !(d.path || d.dir || d.context);
		$('detailsOpen').textContent = d.context ? 'rastrear o contexto' : d.path ? 'abrir' : 'mostrar no explorador';
		$('detailsOpen').onclick = () => open({ ...d, node: '' });
		$('detailsBody').replaceChildren(...(d.groups || []).map((g) => h('section', {},
			h('h4', {}, g.title, h('span', { class: 'muted' }, number(g.items.length))),
			h('ul', {}, g.items.map((item) => h('li', {}, h('button', { class: 'item', type: 'button', disabled: !(item.node || item.path || item.dir || (item.id || '').startsWith('ctx:')), onclick: () => open(item) },
				h('span', {}, item.name), item.sub && h('small', {}, item.sub)))))),
		));
		panel.hidden = false;
		repaint();
	}

	function repaint() {
		if (state.tab === 'trace' && drawTrace.paint) {
			requestAnimationFrame(drawTrace.paint);
		}
		wake(); // the graph follows its own size
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
				state.model = { overview: message.overview };
				setGraph(message.graph);
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
				showDetails(withLinks(message.data));
				return;
			case 'empty':
				state.model = null;
				leaveGraph();
				drawTabs();
				showEmpty(message);
				return;
			default:
		}
	});

	$('search').addEventListener('input', () => {
		state.filter = $('search').value.trim().toLowerCase();
		if (state.tab === 'graph') {
			filterGraph();
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
	// The theme changed: the picture is drawn in the new colours.
	new MutationObserver(() => {
		if (graph.view) {
			graph.view.palette = null;
			wake();
		}
	}).observe(document.body, { attributes: true, attributeFilter: ['class', 'style'] });
	window.addEventListener('keydown', (event) => {
		if (event.key === 'Escape') {
			if (!$('details').hidden) {
				$('closeDetails').click();
				wake();
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
