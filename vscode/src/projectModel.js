'use strict';

const fs = require('fs');
const path = require('path');

/**
 * The project as gofi indexed it — its documents and its code — in the three
 * shapes the graph page draws.
 *
 * `gofi find` answers a question with a path and a line range, which is the
 * right shape for an agent and the wrong one for a person asking what the
 * project is made of. That question is about shape, and each view answers one
 * side of it:
 *
 *   overview — how each context stands: what was written, what is specified,
 *              what is live, which code is its own, what it depends on; and
 *              each tree of code with its size.
 *   graph    — the whole project as a tree — contexts, libraries, each tree of
 *              code down to the source files — and the links that cross it,
 *              for the page to draw folded, a branch opened at a time. Nine
 *              hundred dots at once are a hairball; a branch is a picture.
 *   trace    — one context from its documents to its requirements, and from
 *              its tables to the code that implements them.
 *
 * Everything is read off disk — `.gofi/index/docs` and `.gofi/index/code` are
 * derived, `gofi index` owns them — and nothing is written. The functions are
 * pure over that folder, so the views are tested without an editor.
 */

const DOCS_DIR = path.join('.gofi', 'index', 'docs');
const CODE_DIR = path.join('.gofi', 'index', 'code');
const GRAPH_FILE = path.join(DOCS_DIR, 'graph.json');
const INDEX_FILE = path.join(DOCS_DIR, 'index.json');
const SCOPES_FILE = path.join(CODE_DIR, 'gofi_graph_index.json');
const CODE_GRAPH = 'gofi_graph.json';

const PREFIX = { ctx: 'ctx:', ent: 'ent:', code: 'code:', req: 'req:', cen: 'cen:' };

/** What a document is, by the type the index gave it. */
const DOC_KINDS = {
	prd: 'prd',
	spec: 'spec',
	design: 'design',
	mudanca: 'proposal',
	delta: 'delta',
	memoria: 'memory',
	memory: 'memory',
};

/** The order a context's documents are read in: from the problem to the state. */
const DOC_ORDER = ['prd', 'proposal', 'design', 'delta', 'spec', 'memory', 'doc'];

/** The libraries the agents read from, by the folder they are kept in. */
const LIBRARIES = [
	{ match: /(^|\/)sdk(\/|$)/, name: 'SDK' },
	{ match: /(^|\/)expertise(\/|$)/, name: 'Expertise' },
	{ match: /(^|\/)skills(\/|$)/, name: 'Skills' },
	{ match: /(^|\/)institutional(\/|$)/, name: 'Institucional' },
	{ match: /(^|\/)knowledge(\/|$)/, name: 'Knowledge' },
	{ match: /(^|\/)memory(\/|$)/, name: 'Memória' },
];

/** Parsed files, keyed by absolute path and invalidated by mtime. */
const cache = new Map();

function readJson(file) {
	let mtimeMs;
	try {
		mtimeMs = fs.statSync(file).mtimeMs;
	} catch {
		cache.delete(file);
		return null;
	}
	const hit = cache.get(file);
	if (hit && hit.mtimeMs === mtimeMs) {
		return hit.value;
	}
	let value = null;
	try {
		value = JSON.parse(fs.readFileSync(file, 'utf8'));
	} catch {
		value = null; // corrupt or half-written: the same as absent
	}
	cache.set(file, { mtimeMs, value });
	return value;
}

/** Whether the project has anything indexed to draw. */
function hasGraph(root) {
	return readJson(path.join(root, GRAPH_FILE)) !== null || scopes(root).length > 0;
}

/** A name written as a placeholder in a template, not a thing of the project. */
function placeholder(name) {
	return name.startsWith('{') || name.includes('{');
}

function baseName(p) {
	const i = p.lastIndexOf('/');
	return i >= 0 ? p.slice(i + 1) : p;
}

function dirName(p) {
	const i = p.lastIndexOf('/');
	return i >= 0 ? p.slice(0, i) : '.';
}

