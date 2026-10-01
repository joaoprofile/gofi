'use strict';

const { claudeCodeProvider } = require('./claudeCode.js');

/**
 * The engines the chat can run on. Claude is the only one implemented today;
 * a new engine is one module here plus its slug in the `gofiAI.provider` enum
 * in package.json — nothing above this layer changes.
 *
 * @type {Record<string, import('./types.js').ChatProvider>}
 */
const providers = {
	[claudeCodeProvider.id]: claudeCodeProvider,
};

const DEFAULT_PROVIDER = claudeCodeProvider.id;

/**
 * Resolves the configured engine, falling back to the default when the setting
 * names one that isn't registered (a downgrade, or a hand-edited settings.json).
 *
 * @param {import('vscode').WorkspaceConfiguration} config
 * @returns {import('./types.js').ChatProvider}
 */
function resolveProvider(config) {
	const id = (config.get('provider') || '').trim();
	return providers[id] || providers[DEFAULT_PROVIDER];
}

module.exports = { providers, resolveProvider, DEFAULT_PROVIDER };
