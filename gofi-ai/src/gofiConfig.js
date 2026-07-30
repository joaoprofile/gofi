'use strict';

const fs = require('fs');
const os = require('os');
const path = require('path');

/**
 * @typedef {Object} GofiProject
 * @property {string} name     `project.name`
 * @property {string} model    `ai.model`, or '' when unset
 * @property {string[]} models `ai.models` — model IDs the picker offers; empty when unset
 * @property {string[]} agents Enabled agent slugs, in file order
 */

/**
 * Reads the fields of `.gofi.yaml` the chat actually uses: the project name for
 * the header, `ai.model` as the default model, `ai.models` as the model list
 * the /model picker offers, and the agent list for the slash-command chips.
 *
 * This is a targeted scanner, not a YAML parser — pulling a YAML dependency in
 * to read a handful of scalars would be the whole extension's only runtime dep.
 * It reads the shape `gofi init` writes (indented mappings, block sequences)
 * and returns null on anything it doesn't recognise, which degrades to "no
 * project context" rather than to a wrong answer.
 *
 * @param {string} root Workspace folder
 * @returns {GofiProject | null}
 */
function readGofiProject(root) {
	let raw;
	try {
		raw = fs.readFileSync(path.join(root, CONFIG_FILE), 'utf8');
	} catch {
		return null; // not a gofi project, or unreadable — same outcome either way
	}

	let name = '';
	let model = '';
	/** @type {string[]} */
	const models = [];
	/** @type {string[]} */
	const agents = [];
	/** @type {string|null} */
	let section = null;
	// Sub-block state inside the current section. Only `ai.models` uses it
	// today — an inline sequence that the flat top/section split can't see.
	/** @type {string|null} */
	let subsection = null;

	for (const line of raw.split(/\r?\n/)) {
		if (line.trim() === '' || line.trimStart().startsWith('#')) {
			continue;
		}

		// A non-indented `key:` opens a top-level block.
		const top = /^([A-Za-z_][\w-]*):\s*(.*)$/.exec(line);
		if (top) {
			section = top[1];
			subsection = null;
			continue;
		}

		const indented = /^\s+(.*)$/.exec(line);
		if (!indented) {
			section = null;
			subsection = null;
			continue;
		}
		const body = indented[1];

		if (section === 'agents') {
			const item = /^-\s*(\S+)/.exec(body);
			if (item) {
				agents.push(stripComment(item[1]));
			}
			continue;
		}

		if (section === 'ai' && subsection === 'models') {
			const item = /^-\s*(\S+)/.exec(body);
			if (item) {
				models.push(unquote(stripComment(item[1])));
				continue;
			}
			// A sibling key inside `ai:` closes the models list.
		}

		const pair = /^([A-Za-z_][\w-]*):\s*(.*)$/.exec(body);
		if (!pair) {
			continue;
		}
		const [, key, value] = pair;
		if (section === 'project' && key === 'name') {
			name = unquote(stripComment(value));
		} else if (section === 'ai' && key === 'model') {
			model = unquote(stripComment(value));
			subsection = null;
		} else if (section === 'ai' && key === 'models' && stripComment(value) === '') {
			subsection = 'models';
		} else if (section === 'ai') {
			subsection = null;
		}
	}

	if (name === '') {
		return null; // no identity — treat it as not a gofi project
	}
	return { name, model, models, agents };
}

/**
 * @typedef {Object} GofiSkill
 * @property {string} slug   File stem, e.g. `gofi-eng` — also the slash command
 * @property {string} title  Short role label for the tooltip, may be ''
 * @property {boolean} enabled  Listed under `agents:` in `.gofi.yaml`
 */

/** Where `gofi init` installs the skills. */
const SKILLS_DIR = path.join('.claude', 'skills');

/**
 * The manifest inside each skill folder. Claude Code only discovers a skill as
 * `<skills>/<name>/SKILL.md` with `name` and `description` in its frontmatter —
 * a flat `<name>.md` is silently ignored.
 */
const SKILL_FILE = 'SKILL.md';

/** The file that marks a gofi project root. */
const CONFIG_FILE = '.gofi.yaml';

/**
 * Model IDs the /model picker offers when a project has no `ai.models:` yet,
 * and the baseline used by `persistSelectedModel` when rewriting the yaml so
 * the full cardápio is preserved even for configs that started with a short
 * list. Keep in sync with `cli/internal/config/config.go: AllModels()` — adding
 * a model there without mirroring it here means legacy users won't see it.
 */
const PINNED_MODELS = [
	'claude-fable-5',
	'claude-opus-5',
	'claude-opus-4-8',
	'claude-opus-4-7',
	'claude-sonnet-5',
	'claude-sonnet-4-6',
	'claude-haiku-4-5',
];

