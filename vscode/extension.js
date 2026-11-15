'use strict';

const vscode = require('vscode');
const { findProjectRoot } = require('./src/project.js');
const { hasGraph } = require('./src/projectModel.js');
const { GraphPanel } = require('./src/graphView.js');
const { RunPanel } = require('./src/runPanel.js');
const { ChatTerminal } = require('./src/chatTerminal.js');

/**
 * gofi inside the editor: the chat in a terminal, and the two things a
 * terminal cannot draw — the project's graph and a delivery run's flow.
 *
 * The conversation itself is `gofi chat`, the same one a plain terminal runs.
 * The extension only opens it, lends it the editor's keys and points it at the
 * file being edited; everything a conversation does lives in the CLI, once.
 *
 * @param {vscode.ExtensionContext} context
 */
function activate(context) {
	const chat = new ChatTerminal(context);
	const graph = new GraphPanel(context);
	const runs = new RunPanel(context);

	context.subscriptions.push(
		// Ctrl+Shift+P → "GOFI: Chat in Terminal", or the gofi face in the
		// editor's title bar.
		vscode.commands.registerCommand('gofi-ai.chatInTerminal', () => chat.open()),
		// The id the title-bar button had while it opened a panel: kept so a
		// keybinding made for it still opens the chat.
		vscode.commands.registerCommand('gofi-ai.open', () => chat.open()),
		vscode.commands.registerCommand('gofi-ai.chatTerminalPaste', () => chat.paste()),
		vscode.commands.registerCommand('gofi-ai.mention', () => chat.mention()),
		vscode.commands.registerCommand('gofi-ai.graph', (name) => graph.open(name)),
		vscode.commands.registerCommand('gofi-ai.docsBuild', () => graph.build()),
		vscode.commands.registerCommand('gofi-ai.run', () => runs.pick()),
		vscode.commands.registerCommand('gofi-ai.doctor', () => runDoctor()),
		{ dispose: () => chat.dispose() },
		{ dispose: () => graph.dispose() },
		{ dispose: () => runs.dispose() },
	);

	// A project that already has a graph can open showing it: the panel is a
	// reading surface, and what keeps one from being used is having to know it
	// exists. Off unless asked for.
	if (vscode.workspace.getConfiguration('gofiAI').get('graph.openOnStartup') === true) {
		const folder = vscode.workspace.workspaceFolders?.[0];
		if (folder) {
			const root = findProjectRoot(folder.uri.fsPath);
			if (root && hasGraph(root)) {
				graph.open();
			}
		}
	}
}

/** Runs `gofi doctor` where the person can read it: what is installed, and what is missing. */
function runDoctor() {
	const folder = vscode.workspace.workspaceFolders?.[0];
	if (!folder) {
		vscode.window.showWarningMessage('GOFI: abra uma pasta antes.');
		return;
	}
	const gofi = String(vscode.workspace.getConfiguration('gofiAI').get('gofiPath') || '').trim() || 'gofi';
	const terminal = vscode.window.createTerminal({ name: 'GOFI doctor', cwd: folder.uri });
	terminal.show();
	terminal.sendText(`${/\s/.test(gofi) ? `"${gofi}"` : gofi} doctor`);
}

function deactivate() {}

module.exports = { activate, deactivate };