/** The last two segments of a folder: the layer, and whose layer it is. */
function tail(dir) {
	const parts = dir.split('/').filter(Boolean);
	return parts.length > 2 ? parts.slice(-2).join('/') : parts.join('/') || '.';
}

/** The trees of code gofi scanned, each with its graph. */
function scopes(root) {
	const index = readJson(path.join(root, SCOPES_FILE));
	const list = index && Array.isArray(index.scopes)
		? index.scopes
		: [{ name: 'project', dir: '.', root: '.' }];
	const out = [];
	for (const s of list) {
		const dir = !s.dir || s.dir === '.' ? '' : s.dir;
		const graph = readJson(path.join(root, CODE_DIR, dir, CODE_GRAPH));
		if (!graph || !graph.files) {
			continue;
		}
		const base = !s.root || s.root === '.' ? '' : `${String(s.root).replace(/[/\\]+$/, '')}/`;
		out.push({
			name: s.name === 'project' && base ? base.replace(/\/$/, '') : s.name || 'project',
			base,
			language: graph.language || s.language || '',
			framework: graph.framework || s.framework || '',
			module: graph.module || '',
			graph,
		});
	}
	return out;
}

/**
 * Everything the views read, joined once per build of the index.
 *
 * @returns {object | null} null when nothing was ever indexed
 */
function load(root) {
	const graph = readJson(path.join(root, GRAPH_FILE));
	const codeScopes = scopes(root);
	if (!graph && codeScopes.length === 0) {
		return null;
	}
	const stamp = [graph, ...codeScopes.map((s) => s.graph)];
	const hit = cache.get(`model:${root}`);
	if (hit && hit.stamp.length === stamp.length && hit.stamp.every((g, i) => g === stamp[i])) {
		return hit.value;
	}

	const nos = (graph && graph.nos) || {};
	const arestas = (graph && Array.isArray(graph.arestas) && graph.arestas) || [];
	const index = readJson(path.join(root, INDEX_FILE));
	/** @type {Map<string, object>} */
	const entries = new Map();
	for (const d of (index && index.docs) || []) {
		entries.set(d.path, d);
	}

	/** @type {Map<string, Array<[string, string]>>} */
	const out = new Map();
	/** @type {Map<string, Array<[string, string]>>} */
	const into = new Map();
	for (const [from, to, rel] of arestas) {
		(out.get(from) || out.set(from, []).get(from)).push([to, rel]);
		(into.get(to) || into.set(to, []).get(to)).push([from, rel]);
	}

	// The documents, each with what the index knows of it.
	const docs = new Map();
	const titles = new Map(); // requirement and scenario titles, from the headings
	for (const [id, node] of Object.entries(nos)) {
		if (node.tipo !== 'doc') {
			continue;
		}
		const entry = entries.get(id) || {};
		const secs = Array.isArray(entry.secs) ? entry.secs : [];
		const lines = secs.reduce((max, s) => Math.max(max, s.fim || 0), 0);
		const archived = id.includes('/archive/');
		docs.set(id, {
			id,
			path: id,
			name: baseName(id),
			title: entry.titulo || '',
			kind: DOC_KINDS[entry.tipo] || 'doc',
			ctx: node.ctx || entry.ctx || '',
			corpus: node.corpus || '',
			status: node.status || entry.status || '',
			version: node.versao || '',
			lines: lines || 1,
			sections: secs.length,
			archived,
		});
		for (const s of secs) {
			const m = /^(?:Requisito|Cenário|Cenario|Requirement|Scenario):\s*(REQ-[\d.]+)\s*[—-]\s*(.+)$/.exec(s.h || '');
			if (m && (node.ctx || entry.ctx)) {
				titles.set(`${node.ctx || entry.ctx}/${m[1]}`, { title: m[2].trim(), path: id, line: s.ini });
			}
		}
	}

	// The source files, from every tree, by their path in the project.
	const files = new Map();
	for (const scope of codeScopes) {
		const symbols = new Map();
		for (const n of scope.graph.nodes || []) {
			if (n.file && n.kind !== 'package') {
				symbols.set(n.file, (symbols.get(n.file) || 0) + 1);
			}
		}
		for (const [rel, f] of Object.entries(scope.graph.files || {})) {
			const p = scope.base + rel;
			files.set(p, {
				id: PREFIX.code + p,
				path: p,
				name: baseName(p),
				scope: scope.name,
				unit: f.unit || dirName(p),
				dir: dirName(p),
				lines: f.lines || 1,
				symbols: symbols.get(rel) || 0,
			});
		}
	}

	const value = { root, nos, out, into, docs, files, titles, scopes: codeScopes };
	cache.set(`model:${root}`, { stamp, value });
	return value;
}

