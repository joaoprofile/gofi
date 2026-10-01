'use strict';

const vscode = require('vscode');
const crypto = require('crypto');
const path = require('path');

const { findProjectRoot } = require('./gofiConfig.js');
const docGraph = require('./docGraph.js');

const PANEL_TYPE = 'gofi-ai.graphPanel';

/**
 * The document graph's data and actions, for whatever draws it.
 *
 * `gofi find` answers a question with a path and a line range, which is the
 * right shape for an agent and the wrong one for a person asking what a context
 * is made of. That question is about shape — how many documents, which tables
 * they share, which other contexts they reach — and shape is what a picture
 * carries and a list does not.
 *
 * The graph is drawn in two places — its own editor tab, and the chat panel
 * switched to its graph mode — by the same page code; this is the side that
 * builds the views and answers the page, given how to reach it.
 */
class GraphSource {
	/** @param {(message: object) => void} post Delivers a message to the page. */
	constructor(post) {
		this.post = post;
		/** @type {vscode.FileSystemWatcher | null} */
		this.watcher = null;
		/** @type {string} Context in focus, '' for the project. */
		this.focused = '';
		/** @type {boolean} Whether the documents view includes tables. */
		this.entities = false;
		/** @type {boolean} Every document, instead of the map of contexts. */
		this.documents = false;
	}

	/**
	 * Reloads when `gofi index docs` rewrites the graph.
	 *
	 * Without this the page silently shows the corpus as it was when it was
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

	/** Builds the current view and pushes it to the page. */
	send() {
		const root = this.root();
		if (!root) {
			this.post({ type: 'empty', reason: 'sem-pasta' });
			return;
		}
		if (!docGraph.hasGraph(root)) {
			// The path travels with the reason. Which folder the page looked at
			// is not obvious with several windows open, and without it "no
			// graph" and "wrong window" look the same.
			this.post({
				type: 'empty',
				reason: 'sem-grafo',
				root,
				file: `${root}/${docGraph.GRAPH_FILE}`,
			});
			return;
		}

		let data;
		if (this.focused) {
			data = docGraph.contextView(root, this.focused);
		} else if (this.documents) {
			data = docGraph.repositoryView(root, { entities: this.entities });
		} else {
			data = docGraph.mapView(root);
		}

		if (!data) {
			// The focused context vanished from a rebuild — fall back rather
			// than leaving the page on a view that no longer exists.
			this.focused = '';
			this.send();
			return;
		}
		this.post({
			type: 'graph',
			data,
			contexts: docGraph.listContexts(root),
			entities: this.entities,
			documents: this.documents,
			project: root.slice(root.lastIndexOf('/') + 1),
		});
	}

	onMessage(message) {
		switch (message?.type) {
			case 'ready':
				this.watch();
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
			case 'documents':
				this.documents = message.on === true;
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
	 * Served per context rather than shipped whole: a corpus has dozens of
	 * them, and building every subtree up front costs more than the user will
	 * ever expand. The page caches what it asks for.
	 */
	sendSubtree(name) {
		const root = this.root();
		if (!root || typeof name !== 'string') {
			return;
		}
		const tree = docGraph.contextView(root, name);
		if (!tree) {
			return;
		}
		this.post({ type: 'subtree', context: name, data: tree });
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
	 * `gofi index docs` rewrites files the user has to commit, and validation
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
			name: 'gofi index docs',
			cwd: vscode.Uri.file(root),
			iconPath: new vscode.ThemeIcon('type-hierarchy'),
		});
		terminal.show();
		terminal.sendText('gofi index docs');
	}

	dispose() {
		this.stopWatching();
	}
}

/**
 * The graph in an editor tab of its own. One per window: a graph wants the
 * whole editor area, and two side by side would only ever show the same
 * corpus twice. The chat panel also draws it, switched to its graph mode.
 */
class GraphPanel {
	/** @param {vscode.ExtensionContext} context */
	constructor(context) {
		this.context = context;
		/** @type {vscode.WebviewPanel | null} */
		this.panel = null;
		this.source = new GraphSource((message) => {
			if (this.panel) {
				this.panel.webview.postMessage(message);
			}
		});
	}

	/**
	 * Opens the panel, or reveals the one already open.
	 *
	 * @param {string} [context] Context to focus; omit for the project view.
	 */
	open(context) {
		if (typeof context === 'string') {
			this.source.focused = context;
		}
		if (this.panel) {
			this.panel.reveal(this.panel.viewColumn, true);
			this.source.send();
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
		panel.webview.onDidReceiveMessage((message) => this.source.onMessage(message));
		panel.onDidDispose(() => {
			this.panel = null;
			this.source.stopWatching();
		});
		this.panel = panel;
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
<body class="graph-page">
${graphMarkup()}
<script nonce="${nonce}" src="${asset('graph.js')}"></script>
</body>
</html>`;
	}

	dispose() {
		this.source.dispose();
		if (this.panel) {
			this.panel.dispose();
			this.panel = null;
		}
	}
}

/**
 * The graph page's markup, the same in its own tab and in the chat's graph
 * mode. Everything sits under #graphView, which is what graph.css is scoped
 * to, so it lives beside the chat's own styles without either touching the
 * other.
 */
function graphMarkup() {
	return `<section id="graphView" aria-label="Grafo do projeto">
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
    <button id="back" class="chip" type="button" hidden>&#8592; mapa</button>
    <span id="scope">projeto</span>
    <span id="counts"></span>
    <span class="grow"></span>
    <input id="search" type="search" placeholder="filtrar…" aria-label="Filtrar nós">
    <label id="documentsToggle" class="chip toggle" title="Todos os documentos, em vez do mapa de contextos"><input id="documents" type="checkbox"> documentos</label>
    <label id="entitiesToggle" class="chip toggle" hidden><input id="entities" type="checkbox"> tabelas</label>
    <button id="center" class="chip" type="button" title="Enquadrar o grafo inteiro na tela">centralizar</button>
    <button id="refresh" class="chip" type="button" title="Recarregar do disco">recarregar</button>
  </header>

  <div id="stage">
    <canvas id="canvas"></canvas>
    <div id="legend"></div>
    <div id="empty" hidden>
      <p id="emptyText"></p>
      <button id="buildBtn" class="chip" type="button" hidden>rodar <code>gofi index docs</code></button>
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
</section>`;
}

module.exports = { GraphPanel, GraphSource, graphMarkup };
