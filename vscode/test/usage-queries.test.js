// What asks the index counts as an index query: impact and trace too, by
// the MCP server or the shell — rebuilding the index does not.
const assert = require('assert');
const { UsageLedger, GRAPH_ROW } = require('../src/usage.js');

const ledger = new UsageLedger('/projeto');
ledger.noteToolUse('1', 'mcp__gofi__impact', { targets: ['service.Bill'] });
ledger.noteToolUse('2', 'Bash', { command: 'gofi trace criar-pedidos --report' });
ledger.noteToolUse('3', 'Bash', { command: 'gofi index code' });
ledger.noteToolUse('4', 'mcp__gofi__index', {});
for (const id of ['1', '2', '3', '4']) {
	ledger.noteToolResult(id, 100, false);
}
const graph = ledger.rows.filter((r) => r.name === GRAPH_ROW);
assert.strictEqual(graph.length, 2, JSON.stringify(ledger.rows));
console.log('usage-queries: ok');