function targets(model, id, rel) {
	return (model.out.get(id) || []).filter((e) => !rel || e[1] === rel).map((e) => e[0]);
}

function sources(model, id, rel) {
	return (model.into.get(id) || []).filter((e) => !rel || e[1] === rel).map((e) => e[0]);
}

function unique(list) {
	return [...new Set(list)];
}

/** The contexts of the project, by name. */
function contextNames(model) {
	return Object.keys(model.nos)
		.filter((id) => model.nos[id].tipo === 'contexto')
		.map((id) => id.slice(PREFIX.ctx.length))
		.filter((name) => !placeholder(name))
		.sort();
}

/** One context, with everything that is its own. */
function contextOf(model, name) {
	const id = PREFIX.ctx + name;
	const docs = [...model.docs.values()]
		.filter((d) => d.ctx === name)
		.sort((a, b) => DOC_ORDER.indexOf(a.kind) - DOC_ORDER.indexOf(b.kind) || a.path.localeCompare(b.path));
	const requirements = [];
	let scenarios = 0;
	for (const [nid, node] of Object.entries(model.nos)) {
		if (node.ctx !== name) {
			continue;
		}
		if (node.tipo === 'requisito') {
			const short = nid.slice(nid.lastIndexOf('/') + 1);
			const found = model.titles.get(`${name}/${short}`) || {};
			requirements.push({
				id: nid,
				name: short,
				title: found.title || '',
				path: found.path || '',
				line: found.line || 0,
				status: node.status || '',
				scenarios: targets(model, nid, 'especifica').length,
			});
		} else if (node.tipo === 'cenario') {
			scenarios += 1;
		}
	}
	requirements.sort((a, b) => a.name.localeCompare(b.name, undefined, { numeric: true }));

	const entities = unique(docs.flatMap((d) => targets(model, d.id, 'toca')))
		.map((e) => e.slice(PREFIX.ent.length).replace(/^"|"$/g, ''))
		.filter((e) => !placeholder(e))
		.sort();
	const code = sources(model, id, 'pertence')
		.filter((s) => s.startsWith(PREFIX.code))
		.map((s) => s.slice(PREFIX.code.length))
		.sort();
	const dependsOn = unique(docs.flatMap((d) => targets(model, d.id, 'depende')))
		.map((c) => c.slice(PREFIX.ctx.length))
		.filter((c) => c !== name && !placeholder(c))
		.sort();
	return { name, docs, requirements, scenarios, entities, code, dependsOn };
}

/** Where a context stands, in a word. */
function stageOf(ctx) {
	const live = ctx.requirements.filter((r) => r.status === 'vivo').length;
	if (live > 0 && ctx.code.length > 0) {
		return 'implementado';
	}
	if (ctx.requirements.length > 0) {
		return 'especificado';
	}
	if (ctx.docs.length > 0) {
		return 'descoberta';
	}
	return 'vazio';
}

/**
 * How the project stands, context by context and tree by tree.
 */
