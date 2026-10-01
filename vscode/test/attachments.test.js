'use strict';

/**
 * What an attached file costs.
 *
 * The panel has two ways to put a file in front of the agent and they differ by
 * orders of magnitude in tokens: a path the agent reads if it needs to, or the
 * file's text quoted into the prompt. Which one happens is decided by where the
 * file came from — the project, or the user's own computer through the OS dialog
 * — and getting that backwards would either bill every attachment as a full read
 * or hand the agent a path it cannot open.
 *
 * The reading itself is the webview's job (see working-row.test.js): the bytes
 * arrive in the window, so nothing here needs a filesystem. What is pinned here
 * is the prompt.
 *
 * Run with `node test/attachments.test.js`.
 */

const assert = require('assert');
const fs = require('fs');
const os = require('os');
const path = require('path');

const { installVscodeStub, makeContext, runner } = require('./vscode-stub.js');

installVscodeStub();

const { Chat } = require('../src/chat.js');

const { test, run } = runner();

function tempDir(prefix) {
	const dir = fs.mkdtempSync(path.join(os.tmpdir(), prefix));
	process.on('exit', () => fs.rmSync(dir, { recursive: true, force: true }));
	return dir;
}

const chat = new Chat(makeContext(tempDir('gofi-anexos-')), 1);

// ── do projeto: só o caminho ────────────────────────────────────────────────

test('um arquivo do projeto viaja como caminho, não como conteúdo', () => {
	const preamble = chat.filesPreamble([
		{ name: 'sdd-cobranca.md', rel: 'specs/cobranca/sdd-cobranca.md', size: 9000 },
	]);

	assert.ok(preamble.includes('specs/cobranca/sdd-cobranca.md'), 'o caminho relativo tem de estar no prompt');
	assert.ok(preamble.includes('Read'), 'o agente precisa saber que é ele que lê');
	assert.ok(!preamble.includes('```'), 'nada é citado: citar é pagar pelo arquivo inteiro');
});

// ── do computador: o conteúdo, porque o motor não alcança ────────────────────
//
// É o caso de uso do botão, não uma exceção: o print na pasta de Downloads, a
// planilha que alguém mandou. O motor roda na raiz do projeto e, numa janela
// remota, nem é a mesma máquina — o caminho da máquina do usuário não serviria
// de nada para ele.

test('um arquivo do computador viaja com o conteúdo', () => {
	const preamble = chat.filesPreamble([
		{ name: 'notas.txt', rel: null, size: 26, state: 'text', text: 'reunião: trocar o provedor' },
	]);

	assert.ok(preamble.includes('reunião: trocar o provedor'), 'sem o conteúdo o anexo não serve para nada');
	assert.ok(preamble.includes('anexado do computador'), 'o agente tem de saber de onde veio, e por que não há caminho');
	assert.ok(preamble.includes('notas.txt'), 'o nome é o que sobra para se referir ao arquivo');
});

test('conteúdo truncado é declarado como truncado', () => {
	const whole = chat.filesPreamble([{ name: 'a.txt', rel: null, size: 10, state: 'text', text: 'curto' }]);
	const cut = chat.filesPreamble([{ name: 'b.txt', rel: null, size: 60000, state: 'text', text: 'x'.repeat(40000), truncated: true }]);

	assert.ok(!whole.includes('truncado'), 'um arquivo inteiro não pode dizer que foi cortado');
	assert.ok(cut.includes('truncado'), 'um arquivo cortado não pode passar por completo');
});

test('um binário diz que não deu para ler, em vez de virar tokens de lixo', () => {
	const preamble = chat.filesPreamble([{ name: 'relatorio.pdf', rel: null, size: 4096, state: 'binary' }]);

	assert.ok(preamble.includes('binário'), 'o prompt tem de dizer o que aconteceu');
	assert.ok(!preamble.includes('```'), 'e não citar bytes');
});

test('um arquivo grande demais aparece pelo nome e pelo tamanho', () => {
	const preamble = chat.filesPreamble([{ name: 'dump.sql', rel: null, size: 5 * 1024 * 1024, state: 'too-big' }]);

	assert.ok(preamble.includes('dump.sql') && preamble.includes('5120 KB'), preamble);
	assert.ok(preamble.includes('não veio'), 'o agente não pode achar que recebeu o arquivo');
});

test('um anexo sem conteúdo legível não vira um prompt mentiroso', () => {
	const preamble = chat.filesPreamble([{ name: 'y.md', rel: null }]);

	assert.ok(preamble.includes('não veio'), 'o agente tem de saber que o anexo falhou');
});

test('sem anexos não há cabeçalho nenhum', () => {
	assert.strictEqual(chat.filesPreamble([]), '', 'um prompt sem anexos não paga por uma seção vazia');
});

run();
