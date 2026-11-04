'use strict';

const vscode = require('vscode');
const path = require('path');

const { findProjectRoot } = require('./gofiConfig.js');
const { listRuns, readRun, runMarkup, RUNS } = require('./runView.js');

const PANEL_TYPE = 'gofi-ai.runPanel';

/**
 * A delivery run in an editor tab: its flow as a graph and its trace matrix
 * (runView.js), redrawn as the run writes its folder — a run in progress is
 * followed live.
 */
class RunPanel {
	/** @param {vscode.ExtensionContext} context */
	constructor(context) {
		this.context = context;
		/** @type {vscode.WebviewPanel | null} */
		this.panel = null;
		/** @type {vscode.FileSystemWatcher | null} */
		this.watcher = null;
		this.root = '';
		this.run = '';
	}

	/** Asks which run, newest first, and opens it. */
	async pick() {
		const folder = vscode.workspace.workspaceFolders?.[0];
		const root = folder && findProjectRoot(folder.uri.fsPath);
		if (!root) {
			vscode.window.showWarningMessage('GOFI AI: abra um projeto gofi antes.');
			return;
		}
		const runs = listRuns(root);
		if (!runs.length) {
			vscode.window.showInformationMessage('GOFI AI: nenhuma execução neste projeto — gofi run <mudança>.');
			return;
		}
		const chosen = await vscode.window.showQuickPick(runs.map((r) => ({
			label: r.change || r.run,
			description: r.run,
			detail: r.node ? `em ${r.node}` : `terminou ${r.outcome}`,
			run: r.run,
		})), { placeHolder: 'Qual execução?' });
		if (chosen) {
			this.open(root, chosen.run);
		}
	}

	/**
	 * @param {string} root
	 * @param {string} run
	 */
	open(root, run) {
		this.root = root;
		this.run = run;
		if (!this.panel) {
			this.panel = vscode.window.createWebviewPanel(PANEL_TYPE, 'GOFI — execução', { viewColumn: vscode.ViewColumn.Beside, preserveFocus: true },
				{ enableScripts: false, localResourceRoots: [vscode.Uri.joinPath(this.context.extensionUri, 'media')] });
			this.panel.onDidDispose(() => {
				this.panel = null;
				this.stopWatching();
			});
		}
		this.panel.title = `GOFI — ${run}`;
		this.watch();
		this.draw();
		this.panel.reveal(this.panel.viewColumn, true);
	}

	draw() {
		if (!this.panel) {
			return;
		}
		const webview = this.panel.webview;
		const css = webview.asWebviewUri(vscode.Uri.joinPath(this.context.extensionUri, 'media', 'run.css'));
		const html = runMarkup(readRun(this.root, this.run), String(css));
		const csp = `<meta http-equiv="Content-Security-Policy" content="default-src 'none'; style-src ${webview.cspSource};">`;
		webview.html = html.replace('<meta charset="UTF-8">', `<meta charset="UTF-8">${csp}`);
	}

	watch() {
		this.stopWatching();
		const pattern = new vscode.RelativePattern(path.join(this.root, RUNS, this.run), '*.{json,ndjson}');
		this.watcher = vscode.workspace.createFileSystemWatcher(pattern);
		// A node writes several files at once: one redraw for them, not one
		// each — reassigning the page loses its scroll.
		const redraw = () => {
			clearTimeout(this.pending);
			this.pending = setTimeout(() => this.draw(), 200);
		};
		this.watcher.onDidChange(redraw);
		this.watcher.onDidCreate(redraw);
	}

	stopWatching() {
		clearTimeout(this.pending);
		if (this.watcher) {
			this.watcher.dispose();
			this.watcher = null;
		}
	}

	dispose() {
		this.stopWatching();
		if (this.panel) {
			this.panel.dispose();
		}
	}
}

module.exports = { RunPanel };
