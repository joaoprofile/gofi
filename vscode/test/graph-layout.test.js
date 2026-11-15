'use strict';

// What the graph view computes apart from what it draws: which nodes are on
// screen while branches are folded, which links cross between them, and where
// the simulation puts each one.
const assert = require('assert');
const { runner } = require('./vscode-stub.js');
const G = require('../media/graphLayout.js');

const { test, run } = runner();

const leaf = (id, size) => ({ id, name: id, kind: 'file', size });
const group = (id, kind, children) => ({ id, name: id, kind, children });

/** A project with two contexts, a library and a tree of code three folders deep. */
function tree() {
	return group('root', 'root', [
		group('docs:contexts', 'section', [
			group('ctx:user', 'context', [leaf('user/spec.md', 300), leaf('user/prd.md', 80)]),
			group('ctx:billing', 'context', [leaf('billing/proposal.md', 30)]),
		]),
		group('docs:libraries', 'section', [group('lib:SDK', 'library', [leaf('sdk/http.md', 120)])]),
		group('scope:backend', 'scope', [
			group('dir:domain', 'folder', [
				group('dir:domain/user', 'folder', [leaf('user.go', 400), leaf('entity.go', 200)]),
				group('dir:domain/billing', 'folder', [leaf('charge.go', 100)]),
			]),
		]),
	]);
}

const LINKS = [
	{ from: 'user.go', to: 'entity.go', kind: 'code', weight: 2 },
	{ from: 'charge.go', to: 'entity.go', kind: 'code', weight: 1 },
	{ from: 'charge.go', to: 'user.go', kind: 'code', weight: 3 },
	{ from: 'user.go', to: 'ctx:user', kind: 'pertence', weight: 1 },
	{ from: 'entity.go', to: 'ctx:user', kind: 'pertence', weight: 1 },
	{ from: 'billing/proposal.md', to: 'ctx:user', kind: 'depende', weight: 1 },
	{ from: 'user/spec.md', to: 'ctx:user', kind: 'depende', weight: 1 }, // into its own group
];

const said = (links) => links.map((l) => `${l.from} -${l.kind}×${l.weight}-> ${l.to}`).sort();

test('the tree is indexed with what each group holds', () => {
	const nodes = G.index(tree());
	assert.strictEqual(nodes.size, 17);
	const root = nodes.get('root');
	assert.deepStrictEqual([root.parent, root.depth, root.leaves, root.value], ['', 0, 7, 1230]);
	const user = nodes.get('dir:domain/user');
	assert.deepStrictEqual([user.parent, user.depth, user.group, user.leaves, user.value, user.children], ['dir:domain', 3, true, 2, 600, ['user.go', 'entity.go']]);
	assert.deepStrictEqual([nodes.get('user.go').group, nodes.get('user.go').leaves], [false, 1]);
	assert.deepStrictEqual(G.lineage(nodes, 'user.go'), ['user.go', 'dir:domain/user', 'dir:domain', 'scope:backend', 'root']);
});

test('what is on screen is the root and what the open groups hold, the open groups included', () => {
	const nodes = G.index(tree());
	assert.deepStrictEqual(G.visible(nodes, new Set()), ['root']);
	assert.deepStrictEqual(G.visible(nodes, new Set(['root'])), ['root', 'docs:contexts', 'docs:libraries', 'scope:backend']);
	// A group that is open under one that is folded shows nothing.
	assert.deepStrictEqual(G.visible(nodes, new Set(['root', 'dir:domain'])), ['root', 'docs:contexts', 'docs:libraries', 'scope:backend']);
	assert.deepStrictEqual(G.visible(nodes, new Set(['root', 'scope:backend', 'dir:domain'])),
		['root', 'docs:contexts', 'docs:libraries', 'scope:backend', 'dir:domain', 'dir:domain/user', 'dir:domain/billing']);
});

test('a project opens on its shape: the sections and the trees of code, and a folder that is all its parent holds', () => {
	const nodes = G.index(tree());
	const open = G.opening(nodes);
	assert.deepStrictEqual([...open].sort(), ['dir:domain', 'docs:contexts', 'docs:libraries', 'root', 'scope:backend']);
	assert.ok(!open.has('ctx:user') && !open.has('lib:SDK') && !open.has('dir:domain/user'), 'a context, a library and a folder among others open when asked');
});

