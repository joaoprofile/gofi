'use strict';

const vscode = require('vscode');
const crypto = require('crypto');
const path = require('path');

const { findProjectRoot } = require('./gofiConfig.js');
const docGraph = require('./docGraph.js');

const PANEL_TYPE = 'gofi-ai.graphPanel';

/**
 * The document graph, drawn.
 *
 * `gofi find` answers a question with a path and a line range, which is the
 * right shape for an agent and the wrong one for a person asking what a context
 * is made of. That question is about shape — how many documents, which tables
 * they share, which other contexts they reach — and shape is what a picture
 * carries and a list does not.
 *
 * One panel per window. A graph wants the whole editor area, and two of them
 * side by side would only ever show the same corpus twice.
 */
class GraphPanel {
	/** @param {vscode.ExtensionContext} context */
	constructor(context) {
		this.context = context;
		/** @type {vscode.WebviewPanel | null} */
		this.panel = null;
		/** @type {vscode.FileSystemWatcher | null} */
		this.watcher = null;
		/** @type {string} Context in focus, '' for the repository view. */
		this.focused = '';
		/** @type {boolean} Whether the repository view includes tables. */
		this.entities = false;
	}

	/**
	 * Opens the panel, or reveals the one already open.
	 *
	 * @param {string} [context] Context to focus; omit for the repository view.
	 */
	open(context) {
		if (typeof context === 'string') {
			this.focused = context;
		}
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
		panel.iconPath = {
			light: vscode.Uri.joinPath(this.context.extensionUri, 'images', 'panel-icon-light.svg'),
			dark: vscode.Uri.joinPath(this.context.extensionUri, 'images', 'panel-icon-dark.svg'),
		};
		panel.webview.html = this.html(panel.webview);
		panel.webview.onDidReceiveMessage((message) => this.onMessage(message));
		panel.onDidDispose(() => {
			this.panel = null;
			this.stopWatching();
		});
		this.panel = panel;
		this.watch();
	}

