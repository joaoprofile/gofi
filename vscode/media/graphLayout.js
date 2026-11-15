// What the graph view computes, apart from what it draws: which nodes of the
// project's tree are on screen while branches are folded, which links cross
// between them, and where a simulation puts each one.
//
// It is kept apart from the page so it runs — and is tested — without a
// browser: nothing here touches the document.
(function (root, factory) {
	if (typeof module === 'object' && module.exports) {
		module.exports = factory();
	} else {
		root.GofiGraph = factory();
	}
})(typeof self !== 'undefined' ? self : this, function () {
	'use strict';

	// ── the tree ─────────────────────────────────────────────────────────

	/**
	 * Flattens the project's tree into records by id, each with its parent,
	 * its depth, and what it holds: how many leaves and how many lines.
	 *
	 * @returns {Map<string, object>}
	 */
	function index(tree) {
		const nodes = new Map();
		const walk = (raw, parent, depth) => {
			const node = { ...raw, parent: parent ? parent.id : '', depth, group: Array.isArray(raw.children), children: [] };
			nodes.set(node.id, node);
			if (!node.group) {
				node.value = Math.max(raw.size || 1, 1);
				node.leaves = 1;
				return node;
			}
			node.value = 0;
			node.leaves = 0;
			for (const child of raw.children) {
				const c = walk(child, node, depth + 1);
				node.children.push(c.id);
				node.value += c.value;
				node.leaves += c.leaves;
			}
			return node;
		};
		walk(tree, null, 0);
		return nodes;
	}

	/** A node and the groups over it, itself first. */
	function lineage(nodes, id) {
		const out = [];
		for (let n = nodes.get(id); n; n = nodes.get(n.parent)) {
			out.push(n.id);
		}
		return out;
	}

	/**
	 * The nodes on screen: the root, and everything under a group that is
	 * open — the open groups themselves stay, as the hub of what they hold.
	 *
	 * @param {Map<string, object>} nodes
	 * @param {Set<string>} open
	 * @returns {string[]} in the order of the tree
	 */
	function visible(nodes, open) {
		const out = [];
		const walk = (id) => {
			const n = nodes.get(id);
			if (!n) {
				return;
			}
			out.push(id);
			if (n.group && open.has(id)) {
				n.children.forEach(walk);
			}
		};
		const first = nodes.keys().next();
		if (!first.done) {
			walk(first.value);
		}
		return out;
	}

	/**
	 * The links between what is on screen. A link into a folded branch ends
	 * at the group that hides it, and the links that end up between the same
	 * two nodes are one, as heavy as all of them. A link between a group and
	 * something inside it says nothing the tree does not, and is left out.
	 *
	 * @param {Map<string, object>} nodes
	 * @param {Set<string>} shown what visible() returned
	 * @param {Array<{from: string, to: string, kind: string, weight: number}>} links
	 */
	function aggregate(nodes, shown, links) {
		const seen = new Map();
		const stand = (id) => {
			if (seen.has(id)) {
				return seen.get(id);
			}
			let at = id;
			while (at && !shown.has(at)) {
				const n = nodes.get(at);
				at = n ? n.parent : '';
			}
			seen.set(id, at);
			return at;
		};
		const inside = (id, group) => {
			for (let n = nodes.get(id); n; n = nodes.get(n.parent)) {
				if (n.id === group) {
					return true;
				}
			}
			return false;
		};
		const merged = new Map();
		for (const l of links) {
			const a = stand(l.from);
			const b = stand(l.to);
			if (!a || !b || a === b || inside(a, b) || inside(b, a)) {
				continue;
			}
			const key = `${a}\n${b}\n${l.kind}`;
			const hit = merged.get(key);
			if (hit) {
				hit.weight += l.weight || 1;
			} else {
				merged.set(key, { from: a, to: b, kind: l.kind, weight: l.weight || 1 });
			}
		}
		return [...merged.values()];
	}

	/**
	 * The groups a project opens with: the ones that say how it is organised —
	 * the sections and each tree of code — and, under them, a folder that is
	 * all its parent holds. The rest opens when it is asked for, so the first
	 * picture is the project's shape and not its files.
	 *
	 * @returns {Set<string>}
	 */
	function opening(nodes) {
		const open = new Set();
		const walk = (id) => {
			const n = nodes.get(id);
			open.add(id);
			for (const c of n.children.map((child) => nodes.get(child))) {
				if (c.kind === 'section' || c.kind === 'scope' || (c.kind === 'folder' && n.children.length === 1)) {
					walk(c.id);
				}
			}
		};
		const first = nodes.keys().next();
		if (!first.done) {
			walk(first.value);
		}
		return open;
	}

	/**
	 * The room each node on screen needs: how far from its centre it reaches
	 * with everything it shows — its own radius, or, for a group that is
	 * open, the disc what it holds fits in, each thing with its name under
	 * it — and, for an open group, how far from it the folded things it holds
	 * stand. It is what keeps two open branches from landing on each other;
	 * past a point it stops growing, and the nodes pushing each other do the
	 * rest.
	 *
	 * @returns {{ reach: Map<string, number>, ring: Map<string, number> }}
	 */
	function room(nodes, open, shown) {
		const reach = new Map();
		const ring = new Map();
		const of = (id) => {
			if (reach.has(id)) {
				return reach.get(id);
			}
			const n = nodes.get(id);
			const r = radius(n, open.has(id));
			let total = r;
			if (n.group && open.has(id)) {
				let area = 0;
				let folded = 0;
				let widest = 0;
				for (const c of n.children) {
					// A group's name is always drawn: it takes more room than a leaf's.
					const cr = of(c) + (nodes.get(c).group ? 24 : 13);
					area += cr * cr;
					widest = Math.max(widest, cr);
					if (!open.has(c)) {
						folded += cr * cr;
					}
				}
				// What is folded stands in rings around the group; a branch that
				// is open stands further out, by its own reach.
				ring.set(id, Math.min(280, Math.max(Math.sqrt(folded) * 0.62, 26)));
				total = Math.min(700, r + 8 + Math.max(Math.sqrt(area) * 1.05, widest * 1.25 + 16));
			}
			reach.set(id, total);
			return total;
		};
		for (const id of shown) {
			of(id);
		}
		return { reach, ring };
	}

	/** How large a node is drawn: a group by what it holds, a leaf by its lines. */
	function radius(node, open) {
		if (node.group) {
			const r = Math.min(30, 9 + Math.sqrt(node.leaves) * 2.1);
			return open ? Math.max(6, r * 0.42) : r;
		}
		return Math.max(4, Math.min(12, 3 + Math.sqrt(node.value) * 0.34));
	}

	/** A number in [0, 1) that is always the same for the same text. */
	function hash(text) {
		let h = 2166136261;
		for (let i = 0; i < text.length; i++) {
			h ^= text.charCodeAt(i);
			h = Math.imul(h, 16777619);
		}
		return ((h >>> 0) % 100000) / 100000;
	}

	// ── the simulation ───────────────────────────────────────────────────

	/**
	 * Where each node goes: links pull what is tied together, every node
	 * pushes the others away, nothing overlaps, and the whole is held around
	 * the origin. It cools as it settles, and is warmed again when something
	 * changes — a branch opened, a node dragged.
	 *
	 * Bodies are { id, x, y, vx, vy, r, fx, fy } and may say how hard they
	 * push (q, negative) and how much room they keep around them (pad); a body
	 * with fx and fy stays where it was put. Springs are { source, target,
	 * distance, strength }.
	 */
	class Simulation {
		constructor() {
			this.bodies = [];
			this.springs = [];
			this.alpha = 1;
			this.alphaMin = 0.004;
			this.alphaDecay = 0.028;
			this.alphaTarget = 0;
			this.friction = 0.58;
			this.theta2 = 0.81;
			this.reach = 1100; // past this, two nodes do not push each other
			this.aspect = 1; // the picture's width over its height: it settles into that shape
		}

		set(bodies, springs) {
			this.bodies = bodies;
			this.springs = springs;
			// With a great many nodes, far groups are taken as one from closer by.
			this.theta2 = bodies.length > 1200 ? 1.44 : 0.81;
			const degree = new Map();
			for (const s of springs) {
				degree.set(s.source, (degree.get(s.source) || 0) + 1);
				degree.set(s.target, (degree.get(s.target) || 0) + 1);
			}
			for (const s of springs) {
				// The lighter end moves more: a hub is not dragged by a leaf.
				s.bias = degree.get(s.source) / (degree.get(s.source) + degree.get(s.target));
			}
		}

		warm(alpha) {
			this.alpha = Math.max(this.alpha, alpha === undefined ? 0.6 : alpha);
		}

		get active() {
			return this.alpha > this.alphaMin || this.alphaTarget > this.alphaMin;
		}

		/** One step. Returns whether it is still moving. */
		tick() {
			this.alpha += (this.alphaTarget - this.alpha) * this.alphaDecay;
			const a = this.alpha;
			const bodies = this.bodies;

			for (const s of this.springs) {
				const p = s.source;
				const q = s.target;
				let dx = q.x + q.vx - p.x - p.vx || 1e-6;
				let dy = q.y + q.vy - p.y - p.vy || 1e-6;
				const d = Math.sqrt(dx * dx + dy * dy);
				const k = ((d - s.distance) / d) * a * s.strength;
				dx *= k;
				dy *= k;
				q.vx -= dx * s.bias;
				q.vy -= dy * s.bias;
				p.vx += dx * (1 - s.bias);
				p.vy += dy * (1 - s.bias);
			}

			this.repel(a);

			// Held around the origin, the far ones the most — and flatter on a
			// wide canvas, so the picture uses the room it has.
			const wide = Math.max(0.6, Math.min(2, this.aspect));
			for (const b of bodies) {
				b.vx -= (b.x * 0.01 * a) / wide;
				b.vy -= b.y * 0.01 * a * wide;
			}

			this.separate();

			for (const b of bodies) {
				if (b.fx !== null && b.fx !== undefined) {
					b.x = b.fx;
					b.y = b.fy;
					b.vx = 0;
					b.vy = 0;
					continue;
				}
				b.vx *= this.friction;
				b.vy *= this.friction;
				b.x += b.vx;
				b.y += b.vy;
			}
			return this.active;
		}

		/**
		 * Runs until it cools, for a picture that opens already in place — or
		 * for as long as it is given, when the picture is too large to wait for.
		 */
		settle(limit, budget) {
			const until = budget ? Date.now() + budget : Infinity;
			for (let i = 0; i < (limit || 400) && this.alpha > this.alphaMin && Date.now() < until; i++) {
				this.tick();
			}
		}

		/** Every node pushes the others, far groups of them taken as one. */
		repel(a) {
			const bodies = this.bodies;
			if (bodies.length < 2) {
				return;
			}
			let x0 = Infinity;
			let y0 = Infinity;
			let x1 = -Infinity;
			let y1 = -Infinity;
			for (const b of bodies) {
				if (!(b.q < 0)) {
					b.q = -(46 + b.r * b.r * 0.55);
				}
				x0 = Math.min(x0, b.x);
				y0 = Math.min(y0, b.y);
				x1 = Math.max(x1, b.x);
				y1 = Math.max(y1, b.y);
			}
			const top = { x: x0, y: y0, s: Math.max(x1 - x0, y1 - y0, 1), body: null, more: null, kids: null, q: 0, cx: 0, cy: 0 };
			for (const b of bodies) {
				insert(top, b);
			}
			weigh(top);

			const reach2 = this.reach * this.reach;
			const stack = [];
			for (const b of bodies) {
				stack.length = 0;
				stack.push(top);
				while (stack.length > 0) {
					const cell = stack.pop();
					let dx = cell.cx - b.x;
					let dy = cell.cy - b.y;
					let d2 = dx * dx + dy * dy;
					if (cell.kids) {
						if ((cell.s * cell.s) / this.theta2 < d2) {
							if (d2 < reach2) {
								b.vx += (dx * cell.q * a) / d2;
								b.vy += (dy * cell.q * a) / d2;
							}
							continue;
						}
						for (const kid of cell.kids) {
							if (kid) {
								stack.push(kid);
							}
						}
						continue;
					}
					for (let other = cell.body, i = -1; other; other = cell.more && cell.more[++i]) {
						if (other === b) {
							continue;
						}
						dx = other.x - b.x || (hash(other.id) - 0.5) * 0.1;
						dy = other.y - b.y || (hash(b.id) - 0.5) * 0.1;
						d2 = dx * dx + dy * dy;
						if (d2 >= reach2) {
							continue;
						}
						const near = (b.r + other.r) * (b.r + other.r) * 0.25;
						if (d2 < near) {
							d2 = Math.sqrt(near * d2) || near; // two nodes on each other are not flung apart
						}
						b.vx += (dx * other.q * a) / d2;
						b.vy += (dy * other.q * a) / d2;
					}
				}
			}
		}

		/** Two nodes that overlap are moved apart, the smaller one the most. */
		separate() {
			const bodies = this.bodies;
			let widest = 0;
			for (const b of bodies) {
				b.room = b.r + (b.pad || 3);
				widest = Math.max(widest, b.room);
			}
			const size = widest * 2 + 2;
			const grid = new Map();
			const key = (cx, cy) => (cx + 32768) * 65536 + (cy + 32768);
			for (const b of bodies) {
				b.gx = Math.floor((b.x + b.vx) / size);
				b.gy = Math.floor((b.y + b.vy) / size);
				const k = key(b.gx, b.gy);
				(grid.get(k) || grid.set(k, []).get(k)).push(b);
			}
			for (const b of bodies) {
				for (let ox = -1; ox <= 1; ox++) {
					for (let oy = -1; oy <= 1; oy++) {
						const cell = grid.get(key(b.gx + ox, b.gy + oy));
						if (!cell) {
							continue;
						}
						for (const o of cell) {
							if (o.id <= b.id) {
								continue; // each pair once
							}
							let dx = b.x + b.vx - o.x - o.vx || (hash(b.id) - 0.5) * 0.1;
							let dy = b.y + b.vy - o.y - o.vy || (hash(o.id) - 0.5) * 0.1;
							const min = b.room + o.room;
							const d2 = dx * dx + dy * dy;
							if (d2 >= min * min) {
								continue;
							}
							const d = Math.sqrt(d2);
							const push = ((min - d) / d) * 0.5;
							dx *= push;
							dy *= push;
							const share = (o.r * o.r) / (b.r * b.r + o.r * o.r);
							b.vx += dx * share;
							b.vy += dy * share;
							o.vx -= dx * (1 - share);
							o.vy -= dy * (1 - share);
						}
					}
				}
			}
		}
	}

	/**
	 * Turns the picture so that it is long the way the canvas is: a tree laid
	 * out tall on a wide screen leaves half of it empty and is drawn small.
	 * It turns around its middle, so nothing changes but the direction.
	 */
	function level(bodies, aspect) {
		if (bodies.length < 3) {
			return;
		}
		let mx = 0;
		let my = 0;
		for (const b of bodies) {
			mx += b.x;
			my += b.y;
		}
		mx /= bodies.length;
		my /= bodies.length;
		let xx = 0;
		let yy = 0;
		let xy = 0;
		for (const b of bodies) {
			xx += (b.x - mx) * (b.x - mx);
			yy += (b.y - my) * (b.y - my);
			xy += (b.x - mx) * (b.y - my);
		}
		// The direction the picture is longest in, to lie along the canvas's.
		const turn = (aspect < 1 ? Math.PI / 2 : 0) - 0.5 * Math.atan2(2 * xy, xx - yy);
		const cos = Math.cos(turn);
		const sin = Math.sin(turn);
		for (const b of bodies) {
			const dx = b.x - mx;
			const dy = b.y - my;
			b.x = mx + dx * cos - dy * sin;
			b.y = my + dx * sin + dy * cos;
			const vx = b.vx;
			b.vx = vx * cos - b.vy * sin;
			b.vy = vx * sin + b.vy * cos;
		}
	}

	function insert(cell, body) {
		for (;;) {
			if (!cell.kids) {
				if (!cell.body) {
					cell.body = body;
					return;
				}
				if (cell.s < 0.01) {
					(cell.more || (cell.more = [])).push(body); // the same spot: kept together
					return;
				}
				const held = cell.body;
				cell.body = null;
				cell.kids = [null, null, null, null];
				quadrant(cell, held).body = held;
			}
			cell = quadrant(cell, body);
			if (!cell.body && !cell.kids) {
				cell.body = body;
				return;
			}
		}
	}

	function quadrant(cell, body) {
		const half = cell.s / 2;
		const i = (body.x >= cell.x + half ? 1 : 0) | (body.y >= cell.y + half ? 2 : 0);
		return cell.kids[i] || (cell.kids[i] = { x: cell.x + (i & 1 ? half : 0), y: cell.y + (i & 2 ? half : 0), s: half, body: null, more: null, kids: null, q: 0, cx: 0, cy: 0 });
	}

	/** How hard a cell pushes, and from where: the sum of what is in it. */
	function weigh(cell) {
		let q = 0;
		let x = 0;
		let y = 0;
		if (cell.kids) {
			for (const kid of cell.kids) {
				if (kid) {
					weigh(kid);
					const w = Math.abs(kid.q);
					q += kid.q;
					x += kid.cx * w;
					y += kid.cy * w;
				}
			}
		} else {
			for (let b = cell.body, i = -1; b; b = cell.more && cell.more[++i]) {
				const w = Math.abs(b.q);
				q += b.q;
				x += b.x * w;
				y += b.y * w;
			}
		}
		const total = Math.abs(q) || 1;
		cell.q = q;
		cell.cx = x / total;
		cell.cy = y / total;
	}

	return { index, lineage, visible, aggregate, opening, room, radius, hash, level, Simulation };
});
