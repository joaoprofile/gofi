'use strict';

const fs = require('fs');
const os = require('os');
const path = require('path');

/**
 * The engine's own conversation store, read from disk.
 *
 * The panel does not keep a private notion of a session: every turn it runs is
 * a Claude Code session, written by the CLI to
 * `~/.claude/projects/<project>/<session-id>.jsonl`. Resuming by that id
 * appends to the same file — verified, not assumed — so one conversation is one
 * file no matter which side continues it.
 *
 * That is what makes both directions work. A thread started here can be picked
 * up in a terminal with `claude --resume <id>`; a thread started in the
 * terminal shows up in the panel's list and can be continued in the chat, with
 * its transcript read from the same file.
 *
 * Everything here is best-effort. The format is the engine's private business
 * and can change between versions, so every reader degrades to "no sessions
 * found" rather than to an error — the panel's own transcripts are the
 * fallback, and they are ours.
 */

/** Enough of the head to hold the opening message. */
const HEAD_BYTES = 64 * 1024;

/** Enough of the tail to hold the most recent title the engine wrote. */
const TAIL_BYTES = 256 * 1024;

/** A long conversation is read from its tail — the last hour is what you want. */
const MAX_TRANSCRIPT_BYTES = 12 * 1024 * 1024;

/** Upper bound on a replayed transcript, in events. */
const MAX_EVENTS = 2000;

/** How many conversations to offer. The list is chronological; the tail is old news. */
const MAX_LISTED = 60;

const ID_PATTERN = /^[A-Za-z0-9-]{8,64}$/;

/**
 * Where the engine keeps this workspace's conversations.
 *
 * The folder name is the working directory with its separators flattened. Two
 * spellings of that flattening are tried rather than one guessed at, and a miss
 * simply means no sessions to offer.
 */
function projectDir(cwd) {
	if (!cwd) {
		return null;
	}
	const root = path.join(os.homedir(), '.claude', 'projects');
	const candidates = [cwd.replace(/[^A-Za-z0-9]/g, '-'), cwd.replace(/[\\/.]/g, '-')];
	for (const name of candidates) {
		const dir = path.join(root, name);
		try {
			if (fs.statSync(dir).isDirectory()) {
				return dir;
			}
		} catch {
			/* not this spelling */
		}
	}
	return null;
}

/** Reads part of a file without pulling the whole thing into memory. */
function readSlice(file, from, length) {
	let fd;
	try {
		fd = fs.openSync(file, 'r');
		const buffer = Buffer.alloc(length);
		const read = fs.readSync(fd, buffer, 0, length, from);
		return buffer.slice(0, read).toString('utf8');
	} catch {
		return '';
	} finally {
		if (fd !== undefined) {
			try {
				fs.closeSync(fd);
			} catch {
				/* already closed */
			}
		}
	}
}

/**
 * Splits a slice into records, dropping the first line when the slice started
 * mid-file — half a JSON object is not a record.
 */
function records(text, partialStart) {
	const out = [];
	const all = text.split('\n');
	for (let i = partialStart ? 1 : 0; i < all.length; i++) {
		const line = all[i].trim();
		if (line === '' || line[0] !== '{') {
			continue;
		}
		try {
			out.push(JSON.parse(line));
		} catch {
			// A line still being written. The next read will have it whole.
		}
	}
	return out;
}

/**
 * What to call a conversation.
 *
 * The engine names its own sessions as they go, and its latest name is the one
 * the CLI's own resume list shows — so the same conversation reads the same on
 * both sides. Failing that, the question it opened with.
 */
function titleOf(file, size) {
	const from = Math.max(0, size - TAIL_BYTES);
	for (const record of records(readSlice(file, from, Math.min(size, TAIL_BYTES)), from > 0).reverse()) {
		if (record.type === 'ai-title' && typeof record.aiTitle === 'string' && record.aiTitle !== '') {
			return record.aiTitle;
		}
	}
	for (const record of records(readSlice(file, 0, Math.min(size, HEAD_BYTES)), false)) {
		if (record.type === 'user' && record.isSidechain !== true) {
			const text = userText(record.message && record.message.content);
			if (text !== '') {
				const line = text.split('\n')[0].trim();
				return line.length > 60 ? `${line.slice(0, 60)}…` : line;
			}
		}
	}
	return 'conversa sem título';
}

/**
 * Every conversation the engine has for this folder, newest first.
 *
 * @returns {{engineId: string, title: string, updatedAt: number}[]}
 */
function list(cwd) {
	const dir = projectDir(cwd);
	if (!dir) {
		return [];
	}
	let names;
	try {
		names = fs.readdirSync(dir);
	} catch {
		return [];
	}

	const files = [];
	for (const name of names) {
		if (!name.endsWith('.jsonl')) {
			continue;
		}
		const id = name.slice(0, -'.jsonl'.length);
		if (!ID_PATTERN.test(id)) {
			continue;
		}
		try {
			const stat = fs.statSync(path.join(dir, name));
			if (stat.isFile() && stat.size > 0) {
				files.push({ id, file: path.join(dir, name), updatedAt: stat.mtimeMs, size: stat.size });
			}
		} catch {
			/* vanished between readdir and stat */
		}
	}

	files.sort((a, b) => b.updatedAt - a.updatedAt);
	// Titles are read from disk, so only for the ones that will be shown.
	return files.slice(0, MAX_LISTED).map((entry) => ({
		engineId: entry.id,
		updatedAt: Math.round(entry.updatedAt),
		title: titleOf(entry.file, entry.size),
	}));
}

