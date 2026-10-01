'use strict';

/**
 * The engine's own rate-limit banner must stop the chat, not have it whip
 * through the rest of the queue against the same limit.
 *
 * `onProviderEvent` is exercised directly, with no real engine session behind
 * it — the same shape a `done` or a hard process `error` arrives in, is all
 * this logic reads. What is pinned: the pending queue is dropped instead of
 * fired, no second turn is started, and the panel gets a `rateLimited`
 * message instead of a plain `error`/`done` with the reset time attached.
 *
 * Run with `node test/rate-limit.test.js`.
 */

const assert = require('assert');
const { installVscodeStub, makeContext, runner } = require('./vscode-stub.js');

installVscodeStub();

const { Chat } = require('../src/chat.js');
const { test, run } = runner();

const BANNER = "You've hit your session limit · resets 4:50pm (America/Sao_Paulo)";

function makeChat() {
	const chat = new Chat(makeContext(), 1);
	const posted = [];
	chat.surfaces.add({ postMessage: (m) => posted.push(m) });
	const startTurnCalls = [];
	chat.startTurn = (prompt, images) => startTurnCalls.push({ prompt, images });
	return { chat, posted, startTurnCalls };
}

test('a rate-limited `done` drops the queue instead of chaining into it', () => {
	const { chat, posted, startTurnCalls } = makeChat();
	chat.running = true;
	chat.pending.push({ prompt: 'próxima pergunta', images: [] });

	chat.onProviderEvent({ type: 'done', isError: true, error: BANNER, costUsd: null, durationMs: 900 });

	assert.strictEqual(chat.pending.length, 0, 'a mensagem enfileirada não pode sobreviver ao limite');
	assert.strictEqual(startTurnCalls.length, 0, 'nenhum novo turno pode ser disparado contra o mesmo limite');
	assert.strictEqual(chat.running, false);
	assert.ok(!posted.some((m) => m.type === 'dequeued'), 'não houve turno seguinte para anunciar');

	const rateLimited = posted.find((m) => m.type === 'rateLimited');
	assert.ok(rateLimited, 'o painel deve receber um aviso de limite, não um `done` comum');
	assert.strictEqual(rateLimited.reset, '4:50pm (America/Sao_Paulo)');
	assert.strictEqual(rateLimited.message, BANNER);
	assert.ok(!posted.some((m) => m.type === 'done'), 'não deve haver `done` junto — só o aviso de limite');
});

test('um `done` com erro comum continua encadeando a fila, como antes', () => {
	// O disparo do próximo turno é assíncrono (`setImmediate`); o que dá para
	// pinar de forma síncrona é que a fila foi esvaziada *para seguir* — o
	// `dequeued` só é postado quando há um próximo turno de fato a caminho,
	// ao contrário do ramo de limite de sessão, que nunca o posta.
	const { chat, posted } = makeChat();
	chat.running = true;
	chat.pending.push({ prompt: 'próxima pergunta', images: [] });

	chat.onProviderEvent({ type: 'done', isError: true, error: 'a execução falhou', costUsd: null, durationMs: 400 });

	assert.strictEqual(chat.pending.length, 0, 'a mensagem seguinte saiu da fila para rodar');
	assert.ok(posted.some((m) => m.type === 'dequeued'), 'um erro comum não deve mudar o comportamento existente da fila');
	assert.ok(posted.some((m) => m.type === 'done'), 'o `done` comum continua sendo postado');
	assert.ok(!posted.some((m) => m.type === 'rateLimited'), 'um erro comum não é um limite de sessão');
});

test('um processo que morre citando o limite também vira aviso de espera', () => {
	const { chat, posted } = makeChat();
	chat.running = true;
	chat.pending.push({ prompt: 'próxima pergunta', images: [] });

	chat.onProviderEvent({ type: 'error', message: BANNER, hint: undefined });

	assert.strictEqual(chat.pending.length, 0);
	const rateLimited = posted.find((m) => m.type === 'rateLimited');
	assert.ok(rateLimited, 'a mesma frase, vinda de um erro de processo, deve virar o mesmo aviso');
	assert.strictEqual(rateLimited.reset, '4:50pm (America/Sao_Paulo)');
	assert.ok(!posted.some((m) => m.type === 'error'), 'não deve sobrar um `error` genérico ao lado');
});

run();
