'use strict';

// GOFI: Chat in Terminal — one terminal running `gofi chat` in the project
// root, found again by name; a paste that hands ctrl+v to the chat only when
// the clipboard has no text; and the file in the editor mentioned in the
// prompt. The chat itself is the CLI's: nothing of a conversation lives here.
const assert = require('assert');
const fs = require('fs');
const os = require('os');
const path = require('path');
const { installVscodeStub, runner } = require('./vscode-stub.js');

const vscode = installVscodeStub();
const { test, run } = runner();

const created = [];
const executed = [];
let clipboard = '';
let settings = {};

vscode.window.terminals = [];
vscode.window.activeTerminal = undefined;
vscode.window.onDidChangeActiveTerminal = () => ({ dispose() {} });
vscode.window.onDidCloseTerminal = () => ({ dispose() {} });
vscode.window.createTerminal = (options) => {
	const terminal = { name: options.name, options, exitStatus: undefined, shown: 0, sent: [], show() { this.shown++; }, sendText(text, newline) { this.sent.push([text, newline]); } };
	created.push(terminal);
	vscode.window.terminals.push(terminal);
	return terminal;
};
vscode.commands.executeCommand = (...args) => { executed.push(args); };
vscode.workspace.getConfiguration = () => ({ get: (key) => settings[key] });
vscode.env = { clipboard: { readText: async () => clipboard } };

vscode.window.setStatusBarMessage = () => {};
const { ChatTerminal, chatCommandLine, mentionOf, TERMINAL_NAME, CONTEXT_KEY, PASTE_IMAGE } = require('../src/chatTerminal.js');

function reset() {
	created.length = 0;
	executed.length = 0;
	vscode.window.terminals = [];
	vscode.window.activeTerminal = undefined;
	clipboard = '';
	settings = {};
}

function project() {
	const root = fs.mkdtempSync(path.join(os.tmpdir(), 'gofi-chat-term-'));
	fs.writeFileSync(path.join(root, '.gofi.yaml'), 'project:\n    name: x\n');
	fs.mkdirSync(path.join(root, 'backend', 'api'), { recursive: true });
	return root;
}

test('the command line starts gofi chat, with the model when one is set', () => {
	assert.strictEqual(chatCommandLine({}), 'gofi chat');
	assert.strictEqual(chatCommandLine({ gofiPath: '  ', model: 'sonnet' }), 'gofi chat --model sonnet');
	assert.strictEqual(chatCommandLine({ gofiPath: '/opt/my tools/gofi', platform: 'linux' }), `'/opt/my tools/gofi' chat`);
	assert.strictEqual(chatCommandLine({ gofiPath: 'C:\\my tools\\gofi.exe', platform: 'win32' }), '& "C:\\my tools\\gofi.exe" chat');
	// What is not a model name never reaches the shell.
	assert.strictEqual(chatCommandLine({ model: 'x; rm -rf ~' }), 'gofi chat');
});

test('it opens one terminal in the project root and shows it again when asked', () => {
	reset();
	const root = project();
	vscode.workspace.workspaceFolders = [{ uri: vscode.Uri.file(path.join(root, 'backend', 'api')) }];
	settings = { model: 'opus' };
	const chat = new ChatTerminal();

	const terminal = chat.open();
	assert.strictEqual(created.length, 1);
	assert.strictEqual(terminal.name, TERMINAL_NAME);
	assert.strictEqual(terminal.options.cwd.fsPath, root, 'the chat runs where .gofi.yaml is');
	assert.deepStrictEqual(terminal.options.location, { viewColumn: vscode.ViewColumn.Beside });
	assert.deepStrictEqual(terminal.sent, [['gofi chat --model opus', undefined]]);
	assert.ok(executed.some((c) => c[0] === 'setContext' && c[1] === CONTEXT_KEY && c[2] === true));

	assert.strictEqual(chat.open(), terminal);
	assert.strictEqual(created.length, 1, 'asked again, the terminal that exists is shown');
	assert.strictEqual(terminal.shown, 2);
	assert.strictEqual(terminal.sent.length, 1, 'and the chat is not started over');

	// A terminal whose process ended is not the chat any more.
	terminal.exitStatus = { code: 0 };
	chat.open();
	assert.strictEqual(created.length, 2);
});

test('the panel setting opens it among the terminals', () => {
	reset();
	vscode.workspace.workspaceFolders = [{ uri: vscode.Uri.file(project()) }];
	settings = { 'terminal.location': 'panel' };
	const terminal = new ChatTerminal().open();
	assert.strictEqual(terminal.options.location, undefined);
});

test('without a folder it says so and opens nothing', () => {
	reset();
	vscode.workspace.workspaceFolders = undefined;
	assert.strictEqual(new ChatTerminal().open(), undefined);
	assert.strictEqual(created.length, 0);
});

