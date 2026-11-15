'use strict';

const vscode = require('vscode');
const { findProjectRoot } = require('./gofiConfig.js');

/** The terminal's name — also how it is found again after a window reload. */
const TERMINAL_NAME = 'GOFI Chat';

/** Set while the chat's terminal is the active one; the keybindings read it. */
const CONTEXT_KEY = 'gofi-ai.chatTerminalFocus';

/** What the chat reads as "break the line": alt+enter. */
const NEWLINE = '\u001b\r';

/** What the chat reads as "attach the clipboard's picture": ctrl+v. */
const PASTE_IMAGE = '\u0016';

/**
 * The command line that starts the chat in the user's shell.
 *
 * It goes through the shell rather than as the terminal's own process on
 * purpose: `gofi chat` drives the `claude` binary, and both are usually put on
 * PATH by the shell's own startup files.
 *
 * @param {{ gofiPath?: string, model?: string, platform?: string }} options
 */
function chatCommandLine({ gofiPath, model, platform = process.platform } = {}) {
	let exe = String(gofiPath || '').trim() || 'gofi';
	if (/\s/.test(exe)) {
		exe = platform === 'win32' ? `& "${exe}"` : `'${exe.replace(/'/g, `'\\''`)}'`;
	}
	const name = String(model || '').trim();
	// A model name is a word; anything else is not passed to a shell.
	return /^[\w.:\-\[\]]+$/.test(name) ? `${exe} chat --model ${name}` : `${exe} chat`;
}

/**
 * `gofi chat` in a VSCode terminal: the terminal chat, with the editor around
 * it. One terminal per window — asking for it again shows the one there is.
 */
class ChatTerminal {
	constructor() {
		this.subscriptions = [
			vscode.window.onDidChangeActiveTerminal?.((terminal) => this.setFocus(terminal)),
			vscode.window.onDidCloseTerminal?.(() => this.setFocus(vscode.window.activeTerminal)),
		].filter(Boolean);
		this.setFocus(vscode.window.activeTerminal);
	}

	/** Whether a terminal is the chat's. */
	isChat(terminal) {
		return Boolean(terminal) && terminal.name === TERMINAL_NAME && terminal.exitStatus === undefined;
	}

	setFocus(terminal) {
		vscode.commands.executeCommand('setContext', CONTEXT_KEY, this.isChat(terminal));
	}

	/** Shows the chat's terminal, starting the chat in a new one if there is none. */
	open() {
		const existing = (vscode.window.terminals || []).find((t) => this.isChat(t));
		if (existing) {
			existing.show();
			return existing;
		}
		const folder = vscode.workspace.workspaceFolders?.[0];
		if (!folder) {
			vscode.window.showWarningMessage('GOFI: abra uma pasta antes.');
			return undefined;
		}
		const config = vscode.workspace.getConfiguration('gofiAI');
		// The chat runs where the project is: a workspace opened on a folder
		// inside it still finds the skills and the runs.
		const root = findProjectRoot(folder.uri.fsPath);
		const options = {
			name: TERMINAL_NAME,
			cwd: root ? vscode.Uri.file(root) : folder.uri,
			iconPath: new vscode.ThemeIcon('comment-discussion'),
		};
		if (config.get('terminal.location') !== 'panel') {
			options.location = { viewColumn: vscode.ViewColumn.Beside };
		}
		const terminal = vscode.window.createTerminal(options);
		terminal.show();
		terminal.sendText(chatCommandLine({ gofiPath: config.get('gofiPath'), model: config.get('model') }));
		this.setFocus(terminal);
		return terminal;
	}

	/**
	 * ctrl+v in the chat's terminal. VSCode keeps the key for its own paste,
	 * which pastes text and nothing else: with text in the clipboard that is
	 * what happens; with none, the key is handed to the chat, which asks the
	 * system clipboard for a picture.
	 */
	async paste() {
		const terminal = vscode.window.activeTerminal;
		let text = '';
		try {
			text = await vscode.env.clipboard.readText();
		} catch {
			// An unreadable clipboard is treated as one with no text.
		}
		if (text !== '' || !this.isChat(terminal)) {
			await vscode.commands.executeCommand('workbench.action.terminal.paste');
			return;
		}
		terminal.sendText(PASTE_IMAGE, false);
	}

	dispose() {
		for (const s of this.subscriptions) {
			s.dispose();
		}
	}
}

module.exports = { ChatTerminal, chatCommandLine, TERMINAL_NAME, CONTEXT_KEY, NEWLINE, PASTE_IMAGE };