	/**
	 * Reloads when `gofi docs build` rewrites the graph.
	 *
	 * Without this the panel silently shows the corpus as it was when it was
	 * opened, which is exactly wrong right after someone rebuilds to see their
	 * change land.
	 */
	watch() {
		const root = this.root();
		if (!root || this.watcher) {
			return;
		}
		this.watcher = vscode.workspace.createFileSystemWatcher(
			new vscode.RelativePattern(root, docGraph.GRAPH_FILE.split(path.sep).join('/')),
		);
		const reload = () => this.send();
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

	/** @returns {string | null} */
	root() {
		const folder = vscode.workspace.workspaceFolders?.[0];
		if (!folder) {
			return null;
		}
		return findProjectRoot(folder.uri.fsPath) || folder.uri.fsPath;
	}

	/** Builds the current view and pushes it to the webview. */
	send() {
		if (!this.panel) {
			return;
		}
		const root = this.root();
		if (!root) {
			this.panel.webview.postMessage({ type: 'empty', reason: 'sem-pasta' });
			return;
		}
		if (!docGraph.hasGraph(root)) {
			// The path travels with the reason. Which folder the panel looked at
			// is not obvious with several windows open, and without it "no
			// graph" and "wrong window" look the same.
			this.panel.webview.postMessage({
				type: 'empty',
				reason: 'sem-grafo',
				root,
				file: `${root}/${docGraph.GRAPH_FILE}`,
			});
			return;
		}

		const data = this.focused
			? docGraph.contextView(root, this.focused)
			: docGraph.repositoryView(root, { entities: this.entities });

		if (!data) {
			// The focused context vanished from a rebuild — fall back rather
			// than leaving the panel on a view that no longer exists.
			this.focused = '';
			this.send();
			return;
		}
		this.panel.webview.postMessage({
			type: 'graph',
			data,
			contexts: docGraph.listContexts(root),
			entities: this.entities,
			project: root.slice(root.lastIndexOf('/') + 1),
		});
	}

	onMessage(message) {
		switch (message?.type) {
			case 'ready':
				this.send();
				return;
			case 'focus':
				this.focused = typeof message.context === 'string' ? message.context : '';
				this.send();
				return;
			case 'entities':
				this.entities = message.on === true;
				this.send();
				return;
			case 'open':
				this.openDocument(message.path, message.dir);
				return;
			case 'build':
				this.build();
				return;
			case 'subtree':
				this.sendSubtree(message.context);
				return;
			default:
		}
	}

	/**
	 * The contents of one context, for the tree to expand into.
	 *
	 * Served per context rather than shipped whole: the corpus has 42 of them,
	 * and building every subtree up front costs more than the user will ever
	 * expand. The webview caches what it asks for.
	 */
	sendSubtree(name) {
		const root = this.root();
		if (!this.panel || !root || typeof name !== 'string') {
			return;
		}
		const tree = docGraph.contextView(root, name);
		if (!tree) {
			return;
		}
		this.panel.webview.postMessage({ type: 'subtree', context: name, data: tree });
	}

	/** Opens a document, or reveals a package folder in the explorer. */
	openDocument(relPath, dir) {
		const root = this.root();
		if (!root) {
			return;
		}
		if (dir) {
			const uri = vscode.Uri.file(path.join(root, dir));
			vscode.commands.executeCommand('revealInExplorer', uri);
			return;
		}
		if (typeof relPath !== 'string' || relPath === '') {
			return;
		}
		const uri = vscode.Uri.file(path.join(root, relPath));
		vscode.window.showTextDocument(uri, { preview: true, viewColumn: vscode.ViewColumn.Beside });
	}

	/**
	 * Runs the build in a terminal instead of a hidden subprocess.
	 *
	 * `gofi docs build` rewrites files the user has to commit, and validation
	 * failures are the whole point of running it — swallowing both into a
	 * spinner would hide the output that says why a facet was rejected.
	 */
	build() {
		const root = this.root();
		if (!root) {
			vscode.window.showWarningMessage('GOFI AI: abra uma pasta antes.');
			return;
		}
		const terminal = vscode.window.createTerminal({
			name: 'gofi docs build',
			cwd: vscode.Uri.file(root),
			iconPath: new vscode.ThemeIcon('type-hierarchy'),
		});
		terminal.show();
		terminal.sendText('gofi docs build');
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
<div id="app">
  <aside id="tree">
    <div id="treeHead">
      <span id="treeTitle">Contextos</span>
      <span id="treeCount"></span>
    </div>
    <div id="treeBody"></div>
  </aside>
  <div id="main">
  <header id="bar">
    <button id="back" class="chip" type="button" hidden>&#8592; repositório</button>
    <span id="scope">projeto</span>
    <span id="counts"></span>
    <span class="grow"></span>
    <input id="search" type="search" placeholder="filtrar…" aria-label="Filtrar nós">
    <label id="entitiesToggle" class="chip toggle"><input id="entities" type="checkbox"> tabelas</label>
    <button id="center" class="chip" type="button" title="Enquadrar o grafo inteiro na tela">centralizar</button>
    <button id="refresh" class="chip" type="button" title="Recarregar do disco">recarregar</button>
  </header>

  <div id="stage">
    <canvas id="canvas"></canvas>
    <div id="legend"></div>
    <div id="empty" hidden>
      <p id="emptyText"></p>
      <button id="buildBtn" class="chip" type="button" hidden>rodar <code>gofi docs build</code></button>
    </div>
  </div>

  </div>

  <aside id="details" hidden>
    <button id="closeDetails" class="icon" type="button" aria-label="Fechar">&#215;</button>
    <h2 id="detailsTitle"></h2>
    <p id="detailsMeta"></p>
    <div id="detailsBody"></div>
  </aside>
</div>
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

module.exports = { GraphPanel };
