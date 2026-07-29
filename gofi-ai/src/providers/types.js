'use strict';

/**
 * The contract every LLM engine behind the GOFI AI chat implements.
 *
 * The panel never talks to a vendor SDK: it hands a turn to a provider and
 * consumes the normalised event stream below. Adding a second engine means
 * writing one more module in this folder and registering it — the webview,
 * the session and the commands stay untouched.
 *
 * @typedef {Object} ChatProvider
 * @property {string} id           Stable slug, matches the `gofiAI.provider` enum.
 * @property {string} label        Human-readable name shown in the chat header.
 * @property {boolean} [supportsImages] Whether the engine accepts pasted images.
 * @property {(ctx: ProviderContext) => Promise<ProbeResult>} probe
 *           Cheap reachability check. Never throws — it reports.
 * @property {(ctx: ProviderContext) => ChatSession} createSession
 *           Opens a conversation. The caller owns it and disposes it.
 * @property {(ctx: ProviderContext) => SavedSession[]} [savedSessions]
 *           Conversations the engine itself has for this folder, newest first.
 *           Implementing this is what lets a thread started outside the panel —
 *           in a terminal, say — be listed and continued inside it. A provider
 *           with no store of its own omits it, and the panel falls back to the
 *           transcripts it keeps itself.
 * @property {(ctx: ProviderContext, engineId: string) => object[]} [savedTranscript]
 *           That conversation as webview events, for replay. Empty means "no
 *           transcript", not "an empty conversation".
 * @property {(ctx: ProviderContext, engineId: string) => string} [resumeCommand]
 *           The command line that continues a conversation in a terminal.
 *
 * @typedef {Object} SavedSession
 * @property {string} engineId
 * @property {string} title
 * @property {number} updatedAt  Epoch ms.
 *
 * A session, not a call: engines pay a real startup cost (process spawn,
 * project discovery, prompt-cache warm-up) that must not be repeated per
 * message. Keeping the conversation open is also what keeps the cached prefix
 * alive, so later turns are billed as cache reads instead of cache writes.
 *
 * @typedef {Object} ChatSession
 * @property {(turn: Turn, emit: (e: ProviderEvent) => void) => void} send
 *           Runs one turn. Only one may be in flight at a time.
 * @property {() => void} cancel   Stops the turn in flight. Idempotent.
 * @property {() => void} dispose  Releases the engine. The session is dead after.
 *
 * @typedef {Object} ProviderContext
 * @property {string} cwd              Workspace folder the engine runs in.
 * @property {import('vscode').WorkspaceConfiguration} config
 * @property {import('../gofiConfig.js').GofiProject | null} project
 *           Parsed `.gofi.yaml`, or null outside a gofi project.
 * @property {object} [hookSettings]  Engine settings installing the panel's
 *           per-call approval gate. Absent when approval is off.
 * @property {string | null} [resumeSessionId]
 *           An engine conversation to continue instead of opening a new one —
 *           how a chat reopened from the panel's history keeps its context. A
 *           provider that cannot resume ignores it and starts fresh.
 *
 * @typedef {Object} Turn
 * @property {string} prompt
 * @property {TurnImage[]} [images]  Pasted images, sent alongside the text.
 *
 * @typedef {Object} TurnImage
 * @property {string} mediaType  e.g. `image/png`
 * @property {string} data       base64, without a data: prefix
 *
 * @typedef {Object} ProbeResult
 * @property {boolean} ok
 * @property {string} detail      Version string when ok, the reason when not.
 * @property {string} [hint]      What the user should do about it.
 *
 * Normalised event stream. A turn emits at most one `meta`, any number of
 * `delta` / `message` / `tool_result`, and exactly one terminal `done`|`error`.
 *
 * @typedef {{type: 'meta'} & MetaEvent} ProviderMetaEvent
 * @typedef {{type: 'delta', kind: 'text'|'thinking', text: string}} ProviderDeltaEvent
 * @typedef {{type: 'message', blocks: Block[]}} ProviderMessageEvent
 * @typedef {{type: 'tool_result', toolUseId: string, isError: boolean, preview: string, chars: number}} ProviderToolResultEvent
 * @typedef {{type: 'usage', scope: 'turn'|'total', usage: Usage}} ProviderUsageEvent
 * @typedef {{type: 'done', isError: boolean, costUsd: number|null, durationMs: number|null, error?: string}} ProviderDoneEvent
 * @typedef {{type: 'error', message: string, hint?: string}} ProviderErrorEvent
 * @typedef {ProviderMetaEvent|ProviderDeltaEvent|ProviderMessageEvent|ProviderToolResultEvent|ProviderUsageEvent|ProviderDoneEvent|ProviderErrorEvent} ProviderEvent
 *
 * Token counters, reported per assistant message (`turn`) and once for the
 * whole run (`total`). A provider that cannot meter tokens simply never emits
 * this — the panel then shows retrieval sizes only.
 *
 * @typedef {Object} Usage
 * @property {number} input       Uncached input tokens
 * @property {number} output
 * @property {number} cacheRead   Served from the prompt cache
 * @property {number} cacheWrite  Written to the prompt cache
 *
 * @typedef {Object} MetaEvent
 * @property {string|null} sessionId
 * @property {string|null} model
 * @property {string[]} slashCommands
 *
 * @typedef {{type: 'text', text: string}
 *          |{type: 'thinking', text: string}
 *          |{type: 'tool_use', id: string, name: string, input: unknown}} Block
 */

module.exports = {};