test('a link into a folded branch ends at the group that hides it, and links between the same two are one', () => {
	const nodes = G.index(tree());
	const shown = (open) => new Set(G.visible(nodes, new Set(open)));

	// Everything folded but the top: the code as a whole belongs to the contexts as a whole.
	assert.deepStrictEqual(said(G.aggregate(nodes, shown(['root']), LINKS)), ['scope:backend -pertence×2-> docs:contexts']);

	// The contexts in sight, the code down to its folders.
	const open = ['root', 'docs:contexts', 'scope:backend', 'dir:domain'];
	assert.deepStrictEqual(said(G.aggregate(nodes, shown(open), LINKS)), [
		'ctx:billing -depende×1-> ctx:user',
		'dir:domain/billing -code×4-> dir:domain/user', // 1 + 3, two files into two
		'dir:domain/user -pertence×2-> ctx:user',
	]);

	// A folder open: the links between its files show, and those into it name the file.
	assert.deepStrictEqual(said(G.aggregate(nodes, shown([...open, 'dir:domain/user']), LINKS)), [
		'ctx:billing -depende×1-> ctx:user',
		'dir:domain/billing -code×1-> entity.go',
		'dir:domain/billing -code×3-> user.go',
		'entity.go -pertence×1-> ctx:user',
		'user.go -code×2-> entity.go',
		'user.go -pertence×1-> ctx:user',
	]);

	// A link between a group and what is inside it says nothing the tree does not.
	const inside = G.aggregate(nodes, shown([...open, 'ctx:user']), LINKS);
	assert.ok(!inside.some((l) => l.from === 'user/spec.md'), 'a document does not point at the context that holds it');
});

test('an open group asks for the room of what it holds, and no more than a screen of it', () => {
	const nodes = G.index(tree());
	const open = new Set(['root', 'scope:backend', 'dir:domain', 'dir:domain/user']);
	const { reach, ring } = G.room(nodes, open, G.visible(nodes, open));
	assert.strictEqual(reach.get('user.go'), G.radius(nodes.get('user.go'), false));
	assert.strictEqual(reach.get('dir:domain/billing'), G.radius(nodes.get('dir:domain/billing'), false), 'folded, a group is its own size');
	assert.ok(reach.get('dir:domain/user') > reach.get('dir:domain/billing') + 20, 'open, it reaches around what it holds');
	assert.ok(reach.get('dir:domain') > reach.get('dir:domain/user'), 'and a group reaches around the groups in it');
	assert.ok(ring.get('dir:domain/user') >= 26 && !ring.has('dir:domain/billing'));
	assert.ok(G.radius(nodes.get('dir:domain/user'), true) < G.radius(nodes.get('dir:domain/user'), false), 'an open group is drawn as a small ring');

	// A thousand files in a folder do not ask for a thousand screens.
	const wide = G.index(group('root', 'root', [group('big', 'folder', Array.from({ length: 1500 }, (_, i) => leaf(`f${i}`, 200)))]));
	const all = new Set(['root', 'big']);
	const far = G.room(wide, all, G.visible(wide, all));
	assert.ok(far.reach.get('root') <= 700 && far.ring.get('big') <= 280);
});

/** Lays the project out, everything open, the way the page does. */
function laidOut(pin) {
	const nodes = G.index(tree());
	const open = new Set([...nodes.values()].filter((n) => n.group).map((n) => n.id));
	const shown = G.visible(nodes, open);
	const { reach, ring } = G.room(nodes, open, shown);
	const bodies = new Map();
	for (const id of shown) {
		const n = nodes.get(id);
		const p = bodies.get(n.parent);
		const turn = G.hash(id) * Math.PI * 2;
		const far = p ? p.r + 14 + G.hash(`${id}.`) * 40 : 0;
		bodies.set(id, { id, x: (p ? p.x : 0) + Math.cos(turn) * far, y: (p ? p.y : 0) + Math.sin(turn) * far, vx: 0, vy: 0, r: G.radius(n, open.has(id)), fx: null, fy: null, pad: 4 });
	}
	if (pin) {
		Object.assign(bodies.get(pin.id), { fx: pin.x, fy: pin.y });
	}
	const springs = [];
	for (const id of shown) {
		const p = bodies.get(nodes.get(id).parent);
		if (p) {
			springs.push({ source: p, target: bodies.get(id), distance: p.r + (open.has(id) ? 18 + reach.get(id) * 0.62 : ring.get(p.id)), strength: 0.7 });
		}
	}
	const sim = new G.Simulation();
	sim.set([...bodies.values()], springs);
	sim.settle();
	return { bodies, sim, springs };
}