function overview(root) {
	const model = load(root);
	if (!model) {
		return null;
	}
	const names = contextNames(model);
	const full = names.map((n) => contextOf(model, n));
	const usedBy = new Map();
	for (const c of full) {
		for (const d of c.dependsOn) {
			(usedBy.get(d) || usedBy.set(d, []).get(d)).push(c.name);
		}
	}
	const contexts = full.map((c) => {
		const kinds = {};
		for (const d of c.docs) {
			if (!d.archived) {
				kinds[d.kind] = (kinds[d.kind] || 0) + 1;
			}
		}
		return {
			name: c.name,
			stage: stageOf(c),
			docs: kinds,
			archived: c.docs.filter((d) => d.archived).length,
			requirements: c.requirements.length,
			live: c.requirements.filter((r) => r.status === 'vivo').length,
			scenarios: c.scenarios,
			entities: c.entities,
			files: c.code.length,
			dependsOn: c.dependsOn,
			usedBy: (usedBy.get(c.name) || []).sort(),
		};
	});

	const trees = model.scopes.map((s) => {
		const byUnit = new Map();
		for (const [rel, f] of Object.entries(s.graph.files || {})) {
			const dir = dirName(s.base + rel);
			const u = byUnit.get(dir) || byUnit.set(dir, { name: tail(dir), dir, files: 0, lines: 0 }).get(dir);
			u.files += 1;
			u.lines += f.lines || 0;
		}
		const stats = s.graph.stats || {};
		const packages = [...byUnit.values()].sort((a, b) => b.lines - a.lines);
		return {
			name: s.name,
			base: s.base,
			language: s.language,
			framework: s.framework,
			files: stats.files || Object.keys(s.graph.files || {}).length,
			lines: stats.loc || packages.reduce((sum, p) => sum + p.lines, 0),
			packages: stats.packages || packages.length,
			types: stats.types || 0,
			funcs: (stats.funcs || 0) + (stats.methods || 0),
			top: packages.slice(0, 6),
		};
	});

	const libraries = new Map();
	for (const d of model.docs.values()) {
		if (d.ctx) {
			continue;
		}
		const name = libraryOf(d);
		const lib = libraries.get(name) || libraries.set(name, { name, docs: 0, lines: 0 }).get(name);
		lib.docs += 1;
		lib.lines += d.lines;
	}

	const tipos = (t) => Object.values(model.nos).filter((n) => n.tipo === t).length;
	return {
		project: baseName(root.split(path.sep).join('/')),
		totals: {
			contexts: names.length,
			docs: model.docs.size,
			requirements: tipos('requisito'),
			live: Object.values(model.nos).filter((n) => n.tipo === 'requisito' && n.status === 'vivo').length,
			scenarios: tipos('cenario'),
			entities: unique(contexts.flatMap((c) => c.entities)).length,
			files: model.files.size,
			lines: trees.reduce((sum, t) => sum + t.lines, 0),
		},
		contexts,
		trees,
		libraries: [...libraries.values()].sort((a, b) => b.docs - a.docs),
	};
}

/** The library a document with no context belongs to. */
function libraryOf(d) {
	const where = d.corpus || dirName(d.path);
	const lib = LIBRARIES.find((l) => l.match.test(where));
	return lib ? lib.name : 'Outros';
}

/** The relations of the code graph that tie one symbol to another. */
const CODE_RELS = new Set(['calls', 'uses', 'embeds', 'implements', 'extends']);

/**
 * The whole project as a tree and the links that cross it.
 *
 * The tree is how the project is kept: the contexts with their documents, the
 * libraries the agents read, and each tree of code folder by folder down to
 * the source files. The links are what the two indexes know, between the
 * things of that tree and nothing else:
 *
 *   code       — a file whose symbols call or use another file's;
 *   cita       — a document that cites another;
 *   depende    — a document that needs another context;
 *   pertence   — a source file and the context it is part of.
 *
 * The code that implements a table a document touches is left to the trace:
 * a table is implemented in a hundred places, and drawn here it would tie
 * every context to all of the code.
 *
 * A leaf carries the contexts it belongs to. Every link has both ends in the
 * tree, so the page can fold a branch and still know what crosses into it.
 */
