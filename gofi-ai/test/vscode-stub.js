'use strict';

const path = require('path');
const Module = require('module');

/**
 * A stand-in for the `vscode` module, which only exists inside the extension
 * host.
 *
 * Enough of it to construct a Chat and drive its logic from a plain `node`
 * run — the parts worth testing (what may touch the repository, what is
 * written to disk and read back) are ours, not the editor's.
 */
function installVscodeStub() {
	const stub = {
		Uri: {
			file: (fsPath) => ({ fsPath, scheme: 'file' }),
			joinPath: (base, ...parts) => ({ fsPath: path.join(base.fsPath, ...parts) }),
		},
		workspace: {
			workspaceFolders: undefined,
			getConfiguration: () => ({ get: () => undefined }),
			createFileSystemWatcher: () => ({
				onDidCreate() {},
				onDidChange() {},
				onDidDelete() {},
				dispose() {},
			}),
			asRelativePath: (p) => String(p),
		},
		window: {
			activeTextEditor: undefined,
			onDidChangeActiveTextEditor: () => ({ dispose() {} }),
			onDidChangeTextEditorSelection: () => ({ dispose() {} }),
			showWarningMessage() {},
			showInformationMessage: async () => undefined,
			createTerminal: () => ({ show() {}, sendText() {} }),
		},
		commands: { executeCommand() {} },
		RelativePattern: class {},
		ViewColumn: { One: 1, Beside: 2 },
		Position: class {},
		Selection: class {},
		Range: class {},
		TextEditorRevealType: { InCenter: 2 },
		ThemeIcon: class {},
	};

	const realResolve = Module._resolveFilename;
	Module._resolveFilename = function (request, ...rest) {
		if (request === 'vscode') {
			return 'vscode';
		}
		return realResolve.call(this, request, ...rest);
	};
	require.cache.vscode = { id: 'vscode', filename: 'vscode', loaded: true, exports: stub };
	return stub;
}

/** A minimal `ExtensionContext`, with workspace state kept in memory. */
function makeContext(storagePath) {
	const state = new Map();
	return {
		extensionUri: { fsPath: path.join(__dirname, '..') },
		storageUri: storagePath ? { fsPath: storagePath } : undefined,
		subscriptions: [],
		workspaceState: {
			get: (key, fallback) => (state.has(key) ? state.get(key) : fallback),
			update: (key, value) => state.set(key, value),
		},
	};
}

/** A runner shared by the suites, so each file stays a plain `node` script. */
function runner() {
	const tests = [];
	return {
		test: (name, fn) => tests.push([name, fn]),
		run() {
			let failed = 0;
			for (const [name, fn] of tests) {
				try {
					fn();
					console.log(`  ok    ${name}`);
				} catch (err) {
					failed++;
					console.log(`  FALHA ${name}\n        ${err.message}`);
				}
			}
			console.log(`\n${tests.length - failed}/${tests.length} passaram`);
			process.exit(failed === 0 ? 0 : 1);
		},
	};
}

module.exports = { installVscodeStub, makeContext, runner };
