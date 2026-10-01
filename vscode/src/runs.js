'use strict';

const fs = require('fs');
const path = require('path');

/**
 * The run record of a plan the panel conducts, in the shape gofi writes
 * (`gofi.run/v1`, under .gofi/runs/, outside git): the request, what the
 * intake decided, each phase with its tier, model, time and cost, the QA's
 * verdict and how the plan ended. Best effort: a record that cannot be written
 * never stops the plan.
 */
const SCHEMA = 'gofi.run/v1';
const DIR = path.join('.gofi', 'runs');

function slug(text) {
	const s = String(text || '').toLowerCase().normalize('NFD').replace(/[̀-ͯ]/g, '')
		.replace(/[^a-z0-9]+/g, '-').replace(/^-+|-+$/g, '').slice(0, 40).replace(/-+$/, '');
	return s || 'pedido';
}

function stamp(d) {
	const p = (n) => String(n).padStart(2, '0');
	return `${d.getUTCFullYear()}${p(d.getUTCMonth() + 1)}${p(d.getUTCDate())}-${p(d.getUTCHours())}${p(d.getUTCMinutes())}${p(d.getUTCSeconds())}`;
}

class RunRecord {
	/**
	 * @param {string} root   The project root
	 * @param {any} result    The intake's result
	 */
	constructor(root, result) {
		const now = new Date();
		this.data = {
			schema: SCHEMA,
			id: `${stamp(now)}-${slug(result.request)}`,
			started: now.toISOString(),
			surface: 'panel',
			request: result.request,
			intent: result.intent,
			artifact: result.artifact,
			context: result.context ? result.context.name : undefined,
			signals: result.signals || [],
			questions: (result.questions || []).length,
			assumed: result.assumptions && result.assumptions.length ? result.assumptions : undefined,
			intake_usd: result.spent ? result.spent.usd : undefined,
			phases: [],
			outcome: 'running',
			cost_usd: 0,
		};
		this.file = path.join(root, DIR, `${this.data.id}.json`);
		this.save();
	}

	/** Records a phase that ended. */
	phase(p) {
		this.data.phases.push(p);
		this.data.cost_usd += p.cost_usd || 0;
		this.save();
	}

	/** Closes the record with the plan's outcome and the QA's verdict. */
	finish(outcome, verdict) {
		this.data.outcome = outcome;
		if (verdict) {
			this.data.verdict = verdict;
		}
		this.data.finished = new Date().toISOString();
		this.save();
	}

	save() {
		try {
			fs.mkdirSync(path.dirname(this.file), { recursive: true });
			const tmp = `${this.file}.tmp`;
			fs.writeFileSync(tmp, `${JSON.stringify(this.data, null, 2)}\n`);
			fs.renameSync(tmp, this.file);
		} catch {
			// The record is for calibration; the plan does not wait on it.
		}
	}
}

module.exports = { SCHEMA, RunRecord, slug };
