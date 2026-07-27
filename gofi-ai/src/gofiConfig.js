'use strict';

const fs = require('fs');
const os = require('os');
const path = require('path');

/**
 * @typedef {Object} GofiProject
 * @property {string} name     `project.name`
 * @property {string} model    `ai.model`, or '' when unset
 * @property {string[]} agents Enabled agent slugs, in file order
 */

/**
 * Reads the three fields of `.gofi.yaml` the chat actually uses: the project
 * name for the header, `ai.model` as the default model, and the agent list for
 * the slash-command chips.
 *
 * This is a targeted scanner, not a YAML parser — pulling a YAML dependency in
 * to read three scalars would be the whole extension's only runtime dep. It
 * reads the shape `gofi init` writes (two-space indent, block sequences) and
 * returns null on anything it doesn't recognise, which degrades to "no project
 * context" rather than to a wrong answer.
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
	const agents = [];
	/** @type {string|null} */
	let section = null;

	for (const line of raw.split(/\r?\n/)) {
		if (line.trim() === '' || line.trimStart().startsWith('#')) {
			continue;
		}

		// A non-indented `key:` opens a top-level block.
		const top = /^([A-Za-z_][\w-]*):\s*(.*)$/.exec(line);
		if (top) {
			section = top[1];
			continue;
		}

		const indented = /^\s+(.*)$/.exec(line);
		if (!indented) {
			section = null;
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

		const pair = /^([A-Za-z_][\w-]*):\s*(.*)$/.exec(body);
		if (!pair) {
			continue;
		}
		const [, key, value] = pair;
		if (section === 'project' && key === 'name') {
			name = unquote(stripComment(value));
		} else if (section === 'ai' && key === 'model') {
			model = unquote(stripComment(value));
		}
	}

	if (name === '') {
		return null; // no identity — treat it as not a gofi project
	}
	return { name, model, agents };
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

module.exports = { readGofiProject, readGofiSkills, findProjectRoot, SKILLS_DIR, SKILL_FILE, CONFIG_FILE };