function graph(root) {
	const model = load(root);
	if (!model) {
		return null;
	}
	const ids = new Set();
	const known = (node) => {
		ids.add(node.id);
		return node;
	};
	const group = (id, name, kind, children, extra) => known({ id, name, kind, ...extra, children: children.filter((c) => !c.children || c.children.length > 0) });

	const byCtx = new Map();
	const byLib = new Map();
	for (const d of model.docs.values()) {
		const leaf = known({ id: d.id, name: d.name, kind: d.kind, size: d.lines, path: d.path, ctx: d.ctx ? [d.ctx] : [], title: d.title, archived: d.archived });
		if (d.ctx && !placeholder(d.ctx)) {
			(byCtx.get(d.ctx) || byCtx.set(d.ctx, []).get(d.ctx)).push(leaf);
		} else {
			const lib = libraryOf(d);
			(byLib.get(lib) || byLib.set(lib, []).get(lib)).push(leaf);
		}
	}
	const contexts = [...byCtx.entries()].sort().map(([name, leaves]) => group(PREFIX.ctx + name, name, 'context', leaves));
	const libraries = [...byLib.entries()].sort().map(([name, leaves]) => group(`lib:${name}`, name, 'library', leaves));

	// Which contexts a source file is part of, as the document graph says.
	const owners = new Map();
	for (const name of contextNames(model)) {
		for (const s of sources(model, PREFIX.ctx + name, 'pertence')) {
			if (s.startsWith(PREFIX.code)) {
				const p = s.slice(PREFIX.code.length);
				(owners.get(p) || owners.set(p, []).get(p)).push(name);
			}
		}
	}

	/** The node a folder of a tree is drawn as: the tree itself at its root. */
	const folderId = (scope, dir) => (dir === scope.base.replace(/\/$/, '') || dir === '.' && scope.base === '' ? `scope:${scope.name}` : `dir:${scope.name}:${dir}`);
	const trees = model.scopes.map((s) => {
		const top = { dirs: new Map(), files: [] };
		for (const f of model.files.values()) {
			if (f.scope !== s.name) {
				continue;
			}
			let at = top;
			let dir = s.base.replace(/\/$/, '');
			const inside = dirName(f.path.slice(s.base.length));
			for (const part of inside === '.' ? [] : inside.split('/')) {
				dir = dir ? `${dir}/${part}` : part;
				at = at.dirs.get(part) || at.dirs.set(part, { name: part, path: dir, dirs: new Map(), files: [] }).get(part);
			}
			at.files.push({ id: f.id, name: f.name, kind: 'file', size: f.lines, path: f.path, ctx: owners.get(f.path) || [], symbols: f.symbols });
		}
		const inside = (d) => [
			...[...d.dirs.values()].sort((a, b) => a.name.localeCompare(b.name)).map(fold),
			...d.files.sort((a, b) => a.name.localeCompare(b.name)).map(known),
		];
		// A folder that only holds another folder is read as one: a/b.
		function fold(d) {
			let name = d.name;
			while (d.files.length === 0 && d.dirs.size === 1) {
				d = [...d.dirs.values()][0];
				name = `${name}/${d.name}`;
			}
			return group(folderId(s, d.path), name, 'folder', inside(d), { path: d.path });
		}
		return group(`scope:${s.name}`, s.name, 'scope', inside(top), { language: s.language, path: s.base.replace(/\/$/, '') });
	});

	const weights = new Map();
	const link = (from, to, kind, n) => {
		if (from !== to && ids.has(from) && ids.has(to)) {
			const key = `${from}\n${to}\n${kind}`;
			weights.set(key, (weights.get(key) || 0) + (n || 1));
		}
	};

	// What the documents say: who cites whom, who needs which context, and
	// which code is a context's own.
	for (const d of model.docs.values()) {
		for (const [to, rel] of model.out.get(d.id) || []) {
			if (rel === 'cita') {
				link(d.id, to, 'cita');
			} else if (rel === 'depende' && to !== PREFIX.ctx + d.ctx) {
				link(d.id, to, 'depende');
			}
		}
	}
	for (const [file, names] of owners) {
		for (const name of names) {
			link(PREFIX.code + file, PREFIX.ctx + name, 'pertence');
		}
	}

	// What the code says: a file whose symbols reach into another file. Two
	// packages that only import each other are tied folder to folder.
	for (const s of model.scopes) {
		const fileOf = new Map();
		for (const n of s.graph.nodes || []) {
			if (n.file && n.kind !== 'package') {
				fileOf.set(n.id, s.base + n.file);
			}
		}
		const dirOfUnit = new Map();
		for (const [rel, f] of Object.entries(s.graph.files || {})) {
			if (f.unit) {
				dirOfUnit.set(f.unit, dirName(s.base + rel));
			}
		}
		const tied = new Set();
		const imports = [];
		for (const e of s.graph.edges || []) {
			if (CODE_RELS.has(e.rel)) {
				const a = fileOf.get(e.from);
				const b = fileOf.get(e.to);
				if (a && b && a !== b) {
					link(PREFIX.code + a, PREFIX.code + b, 'code');
					tied.add(`${dirName(a)}\n${dirName(b)}`);
				}
			} else if (e.rel === 'imports') {
				imports.push(e);
			}
		}
		for (const e of imports) {
			const a = dirOfUnit.get(String(e.from).replace(/^pkg:/, ''));
			const b = dirOfUnit.get(String(e.to).replace(/^pkg:/, ''));
			if (a && b && a !== b && !tied.has(`${a}\n${b}`)) {
				link(folderId(s, a), folderId(s, b), 'code');
			}
		}
	}

	const tree = group('root', baseName(root.split(path.sep).join('/')), 'root', [
		group('docs:contexts', 'Contextos', 'section', contexts),
		group('docs:libraries', 'Bibliotecas', 'section', libraries),
		...trees,
	]);
	const links = [...weights.entries()].map(([key, weight]) => {
		const [from, to, kind] = key.split('\n');
		return { from, to, kind, weight };
	});
	return { tree, links };
}

