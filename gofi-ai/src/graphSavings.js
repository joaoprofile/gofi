'use strict';

const fs = require('fs');
const path = require('path');
const { approxTokens } = require('./usage.js');

/**
 * What a graph query saved, measured instead of guessed.
 *
 * `gofi graph explain` answers with the exact places the symbol lives and is
 * called from — `arquivo.go:42`, one per edge. Those files are precisely the
 * ones the alternative would have opened: grep the tree, then read what
 * matched. So the saving is not a rule of thumb, it is the size of those files
 * minus what the graph answer actually cost.
 *
 * The whole calculation is local. It reads the answer the agent already
 * received (no round trip, no tokens) and stats the cited files, never opening
 * them. Every stat is async and the sizes are cached, so nothing here runs on
 * the path that renders the panel: the number arrives with the next throttled
 * update, or not at all.
 *
 * It is an upper bound on the alternative — a grep sometimes answers from the
 * matched line alone, without the file being opened — and it only counts files
 * the session never went on to read anyway.
 */

/** Cap on the answer text scanned. Beyond this a query is already too broad. */
const MAX_TEXT = 64 * 1024;

/** Cap on distinct files measured for one answer. */
const MAX_FILES = 40;

/** `caminho/arquivo.go:42` — how the graph reports where something lives. */
const CITED = /([A-Za-z0-9_@.\-/\\]+\.[A-Za-z0-9]+):\d+/g;

/**
 * Sizes read off disk, keyed by absolute path.
 *
 * Not invalidated: this measures how much text a file would have dragged into
 * the context, and a file that grew by twenty lines mid-session does not change
 * that answer in any way a reader would notice.
 *
 * @type {Map<string, number>}
 */
const sizes = new Map();

/** Scope roots per project, and when they were read. */
const scopeCache = new Map();

/**
 * Where each scope's paths are rooted.
 *
 * The graph records a file relative to the scope that was scanned — `backend`,
 * `frontend`, the vendored SDK — never to the project. Resolving against the
 * project root alone would find nothing in exactly the repositories the graph
 * is most useful in.
 */
function scopePrefixes(root) {
	const indexPath = path.join(root, '.gofi', 'graph', 'gofi_graph_index.json');
	let mtimeMs;
	try {
		mtimeMs = fs.statSync(indexPath).mtimeMs;
	} catch {
		return ['']; // single-scope build: the graph is rooted at the project
	}
	const cached = scopeCache.get(root);
	if (cached && cached.mtimeMs === mtimeMs) {
		return cached.prefixes;
	}

	const prefixes = [''];
	try {
		const index = JSON.parse(fs.readFileSync(indexPath, 'utf8'));
		for (const scope of index.scopes || []) {
			if (typeof scope.root === 'string' && scope.root !== '' && scope.root !== '.') {
				prefixes.push(`${scope.root.replace(/[/\\]+$/, '')}/`);
			}
		}
	} catch {
		// A malformed index still leaves the project root worth trying.
	}
	scopeCache.set(root, { mtimeMs, prefixes });
	return prefixes;
}

/** The distinct `arquivo:linha` paths an answer named, in order. */
function cited(text) {
	const out = [];
	const seen = new Set();
	const body = text.length > MAX_TEXT ? text.slice(0, MAX_TEXT) : text;
	for (const match of body.matchAll(CITED)) {
		const file = match[1];
		if (seen.has(file)) {
			continue;
		}
		seen.add(file);
		out.push(file);
		if (out.length >= MAX_FILES) {
			break;
		}
	}
	return out;
}

/** The size of one cited path, trying each scope root until one exists. */
async function sizeOf(root, prefixes, file) {
	for (const prefix of prefixes) {
		const rel = `${prefix}${file}`;
		const absolute = path.join(root, rel);
		const known = sizes.get(absolute);
		if (known !== undefined) {
			return known > 0 ? { path: rel, tokens: known } : null;
		}
		try {
			const stat = await fs.promises.stat(absolute);
			const tokens = approxTokens(stat.size);
			sizes.set(absolute, tokens);
			return { path: rel, tokens };
		} catch {
			sizes.set(absolute, 0); // absent under this scope, and it stays absent
		}
	}
	return null;
}

/**
 * Measures the files a graph answer cited.
 *
 * @param {string} root Project root
 * @param {string} text The answer the agent received
 * @returns {Promise<{path: string, tokens: number}[]>}
 */
async function measure(root, text) {
	if (!root || typeof text !== 'string' || text === '') {
		return [];
	}
	const files = cited(text);
	if (files.length === 0) {
		return [];
	}
	const prefixes = scopePrefixes(root);
	const found = await Promise.all(files.map((file) => sizeOf(root, prefixes, file)));
	return found.filter((entry) => entry !== null);
}

module.exports = { measure, cited };
