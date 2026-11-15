'use strict';

const vscode = require('vscode');
const crypto = require('crypto');
const path = require('path');

const { findProjectRoot } = require('./project.js');
const model = require('./projectModel.js');

const PANEL_TYPE = 'gofi-ai.graphPanel';

/**
 * The project's graph in an editor tab: what gofi indexed — the documents and
 * the code — drawn in the three views projectModel.js builds (the simulation
 * that lays the graph out is media/graphLayout.js). One per window:
 * the page wants the whole editor area, and two side by side would only ever
 * show the same project twice.
 *
 * This side reads the disk and answers the page; the page (media/graph.js)
 * draws. Nothing is written, and nothing is asked of a model.
 */
class GraphPanel {
	/** @param {vscode.ExtensionContext} context */
	constructor(context) {
		this.context = context;
		/** @type {vscode.WebviewPanel | null} */
		this.panel = null;
		/** @type {vscode.FileSystemWatcher | null} */
		this.watcher = null;
		/** The context the page opens on, when one was asked for. */
		this.focused = '';
	}

	/** @returns {string | null} */
	root() {
		const folder = vscode.workspace.workspaceFolders?.[0];
		if (!folder) {
			return null;
		}
		return findProjectRoot(folder.uri.fsPath) || folder.uri.fsPath;
	}

	post(message) {
		if (this.panel) {
			this.panel.webview.postMessage(message);
		}
	}

	/**
	 * Opens the panel, or reveals the one already open.
	 *
	 * @param {string} [context] A context to open on, traced.
	 */
	open(context) {
		this.focused = typeof context === 'string' ? context : '';
		if (this.panel) {
			this.panel.reveal(this.panel.viewColumn, true);
			this.send();
			return;
		}
		const panel = vscode.window.createWebviewPanel(
			PANEL_TYPE,
			'GOFI — grafo do projeto',
			{ viewColumn: vscode.ViewColumn.Beside, preserveFocus: true },
			{
				enableScripts: true,
				retainContextWhenHidden: true,
				localResourceRoots: [vscode.Uri.joinPath(this.context.extensionUri, 'media')],
			},
		);
		panel.iconPath = vscode.Uri.joinPath(this.context.extensionUri, 'images', 'gofi.svg');
		panel.webview.html = this.html(panel.webview);
		panel.webview.onDidReceiveMessage((message) => this.onMessage(message));
		panel.onDidDispose(() => {
			this.panel = null;
			this.stopWatching();
		});
		this.panel = panel;
	}

	/**
	 * Reloads when `gofi index` rewrites what the page draws.
	 *
	 * Without this the page silently shows the project as it was when it was
	 * opened, which is exactly wrong right after someone rebuilds to see their
	 * change land.
	 */
	watch() {
		const root = this.root();
		if (!root || this.watcher) {
			return;
		}
		this.watcher = vscode.workspace.createFileSystemWatcher(new vscode.RelativePattern(root, '.gofi/index/**/*.json'));
		// A build rewrites several files in a row: one reload after the last.
		let pending = null;
		const reload = () => {
			clearTimeout(pending);
			pending = setTimeout(() => this.send(), 400);
		};
		this.watcher.onDidCreate(reload);
		this.watcher.onDidChange(reload);
		this.watcher.onDidDelete(reload);
	}

	stopWatching() {
		if (this.watcher) {
			this.watcher.dispose();
			this.watcher = null;
		}
	}

	/** Builds the views and hands them to the page. */
	send() {
		const root = this.root();
		if (!root) {
			this.post({ type: 'empty', reason: 'sem-pasta' });
			return;
		}
		const overview = model.overview(root);
		if (!overview) {
			// The path travels with the reason: which folder the page looked
			// at is not obvious with several windows open, and without it "not
			// indexed" and "wrong window" look the same.
			this.post({ type: 'empty', reason: 'sem-grafo', file: path.join(root, model.DOCS_DIR) });
			return;
		}
		this.post({ type: 'model', overview, graph: model.graph(root), context: this.focused });
		this.focused = '';
	}