/**
 * One context, left to right: its documents, the requirements they add and
 * specify, the tables they touch, and the code that implements those tables
 * or is the context's own. The links are the graph's edges, nothing inferred.
 */
function trace(root, name) {
	const model = load(root);
	if (!model || !model.nos[PREFIX.ctx + name]) {
		return null;
	}
	const ctx = contextOf(model, name);
	const live = ctx.docs.filter((d) => !d.archived);
	const links = [];
	const link = (from, to, kind) => links.push({ from, to, kind });

	const reqIds = new Set(ctx.requirements.map((r) => r.id));
	for (const d of live) {
		for (const [to, rel] of model.out.get(d.id) || []) {
			if (reqIds.has(to) && (rel === 'adiciona' || rel === 'especifica' || rel === 'modifica')) {
				link(d.id, to, rel);
			}
		}
	}

	const entities = ctx.entities.map((e) => {
		const id = model.nos[`${PREFIX.ent}"${e}"`] ? `${PREFIX.ent}"${e}"` : PREFIX.ent + e;
		return { id, name: e };
	});
	for (const d of live) {
		for (const to of targets(model, d.id, 'toca')) {
			if (entities.some((e) => e.id === to)) {
				link(d.id, to, 'toca');
			}
		}
	}

	// The code is drawn by package: a context's files are many, their folders few.
	const packages = new Map();
	const pkgOf = (file) => {
		const dir = dirName(file);
		return packages.get(dir) || packages.set(dir, { id: `pkg:${dir}`, name: tail(dir), path: dir, files: [], own: false }).get(dir);
	};
	for (const file of ctx.code) {
		const p = pkgOf(file);
		p.files.push(file);
		p.own = true;
	}
	for (const e of entities) {
		for (const to of targets(model, e.id, 'implementa')) {
			const file = to.slice(PREFIX.code.length);
			const p = pkgOf(file);
			if (!p.files.includes(file)) {
				p.files.push(file);
			}
			if (!links.some((l) => l.from === e.id && l.to === p.id)) {
				link(e.id, p.id, 'implementa');
			}
		}
	}

	return {
		context: name,
		stage: stageOf(ctx),
		dependsOn: ctx.dependsOn,
		archived: ctx.docs.length - live.length,
		columns: [
			{ key: 'docs', nodes: live.map((d) => ({ id: d.id, name: d.name, kind: d.kind, title: d.title, status: d.status, version: d.version, path: d.path })) },
			{ key: 'requirements', nodes: ctx.requirements.map((r) => ({ id: r.id, name: r.name, kind: 'requirement', title: r.title, status: r.status, scenarios: r.scenarios, path: r.path, line: r.line })) },
			{ key: 'entities', nodes: entities.map((e) => ({ id: e.id, name: e.name, kind: 'entity' })) },
			{
				key: 'code',
				nodes: [...packages.values()]
					.sort((a, b) => Number(b.own) - Number(a.own) || a.path.localeCompare(b.path))
					.map((p) => ({ id: p.id, name: p.name, kind: 'package', path: p.path, files: p.files.sort(), own: p.own })),
			},
		],
		links,
	};
}

