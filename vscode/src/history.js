'use strict';

const fs = require('fs');
const path = require('path');
const crypto = require('crypto');

/**
 * Conversations kept on disk, so closing the panel is not the end of one.
 *
 * Two things are stored, and they answer different questions. The transcript is
 * what the user reads back — the same events the webview already knows how to
 * render, replayed in order. The engine session id is what the *model* needs:
 * handed to the CLI as `--resume`, it puts the agent back in the conversation
 * it was having, instead of a stranger reading a transcript of one.
 *
 * Storage is local and per-workspace (`context.storageUri`), which is the
 * honest scope: a conversation is about this repo, and listing another
 * project's threads here would be noise at best.
 *
 * Every record is two files. `<id>.meta.json` holds what the list needs —
 * title, timestamps, counts — and `<id>.json` holds the events. Listing then
 * reads a few hundred bytes per session instead of every transcript in full,
 * and there is no index file to drift out of sync with the directory.
 *
 * Writes are synchronous on purpose: the last save happens while VSCode is
 * tearing the extension down, and an awaited write at that moment is a write
 * that never lands.
 */

/** Sub-folder inside the extension's storage for this workspace. */
const DIR = 'sessions';

/** How many conversations to keep. Oldest beyond this are dropped. */
const MAX_SESSIONS = 80;

/** Upper bound on a single transcript, in events. Oldest are dropped first. */
const MAX_EVENTS = 4000;

/** Ids come back from the webview, so they are checked before touching a path. */
const ID_PATTERN = /^[A-Za-z0-9-]{8,64}$/;

class SessionStore {
	/** @param {import('vscode').ExtensionContext} context */
	constructor(context) {
		// `storageUri` is per-workspace and undefined with no folder open — and
		// with no folder open there is no chat either, so the global fallback
		// only ever holds strays. With neither, the store simply does nothing:
		// a chat that cannot save its transcript is still a working chat.
		const base = context.storageUri || context.globalStorageUri;
		this.dir = base && base.fsPath ? path.join(base.fsPath, DIR) : null;
	}

	/** A fresh, empty conversation. Not on disk until it has something in it. */
	newRecord(title) {
		const now = Date.now();
		return {
			id: crypto.randomUUID(),
			title,
			createdAt: now,
			updatedAt: now,
			/** The engine's own conversation id, for `--resume`. */
			engineSessionId: null,
			/** @type {object[]} Webview events, in order. */
			events: [],
		};
	}

	metaPath(id) {
		return path.join(this.dir, `${id}.meta.json`);
	}

	eventsPath(id) {
		return path.join(this.dir, `${id}.json`);
	}

	/**
	 * Every saved conversation, newest first — meta only.
	 *
	 * @returns {{id: string, title: string, createdAt: number, updatedAt: number, messages: number}[]}
	 */
	list() {
		if (this.dir === null) {
			return [];
		}
		let names;
		try {
			names = fs.readdirSync(this.dir);
		} catch {
			return []; // nothing saved yet
		}
		const items = [];
		for (const name of names) {
			if (!name.endsWith('.meta.json')) {
				continue;
			}
			try {
				const meta = JSON.parse(fs.readFileSync(path.join(this.dir, name), 'utf8'));
				if (meta && typeof meta.id === 'string') {
					items.push(meta);
				}
			} catch {
				// A truncated file from a crash mid-write. Skipping it is better
				// than failing the whole list over one bad record.
			}
		}
		items.sort((a, b) => (b.updatedAt || 0) - (a.updatedAt || 0));
		return items;
	}

	/** @returns {object|null} The full record, transcript included. */
	load(id) {
		if (this.dir === null || !ID_PATTERN.test(id)) {
			return null;
		}
		try {
			const meta = JSON.parse(fs.readFileSync(this.metaPath(id), 'utf8'));
			const events = JSON.parse(fs.readFileSync(this.eventsPath(id), 'utf8'));
			return { ...meta, events: Array.isArray(events) ? events : [] };
		} catch {
			return null;
		}
	}

	/**
	 * Persists a record, trimming it first. Mutates `record.events` when it is
	 * over the cap, so the in-memory copy and the file agree.
	 */
	save(record) {
		if (this.dir === null || !record || !ID_PATTERN.test(record.id) || record.events.length === 0) {
			return;
		}
		if (record.events.length > MAX_EVENTS) {
			record.events.splice(0, record.events.length - MAX_EVENTS);
		}
		const meta = {
			id: record.id,
			title: record.title,
			createdAt: record.createdAt,
			updatedAt: record.updatedAt,
			engineSessionId: record.engineSessionId,
			messages: record.events.filter((event) => event.type === 'user').length,
		};
		try {
			fs.mkdirSync(this.dir, { recursive: true });
			// Events first: the meta file is what the list reads, so it should
			// never advertise a transcript that is not there yet.
			fs.writeFileSync(this.eventsPath(record.id), JSON.stringify(record.events));
			fs.writeFileSync(this.metaPath(record.id), JSON.stringify(meta));
		} catch {
			// Losing a transcript is not worth interrupting the conversation over.
			return;
		}
		this.prune();
	}

	remove(id) {
		if (this.dir === null || !ID_PATTERN.test(id)) {
			return;
		}
		for (const file of [this.metaPath(id), this.eventsPath(id)]) {
			try {
				fs.unlinkSync(file);
			} catch {
				/* already gone */
			}
		}
	}

	/** Drops the oldest conversations once there are more than the cap. */
	prune() {
		const items = this.list();
		for (const item of items.slice(MAX_SESSIONS)) {
			this.remove(item.id);
		}
	}
}

module.exports = { SessionStore, MAX_EVENTS, MAX_SESSIONS };