	onMessage(message) {
		const root = this.root();
		switch (message?.type) {
			case 'ready':
				this.watch();
				this.send();
				return;
			case 'trace':
				if (root && typeof message.context === 'string') {
					this.post({ type: 'trace', data: model.trace(root, message.context) });
				}
				return;
			case 'details':
				if (root) {
					this.post({ type: 'details', data: model.details(root, message.id) });
				}
				return;
			case 'open':
				this.openFile(message.path, message.line);
				return;
			case 'reveal':
				if (root && typeof message.dir === 'string') {
					vscode.commands.executeCommand('revealInExplorer', vscode.Uri.file(path.join(root, message.dir)));
				}
				return;
			case 'build':
				this.build();
				return;
			default:
		}
	}

	/** Opens a document or a source file beside the page, at a line when one is given. */
	openFile(relPath, line) {
		const root = this.root();
		if (!root || typeof relPath !== 'string' || relPath === '') {
			return;
		}
		const options = { preview: true, viewColumn: vscode.ViewColumn.Beside };
		if (Number.isInteger(line) && line > 0) {
			const at = new vscode.Position(line - 1, 0);
			options.selection = new vscode.Range(at, at);
		}
		vscode.window.showTextDocument(vscode.Uri.file(path.join(root, relPath)), options);
	}

	/**
	 * Runs the build in a terminal instead of a hidden subprocess.
	 *
	 * `gofi index` rewrites files the person has to commit, and its validation
	 * failures are the whole point of running it — swallowing both into a
	 * spinner would hide the output that says why something was rejected.
	 */
	build() {
		const root = this.root();
		if (!root) {
			vscode.window.showWarningMessage('GOFI: abra uma pasta antes.');
			return;
		}
		const terminal = vscode.window.createTerminal({
			name: 'gofi index',
			cwd: vscode.Uri.file(root),
			iconPath: new vscode.ThemeIcon('type-hierarchy'),
		});
		terminal.show();
		const gofi = vscode.workspace.getConfiguration('gofiAI').get('gofiPath') || 'gofi';
		terminal.sendText(`${/\s/.test(gofi) ? `"${gofi}"` : gofi} index`);
	}

	/** @param {vscode.Webview} webview */
	html(webview) {
		const asset = (name) => webview.asWebviewUri(vscode.Uri.joinPath(this.context.extensionUri, 'media', name));
		const nonce = crypto.randomBytes(16).toString('base64');
		const csp = [
			"default-src 'none'",
			`style-src ${webview.cspSource}`,
			`img-src ${webview.cspSource} data:`,
			`font-src ${webview.cspSource}`,
			`script-src 'nonce-${nonce}'`,
		].join('; ');

		return `<!DOCTYPE html>
<html lang="pt-BR">
<head>
<meta charset="UTF-8">
<meta http-equiv="Content-Security-Policy" content="${csp}">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<link href="${asset('graph.css')}" rel="stylesheet">
<title>GOFI — grafo do projeto</title>
</head>
<body>
${markup()}
<script nonce="${nonce}" src="${asset('graphLayout.js')}"></script>
<script nonce="${nonce}" src="${asset('graph.js')}"></script>
</body>
</html>`;
	}

	dispose() {
		this.stopWatching();
		if (this.panel) {
			this.panel.dispose();
			this.panel = null;
		}
	}
}

/** The page's frame: what stays while the views change inside it. */
function markup() {
	return `<div id="app">
  <header id="top">
    <span id="project"></span>
    <span id="totals"></span>
    <nav id="tabs" role="tablist" aria-label="Visões do projeto"></nav>
    <span class="grow"></span>
    <input id="search" type="search" placeholder="filtrar…" aria-label="Filtrar pelo nome ou caminho" hidden>
    <button id="refresh" class="chip" type="button" title="Ler de novo do disco">recarregar</button>
  </header>
  <div id="body">
    <main id="view"></main>
    <div id="empty" hidden>
      <p id="emptyText"></p>
      <p id="emptyFile"></p>
      <button id="buildBtn" class="chip" type="button" hidden>rodar <code>gofi index</code></button>
    </div>
    <aside id="details" hidden aria-label="Detalhes do que está selecionado">
      <button id="closeDetails" type="button" aria-label="Fechar">&#215;</button>
      <span id="detailsKind" class="badge"></span>
      <h2 id="detailsTitle"></h2>
      <p id="detailsPath"></p>
      <p id="detailsFacts"></p>
      <button id="detailsOpen" class="chip" type="button">abrir</button>
      <div id="detailsBody"></div>
    </aside>
  </div>
</div>`;
}

module.exports = { GraphPanel, markup };
