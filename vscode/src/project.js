'use strict';

const fs = require('fs');
const os = require('os');
const path = require('path');

/** The file that makes a folder a gofi project. */
const CONFIG_FILE = '.gofi.yaml';

/**
 * Finds the gofi project root at or above `start`.
 *
 * Opening a subfolder of a monorepo is normal, and the extension would
 * otherwise report "no gofi project" while `gofi`, run from the same directory,
 * finds one fine. So the search walks up — but only to the repository root, and
 * never into the home directory.
 *
 * The bound is load-bearing, not defensive: a stray `~/.gofi.yaml` is common
 * (someone ran `gofi init` in their home once), and an unbounded walk would
 * make every project on the machine claim that config's identity.
 *
 * @param {string} start Workspace folder
 * @returns {string | null} The folder holding `.gofi.yaml`, or null
 */
function findProjectRoot(start) {
	const home = os.homedir();
	let dir = start;

	while (dir && dir !== home) {
		if (fs.existsSync(path.join(dir, CONFIG_FILE))) {
			return dir;
		}
		// A `.git` here means this is the repo root: a `.gofi.yaml` further up
		// belongs to something else.
		if (fs.existsSync(path.join(dir, '.git'))) {
			return null;
		}
		const parent = path.dirname(dir);
		if (parent === dir) {
			return null; // reached the filesystem root
		}
		dir = parent;
	}
	return null;
}

module.exports = { findProjectRoot, CONFIG_FILE };
