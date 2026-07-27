'use strict';

const fs = require('fs');
const path = require('path');

/**
 * Inspects the project's RAG corpora against the retrieval protocol, and turns
 * what it finds into fixes the agent can apply.
 *
 * The token ledger says *that* a read was expensive. This says *why*, by
 * opening the file the agent read and checking the three things the protocol
 * depends on:
 *
 *   - frontmatter with `keywords`, so the doc is discoverable without opening it;
 *   - `## ` headings, so a section can be read instead of the whole file;
 *   - an INDEX.md in the corpus, so discovery starts from a cheap listing.
 *
 * A doc missing those cannot be read cheaply — the agent has no choice but to
 * pull it in whole, and no amount of prompting fixes that. So each finding
 * carries a `prompt`: a bounded instruction the user can hand straight back to
 * the agent, which then edits the actual file. The extension never rewrites a
 * spec itself — the agent owns the project's templates and conventions, and it
 * is the one that has to live with the result.
 */

/** Corpora the retrieval protocol governs, and what each one is. */
const CORPORA = [
	{ prefix: 'specs/', label: 'spec', index: 'specs/INDEX.md' },
	{ prefix: 'prd/', label: 'PRD', index: 'prd/INDEX.md' },
	{ prefix: '.claude/memory/contexts/', label: 'contexto', index: null },
	{ prefix: '.claude/institutional/', label: 'institucional', index: null },
];

/** Below this a doc is short enough that reading it whole costs little. */
const SMALL_DOC_LINES = 120;

/** Above this even a well-sectioned doc is worth splitting. */
const OVERSIZED_DOC_LINES = 700;

/** How many proactive findings to surface at once. A wall of them is noise. */
const MAX_SCAN_FINDINGS = 6;

/** Cap on files walked per corpus, so a huge tree can't stall the panel. */
const MAX_SCAN_FILES = 400;

function corpusOf(relPath) {
	return CORPORA.find((c) => relPath.startsWith(c.prefix)) || null;
}

/**
 * Cache of parsed docs, keyed by absolute path. The audit reruns on every
 * provider event — several times a second while the agent works — and without
 * this it would re-read and re-parse the same specs each time. Keyed on mtime
 * so an edit invalidates the entry, which matters because applying a fix is
 * supposed to make the finding disappear.
 *
 * @type {Map<string, {mtimeMs: number, doc: object|null}>}
 */
const docCache = new Map();

/**
 * Reads the structural facts a retrieval decision depends on.
 *
 * @returns {{lines: number, hasFrontmatter: boolean, keywords: string[], headings: string[],
 *            firstHeadingLine: number, frontmatterEnd: number} | null}
 */
function inspectDoc(root, relPath) {
	const absolute = path.join(root, relPath);

	let mtimeMs;
	try {
		mtimeMs = fs.statSync(absolute).mtimeMs;
	} catch {
		docCache.delete(absolute);
		return null; // deleted, or outside the workspace — nothing to advise on
	}
	const cached = docCache.get(absolute);
	if (cached && cached.mtimeMs === mtimeMs) {
		return cached.doc;
	}

	let raw;
	try {
		raw = fs.readFileSync(absolute, 'utf8');
	} catch {
		return null;
	}

	const lines = raw.split(/\r?\n/);
	const headings = [];
	let hasFrontmatter = false;
	let frontmatterEnd = 0;
	/** @type {string[]} */
	let keywords = [];
	let keywordsLine = 0;

	if (lines[0] === '---') {
		const close = lines.indexOf('---', 1);
		if (close > 0) {
			hasFrontmatter = true;
			frontmatterEnd = close + 1; // 1-based line of the closing `---`
			for (let i = 1; i < close; i++) {
				const match = /^keywords:\s*(.*)$/.exec(lines[i]);
				if (match) {
					keywords = parseKeywords(match[1]);
					keywordsLine = i + 1;
				}
			}
		}
	}

	let firstHeadingLine = 0;
	lines.forEach((line, index) => {
		const heading = /^##\s+(.+)$/.exec(line);
		if (heading) {
			headings.push(heading[1].trim());
			if (firstHeadingLine === 0) {
				firstHeadingLine = index + 1;
			}
		}
	});

	const doc = {
		lines: lines.length,
		hasFrontmatter,
		frontmatterEnd,
		keywords,
		keywordsLine,
		headings,
		firstHeadingLine,
	};
	docCache.set(absolute, { mtimeMs, doc });
	return doc;
}