/**
 * Finds the gofi project root at or above `start`.
 *
 * Opening a subfolder of a monorepo is normal, and the chat would otherwise
 * report "no gofi project" while `gofi`, run from the same directory, finds one
 * fine. So the search walks up — but only to the repository root, and never
 * into the home directory.
 *
 * The bound is load-bearing, not defensive: a stray `~/.gofi.yaml` is common
 * (someone ran `gofi init` in their home once), and an unbounded walk would
 * make every project on the machine claim that config's identity.
 *
 * @param {string} start Workspace folder
 * @returns {string | null} The folder holding `.gofi.yaml`, or null
 */
function findProjectRoot(start) {
	const home = os.homedir();
	let dir = start;

	while (dir && dir !== home) {
		if (fs.existsSync(path.join(dir, CONFIG_FILE))) {
			return dir;
		}
		// A `.git` here means this is the repo root: a `.gofi.yaml` further up
		// belongs to something else.
		if (fs.existsSync(path.join(dir, '.git'))) {
			return null;
		}
		const parent = path.dirname(dir);
		if (parent === dir) {
			return null; // reached the filesystem root
		}
		dir = parent;
	}
	return null;
}

/**
 * Lists the gofi skills actually installed in the project.
 *
 * The `agents:` list in `.gofi.yaml` scopes per-agent knowledge folders, but
 * the CLI installs *every* skill regardless — so the directory, not the
 * config, is the truth about what the user can invoke. Reading it here means a
 * skill added by `gofi agent add` shows up in the chat with no extra wiring,
 * and one the project never installed never does.
 *
 * A skill is a folder holding SKILL.md, which is the only layout Claude Code
 * discovers. Flat `<name>.md` files, written by gofi before that was fixed,
 * are deliberately not listed: the engine ignores them, so offering them as
 * chips would advertise commands that answer "unknown command".
 *
 * @param {string} root Project root (the folder holding `.gofi.yaml`)
 * @param {string[]} enabledAgents Slugs from `.gofi.yaml`
 * @returns {GofiSkill[]} Sorted by slug
 */
function readGofiSkills(root, enabledAgents) {
	const skillsRoot = path.join(root, SKILLS_DIR);
	let entries;
	try {
		entries = fs.readdirSync(skillsRoot, { withFileTypes: true });
	} catch {
		return [];
	}

	const enabled = new Set(enabledAgents || []);
	/** @type {GofiSkill[]} */
	const skills = [];
	for (const entry of entries) {
		if (!entry.isDirectory() || !entry.name.startsWith('gofi-')) {
			continue;
		}
		const manifest = path.join(skillsRoot, entry.name, SKILL_FILE);
		if (!fs.existsSync(manifest)) {
			continue; // a folder without SKILL.md is not a skill the engine can run
		}
		skills.push({ slug: entry.name, title: readSkillTitle(manifest), enabled: enabled.has(entry.name) });
	}
	skills.sort((a, b) => a.slug.localeCompare(b.slug));
	return skills;
}

/**
 * Pulls a one-line role label out of a skill file.
 *
 * Handles both shapes a skill can arrive in: YAML frontmatter with a
 * `description:`, and the plain `# /gofi-eng — Context Engineer` heading the
 * gofi skills use today. Returns '' when neither is present — a chip without a
 * tooltip still works.
 */
function readSkillTitle(file) {
	let raw;
	try {
		// Skills run to hundreds of lines; the label is always near the top.
		raw = fs.readFileSync(file, 'utf8').slice(0, 4096);
	} catch {
		return '';
	}

	const lines = raw.split(/\r?\n/);
	if (lines[0] === '---') {
		for (let i = 1; i < lines.length; i++) {
			if (lines[i] === '---') {
				break;
			}
			const match = /^description:\s*(.+)$/.exec(lines[i]);
			if (match) {
				return unquote(match[1].trim());
			}
		}
	}

	for (const line of lines) {
		const heading = /^#\s+(.+)$/.exec(line);
		if (heading) {
			// `# /gofi-eng — Context Engineer` → `Context Engineer`
			const parts = heading[1].split(/\s+[—–-]\s+/);
			return (parts.length > 1 ? parts.slice(1).join(' — ') : parts[0]).trim();
		}
	}
	return '';
}

/** Drops a trailing ` # comment`, which `gofi init` emits on most scalars. */
function stripComment(value) {
	const hash = value.indexOf(' #');
	return (hash === -1 ? value : value.slice(0, hash)).trim();
}

function unquote(value) {
	if (value.length >= 2 && (value[0] === '"' || value[0] === "'") && value[value.length - 1] === value[0]) {
		return value.slice(1, -1);
	}
	return value;
}

