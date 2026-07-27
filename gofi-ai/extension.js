'use strict';

const vscode = require('vscode');
const { ChatManager, VIEW_ID } = require('./src/chatManager.js');
const { resolveProvider } = require('./src/providers/index.js');
const { readGofiProject } = require('./src/gofiConfig.js');

/** @param {vscode.ExtensionContext} context */
function activate(context) {
	const chats = new ChatManager(context);

	context.subscriptions.push(
		// `retainContextWhenHidden` keeps the transcript alive when the user
		// switches to another view container — losing a conversation because the
		// panel scrolled out of sight is not a tradeoff worth the memory.
		vscode.window.registerWebviewViewProvider(VIEW_ID, chats, {
			webviewOptions: { retainContextWhenHidden: true },
		}),

		vscode.commands.registerCommand('gofi-ai.focus', () => vscode.commands.executeCommand(`${VIEW_ID}.focus`)),
		vscode.commands.registerCommand('gofi-ai.openInEditor', () => chats.openInEditor()),
		// The editor title-bar button routes through the user's preference, so
		// one icon serves both "dock it in the sidebar" and "give me a page".
		vscode.commands.registerCommand('gofi-ai.open', () => {
			const where = vscode.workspace.getConfiguration('gofiAI').get('openLocation');
			if (where === 'beside') {
				chats.openInEditor();
			} else {
				vscode.commands.executeCommand(`${VIEW_ID}.focus`);
			}
		}),
		vscode.commands.registerCommand('gofi-ai.newSession', () => chats.newSession()),
		vscode.commands.registerCommand('gofi-ai.stop', () => chats.stop()),
		vscode.commands.registerCommand('gofi-ai.openTerminal', () => openEngineTerminal()),
		vscode.commands.registerCommand('gofi-ai.doctor', () => runDoctor()),
		{ dispose: () => chats.dispose() },
	);
}

/**
 * Drops the user into the engine's own interactive TUI in a VSCode terminal.
 *
 * The chat panel covers the normal path; this is the escape hatch for what a
 * webview cannot host — the engine's own approval prompts, `/resume`, and any
 * interactive flow that expects a real terminal.
 */
function openEngineTerminal() {
	const folder = vscode.workspace.workspaceFolders?.[0];
	if (!folder) {
		vscode.window.showWarningMessage('GOFI AI: abra uma pasta antes.');
		return;
	}
	const config = vscode.workspace.getConfiguration('gofiAI');
	const exe = (config.get('claudeCode.executable') || '').trim() || 'claude';
	const project = readGofiProject(folder.uri.fsPath);
	const model = (config.get('model') || '').trim() || (project && project.model) || '';

	const terminal = vscode.window.createTerminal({
		name: 'GOFI AI',
		cwd: folder.uri,
		iconPath: new vscode.ThemeIcon('comment-discussion'),
	});
	terminal.show();
	terminal.sendText(model === '' ? exe : `${exe} --model ${model}`);
}

/** Reports whether the configured engine is actually reachable, and from where. */
async function runDoctor() {
	const folder = vscode.workspace.workspaceFolders?.[0];
	const config = vscode.workspace.getConfiguration('gofiAI');
	const provider = resolveProvider(config);
	const cwd = folder ? folder.uri.fsPath : process.cwd();
	const project = folder ? readGofiProject(cwd) : null;

	const result = await provider.probe({ cwd, config, project });
	const where = project ? `projeto ${project.name}` : 'fora de um projeto gofi';

	if (result.ok) {
		vscode.window.showInformationMessage(`GOFI AI — ${provider.label} ok (${result.detail}), ${where}.`);
		return;
	}
	const choice = await vscode.window.showErrorMessage(
		`GOFI AI — ${provider.label} indisponível: ${result.detail}`,
		...(result.hint ? ['Abrir configurações'] : []),
	);
	if (choice === 'Abrir configurações') {
		vscode.commands.executeCommand('workbench.action.openSettings', 'gofiAI');
	}
}

function deactivate() {}

module.exports = { activate, deactivate };