/** Handles both `keywords: [a, b]` and a bare comma-separated list. */
function parseKeywords(value) {
	const body = value.trim().replace(/^\[|\]$/g, '');
	return body
		.split(',')
		.map((k) => k.trim().replace(/^["']|["']$/g, ''))
		.filter((k) => k !== '');
}

/**
 * Reviews the retrieval that actually happened and returns actionable findings.
 *
 * Only files the agent really opened are audited — a project-wide lint would
 * bury the user in findings about docs nobody reads. What shows up here is,
 * by construction, what is costing tokens right now.
 *
 * @param {string} root Workspace folder
 * @param {{name: string, target: string, scoped: boolean, tokens: number|null}[]} rows
 * @returns {{id: string, level: 'warn'|'info', text: string, action: {label: string, prompt: string}}[]}
 */
function review(root, rows) {
	if (!root) {
		return [];
	}

	/** @type {Map<string, {tokens: number, count: number, scoped: boolean}>} */
	const reads = new Map();
	for (const row of rows) {
		if (row.name !== 'Read' || !row.target || !corpusOf(row.target)) {
			continue;
		}
		const entry = reads.get(row.target) || { tokens: 0, count: 0, scoped: true };
		entry.tokens += row.tokens || 0;
		entry.count += 1;
		entry.scoped = entry.scoped && row.scoped;
		reads.set(row.target, entry);
	}

	const findings = [];
	const seenCorpora = new Set();

	for (const [relPath, read] of reads) {
		const corpus = corpusOf(relPath);
		const doc = inspectDoc(root, relPath);
		if (!doc || doc.lines < SMALL_DOC_LINES) {
			continue; // short enough that structure wouldn't have saved anything
		}
		seenCorpora.add(corpus.prefix);

		if (!doc.hasFrontmatter || doc.keywords.length === 0) {
			findings.push({
				id: `frontmatter:${relPath}`,
				level: 'warn',
				text: `${relPath} (${doc.lines} linhas) não tem ${doc.hasFrontmatter ? '`keywords` no frontmatter' : 'frontmatter'} — sem isso o agente não consegue decidir se o doc serve sem abrir tudo (~${read.tokens} tokens gastos aqui).`,
				where: doc.hasFrontmatter
					? `no frontmatter, antes do \`---\` da linha ${doc.frontmatterEnd}`
					: 'no topo do arquivo, antes da linha 1',
				anchor: { path: relPath, line: doc.hasFrontmatter ? doc.frontmatterEnd : 1 },
				insert: doc.hasFrontmatter
					? 'keywords: [entidade, fluxo, termo-de-negocio]'
					: '---\nversao: "1.0"\nstatus: ativo\nkeywords: [entidade, fluxo, termo-de-negocio]\n---',
				action: {
					label: 'Indexar o doc',
					prompt: buildFrontmatterPrompt(relPath, doc, corpus),
				},
			});
			continue;
		}

		if (doc.headings.length === 0) {
			findings.push({
				id: `headings:${relPath}`,
				level: 'warn',
				text: `${relPath} (${doc.lines} linhas) não tem seções \`## \` — o protocolo lê a seção relevante, e sem elas só resta ler o arquivo inteiro.`,
				where: `ao longo do corpo, a partir da linha ${doc.frontmatterEnd + 1}`,
				anchor: { path: relPath, line: doc.frontmatterEnd + 1 },
				insert: '## Nome da seção',
				action: {
					label: 'Seccionar o doc',
					prompt: buildHeadingsPrompt(relPath, doc, corpus),
				},
			});
			continue;
		}

		if (doc.lines > OVERSIZED_DOC_LINES && !read.scoped) {
			findings.push({
				id: `split:${relPath}`,
				level: 'info',
				text: `${relPath} tem ${doc.lines} linhas em ${doc.headings.length} seções e foi lido inteiro. Um doc desse tamanho rende mais dividido.`,
				where: `${doc.headings.length} seções a partir da linha ${doc.firstHeadingLine}: ${doc.headings.slice(0, 4).join(' · ')}${doc.headings.length > 4 ? ' …' : ''}`,
				anchor: { path: relPath, line: doc.firstHeadingLine },
				action: {
					label: 'Propor divisão',
					prompt: buildSplitPrompt(relPath, doc, corpus),
				},
			});
		}

		if (read.count > 1) {
			findings.push({
				id: `reread:${relPath}`,
				level: 'info',
				text: `${relPath} foi lido ${read.count}× nesta sessão — o índice do corpus provavelmente não descreve bem o que tem dentro.`,
				where: `na linha ${doc.keywordsLine}: keywords atuais [${doc.keywords.join(', ')}] vs ${doc.headings.length} seções no doc`,
				anchor: { path: relPath, line: doc.keywordsLine || 1 },
				action: {
					label: 'Melhorar as keywords',
					prompt: buildKeywordsPrompt(relPath, doc, corpus),
				},
			});
		}
	}

	// A corpus whose INDEX.md is missing forces discovery by opening files.
	for (const prefix of seenCorpora) {
		const corpus = CORPORA.find((c) => c.prefix === prefix);
		if (!corpus || !corpus.index) {
			continue;
		}
		if (!fs.existsSync(path.join(root, corpus.index))) {
			findings.push({
				id: `index:${corpus.index}`,
				level: 'warn',
				text: `${corpus.index} não existe — a descoberta em ${corpus.prefix} começa abrindo arquivos em vez de consultar um índice.`,
				where: `criar \`${corpus.index}\` — uma linha por doc de ${corpus.prefix}`,
				anchor: { path: corpus.index, line: 1 },
				action: {
					label: 'Gerar o índice',
					prompt: buildIndexPrompt(corpus),
				},
			});
		}
	}

	return findings;
}

// ── remediation prompts ─────────────────────────────────────────────────────
//
// Each one is scoped to a single file and states the acceptance criterion, so
// the turn it starts is short, checkable, and cannot wander into rewriting the
// document's content.

function buildFrontmatterPrompt(relPath, doc, corpus) {
	return [
		`Indexe o ${corpus.label} \`${relPath}\` para recuperação, seguindo o protocolo em`,
		'`.claude/knowledge/shared/rag-retrieval-protocol.md` e o template correspondente em `.claude/templates/`.',
		'',
		`Hoje o arquivo tem ${doc.lines} linhas e ${doc.hasFrontmatter ? 'frontmatter sem `keywords`' : 'nenhum frontmatter'},`,
		'então qualquer agente que precise dele é obrigado a abrir o arquivo inteiro.',
		'',
		'Faça só isto:',
		'1. Adicione (ou complete) o frontmatter com os campos que o template pede, incluindo `keywords`',
		'   que cubram as entidades, os fluxos e os termos de negócio pelos quais alguém buscaria este doc.',
		'2. Não altere o conteúdo do documento — nem uma seção, nem uma frase.',
		'3. Regenere o índice do corpus (`.claude/scripts/gen-index.sh`) se ele existir.',
		'4. Ao terminar, liste as keywords que escolheu e por quê.',
	].join('\n');
}

function buildHeadingsPrompt(relPath, doc, corpus) {
	return [
		`Estruture o ${corpus.label} \`${relPath}\` em seções \`## \`, sem reescrever o conteúdo.`,
		'',
		`O arquivo tem ${doc.lines} linhas e nenhuma seção de segundo nível, então o protocolo de`,
		'recuperação (`grep -n "^## "` e depois `Read` só da seção) não tem onde se apoiar — todo acesso',
		'lê o documento inteiro.',
		'',
		'Faça só isto:',
		'1. Insira cabeçalhos `## ` delimitando os blocos que já existem no texto, usando a nomenclatura',
		'   do template em `.claude/templates/`.',
		'2. Preserve o texto palavra por palavra; você está adicionando marcadores, não editando prosa.',
		'3. Atualize as `keywords` do frontmatter se as seções revelarem termos que faltavam.',
		'4. Ao terminar, mostre a lista de seções resultante.',
	].join('\n');
}

function buildSplitPrompt(relPath, doc, corpus) {
	return [
		`Avalie dividir o ${corpus.label} \`${relPath}\` (${doc.lines} linhas, ${doc.headings.length} seções).`,
		'',
		'Não divida ainda. Primeiro me diga:',
		'1. Quais seções formam unidades independentes o bastante para virar arquivos próprios.',
		'2. O que ficaria no documento principal como índice/visão geral.',
		'3. Que links e keywords cada parte precisaria para continuar encontrável.',
		'',
		'Se a conclusão for que dividir não compensa, diga isso e pare — um doc coeso e grande é melhor',
		'que três fragmentos que sempre precisam ser lidos juntos.',
	].join('\n');
}

function buildKeywordsPrompt(relPath, doc, corpus) {
	return [
		`Revise as \`keywords\` do ${corpus.label} \`${relPath}\`.`,
		'',
		`As keywords atuais são: ${doc.keywords.join(', ')}.`,
		'Este documento foi aberto mais de uma vez na mesma sessão, o que costuma indicar que o índice do',
		'corpus não deixa claro o que tem dentro — o agente abre, não acha, e volta.',
		'',
		'Faça só isto:',
		'1. Compare as seções do arquivo com as keywords declaradas e acrescente o que ficou de fora.',
		'2. Remova keywords genéricas demais para discriminar este doc dos outros do corpus.',
		'3. Regenere o índice (`.claude/scripts/gen-index.sh`) se ele existir.',
		'4. Não altere o conteúdo do documento.',
	].join('\n');
}

function buildIndexPrompt(corpus) {
	return [
		`Gere \`${corpus.index}\`, o índice de recuperação de \`${corpus.prefix}\`.`,
		'',
		'Sem ele, descobrir qual documento serve para uma tarefa exige abrir os arquivos um a um.',
		'',
		'Faça só isto:',
		'1. Se `.claude/scripts/gen-index.sh` existir, rode-o e verifique a saída.',
		'2. Se não existir, monte o índice à mão a partir do frontmatter de cada documento do corpus:',
		'   caminho, título, status e keywords — uma linha por documento, nada além disso.',
		'3. Aponte quais documentos estão sem frontmatter e por isso não puderam entrar no índice.',
	].join('\n');
}

module.exports = { review, scanCorpora, inspectDoc, CORPORA };


/**
 * Walks the project's corpora looking for docs that cannot be retrieved
 * cheaply, without waiting for the agent to open one.
 *
 * The read-driven review above is evidence-based — it can say "this cost you
 * 10k tokens just now" — but it only fires once the damage is done, and only
 * for `Read`; a session that discovers via `Grep` never triggers it. This scan
 * is the other half: it finds the same problems before they cost anything.
 *
 * It reads files with `fs` and never asks the agent anything, so it costs no
 * tokens at all. Results are capped and the per-file parse is cached by mtime.
 *
 * @param {string} root Project root
 * @param {Set<string>} already Ids already reported by the read-driven review
 * @returns {ReturnType<typeof review>}
 */
function scanCorpora(root, already) {
	if (!root) {
		return [];
	}
	const findings = [];

	for (const corpus of CORPORA) {
		const dir = path.join(root, corpus.prefix);
		if (!fs.existsSync(dir)) {
			continue;
		}

		// The corpus index is the cheapest win: without it, discovery means
		// opening files one by one.
		if (corpus.index && !fs.existsSync(path.join(root, corpus.index))) {
			const id = `index:${corpus.index}`;
			if (!already.has(id)) {
				findings.push({
					id,
					level: 'warn',
					text: `${corpus.index} não existe — a descoberta em ${corpus.prefix} começa abrindo arquivos em vez de consultar um índice.`,
					where: `criar \`${corpus.index}\` — uma linha por doc de ${corpus.prefix}`,
					anchor: { path: corpus.index, line: 1 },
					action: { label: 'Gerar o índice', prompt: buildIndexPrompt(corpus) },
				});
			}
		}

		for (const relPath of listMarkdown(root, corpus.prefix)) {
			if (findings.length >= MAX_SCAN_FINDINGS) {
				return findings;
			}
			const doc = inspectDoc(root, relPath);
			if (!doc || doc.lines < SMALL_DOC_LINES) {
				continue;
			}

			if (!doc.hasFrontmatter || doc.keywords.length === 0) {
				const id = `frontmatter:${relPath}`;
				if (already.has(id)) {
					continue;
				}
				findings.push({
					id,
					level: 'warn',
					text: `${relPath} (${doc.lines} linhas) não tem ${doc.hasFrontmatter ? '`keywords` no frontmatter' : 'frontmatter'} — quem precisar dele é obrigado a abrir o arquivo inteiro.`,
					where: doc.hasFrontmatter
						? `no frontmatter, antes do \`---\` da linha ${doc.frontmatterEnd}`
						: 'no topo do arquivo, antes da linha 1',
					anchor: { path: relPath, line: doc.hasFrontmatter ? doc.frontmatterEnd : 1 },
					insert: doc.hasFrontmatter
						? 'keywords: [entidade, fluxo, termo-de-negocio]'
						: '---\nversao: "1.0"\nstatus: ativo\nkeywords: [entidade, fluxo, termo-de-negocio]\n---',
					action: { label: 'Indexar o doc', prompt: buildFrontmatterPrompt(relPath, doc, corpus) },
				});
				continue;
			}

			if (doc.headings.length === 0) {
				const id = `headings:${relPath}`;
				if (already.has(id)) {
					continue;
				}
				findings.push({
					id,
					level: 'warn',
					text: `${relPath} (${doc.lines} linhas) não tem seções \`## \` — sem elas só resta ler o arquivo inteiro.`,
					where: `ao longo do corpo, a partir da linha ${doc.frontmatterEnd + 1}`,
					anchor: { path: relPath, line: doc.frontmatterEnd + 1 },
					insert: '## Nome da seção',
					action: { label: 'Seccionar o doc', prompt: buildHeadingsPrompt(relPath, doc, corpus) },
				});
			}
		}
	}
	return findings;
}

/**
 * Lists the markdown under a corpus, two levels deep — `specs/<contexto>/*.md`
 * is the shape gofi writes, and going deeper costs more than it finds.
 */
function listMarkdown(root, prefix) {
	const out = [];
	const walk = (relDir, depth) => {
		if (depth > 2 || out.length >= MAX_SCAN_FILES) {
			return;
		}
		let entries;
		try {
			entries = fs.readdirSync(path.join(root, relDir), { withFileTypes: true });
		} catch {
			return;
		}
		for (const entry of entries) {
			if (out.length >= MAX_SCAN_FILES) {
				return;
			}
			const rel = `${relDir}${entry.name}`;
			if (entry.isDirectory()) {
				walk(`${rel}/`, depth + 1);
			} else if (entry.name.endsWith('.md') && entry.name !== 'INDEX.md') {
				out.push(rel);
			}
		}
	};
	walk(prefix, 0);
	return out;
}