test('the simulation settles with nothing on top of anything, and each thing near its group', () => {
	const { bodies, sim, springs } = laidOut();
	assert.ok(!sim.active, 'it cools and stops');
	const list = [...bodies.values()];
	assert.ok(list.every((b) => Number.isFinite(b.x) && Number.isFinite(b.y)));
	for (let i = 0; i < list.length; i++) {
		for (let j = i + 1; j < list.length; j++) {
			const d = Math.hypot(list[i].x - list[j].x, list[i].y - list[j].y);
			assert.ok(d >= list[i].r + list[j].r, `${list[i].id} and ${list[j].id} overlap`);
		}
	}
	for (const s of springs) {
		const d = Math.hypot(s.source.x - s.target.x, s.source.y - s.target.y);
		assert.ok(d < s.distance * 2.5 + 60, `${s.target.id} strayed from ${s.source.id}: ${Math.round(d)} for ${Math.round(s.distance)}`);
	}
});

test('the same project is laid out the same way every time, and a node put somewhere stays there', () => {
	const a = laidOut();
	const b = laidOut();
	for (const [id, body] of a.bodies) {
		assert.deepStrictEqual([body.x, body.y], [b.bodies.get(id).x, b.bodies.get(id).y], id);
	}
	const pinned = laidOut({ id: 'ctx:user', x: 500, y: -300 });
	assert.deepStrictEqual([pinned.bodies.get('ctx:user').x, pinned.bodies.get('ctx:user').y], [500, -300]);
	const doc = pinned.bodies.get('user/spec.md');
	assert.ok(Math.hypot(doc.x - 500, doc.y + 300) < 200, 'what it holds went with it');
});

test('a picture is turned to lie the way the canvas does, and nothing else of it changes', () => {
	const spread = (bodies) => {
		const xs = bodies.map((b) => b.x);
		const ys = bodies.map((b) => b.y);
		return [Math.max(...xs) - Math.min(...xs), Math.max(...ys) - Math.min(...ys)];
	};
	const far = (bodies) => bodies.map((b) => Math.round(Math.hypot(b.x - bodies[0].x, b.y - bodies[0].y)));
	// A tall line of nodes, leaning.
	const make = () => Array.from({ length: 9 }, (_, i) => ({ id: `n${i}`, x: 20 + i * 12 + (i % 2) * 6, y: -200 + i * 55, vx: 0, vy: 0 }));
	const wide = make();
	const before = far(wide);
	G.level(wide, 1.6);
	const [w, h] = spread(wide);
	assert.ok(w > h * 4, `on a wide canvas it lies down: ${Math.round(w)}x${Math.round(h)}`);
	assert.deepStrictEqual(far(wide), before, 'the distances between the nodes are the same');

	const tall = make();
	G.level(tall, 0.6);
	const [w2, h2] = spread(tall);
	assert.ok(h2 > w2 * 4, `on a tall canvas it stands: ${Math.round(w2)}x${Math.round(h2)}`);
});

test('the simulation is warmed by a change and holds its heat while a node is dragged', () => {
	const { sim } = laidOut();
	sim.warm(0.5);
	assert.ok(sim.active);
	sim.settle();
	assert.ok(!sim.active);
	sim.alphaTarget = 0.25;
	for (let i = 0; i < 400; i++) {
		sim.tick();
	}
	assert.ok(sim.active && sim.alpha > 0.2, 'dragging, it keeps moving');
	sim.alphaTarget = 0;
	sim.settle();
	assert.ok(!sim.active);
});

run();