/**
 * Rebuilds a conversation as the events the panel already knows how to render.
 *
 * Only the parts a reader needs: what was asked, what was answered, and which
 * tools ran. The engine's own bookkeeping — queue operations, file snapshots,
 * sub-agent chatter — is not part of the conversation.
 *
 * @returns {object[]} Possibly empty, which the caller should read as "no
 *   transcript here" rather than "an empty conversation".
 */
function transcript(cwd, engineId) {
	const dir = projectDir(cwd);
	if (!dir || !ID_PATTERN.test(engineId)) {
		return [];
	}
	const file = path.join(dir, `${engineId}.jsonl`);
	let size;
	try {
		size = fs.statSync(file).size;
	} catch {
		return [];
	}

	const from = Math.max(0, size - MAX_TRANSCRIPT_BYTES);
	const parsed = records(readSlice(file, from, Math.min(size, MAX_TRANSCRIPT_BYTES)), from > 0);

	const events = [];
	for (const record of parsed) {
		// Sub-agent transcripts share the file with the conversation. They are
		// the agent's own business, not what the user said and was told.
		if (record.isSidechain === true) {
			continue;
		}
		if (record.type === 'user') {
			events.push(...fromUser(record.message && record.message.content));
		} else if (record.type === 'assistant') {
			const blocks = fromAssistant(record.message && record.message.content);
			if (blocks.length > 0) {
				events.push({ type: 'blocks', blocks });
			}
		}
	}
	return events.length > MAX_EVENTS ? events.slice(events.length - MAX_EVENTS) : events;
}

/** @returns {object[]} */
function fromUser(content) {
	if (typeof content === 'string') {
		const text = clean(content);
		return text === '' ? [] : [{ type: 'user', text, images: 0 }];
	}
	if (!Array.isArray(content)) {
		return [];
	}

	const events = [];
	const parts = [];
	let images = 0;
	for (const block of content) {
		if (block.type === 'tool_result') {
			events.push({
				type: 'toolResult',
				toolUseId: block.tool_use_id || '',
				isError: block.is_error === true,
				preview: firstLine(flatten(block.content)),
			});
		} else if (block.type === 'text' && typeof block.text === 'string') {
			parts.push(block.text);
		} else if (block.type === 'image') {
			images++;
		}
	}
	const text = clean(parts.join('\n'));
	if (text !== '' || images > 0) {
		// The message comes before the results of the tools it caused.
		events.unshift({ type: 'user', text, images });
	}
	return events;
}

/** @returns {object[]} */
function fromAssistant(content) {
	const blocks = [];
	for (const block of Array.isArray(content) ? content : []) {
		if (block.type === 'text' && typeof block.text === 'string' && block.text !== '') {
			blocks.push({ type: 'text', text: block.text });
		} else if (block.type === 'thinking' && typeof block.thinking === 'string' && block.thinking !== '') {
			blocks.push({ type: 'thinking', text: block.thinking });
		} else if (block.type === 'tool_use') {
			blocks.push({ type: 'tool_use', id: block.id || '', name: block.name || 'tool', input: block.input });
		}
	}
	return blocks;
}

function userText(content) {
	return typeof content === 'string' ? clean(content) : clean(flatten(content));
}

function flatten(content) {
	if (typeof content === 'string') {
		return content;
	}
	const parts = [];
	for (const block of Array.isArray(content) ? content : []) {
		if (block && block.type === 'text' && typeof block.text === 'string') {
			parts.push(block.text);
		}
	}
	return parts.join('\n');
}

/**
 * Strips what the engine wraps around a message on its way in.
 *
 * Reminders and command scaffolding are addressed to the model, not to the
 * person reading the transcript back — showing them would make a one-line
 * question look like a page of machinery.
 */
function clean(text) {
	return String(text || '')
		.replace(/<system-reminder>[\s\S]*?<\/system-reminder>/g, '')
		.replace(/<local-command-[\s\S]*?>[\s\S]*?<\/local-command-[^>]*>/g, '')
		.replace(/<command-message>[\s\S]*?<\/command-message>/g, '')
		.replace(/<command-args>([\s\S]*?)<\/command-args>/g, '$1')
		.replace(/<command-name>([\s\S]*?)<\/command-name>/g, '$1')
		.trim();
}

function firstLine(text) {
	const trimmed = String(text || '').trim();
	const cut = trimmed.indexOf('\n');
	const line = cut === -1 ? trimmed : trimmed.slice(0, cut);
	return line.length > 160 ? `${line.slice(0, 160)}…` : line;
}

module.exports = { list, transcript, projectDir };
