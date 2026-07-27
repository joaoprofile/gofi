// @ts-check
'use strict';

/**
 * A small Markdown renderer that builds DOM nodes.
 *
 * Two constraints shape it. First, the input is untrusted — model output, tool
 * results, file contents — and this document holds the extension's message
 * channel, so nothing may go through `innerHTML`; every node is created and
 * every string lands as `textContent`. Second, the extension ships with no
 * runtime dependencies and a CSP that blocks external scripts, so a library is
 * not an option.
 *
 * It covers what the agents actually emit: headings, tables, lists,
 * blockquotes, rules, fenced and inline code, bold/italic/strike, links, and
 * bare file paths — which the gofi agents produce constantly and which are far
 * more useful as something you can click.
 *
 * Exposed as `window.gofiMarkdown`.
 */

(function () {
	/** Schemes allowed in a link. Anything else renders as plain text. */
	const SAFE_SCHEME = /^(https?:|mailto:)/i;

	/**
	 * A relative path worth turning into a link: has a directory separator and
	 * an extension we recognise. Deliberately narrow — a false positive turns
	 * prose into a dead link, which is worse than leaving it as text.
	 */
	const FILE_PATH = /\b((?:[\w.@-]+\/)+[\w.@-]+\.(?:md|go|ts|tsx|js|jsx|json|ya?ml|sql|sh|css|html|txt|toml|mod))\b/;

	/**
	 * Inline constructs, tried in this order at each position.
	 *
	 * Kept as a source string, not a compiled regex: `inline()` recurses into
	 * the contents of emphasis, and a shared /g/ regex carries `lastIndex`
	 * across those nested calls — the inner pass rewinds the outer one onto the
	 * match it just consumed, and the two spin forever. Each call compiles its
	 * own.
	 */
	const INLINE_SOURCE = (
		[
			'`([^`]+)`', // code
			'\\*\\*([^*]+)\\*\\*', // bold
			'__([^_]+)__', // bold
			'~~([^~]+)~~', // strike
			'\\*([^*\\n]+)\\*', // italic
			'\\[([^\\]]*)\\]\\(([^)\\s]+)\\)', // link
			FILE_PATH.source, // bare file path
		].join('|')
	);

	/** @param {(path: string) => void} [onOpenPath] Called when a path link is clicked. */
	let openPath = null;

	function setPathHandler(handler) {
		openPath = handler;
	}

	// ── inline ──────────────────────────────────────────────────────────────

	/** Appends the inline-formatted form of `text` to `parent`. */
	function inline(parent, text) {
		const re = new RegExp(INLINE_SOURCE, 'g');
		let last = 0;
		let match;

		while ((match = re.exec(text)) !== null) {
			if (match.index > last) {
				parent.appendChild(document.createTextNode(text.slice(last, match.index)));
			}
			const [full, code, boldA, boldB, strike, italic, linkText, linkHref, filePath] = match;

			if (code !== undefined) {
				parent.appendChild(tag('code', code));
			} else if (boldA !== undefined || boldB !== undefined) {
				parent.appendChild(inlineTag('strong', boldA ?? boldB));
			} else if (strike !== undefined) {
				parent.appendChild(inlineTag('s', strike));
			} else if (italic !== undefined) {
				parent.appendChild(inlineTag('em', italic));
			} else if (linkHref !== undefined) {
				parent.appendChild(link(linkText || linkHref, linkHref));
			} else if (filePath !== undefined) {
				parent.appendChild(pathLink(filePath));
			} else {
				parent.appendChild(document.createTextNode(full));
			}
			last = match.index + full.length;
		}

		if (last < text.length) {
			parent.appendChild(document.createTextNode(text.slice(last)));
		}
	}

	function tag(name, text) {
		const el = document.createElement(name);
		el.textContent = text;
		return el;
	}

	/** Emphasis can nest, so its content goes back through the inline pass. */
	function inlineTag(name, text) {
		const el = document.createElement(name);
		inline(el, text);
		return el;
	}

	function link(text, href) {
		if (!SAFE_SCHEME.test(href)) {
			// A relative href is still a path worth opening; anything else
			// (javascript:, data:) is rendered inert as text.
			return FILE_PATH.test(href) ? pathLink(href, text) : document.createTextNode(text);
		}
		const el = document.createElement('a');
		el.href = href;
		el.textContent = text;
		return el;
	}

	/** A workspace-relative path, clickable to open the file in the editor. */
	function pathLink(path, label) {
		const el = document.createElement('button');
		el.type = 'button';
		el.className = 'path';
		el.textContent = label || path;
		el.title = `Abrir ${path}`;
		el.addEventListener('click', () => {
			if (openPath) {
				openPath(path);
			}
		});
		return el;
	}

	// ── blocks ──────────────────────────────────────────────────────────────

	const FENCE = /^\s*```(.*)$/;
	const HEADING = /^(#{1,6})\s+(.*)$/;
	const RULE = /^\s*([-*_])\1{2,}\s*$/;
	const QUOTE = /^\s*>\s?(.*)$/;
	const BULLET = /^(\s*)[-*+]\s+(.*)$/;
	const ORDERED = /^(\s*)\d+[.)]\s+(.*)$/;
	const TABLE_DIVIDER = /^\s*\|?\s*:?-{2,}:?\s*(\|\s*:?-{2,}:?\s*)*\|?\s*$/;

	/**
	 * Renders `text` into `container`, replacing its contents.
	 *
	 * @param {HTMLElement} container
	 * @param {string} text
	 */
	function render(container, text) {
		container.textContent = '';
		const lines = String(text).replace(/\r\n/g, '\n').split('\n');
		let i = 0;
		while (i < lines.length) {
			i = block(container, lines, i);
		}
	}

	/** Consumes one block starting at `i` and returns the next index. */
	function block(parent, lines, i) {
		const line = lines[i];

		if (line.trim() === '') {
			return i + 1;
		}

		const fence = FENCE.exec(line);
		if (fence) {
			return codeBlock(parent, lines, i, fence[1].trim());
		}

		const heading = HEADING.exec(line);
		if (heading) {
			// Cap at h4: these live inside a chat bubble, not a document, and an
			// h1 rendered at document scale looks broken next to the message.
			const level = Math.min(heading[1].length + 2, 6);
			const el = document.createElement(`h${level}`);
			inline(el, heading[2].trim());
			parent.appendChild(el);
			return i + 1;
		}

		if (RULE.test(line)) {
			parent.appendChild(document.createElement('hr'));
			return i + 1;
		}

		if (line.includes('|') && i + 1 < lines.length && TABLE_DIVIDER.test(lines[i + 1])) {
			return table(parent, lines, i);
		}

		if (QUOTE.test(line)) {
			return blockquote(parent, lines, i);
		}

		if (BULLET.test(line) || ORDERED.test(line)) {
			return list(parent, lines, i);
		}

		return paragraph(parent, lines, i);
	}

	function codeBlock(parent, lines, i, language) {
		const body = [];
		let j = i + 1;
		while (j < lines.length && !FENCE.test(lines[j])) {
			body.push(lines[j]);
			j++;
		}
		const pre = document.createElement('pre');
		if (language !== '') {
			pre.dataset.lang = language;
		}
		pre.appendChild(tag('code', body.join('\n')));
		parent.appendChild(pre);
		// Skip the closing fence, or stop at the end when it never came.
		return j < lines.length ? j + 1 : j;
	}

	/** Splits a table row on unescaped pipes, dropping the outer ones. */
	function cells(line) {
		const parts = line.split('|');
		if (parts.length > 1 && parts[0].trim() === '') {
			parts.shift();
		}
		if (parts.length > 0 && parts[parts.length - 1].trim() === '') {
			parts.pop();
		}
		return parts.map((c) => c.trim());
	}

	function alignments(divider) {
		return cells(divider).map((c) => {
			const left = c.startsWith(':');
			const right = c.endsWith(':');
			if (left && right) {
				return 'center';
			}
			return right ? 'right' : left ? 'left' : '';
		});
	}

	function table(parent, lines, i) {
		const header = cells(lines[i]);
		const align = alignments(lines[i + 1]);

		const el = document.createElement('table');
		const thead = document.createElement('thead');
		const headRow = document.createElement('tr');
		header.forEach((text, index) => {
			const th = document.createElement('th');
			if (align[index]) {
				th.style.textAlign = align[index];
			}
			inline(th, text);
			headRow.appendChild(th);
		});
		thead.appendChild(headRow);
		el.appendChild(thead);

		const tbody = document.createElement('tbody');
		let j = i + 2;
		for (; j < lines.length; j++) {
			if (lines[j].trim() === '' || !lines[j].includes('|')) {
				break;
			}
			const row = document.createElement('tr');
			cells(lines[j]).forEach((text, index) => {
				const td = document.createElement('td');
				if (align[index]) {
					td.style.textAlign = align[index];
				}
				inline(td, text);
				row.appendChild(td);
			});
			tbody.appendChild(row);
		}
		el.appendChild(tbody);

		// Wide tables scroll inside themselves rather than stretching the chat.
		const scroller = document.createElement('div');
		scroller.className = 'table-scroll';
		scroller.appendChild(el);
		parent.appendChild(scroller);
		return j;
	}

	function blockquote(parent, lines, i) {
		const body = [];
		let j = i;
		for (; j < lines.length; j++) {
			const m = QUOTE.exec(lines[j]);
			if (!m) {
				break;
			}
			body.push(m[1]);
		}
		const el = document.createElement('blockquote');
		let k = 0;
		while (k < body.length) {
			k = block(el, body, k);
		}
		parent.appendChild(el);
		return j;
	}

	/**
	 * Renders a list, nesting by indentation.
	 *
	 * Only the depth matters, not the exact column count — agents indent with
	 * two spaces, four, or a tab, and all three mean "one level in".
	 */
	function list(parent, lines, i) {
		/** @type {{indent: number, el: HTMLElement}[]} */
		const stack = [];
		let j = i;

		for (; j < lines.length; j++) {
			const line = lines[j];
			if (line.trim() === '') {
				// A blank line ends the list unless the next line continues it.
				if (j + 1 < lines.length && (BULLET.test(lines[j + 1]) || ORDERED.test(lines[j + 1]))) {
					continue;
				}
				break;
			}
			const bullet = BULLET.exec(line);
			const ordered = bullet ? null : ORDERED.exec(line);
			const m = bullet || ordered;
			if (!m) {
				break;
			}

			const indent = m[1].replace(/\t/g, '  ').length;
			const kind = bullet ? 'ul' : 'ol';

			while (stack.length > 0 && indent < stack[stack.length - 1].indent) {
				stack.pop();
			}
			if (stack.length === 0) {
				const el = document.createElement(kind);
				parent.appendChild(el);
				stack.push({ indent, el });
			} else if (indent > stack[stack.length - 1].indent) {
				const el = document.createElement(kind);
				const items = stack[stack.length - 1].el.children;
				// Nest under the item it belongs to, not next to the list.
				(items.length > 0 ? items[items.length - 1] : stack[stack.length - 1].el).appendChild(el);
				stack.push({ indent, el });
			}

			const li = document.createElement('li');
			inline(li, m[2]);
			stack[stack.length - 1].el.appendChild(li);
		}
		return j;
	}

	function paragraph(parent, lines, i) {
		const body = [];
		let j = i;
		for (; j < lines.length; j++) {
			const line = lines[j];
			if (
				line.trim() === '' ||
				FENCE.test(line) ||
				HEADING.test(line) ||
				RULE.test(line) ||
				QUOTE.test(line) ||
				BULLET.test(line) ||
				ORDERED.test(line)
			) {
				break;
			}
			body.push(line);
		}
		const el = document.createElement('p');
		// A single newline inside a paragraph is a soft break, as every chat
		// renders it — agents rely on that for wrapped prose.
		body.forEach((line, index) => {
			if (index > 0) {
				el.appendChild(document.createElement('br'));
			}
			inline(el, line);
		});
		parent.appendChild(el);
		return j;
	}

	window.gofiMarkdown = { render, setPathHandler };
})();
