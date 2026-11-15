'use strict';

const path = require('path');
const vscode = require('vscode');
const { findProjectRoot } = require('./project.js');

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
 * How the chat is told about a file: `@path`, with the lines when some are
 * selected — `@src/app.go#L10-24`. The path is relative to where the chat
 * runs, which is what the agent resolves it against.
 *
 * @param {string} root Where the chat runs
 * @param {string} file Absolute path of the file
 * @param {{ start: number, end: number } | null} lines 1-based, inclusive
 */
function mentionOf(root, file, lines) {
	let rel = path.relative(root, file);
	if (rel === '' || rel.startsWith('..') || path.isAbsolute(rel)) {
		rel = file; // outside the project: the whole path is the only honest one
	}
	rel = rel.split(path.sep).join('/');
	// A path with a space would end at it: quoted, it stays one word.
	const target = /\s/.test(rel) ? `"${rel}"` : rel;
	if (!lines) {
		return `@${target} `;
	}
	return lines.start === lines.end ? `@${target}#L${lines.start} ` : `@${target}#L${lines.start}-${lines.end} `;
}

/**
 * `gofi chat` in a VSCode terminal: the terminal chat, with the editor around
 * it. One terminal per window — asking for it again shows the one there is.
 */
class ChatTerminal {
	/** @param {vscode.ExtensionContext} [context] */
	constructor(context) {
		this.context = context;
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

	/** The chat's terminal, when there is one. */
	find() {
		return (vscode.window.terminals || []).find((t) => this.isChat(t));
	}

	/** Where the chat runs: the project's root, or the folder that is open. */
	root() {
		const folder = vscode.workspace.workspaceFolders?.[0];
		if (!folder) {
			return null;
		}
		// The chat runs where the project is: a workspace opened on a folder
		// inside it still finds the skills and the runs.
		return findProjectRoot(folder.uri.fsPath) || folder.uri.fsPath;
	}

	/** Shows the chat's terminal, starting the chat in a new one if there is none. */
	open() {
		const existing = this.find();
		if (existing) {
			existing.show();
			return existing;
		}
		const root = this.root();
		if (!root) {
			vscode.window.showWarningMessage('GOFI: abra uma pasta antes.');
			return undefined;
		}
		const config = vscode.workspace.getConfiguration('gofiAI');
		const options = {
			name: TERMINAL_NAME,
			cwd: vscode.Uri.file(root),
			// The tab wears the gofi face, like the button that opens it.
			iconPath: this.context
				? vscode.Uri.joinPath(this.context.extensionUri, 'images', 'gofi.svg')
				: new vscode.ThemeIcon('comment-discussion'),
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

	/**
	 * Points the chat at the file in the editor: its path goes into the prompt
	 * as an `@` mention, with the selected lines when there are some. Nothing
	 * is sent — the person writes the request around it.
	 *
	 * With no chat open there is no prompt to write into: the chat is opened,
	 * and the mention waits for the person to ask again once it is up.
	 */
	mention() {
		const editor = vscode.window.activeTextEditor;
		if (!editor || editor.document.uri.scheme !== 'file') {
			vscode.window.showWarningMessage('GOFI: abra um arquivo para mencioná-lo no chat.');
			return;
		}
		const terminal = this.find();
		if (!terminal) {
			this.open();
			vscode.window.setStatusBarMessage?.('GOFI: chat aberto — repita o comando para mencionar o arquivo', 5000);
			return;
		}
		const selection = editor.selection;
		const lines = selection && !selection.isEmpty
			? {
				start: selection.start.line + 1,
				// A selection that ends at the start of a line does not include it.
				end: selection.end.character === 0 && selection.end.line > selection.start.line ? selection.end.line : selection.end.line + 1,
			}
			: null;
		terminal.show();
		terminal.sendText(mentionOf(this.root() || '', editor.document.uri.fsPath, lines), false);
	}

	dispose() {
		for (const s of this.subscriptions) {
			s.dispose();
		}
	}
}

module.exports = { ChatTerminal, chatCommandLine, mentionOf, TERMINAL_NAME, CONTEXT_KEY, NEWLINE, PASTE_IMAGE };