/**
 * What is known of one thing of the project: a document, a source file, a
 * context. For the panel beside the picture.
 */
function details(root, id) {
	const model = load(root);
	if (!model || typeof id !== 'string') {
		return null;
	}
	const doc = model.docs.get(id);
	if (doc) {
		const label = (other) => {
			const d = model.docs.get(other);
			return d ? { id: other, name: d.title || d.name, path: d.path, sub: d.path } : null;
		};
		const reqs = targets(model, id)
			.filter((t) => t.startsWith(PREFIX.req))
			.map((t) => {
				const key = t.slice(PREFIX.req.length);
				const found = model.titles.get(key) || {};
				return { id: t, name: key.slice(key.lastIndexOf('/') + 1), sub: found.title || '', path: found.path || '', line: found.line || 0 };
			});
		return {
			id,
			kind: doc.kind,
			name: doc.name,
			title: doc.title,
			path: doc.path,
			facts: [doc.ctx && `contexto ${doc.ctx}`, doc.status, doc.version && `v${doc.version}`, `${doc.lines} linhas`, `${doc.sections} seções`].filter(Boolean),
			groups: [
				{ title: 'Requisitos', items: reqs },
				{ title: 'Tabelas', items: targets(model, id, 'toca').map((e) => ({ id: e, name: e.slice(PREFIX.ent.length).replace(/^"|"$/g, '') })).filter((e) => !placeholder(e.name)) },
				{ title: 'Cita', items: targets(model, id, 'cita').map(label).filter(Boolean) },
				{ title: 'Citado por', items: sources(model, id, 'cita').map(label).filter(Boolean) },
			].filter((g) => g.items.length > 0),
		};
	}
	if (id.startsWith(PREFIX.code)) {
		const file = model.files.get(id.slice(PREFIX.code.length));
		const p = id.slice(PREFIX.code.length);
		const scope = file && model.scopes.find((s) => s.name === file.scope);
		const rel = file && scope ? p.slice(scope.base.length) : '';
		const symbols = scope
			? (scope.graph.nodes || [])
				.filter((n) => n.file === rel && n.kind !== 'package')
				.sort((a, b) => (b.score || 0) - (a.score || 0) || a.line - b.line)
				.slice(0, 24)
				.map((n) => ({ id: n.id, name: n.name, sub: `${n.kind} · linha ${n.line}`, path: p, line: n.line }))
			: [];
		return {
			id,
			kind: 'file',
			name: baseName(p),
			title: '',
			path: p,
			facts: file ? [file.scope, `${file.lines} linhas`, `${file.symbols} símbolos`] : [],
			groups: [
				{ title: 'Contextos', items: targets(model, id, 'pertence').map((c) => ({ id: c, name: c.slice(PREFIX.ctx.length) })) },
				{ title: 'Implementa as tabelas', items: sources(model, id, 'implementa').map((e) => ({ id: e, name: e.slice(PREFIX.ent.length).replace(/^"|"$/g, '') })) },
				{ title: 'Símbolos', items: symbols },
			].filter((g) => g.items.length > 0),
		};
	}
	return null;
}

module.exports = { hasGraph, overview, graph, trace, details, GRAPH_FILE, DOCS_DIR, CODE_DIR };