test('ctrl+v pastes text as always and hands the key to the chat when there is none', async () => {
	reset();
	vscode.workspace.workspaceFolders = [{ uri: vscode.Uri.file(project()) }];
	const chat = new ChatTerminal();
	const terminal = chat.open();
	vscode.window.activeTerminal = terminal;
	executed.length = 0;

	clipboard = 'some text';
	await chat.paste();
	assert.deepStrictEqual(executed, [['workbench.action.terminal.paste']]);
	assert.strictEqual(terminal.sent.length, 1, 'text goes through the terminal\'s own paste');

	executed.length = 0;
	clipboard = '';
	await chat.paste();
	assert.deepStrictEqual(executed, []);
	assert.deepStrictEqual(terminal.sent[1], [PASTE_IMAGE, false], 'no text: the chat is asked for the picture');

	// In any other terminal the key stays VSCode's.
	vscode.window.activeTerminal = { name: 'zsh', exitStatus: undefined, sendText() { throw new Error('not the chat'); } };
	await chat.paste();
	assert.deepStrictEqual(executed, [['workbench.action.terminal.paste']]);
});

test('a mention is the path as the chat resolves it, with the lines that are selected', () => {
	const root = path.join(path.sep, 'work', 'shop');
	assert.strictEqual(mentionOf(root, path.join(root, 'internal', 'cart', 'cart.go'), null), '@internal/cart/cart.go ');
	assert.strictEqual(mentionOf(root, path.join(root, 'internal', 'cart', 'cart.go'), { start: 10, end: 24 }), '@internal/cart/cart.go#L10-24 ');
	assert.strictEqual(mentionOf(root, path.join(root, 'main.go'), { start: 7, end: 7 }), '@main.go#L7 ');
	assert.strictEqual(mentionOf(root, path.join(root, 'docs', 'my notes.md'), null), '@"docs/my notes.md" ');
	// A file outside the project keeps its whole path.
	const outside = path.join(path.sep, 'etc', 'hosts');
	assert.strictEqual(mentionOf(root, outside, null), `@${outside.split(path.sep).join('/')} `);
});

test('mentioning writes into the prompt of the chat that is open, and sends nothing', () => {
	reset();
	const root = project();
	vscode.workspace.workspaceFolders = [{ uri: vscode.Uri.file(root) }];
	const chat = new ChatTerminal();
	const file = path.join(root, 'backend', 'api', 'server.go');
	vscode.window.activeTextEditor = {
		document: { uri: { scheme: 'file', fsPath: file } },
		selection: { isEmpty: false, start: { line: 4, character: 2 }, end: { line: 9, character: 0 } },
	};

	// With no chat open there is no prompt to write into: the chat is opened.
	chat.mention();
	assert.strictEqual(created.length, 1);
	assert.strictEqual(created[0].sent.length, 1, 'only the command that starts the chat was sent');

	chat.mention();
	assert.deepStrictEqual(created[0].sent[1], ['@backend/api/server.go#L5-9 ', false], 'the lines selected, the last one left out when the selection ends at its start');

	vscode.window.activeTextEditor.selection = { isEmpty: true, start: { line: 0, character: 0 }, end: { line: 0, character: 0 } };
	chat.mention();
	assert.deepStrictEqual(created[0].sent[2], ['@backend/api/server.go ', false]);

	vscode.window.activeTextEditor = undefined;
	chat.mention();
	assert.strictEqual(created[0].sent.length, 3, 'with no file in the editor nothing is written');
});

test('the manifest opens the chat from the gofi button and keeps no panel of its own', () => {
	const manifest = JSON.parse(fs.readFileSync(path.join(__dirname, '..', 'package.json'), 'utf8'));
	const contributes = manifest.contributes;
	const command = contributes.commands.find((c) => c.command === 'gofi-ai.chatInTerminal');
	assert.strictEqual(`${command.category}: ${command.title}`, 'GOFI: Chat in Terminal');
	assert.strictEqual(command.icon, 'images/gofi.svg');
	assert.ok(fs.existsSync(path.join(__dirname, '..', command.icon)), 'the button has its picture');
	assert.deepStrictEqual(contributes.menus['editor/title'].map((m) => m.command), ['gofi-ai.chatInTerminal']);
	assert.strictEqual(contributes.views, undefined, 'the conversation is the terminal chat: there is no webview panel');
	assert.strictEqual(contributes.viewsContainers, undefined);

	for (const binding of contributes.keybindings.filter((b) => b.command !== 'gofi-ai.mention')) {
		assert.match(binding.when, /terminalFocus && gofi-ai\.chatTerminalFocus/, 'the chat\'s keys are bound in its terminal only');
	}
	const newline = contributes.keybindings.find((b) => b.key === 'shift+enter');
	assert.strictEqual(newline.args.text, '\u001b\r', 'shift+enter is what the chat reads as alt+enter');

	// Every command the manifest offers is one the extension registers.
	const source = fs.readFileSync(path.join(__dirname, '..', 'extension.js'), 'utf8');
	for (const c of contributes.commands) {
		assert.ok(source.includes(`'${c.command}'`), `${c.command} is offered and not registered`);
	}
	// And every setting it declares is one it reads.
	const code = ['extension.js', 'src/chatTerminal.js', 'src/graphView.js', 'src/runPanel.js']
		.map((f) => fs.readFileSync(path.join(__dirname, '..', f), 'utf8')).join('\n');
	for (const key of Object.keys(contributes.configuration.properties)) {
		assert.ok(code.includes(`'${key.replace('gofiAI.', '')}'`), `${key} is declared and never read`);
	}
});

run();
