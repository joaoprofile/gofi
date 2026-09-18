'use strict';

/**
 * The document graph, drawn as a force layout.
 *
 * Everything here is hand-rolled: the extension has no runtime dependency, and
 * pulling in a layout library to move a few hundred points would be the only
 * one. The physics is the standard three forces — repulsion between every pair,
 * a spring per link, and a pull towards the centre so disconnected islands do
 * not drift off screen — with the pair loop left as O(n²) because the views are
 * bounded: the repository view drops source files and the context view rolls
 * them up per package, so nothing here ever draws more than a few hundred
 * nodes.
 *
 * The one deliberate departure from a generic force graph: a link's spring
 * length depends on what the link means. A document sits close to the context
 * it belongs to and far from a context it merely cites, because that distance
 * is the thing the picture is supposed to say.
 */

(function () {
	const vscode = acquireVsCodeApi();

	const canvas = document.getElementById('canvas');
	const ctx2d = canvas.getContext('2d');
	const bar = {
		back: document.getElementById('back'),
		scope: document.getElementById('scope'),
		counts: document.getElementById('counts'),
		search: document.getElementById('search'),
		entities: document.getElementById('entities'),
		entitiesToggle: document.getElementById('entitiesToggle'),
		refresh: document.getElementById('refresh'),
		center: document.getElementById('center'),
	};
	const legend = document.getElementById('legend');
	const tree = {
		body: document.getElementById('treeBody'),
		count: document.getElementById('treeCount'),
	};
	const empty = {
		box: document.getElementById('empty'),
		text: document.getElementById('emptyText'),
		build: document.getElementById('buildBtn'),
	};
	const details = {
		box: document.getElementById('details'),
		close: document.getElementById('closeDetails'),
		title: document.getElementById('detailsTitle'),
		meta: document.getElementById('detailsMeta'),
		body: document.getElementById('detailsBody'),
	};

	/** Colour per node kind, read from the stylesheet so themes stay in one place. */
	let palette = readPalette();

	/**
	 * The theme colours the canvas paints with, resolved once.
	 *
	 * Canvas has no cascade, so every stroke needs a literal colour, and the
	 * obvious way to get one — asking the DOM mid-loop — makes the browser
	 * resolve style hundreds of times per frame for an answer that changes only
	 * when the user switches theme. So it is resolved here and invalidated by
	 * the one event that can change it.
	 */
	let theme = readTheme();

	function readTheme() {
		const body = getComputedStyle(document.body);
		const styles = getComputedStyle(document.documentElement);
		return {
			fg: body.color,
			bg: body.backgroundColor,
			font: body.fontFamily,
			dim: (styles.getPropertyValue('--label-dim') || body.color).trim(),
		};
	}

	// VSCode signals a theme change by swapping the class on <body>.
	new MutationObserver(() => {
		palette = readPalette();
		theme = readTheme();
		drawLegend();
		requestDraw();
	}).observe(document.body, { attributes: true, attributeFilter: ['class'] });

	/** Human labels for the legend and the details pane. */
	const KIND_LABEL = {
		contexto: 'contexto',
		specs: 'spec',
		prd: 'PRD',
		memory: 'memória',
		entidade: 'tabela',
		pacote: 'pacote',
	};

	/** How far apart a link wants its ends, by meaning. */
	const REST_LENGTH = {
		pertence: 55,
		declara: 90,
		cita: 130,
		wikilink: 110,
		toca: 80,
		implementa: 70,
	};

	const state = {
		view: 'repositorio',
		context: '',
		nodes: [],
		links: [],
		byId: new Map(),
		/** nodeId -> the links touching it, indexed at load. */
		adjacent: new Map(),
		history: [],
		counts: {},
		omitted: null,
		/** Viewport transform: world = (screen - offset) / scale. */
		scale: 1,
		offsetX: 0,
		offsetY: 0,
		alpha: 0,
		hover: null,
		selected: null,
		dragging: null,
		panning: false,
		pointer: { x: 0, y: 0 },
		query: '',
		/** Neighbours of the hovered node, for the Obsidian-style dimming. */
		near: new Set(),
		/** Every context in the project — the tree lists them all, always. */
		contexts: [],
		/** Contexts expanded in the tree. */
		open: new Set(),
		/** Subtrees already fetched, keyed by context. */
		subtrees: new Map(),
		/** Whether the viewport still follows the drawing. */
		autoFit: true,
	};

	// ── incoming ──────────────────────────────────────────────────────────

	window.addEventListener('message', (event) => {
		const message = event.data;
		if (message.type === 'graph') {
			load(message);
			return;
		}
		if (message.type === 'subtree') {
			state.subtrees.set(message.context, message.data);
			drawTree();
			return;
		}
		if (message.type === 'empty') {
			showEmpty(message.reason, message);
		}
	});

	function load(message) {
		const data = message.data;
		empty.box.hidden = true;
		canvas.hidden = false;

		state.view = data.view;
		state.context = data.context || '';
		state.history = data.history || [];
		state.counts = data.counts || {};
		state.omitted = data.omitted || null;
		state.contexts = message.contexts || [];
		if (state.view === 'contexto') {
			// Opening a context in the graph opens it in the tree too: the two
			// are one navigation, not two.
			state.open.add(state.context);
			state.subtrees.set(state.context, data);
		}
		state.selected = null;
		state.hover = null;
		state.near.clear();
		hideDetails();

		// Positions carry over for nodes that survive a reload, so a rebuild
		// nudges the picture instead of reshuffling it.
		const previous = state.byId;
		state.nodes = data.nodes.map((n) => {
			const old = previous.get(n.id);
			return Object.assign({}, n, {
				x: old ? old.x : 0,
				y: old ? old.y : 0,
				vx: 0,
				vy: 0,
				placed: Boolean(old),
			});
		});
		state.byId = new Map(state.nodes.map((n) => [n.id, n]));
		state.adjacent = new Map();
		state.links = data.links
			.map((l) => ({
				source: state.byId.get(l.source),
				target: state.byId.get(l.target),
				kinds: l.kinds,
			}))
			.filter((l) => l.source && l.target);

		// Hover recomputes a neighbourhood on every node the cursor crosses.
		// Scanning the link list each time is O(edges) per mouse move for an
		// answer that never changes between loads, so it is indexed once.
		for (const l of state.links) {
			if (!state.adjacent.has(l.source.id)) {
				state.adjacent.set(l.source.id, []);
			}
			if (!state.adjacent.has(l.target.id)) {
				state.adjacent.set(l.target.id, []);
			}
			state.adjacent.get(l.source.id).push({ node: l.target, kinds: l.kinds });
			state.adjacent.get(l.target.id).push({ node: l.source, kinds: l.kinds });
		}

		seed();
		bar.back.hidden = state.view !== 'contexto';
		bar.entitiesToggle.hidden = state.view === 'contexto';
		bar.entities.checked = message.entities === true;
		bar.scope.textContent = state.view === 'contexto' ? state.context : 'projeto';
		bar.counts.textContent = summary(state.counts) + omittedNote();
		bar.counts.title = state.omitted && state.omitted.packages
			? `${state.omitted.packages} pacotes ficaram fora do corte de 24`
			: '';
		drawLegend();
		drawTree();

		if (state.view === 'contexto') {
			showContextDetails();
		}
		state.alpha = 1;
		state.autoFit = true;
		resize();
		tick();
	}

	function showEmpty(reason, message) {
		canvas.hidden = true;
		legend.innerHTML = '';
		tree.body.innerHTML = '';
		tree.count.textContent = '';
		bar.counts.textContent = '';
		hideDetails();
		empty.box.hidden = false;
		if (reason === 'sem-pasta') {
			empty.text.textContent = 'Abra uma pasta para ver o grafo do projeto.';
			empty.build.hidden = true;
			return;
		}
		const where = message && message.file ? `\n\nProcurei em ${message.file}` : '';
		empty.text.textContent =
			'Este projeto ainda não tem grafo de documentos. Ele é derivado: gerado a partir das specs, PRDs e memória de contexto.' +
			where;
		empty.build.hidden = false;
	}

	/** Places new nodes on a ring per kind — a decent guess beats the origin. */
	function seed() {
		const groups = new Map();
		for (const n of state.nodes) {
			const key = groupOf(n);
			if (!groups.has(key)) {
				groups.set(key, []);
			}
			groups.get(key).push(n);
		}
		let ring = 0;
		for (const list of groups.values()) {
			const radius = 90 + ring * 120;
			list.forEach((n, i) => {
				if (n.placed) {
					return;
				}
				const angle = (i / Math.max(1, list.length)) * Math.PI * 2 + ring;
				n.x = Math.cos(angle) * radius;
				n.y = Math.sin(angle) * radius;
			});
			ring += 1;
		}
		const focus = state.nodes.find((n) => n.focus);
		if (focus) {
			focus.x = 0;
			focus.y = 0;
		}
	}

	function groupOf(n) {
		return n.kind === 'doc' ? n.corpus : n.kind;
	}

	const ORDER = ['contexto', 'specs', 'prd', 'memory', 'entidade', 'pacote'];

	function summary(counts) {
		const parts = [];
		for (const key of ORDER) {
			if (counts[key]) {
				parts.push(`${counts[key]} ${KIND_LABEL[key] || key}`);
			}
		}
		return parts.join(' · ');
	}

	/**
	 * What the view left out, stated rather than hidden.
	 *
	 * A capped picture that reads as complete is the failure mode worth
	 * avoiding: someone would conclude the context has 24 packages.
	 */
	function omittedNote() {
		if (!state.omitted || !state.omitted.packages) {
			return '';
		}
		return ` · +${state.omitted.packages} fora`;
	}

	function drawLegend() {
		legend.innerHTML = '';
		for (const [key, count] of Object.entries(state.counts)) {
			const row = document.createElement('div');
			row.className = 'row';
			const dot = document.createElement('span');
			dot.className = 'dot';
			dot.style.background = palette[key] || palette.pacote;
			const label = document.createElement('span');
			label.textContent = KIND_LABEL[key] || key;
			const n = document.createElement('span');
			n.className = 'count';
			n.textContent = String(count);
			row.append(dot, label, n);
			legend.append(row);
		}
	}

	// ── tree ──────────────────────────────────────────────────────────────

	/**
	 * The corpus as a list, next to the corpus as a picture.
	 *
	 * The graph shows shape — which contexts cluster, what bridges them — and
	 * is bad at the question "what exactly is filed under pricing", because
	 * reading a label off a moving node is not how anyone answers that. The
	 * tree is exact, enumerable and scrollable; the two share a selection, so
	 * clicking either one moves the other.
	 */
	function drawTree() {
		tree.body.innerHTML = '';
		tree.count.textContent = state.contexts.length ? `${state.contexts.length}` : '';

		for (const name of state.contexts) {
			const open = state.open.has(name);
			const row = rowItem({
				depth: 0,
				twist: open ? '▾' : '▸',
				colour: palette.contexto,
				label: name,
				selected: state.view === 'contexto' && state.context === name,
				onClick: () => toggleContext(name),
			});
			tree.body.append(row);
			if (open) {
				drawSubtree(name);
			}
		}
	}

	function drawSubtree(name) {
		const data = state.subtrees.get(name);
		if (!data) {
			tree.body.append(rowItem({ depth: 1, label: 'carregando…', muted: true }));
			return;
		}

		const docs = data.nodes.filter((n) => n.kind === 'doc');
		for (const [key, title] of [['specs', 'specs'], ['prd', 'prd'], ['memory', 'memória']]) {
			const list = docs.filter((d) => d.corpus === key);
			if (list.length === 0) {
				continue;
			}
			tree.body.append(rowItem({ depth: 1, label: `${title} (${list.length})`, group: true }));
			for (const d of list) {
				tree.body.append(
					rowItem({
						depth: 2,
						colour: palette[d.corpus],
						label: d.label,
						meta: d.version ? `v${d.version}` : '',
						title: `${d.path}${d.status ? ` · ${d.status}` : ''}`,
						onClick: () => vscode.postMessage({ type: 'open', path: d.path }),
					}),
				);
			}
		}

		const tables = data.nodes.filter((n) => n.kind === 'entidade');
		if (tables.length > 0) {
			tree.body.append(rowItem({ depth: 1, label: `tabelas (${tables.length})`, group: true }));
			for (const t of tables) {
				tree.body.append(
					rowItem({
						depth: 2,
						colour: palette.entidade,
						label: t.label,
						meta: t.contexts > 1 ? `${t.contexts} ctx` : '',
						title: t.common ? 'Tabela comum a boa parte do corpus' : 'tabela do schema',
					}),
				);
			}
		}

		const packages = data.nodes.filter((n) => n.kind === 'pacote');
		if (packages.length > 0) {
			const extra = data.omitted && data.omitted.packages ? ` +${data.omitted.packages} fora` : '';
			tree.body.append(rowItem({ depth: 1, label: `código (${packages.length})${extra}`, group: true }));
			for (const p of packages) {
				tree.body.append(
					rowItem({
						depth: 2,
						colour: palette.pacote,
						label: p.dir,
						meta: String(p.files),
						title: p.dir,
						onClick: () => vscode.postMessage({ type: 'open', dir: p.dir }),
					}),
				);
			}
		}

		const history = data.history || [];
		if (history.length > 0) {
			tree.body.append(rowItem({ depth: 1, label: `versões (${history.length})`, group: true }));
			for (const h of history) {
				// The full line is the tooltip: these run long, and truncating
				// them in the row is what makes the list readable at all.
				tree.body.append(rowItem({ depth: 2, label: h.text, title: h.text, muted: true }));
			}
		}
	}

	/** Expands a context, fetching its subtree the first time. */
	function toggleContext(name) {
		if (state.open.has(name)) {
			state.open.delete(name);
			drawTree();
			return;
		}
		state.open.add(name);
		if (!state.subtrees.has(name)) {
			vscode.postMessage({ type: 'subtree', context: name });
		}
		drawTree();
		// Expanding is also asking to see it: the graph follows.
		if (!(state.view === 'contexto' && state.context === name)) {
			vscode.postMessage({ type: 'focus', context: name });
		}
	}

	function rowItem(opts) {
		const el = document.createElement('button');
		el.type = 'button';
		el.className = 'row-item' + (opts.selected ? ' selected' : '') + (opts.group ? ' group' : '');
		el.style.paddingLeft = `${8 + (opts.depth || 0) * 14}px`;
		if (opts.title) {
			el.title = opts.title;
		}
		if (opts.twist) {
			const t = document.createElement('span');
			t.className = 'twist';
			t.textContent = opts.twist;
			el.append(t);
		}
		if (opts.colour) {
			const dot = document.createElement('span');
			dot.className = 'dot';
			dot.style.background = opts.colour;
			el.append(dot);
		}
		const label = document.createElement('span');
		label.className = 'label';
		label.textContent = opts.label;
		if (opts.muted) {
			label.style.opacity = '0.75';
		}
		el.append(label);
		if (opts.meta) {
			const meta = document.createElement('span');
			meta.className = 'meta';
			meta.textContent = opts.meta;
			el.append(meta);
		}
		if (opts.onClick) {
			el.addEventListener('click', opts.onClick);
		} else {
			el.style.cursor = 'default';
		}
		return el;
	}

	// ── simulation ────────────────────────────────────────────────────────

	/**
	 * Side of the bucket the repulsion is binned into.
	 *
	 * Equal to the cutoff distance, and that is the whole trick: two nodes more
	 * than one bucket apart on either axis are already further than the cutoff,
	 * so the pairs this skips are exactly the pairs the old loop measured only
	 * to discard. Against the real corpus that was seven of every ten. The
	 * physics is unchanged — this is not an approximation like Barnes-Hut.
	 */
	const CELL = 600;

	/**
	 * Below this the buckets cost more than they save.
	 *
	 * Measured, not guessed: binning is a Map build per tick, and under roughly
	 * 150 nodes that build outweighs the pairs it skips — at 60 nodes the plain
	 * loop is over twice as fast. Above it the ratio settles around 1.7x. Both
	 * sides of that line are microseconds against a 16ms frame, so this guard
	 * is not what makes the panel fast; it is what stops a focused context,
	 * which averages two dozen nodes, from paying for a structure built to help
	 * the repository view.
	 */
	const BUCKET_FROM = 150;

	/** Half-neighbourhood: each pair of buckets is visited once, not twice. */
	const NEIGHBOURS = [
		[1, 0],
		[-1, 1],
		[0, 1],
		[1, 1],
	];

	function bucketKey(cx, cy) {
		// Two signed cell coordinates packed into one integer key: a Map of
		// numbers avoids building a string per node per tick.
		return (cx + 32768) * 65536 + (cy + 32768);
	}

	function pushApart(a, b, alpha) {
		let dx = b.x - a.x;
		let dy = b.y - a.y;
		let d2 = dx * dx + dy * dy;
		if (d2 === 0) {
			// Exactly coincident points give no direction to push along, and
			// would stay stuck on top of each other forever.
			dx = 0.5;
			dy = 0.5;
			d2 = 0.5;
		}
		if (d2 > CELL * CELL) {
			return;
		}
		const d = Math.sqrt(d2);
		const force = (900 * alpha) / d2;
		const fx = (dx / d) * force;
		const fy = (dy / d) * force;
		a.vx -= fx;
		a.vy -= fy;
		b.vx += fx;
		b.vy += fy;
	}

	function repel(nodes, alpha) {
		if (nodes.length < BUCKET_FROM) {
			for (let i = 0; i < nodes.length; i++) {
				for (let j = i + 1; j < nodes.length; j++) {
					pushApart(nodes[i], nodes[j], alpha);
				}
			}
			return;
		}
		const buckets = new Map();
		for (const n of nodes) {
			const key = bucketKey(Math.floor(n.x / CELL), Math.floor(n.y / CELL));
			const bucket = buckets.get(key);
			if (bucket) {
				bucket.push(n);
			} else {
				buckets.set(key, [n]);
			}
		}
		for (const [key, bucket] of buckets) {
			for (let i = 0; i < bucket.length; i++) {
				for (let j = i + 1; j < bucket.length; j++) {
					pushApart(bucket[i], bucket[j], alpha);
				}
			}
			const cx = Math.floor(key / 65536) - 32768;
			const cy = (key % 65536) - 32768;
			for (const [ox, oy] of NEIGHBOURS) {
				const other = buckets.get(bucketKey(cx + ox, cy + oy));
				if (!other) {
					continue;
				}
				for (const a of bucket) {
					for (const b of other) {
						pushApart(a, b, alpha);
					}
				}
			}
		}
	}

	function step() {
		const nodes = state.nodes;
		const alpha = state.alpha;

		repel(nodes, alpha);

		// Springs: the rest length is what the link means.
		for (const l of state.links) {
			const rest = restLength(l);
			const dx = l.target.x - l.source.x;
			const dy = l.target.y - l.source.y;
			const d = Math.sqrt(dx * dx + dy * dy) || 0.01;
			const force = ((d - rest) / d) * 0.06 * alpha;
			const fx = dx * force;
			const fy = dy * force;
			l.source.vx += fx;
			l.source.vy += fy;
			l.target.vx -= fx;
			l.target.vy -= fy;
		}

		// Collision. Repulsion falls off with distance and never quite stops
		// two nodes sitting on top of each other; this is a hard constraint,
		// applied as a position correction rather than a force, so circles
		// simply do not overlap. It is what turns a clump into a readable
		// arrangement.
		for (let i = 0; i < nodes.length; i++) {
			const a = nodes[i];
			const ra = radiusOf(a) + 3;
			for (let j = i + 1; j < nodes.length; j++) {
				const b = nodes[j];
				const min = ra + radiusOf(b) + 3;
				const dx = b.x - a.x;
				const dy = b.y - a.y;
				const d2 = dx * dx + dy * dy;
				if (d2 >= min * min || d2 === 0) {
					continue;
				}
				const d = Math.sqrt(d2);
				const push = ((min - d) / d) * 0.5;
				const ox = dx * push;
				const oy = dy * push;
				a.x -= ox;
				a.y -= oy;
				b.x += ox;
				b.y += oy;
			}
		}

		// Gravity, and the focused context pinned at the origin.
		for (const n of nodes) {
			if (n === state.dragging) {
				n.vx = 0;
				n.vy = 0;
				continue;
			}
			if (n.focus) {
				n.vx += (0 - n.x) * 0.12 * alpha;
				n.vy += (0 - n.y) * 0.12 * alpha;
			} else {
				n.vx += -n.x * 0.006 * alpha;
				n.vy += -n.y * 0.006 * alpha;
			}
			n.vx *= 0.82;
			n.vy *= 0.82;
			n.x += n.vx;
			n.y += n.vy;
		}

		state.alpha *= 0.985;
	}

	function restLength(link) {
		let rest = 120;
		for (const kind of link.kinds) {
			rest = Math.min(rest, REST_LENGTH[kind] ?? 120);
		}
		return rest;
	}

	let frame = null;

	/**
	 * Coalesces every redraw request into one per frame.
	 *
	 * Hover, pan and zoom all arrive from input events, which fire faster than
	 * the display refreshes — on a 120Hz trackpad a single flick can ask for
	 * several redraws inside one frame, and all but the last are thrown away
	 * after being paid for.
	 */
	function requestDraw() {
		if (frame !== null) {
			return;
		}
		frame = requestAnimationFrame(() => {
			frame = null;
			if (state.alpha > 0.005) {
				step();
				if (state.autoFit) {
					fit(true);
				}
				draw();
				requestDraw();
				return;
			}
			// Settled: one exact fit, then nothing schedules another frame
			// until input asks.
			if (state.autoFit) {
				fit(false);
				state.autoFit = false;
			}
			draw();
		});
	}

	/** Wakes the simulation after a change that moves nodes. */
	function tick() {
		requestDraw();
	}

	// ── drawing ───────────────────────────────────────────────────────────

	function radiusOf(n) {
		if (n.focus) {
			return 16;
		}
		if (n.kind === 'contexto') {
			return 9 + Math.min(7, n.degree * 0.5);
		}
		if (n.kind === 'pacote') {
			return 4.5 + Math.min(4, Math.sqrt(n.files || 1));
		}
		if (n.kind === 'entidade' && n.common) {
			return 4; // a table half the corpus shares carries little signal here
		}
		return 5 + Math.min(6, n.degree * 0.6);
	}

	function colorOf(n) {
		return palette[groupOf(n)] || palette.pacote;
	}

	/** A node matches the filter box, or there is no filter. */
	function matches(n) {
		if (state.query === '') {
			return true;
		}
		return n.label.toLowerCase().includes(state.query) || (n.path || '').toLowerCase().includes(state.query);
	}

	/** Dimming rule: hover isolates a neighbourhood, the filter isolates hits. */
	function opacityOf(n) {
		if (state.hover && !state.near.has(n.id)) {
			return 0.12;
		}
		if (!matches(n)) {
			return 0.12;
		}
		return 1;
	}

	/**
	 * The visible rectangle, in world coordinates.
	 *
	 * Zooming in is the case that matters: at 3x the viewport holds a ninth of
	 * the graph, and without this the other eight ninths are still stroked,
	 * filled and — the expensive part — have their labels laid out by the text
	 * engine, entirely outside the canvas.
	 */
	function viewport() {
		const margin = 60 / state.scale;
		return {
			x0: (0 - state.offsetX) / state.scale - margin,
			y0: (0 - state.offsetY) / state.scale - margin,
			x1: (canvas.clientWidth - state.offsetX) / state.scale + margin,
			y1: (canvas.clientHeight - state.offsetY) / state.scale + margin,
		};
	}

	/** Converts #rrggbb to rgba, for haloes and translucent fills. */
	function alpha(hex, a) {
		const h = hex.trim();
		if (h[0] !== '#' || h.length < 7) {
			return h;
		}
		const r = parseInt(h.slice(1, 3), 16);
		const g = parseInt(h.slice(3, 5), 16);
		const b = parseInt(h.slice(5, 7), 16);
		return `rgba(${r}, ${g}, ${b}, ${a})`;
	}

	function draw() {
		const width = canvas.clientWidth;
		const height = canvas.clientHeight;
		const box = viewport();
		const inside = (n) => n.x >= box.x0 && n.x <= box.x1 && n.y >= box.y0 && n.y <= box.y1;

		ctx2d.setTransform(dpr(), 0, 0, dpr(), 0, 0);
		ctx2d.clearRect(0, 0, width, height);
		ctx2d.save();
		ctx2d.translate(state.offsetX, state.offsetY);
		ctx2d.scale(state.scale, state.scale);
		ctx2d.lineCap = 'round';

		// Links first, so nodes sit on top of their own edges. A link is drawn
		// unless both ends fall off the same side, which is the cheap test that
		// still keeps a line crossing the screen from vanishing.
		let dashed = false;
		for (const l of state.links) {
			const a = l.source;
			const b = l.target;
			if (
				(a.x < box.x0 && b.x < box.x0) ||
				(a.x > box.x1 && b.x > box.x1) ||
				(a.y < box.y0 && b.y < box.y0) ||
				(a.y > box.y1 && b.y > box.y1)
			) {
				continue;
			}
			const lit = state.hover && state.near.has(a.id) && state.near.has(b.id);
			const faded = state.hover && !lit;
			ctx2d.globalAlpha = faded ? 0.05 : lit ? 0.75 : 0.2;
			ctx2d.strokeStyle = lit ? colorOf(b) : theme.fg;
			ctx2d.lineWidth = (lit ? 1.8 : 1.1) / state.scale;
			// A citation is a weaker claim than a declaration, and reads as one.
			const wantsDash = l.kinds.includes('cita') && !l.kinds.includes('declara');
			if (wantsDash !== dashed) {
				ctx2d.setLineDash(wantsDash ? [4 / state.scale, 4 / state.scale] : []);
				dashed = wantsDash;
			}
			// A slight arc rather than a straight line: parallel edges between
			// the same cluster stop overprinting into one thick smear, and the
			// eye follows a curve to its end more easily than a line in a
			// bundle of lines.
			const mx = (a.x + b.x) / 2;
			const my = (a.y + b.y) / 2;
			const dx = b.x - a.x;
			const dy = b.y - a.y;
			ctx2d.beginPath();
			ctx2d.moveTo(a.x, a.y);
			ctx2d.quadraticCurveTo(mx - dy * 0.08, my + dx * 0.08, b.x, b.y);
			ctx2d.stroke();
		}
		ctx2d.setLineDash([]);

		// Visible nodes, kept for the label pass so the filter runs once.
		const visible = [];
		for (const n of state.nodes) {
			if (!inside(n)) {
				continue;
			}
			visible.push(n);
			const r = radiusOf(n);
			const colour = colorOf(n);
			const op = opacityOf(n);
			const active = n === state.selected || n === state.hover;

			// A halo, not a hard edge: it lifts the node off the links running
			// under it without adding a second colour to read.
			if (active || n.focus) {
				ctx2d.globalAlpha = op;
				ctx2d.fillStyle = alpha(colour, active ? 0.22 : 0.14);
				ctx2d.beginPath();
				ctx2d.arc(n.x, n.y, r + (active ? 9 : 6) / state.scale, 0, Math.PI * 2);
				ctx2d.fill();
			}

			ctx2d.globalAlpha = op;
			ctx2d.fillStyle = colour;
			ctx2d.beginPath();
			ctx2d.arc(n.x, n.y, r, 0, Math.PI * 2);
			ctx2d.fill();

			// A document nobody has approved is drawn hollow — the status is
			// worth seeing without opening the file.
			if (n.kind === 'doc' && n.status === '') {
				ctx2d.fillStyle = theme.bg;
				ctx2d.beginPath();
				ctx2d.arc(n.x, n.y, Math.max(1, r - 2.5), 0, Math.PI * 2);
				ctx2d.fill();
				ctx2d.strokeStyle = colour;
				ctx2d.lineWidth = 1.4 / state.scale;
				ctx2d.beginPath();
				ctx2d.arc(n.x, n.y, Math.max(1, r - 1.2), 0, Math.PI * 2);
				ctx2d.stroke();
			}

			if (active) {
				ctx2d.strokeStyle = colour;
				ctx2d.lineWidth = 2 / state.scale;
				ctx2d.globalAlpha = 0.95;
				ctx2d.beginPath();
				ctx2d.arc(n.x, n.y, r + 4 / state.scale, 0, Math.PI * 2);
				ctx2d.stroke();
			}
		}

		drawLabels(visible);

		ctx2d.restore();
		ctx2d.globalAlpha = 1;
	}

	/**
	 * Labels, in importance order, skipping any that would collide.
	 *
	 * Overlapping text is worse than absent text: two names printed over each
	 * other leave you with neither, and the graph reads as noise. So the
	 * important nodes claim their box first — the focused context, the
	 * contexts, whatever the cursor is on — and a label that cannot find clear
	 * space is simply not drawn. Zooming in frees the space and it appears.
	 */
	function drawLabels(visible) {
		const size = 11 / state.scale;
		ctx2d.font = `500 ${size}px ${theme.font}`;
		ctx2d.textAlign = 'center';
		ctx2d.textBaseline = 'top';

		const rank = (n) => {
			if (n.focus) return 0;
			if (n === state.hover || n === state.selected) return 1;
			if (n.kind === 'contexto') return 2;
			if (n.kind === 'doc') return 3;
			if (n.kind === 'entidade') return 4;
			return 5;
		};
		const ordered = visible.slice().sort((a, b) => rank(a) - rank(b) || b.degree - a.degree);

		const taken = [];
		const pad = 2 / state.scale;
		for (const n of ordered) {
			const op = opacityOf(n) * (rank(n) <= 2 ? 1 : 0.82);
			if (op < 0.2) {
				continue;
			}
			const w = ctx2d.measureText(n.label).width;
			const x = n.x - w / 2;
			const y = n.y + radiusOf(n) + 4 / state.scale;
			const boxW = w + pad * 2;
			const boxH = size + pad * 2;

			let clash = false;
			for (const t of taken) {
				if (x < t.x + t.w && x + boxW > t.x && y < t.y + t.h && y + boxH > t.y) {
					clash = true;
					break;
				}
			}
			if (clash) {
				continue;
			}
			taken.push({ x, y, w: boxW, h: boxH });

			// Drawn as a stroke under the fill: the label stays readable where
			// it crosses a link, without a box that would clutter the picture.
			ctx2d.globalAlpha = op * 0.85;
			ctx2d.strokeStyle = theme.bg;
			ctx2d.lineWidth = 3 / state.scale;
			ctx2d.lineJoin = 'round';
			ctx2d.strokeText(n.label, n.x, y);

			ctx2d.globalAlpha = op;
			ctx2d.fillStyle = rank(n) <= 2 ? theme.fg : theme.dim;
			ctx2d.fillText(n.label, n.x, y);
		}
	}

	function dpr() {
		return window.devicePixelRatio || 1;
	}

	function resize() {
		const rect = canvas.getBoundingClientRect();
		canvas.width = Math.max(1, Math.round(rect.width * dpr()));
		canvas.height = Math.max(1, Math.round(rect.height * dpr()));
		requestDraw();
	}

	/**
	 * The box the drawing actually occupies, labels included.
	 *
	 * Not the same as the origin. Gravity here is deliberately weak — a strong
	 * pull to the centre would squash the clusters the picture exists to show —
	 * so the settled layout sits wherever the forces left it, which is rarely
	 * around (0,0).
	 */
	function contentBounds() {
		if (state.nodes.length === 0) {
			return null;
		}
		let x0 = Infinity;
		let y0 = Infinity;
		let x1 = -Infinity;
		let y1 = -Infinity;
		for (const n of state.nodes) {
			const r = radiusOf(n) + 18; // the label hangs below the node
			x0 = Math.min(x0, n.x - r);
			y0 = Math.min(y0, n.y - r);
			x1 = Math.max(x1, n.x + r);
			y1 = Math.max(y1, n.y + r);
		}
		return { x0, y0, x1, y1 };
	}

	/**
	 * Frames the whole graph in the canvas.
	 *
	 * Applied every frame while the layout settles, easing rather than
	 * snapping: the nodes are still moving, and a hard fit each tick reads as
	 * jitter. Any pan or zoom cancels it — once the user has taken the
	 * viewport, it is theirs.
	 */
	function fit(ease) {
		const box = contentBounds();
		const w = canvas.clientWidth;
		const h = canvas.clientHeight;
		if (!box || w <= 0 || h <= 0) {
			return;
		}
		const width = Math.max(1, box.x1 - box.x0);
		const height = Math.max(1, box.y1 - box.y0);
		// 0.92 leaves a margin, so nodes at the edge are not clipped by the
		// legend or the top bar.
		const scale = Math.min(4, Math.max(0.15, Math.min(w / width, h / height) * 0.92));
		const cx = (box.x0 + box.x1) / 2;
		const cy = (box.y0 + box.y1) / 2;
		const offsetX = w / 2 - cx * scale;
		const offsetY = h / 2 - cy * scale;

		if (!ease) {
			state.scale = scale;
			state.offsetX = offsetX;
			state.offsetY = offsetY;
			return;
		}
		const k = 0.18;
		state.scale += (scale - state.scale) * k;
		state.offsetX += (offsetX - state.offsetX) * k;
		state.offsetY += (offsetY - state.offsetY) * k;
	}

	/** The user taking the viewport ends the automatic framing. */
	function releaseFit() {
		state.autoFit = false;
	}

	window.addEventListener('resize', () => {
		resize();
		tick();
	});

	// ── interaction ───────────────────────────────────────────────────────

	function toWorld(clientX, clientY) {
		const rect = canvas.getBoundingClientRect();
		return {
			x: (clientX - rect.left - state.offsetX) / state.scale,
			y: (clientY - rect.top - state.offsetY) / state.scale,
		};
	}

	function nodeAt(clientX, clientY) {
		const p = toWorld(clientX, clientY);
		let best = null;
		let bestDistance = Infinity;
		for (const n of state.nodes) {
			const dx = n.x - p.x;
			const dy = n.y - p.y;
			const d = Math.sqrt(dx * dx + dy * dy);
			const reach = radiusOf(n) + 4 / state.scale;
			if (d <= reach && d < bestDistance) {
				best = n;
				bestDistance = d;
			}
		}
		return best;
	}

	/** The hovered node plus everything one hop away. */
	function neighbourhood(node) {
		const set = new Set();
		if (!node) {
			return set;
		}
		set.add(node.id);
		for (const l of state.adjacent.get(node.id) || []) {
			set.add(l.node.id);
		}
		return set;
	}

	canvas.addEventListener('mousemove', (event) => {
		state.pointer = { x: event.clientX, y: event.clientY };

		if (state.dragging) {
			const p = toWorld(event.clientX, event.clientY);
			state.dragging.x = p.x;
			state.dragging.y = p.y;
			state.alpha = Math.max(state.alpha, 0.25);
			tick();
			return;
		}
		if (state.panning) {
			releaseFit();
			state.offsetX += event.movementX;
			state.offsetY += event.movementY;
			requestDraw();
			return;
		}

		const hit = nodeAt(event.clientX, event.clientY);
		if (hit === state.hover) {
			return;
		}
		state.hover = hit;
		state.near = neighbourhood(hit);
		canvas.title = hit ? tooltip(hit) : '';
		requestDraw();
	});

	function tooltip(n) {
		if (n.kind === 'contexto') {
			return `${n.label} — contexto (clique para abrir)`;
		}
		if (n.kind === 'pacote') {
			return `${n.dir} — ${n.files} arquivo${n.files > 1 ? 's' : ''}`;
		}
		if (n.kind === 'entidade') {
			const shared = n.contexts > 1 ? ` — ${n.contexts} contextos` : '';
			return `${n.label} — tabela${shared}`;
		}
		const version = n.version ? ` v${n.version}` : '';
		const status = n.status ? ` · ${n.status}` : '';
		return `${n.path}${version}${status}`;
	}

	canvas.addEventListener('mousedown', (event) => {
		const hit = nodeAt(event.clientX, event.clientY);
		if (hit) {
			// Dragging a node moves the drawing under a viewport the user is
			// looking at; re-framing mid-drag would fight the hand.
			releaseFit();
			state.dragging = hit;
			select(hit);
		} else {
			state.panning = true;
			canvas.classList.add('dragging');
		}
	});

	window.addEventListener('mouseup', () => {
		state.dragging = null;
		state.panning = false;
		canvas.classList.remove('dragging');
	});

	canvas.addEventListener('dblclick', (event) => {
		const hit = nodeAt(event.clientX, event.clientY);
		if (!hit) {
			return;
		}
		activate(hit);
	});

	canvas.addEventListener(
		'wheel',
		(event) => {
			event.preventDefault();
			const rect = canvas.getBoundingClientRect();
			const mx = event.clientX - rect.left;
			const my = event.clientY - rect.top;
			releaseFit();
			const factor = Math.exp(-event.deltaY * 0.0015);
			const next = Math.min(4, Math.max(0.15, state.scale * factor));
			// Zoom about the cursor, not the origin: zooming into the middle of
			// the screen while the cursor points somewhere else is the fastest
			// way to lose the node you were looking at.
			state.offsetX = mx - (mx - state.offsetX) * (next / state.scale);
			state.offsetY = my - (my - state.offsetY) * (next / state.scale);
			state.scale = next;
			requestDraw();
		},
		{ passive: false },
	);

	/**
	 * What a node does when opened: a context switches the view to it, anything
	 * backed by a file opens that file.
	 */
	function activate(n) {
		if (n.kind === 'contexto' && !n.focus) {
			vscode.postMessage({ type: 'focus', context: n.context });
			return;
		}
		if (n.kind === 'pacote') {
			vscode.postMessage({ type: 'open', dir: n.dir });
			return;
		}
		if (n.path) {
			vscode.postMessage({ type: 'open', path: n.path });
		}
	}

	// ── details pane ──────────────────────────────────────────────────────

	function select(n) {
		state.selected = n;
		showNodeDetails(n);
		requestDraw();
	}

	function hideDetails() {
		details.box.hidden = true;
	}

	function section(title) {
		const h = document.createElement('h3');
		h.textContent = title;
		details.body.append(h);
	}

	function list(items, onClick) {
		const ul = document.createElement('ul');
		for (const item of items) {
			const li = document.createElement('li');
			if (onClick) {
				const button = document.createElement('button');
				button.className = 'link';
				button.textContent = item.label;
				button.addEventListener('click', () => onClick(item));
				li.append(button);
			} else {
				li.textContent = item.label;
			}
			ul.append(li);
		}
		details.body.append(ul);
	}

	function tags(values) {
		const wrap = document.createElement('div');
		for (const value of values) {
			const span = document.createElement('span');
			span.className = 'tag';
			span.textContent = value;
			wrap.append(span);
		}
		details.body.append(wrap);
	}

	function note(text) {
		const p = document.createElement('p');
		p.className = 'empty';
		p.textContent = text;
		details.body.append(p);
	}

	function action(label, handler) {
		const button = document.createElement('button');
		button.className = 'chip action';
		button.textContent = label;
		button.addEventListener('click', handler);
		details.body.append(button);
	}

	/** Everything reachable from a node in one hop, grouped for reading. */
	function linksOf(node) {
		return state.adjacent.get(node.id) || [];
	}

	function showNodeDetails(n) {
		details.box.hidden = false;
		details.body.innerHTML = '';
		details.title.textContent = n.kind === 'pacote' ? n.dir : n.label;

		if (n.kind === 'contexto') {
			details.meta.textContent = n.focus ? 'contexto em foco' : 'contexto';
			const neighbours = linksOf(n)
				.map((l) => l.node)
				.filter((x) => x.kind === 'doc');
			if (neighbours.length > 0) {
				section(`documentos (${neighbours.length})`);
				list(
					neighbours.map((x) => ({ label: x.label, node: x })),
					(item) => activate(item.node),
				);
			}
			if (!n.focus) {
				action('abrir este contexto', () => vscode.postMessage({ type: 'focus', context: n.context }));
			}
			if (n.focus) {
				appendHistory();
			}
			return;
		}

		if (n.kind === 'entidade') {
			details.meta.textContent = n.contexts > 1
				? `tabela do schema · descrita por ${n.contexts} contextos`
				: 'tabela do schema';
			if (n.common) {
				note('Tabela comum a boa parte do corpus: os pacotes que a implementam não foram expandidos, porque seriam quase o repositório inteiro.');
			}
			const related = linksOf(n);
			const docs = related.filter((l) => l.node.kind === 'doc').map((l) => l.node);
			const packages = related.filter((l) => l.node.kind === 'pacote').map((l) => l.node);
			if (docs.length > 0) {
				section(`documentada em (${docs.length})`);
				list(docs.map((d) => ({ label: d.label, node: d })), (item) => activate(item.node));
			}
			if (packages.length > 0) {
				section(`implementada em (${packages.length})`);
				list(packages.map((p) => ({ label: p.dir, node: p })), (item) => activate(item.node));
			}
			if (docs.length === 0 && packages.length === 0) {
				note('Nada aponta para esta tabela nesta visão.');
			}
			return;
		}

		if (n.kind === 'pacote') {
			details.meta.textContent = `${n.files} arquivo${n.files > 1 ? 's' : ''}${
				n.declared ? ' · declara //gofi:context' : ''
			}`;
			const entities = linksOf(n)
				.filter((l) => l.node.kind === 'entidade')
				.map((l) => l.node.label);
			if (entities.length > 0) {
				section('tabelas');
				tags(entities);
			}
			action('revelar no explorador', () => vscode.postMessage({ type: 'open', dir: n.dir }));
			return;
		}

		const meta = [KIND_LABEL[n.corpus] || n.corpus];
		if (n.version) {
			meta.push(`v${n.version}`);
		}
		meta.push(n.status || 'sem status');
		if (n.context) {
			meta.push(n.context);
		}
		details.meta.textContent = meta.join(' · ');

		const related = linksOf(n);
		const entities = related.filter((l) => l.node.kind === 'entidade').map((l) => l.node.label);
		if (entities.length > 0) {
			section('tabelas que descreve');
			tags(entities);
		}
		const docs = related.filter((l) => l.node.kind === 'doc');
		if (docs.length > 0) {
			section(`ligado a (${docs.length})`);
			list(
				docs.map((l) => ({ label: `${l.node.label} — ${l.kinds.join(', ')}`, node: l.node })),
				(item) => activate(item.node),
			);
		}
		const contexts = related.filter((l) => l.node.kind === 'contexto' && !l.node.focus);
		if (contexts.length > 0) {
			section('alcança os contextos');
			list(
				contexts.map((l) => ({ label: l.node.label, node: l.node })),
				(item) => vscode.postMessage({ type: 'focus', context: item.node.context }),
			);
		}
		action('abrir o documento', () => vscode.postMessage({ type: 'open', path: n.path }));
	}

	/** The context view opens on the context itself, history included. */
	function showContextDetails() {
		const focus = state.nodes.find((x) => x.focus);
		if (focus) {
			select(focus);
		}
	}

	function appendHistory() {
		section('histórico de versões');
		if (state.history.length === 0) {
			note('A memória deste contexto não registra versões ainda.');
			return;
		}
		list(state.history.map((h) => ({ label: h.text })));
	}

	details.close.addEventListener('click', hideDetails);

	// ── bar ───────────────────────────────────────────────────────────────

	bar.back.addEventListener('click', () => vscode.postMessage({ type: 'focus', context: '' }));
	bar.refresh.addEventListener('click', () => vscode.postMessage({ type: 'ready' }));
	bar.center.addEventListener('click', () => {
		state.autoFit = true;
		fit(false);
		state.autoFit = false;
		requestDraw();
	});
	bar.entities.addEventListener('change', () =>
		vscode.postMessage({ type: 'entities', on: bar.entities.checked }),
	);
	bar.search.addEventListener('input', () => {
		state.query = bar.search.value.trim().toLowerCase();
		requestDraw();
	});
	empty.build.addEventListener('click', () => vscode.postMessage({ type: 'build' }));

	window.addEventListener('keydown', (event) => {
		if (event.key === 'Escape') {
			if (!details.box.hidden) {
				hideDetails();
				return;
			}
			if (state.view === 'contexto') {
				vscode.postMessage({ type: 'focus', context: '' });
			}
		}
	});

	/** Node colours live in the stylesheet; canvas reads them from there. */
	function readPalette() {
		const styles = getComputedStyle(document.documentElement);
		const read = (name, fallback) => (styles.getPropertyValue(name) || fallback).trim();
		return {
			contexto: read('--node-context', '#22d3ee'),
			specs: read('--node-spec', '#7dd3fc'),
			prd: read('--node-prd', '#a78bfa'),
			memory: read('--node-memory', '#fbbf24'),
			entidade: read('--node-entity', '#34d399'),
			pacote: read('--node-package', '#94a3b8'),
		};
	}

	resize();
	vscode.postMessage({ type: 'ready' });
})();
