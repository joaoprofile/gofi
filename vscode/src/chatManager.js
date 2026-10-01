'use strict';

const vscode = require('vscode');
const { Chat, VIEW_ID, RUNNING_CONTEXT } = require('./chat.js');
const { SessionStore } = require('./history.js');

/**
 * Owns the open chats and decides which one a command acts on.
 *
 * There are two kinds of surface. The sidebar holds one long-lived chat, since
 * a view container can only host a single view. Editor tabs each hold their
 * own — that is the whole point of opening a second one: an independent
 * context, so a `/gofi-spec` thread and a `/gofi-eng` thread can run at the
 * same time without sharing history, ledger or cached prefix.
 *
 * @implements {vscode.WebviewViewProvider}
 */
class ChatManager {
	/** @param {vscode.ExtensionContext} context */
	constructor(context) {
		this.context = context;
		/**
		 * Saved conversations, shared by every chat: the history is a property of
		 * the workspace, so a thread started in a tab is one the sidebar can pick
		 * up tomorrow.
		 */
		this.store = new SessionStore(context);
		/** @type {Chat | null} The chat docked in the sidebar. */
		this.sidebar = null;
		/** @type {Map<vscode.WebviewPanel, Chat>} */
		this.panels = new Map();
		/** @type {Chat | null} What a command without a target acts on. */
		this.active = null;
		/** Ordinal for default titles. */
		this.nextId = 1;
	}

	// ── sidebar ─────────────────────────────────────────────────────────────

	/** @param {vscode.WebviewView} view */
	resolveWebviewView(view) {
		if (!this.sidebar) {
			this.sidebar = new Chat(this.context, 0, this.store);
		}
		const chat = this.sidebar;
		chat.attach(view.webview);
		this.focus(chat);

		view.onDidChangeVisibility(() => {
			if (view.visible) {
				this.focus(chat);
			}
		});
		view.onDidDispose(() => {
			chat.detach(view.webview);
			if (this.sidebar === chat) {
				this.sidebar = null;
			}
			this.forget(chat);
		});
	}

	// ── editor tabs ─────────────────────────────────────────────────────────

	/**
	 * Opens a new conversation as an editor tab beside the code.
	 *
	 * Always a new one. Revealing an existing tab instead would collapse "open
	 * the chat" and "start another thread" into one gesture, leaving no way to
	 * ask for the second.
	 */
	openInEditor() {
		const chat = new Chat(this.context, this.nextId++, this.store);
		const panel = vscode.window.createWebviewPanel(
			'gofi-ai.chatPanel',
			chat.title,
			{ viewColumn: vscode.ViewColumn.Beside, preserveFocus: false },
			{
				enableScripts: true,
				retainContextWhenHidden: true,
				localResourceRoots: [vscode.Uri.joinPath(this.context.extensionUri, 'media')],
			},
		);
		panel.iconPath = {
			light: vscode.Uri.joinPath(this.context.extensionUri, 'images', 'panel-icon-light.svg'),
			dark: vscode.Uri.joinPath(this.context.extensionUri, 'images', 'panel-icon-dark.svg'),
		};
		// The tab takes the name of the first question asked in it — several
		// tabs called "GOFI AI" are indistinguishable.
		chat.onTitle = (title) => {
			panel.title = title;
		};

		this.panels.set(panel, chat);
		chat.attach(panel.webview);
		this.focus(chat);

		panel.onDidChangeViewState(() => {
			if (panel.active) {
				this.focus(chat);
			}
		});
		panel.onDidDispose(() => {
			this.panels.delete(panel);
			chat.detach(panel.webview);
			this.forget(chat);
		});
		return chat;
	}

	// ── active chat ─────────────────────────────────────────────────────────

	/**
	 * The chat a command acts on: whatever the user last looked at, falling
	 * back to any open one so a command from the palette is never a no-op.
	 */
	current() {
		if (this.active) {
			return this.active;
		}
		for (const chat of this.panels.values()) {
			return chat;
		}
		return this.sidebar;
	}

	focus(chat) {
		this.active = chat;
		// The stop button follows the chat in view, not whichever turn happens
		// to be running somewhere else.
		vscode.commands.executeCommand('setContext', RUNNING_CONTEXT, chat.running === true);
	}

	forget(chat) {
		if (this.active === chat) {
			this.active = null;
		}
	}

	// ── commands ────────────────────────────────────────────────────────────

	newSession() {
		const chat = this.current();
		if (chat) {
			chat.newSession();
		}
	}

	/** Opens (or closes) the list of saved conversations in the chat in view. */
	history() {
		const chat = this.current();
		if (chat) {
			chat.post({ type: 'toggleHistory' });
		}
	}

	stop() {
		const chat = this.current();
		if (chat) {
			chat.stop();
		}
	}

	/** Ends every conversation. */
	dispose() {
		for (const [panel, chat] of this.panels) {
			chat.dispose();
			panel.dispose();
		}
		this.panels.clear();
		if (this.sidebar) {
			this.sidebar.dispose();
			this.sidebar = null;
		}
		this.active = null;
	}
}

module.exports = { ChatManager, VIEW_ID, RUNNING_CONTEXT };