/**
 * Rewrites the `ai:` block of `.gofi.yaml` so `newModel` becomes both the
 * default (`ai.model`) and the only active entry in `ai.models` — every other
 * known model is written back as a `# - id` comment, preserving the
 * "cardápio" pattern that `gofi init` establishes.
 *
 * The known-models list is taken from the current file (active + commented),
 * so anything the user or `gofi config` already introduced is kept. If
 * `newModel` isn't listed at all, it's added to the top.
 *
 * The write is line-based on purpose: pulling a YAML dependency in just to
 * rewrite a single block would be the extension's only runtime dep, and a
 * naïve full re-serialisation would drop the comments the Go marshaller
 * produces. Only the `ai:` block is touched — every other section of the
 * file is spliced through byte-for-byte.
 *
 * @param {string} root      Project root (folder holding `.gofi.yaml`)
 * @param {string} newModel  Model ID (e.g. `claude-opus-5`)
 * @returns {'written' | 'no-project' | 'no-ai-block'}
 * @throws {Error} on filesystem errors (caller decides how to surface them)
 */
function persistSelectedModel(root, newModel) {
	if (!root || typeof newModel !== 'string' || newModel.trim() === '') {
		return 'no-project';
	}
	const filePath = path.join(root, CONFIG_FILE);
	let raw;
	try {
		raw = fs.readFileSync(filePath, 'utf8');
	} catch {
		return 'no-project';
	}

	// Preserve the original line ending so the diff is minimal on both LF and CRLF checkouts.
	const eol = raw.includes('\r\n') ? '\r\n' : '\n';
	const lines = raw.split(/\r?\n/);

	// Find `ai:` (top-level key, no indent) and the next top-level key that closes it.
	let aiStart = -1;
	let aiEnd = lines.length;
	for (let i = 0; i < lines.length; i++) {
		const line = lines[i];
		if (aiStart === -1) {
			if (/^ai\s*:\s*(#.*)?$/.test(line)) {
				aiStart = i;
			}
			continue;
		}
		// A non-indented `key:` after `ai:` ends the block. Comments and
		// blanks belong to whatever comes next — go-yaml groups them with
		// the following key — so they close the ai block too.
		if (line.trim() === '' || /^#/.test(line)) {
			aiEnd = i;
			break;
		}
		if (/^[A-Za-z_][\w-]*\s*:/.test(line)) {
			aiEnd = i;
			break;
		}
	}
	if (aiStart === -1) {
		return 'no-ai-block';
	}

	// Walk the ai: block to pull host and the union of active + commented models.
	let host = 'claude-vscode';
	/** @type {string[]} */
	const knownModels = [];
	const seen = new Set();
	let inModels = false;
	for (let i = aiStart + 1; i < aiEnd; i++) {
		const line = lines[i];
		const hostMatch = /^\s+host\s*:\s*(\S+)/.exec(line);
		if (hostMatch) {
			host = unquote(stripComment(hostMatch[1]));
			inModels = false;
			continue;
		}
		if (/^\s+models\s*:\s*$/.test(line)) {
			inModels = true;
			continue;
		}
		if (inModels) {
			const active = /^\s+-\s*(\S+)/.exec(line);
			if (active) {
				const id = unquote(stripComment(active[1]));
				if (id && !seen.has(id)) { seen.add(id); knownModels.push(id); }
				continue;
			}
			const commented = /^\s*#\s*-\s*(\S+)/.exec(line);
			if (commented) {
				const id = unquote(stripComment(commented[1]));
				if (id && !seen.has(id)) { seen.add(id); knownModels.push(id); }
				continue;
			}
			// Another key at the same indent as `models` ends the sublist.
			if (/^\s+[A-Za-z_][\w-]*\s*:/.test(line)) {
				inModels = false;
			}
		}
	}
	// Fold in the full cardápio so a yaml that started with a short `ai.models:`
	// list ends up carrying every pinned version — the ones not in use as
	// `# - id` comments. Without this, picking one model would silently prune
	// the menu the next `/model` sees.
	for (const id of PINNED_MODELS) {
		if (!seen.has(id)) { seen.add(id); knownModels.push(id); }
	}
	if (!seen.has(newModel)) {
		knownModels.unshift(newModel);
	}

	// Rebuild the ai: block. Indentation matches what MarshalYAML in Go emits (4 spaces).
	const indent = '    ';
	const rebuilt = [
		'ai:',
		`${indent}host: ${host}`,
		`${indent}model: ${newModel}`,
		`${indent}# Modelos exibidos pelo /model no gofi-ai. Descomente para adicionar ao picker.`,
		`${indent}models:`,
		`${indent}${indent}- ${newModel}`,
	];
	for (const id of knownModels) {
		if (id === newModel) { continue; }
		rebuilt.push(`${indent}${indent}# - ${id}`);
	}

	const updated = [...lines.slice(0, aiStart), ...rebuilt, ...lines.slice(aiEnd)].join(eol);
	fs.writeFileSync(filePath, updated, 'utf8');
	return 'written';
}

module.exports = { readGofiProject, readGofiSkills, findProjectRoot, persistSelectedModel, PINNED_MODELS, SKILLS_DIR, SKILL_FILE, CONFIG_FILE };
