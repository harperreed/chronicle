// ABOUTME: Tests for the page's theme modes: reading a stored value, cycling, and applying a mode.
// ABOUTME: Runs under Node's test runner against plain objects, so it needs no browser.
'use strict';
const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');

const themePath = path.join(__dirname, '..', 'assets', 'theme.js');
require('../assets/theme.js');
const { normalize, next, apply } = globalThis.ChronicleTheme;

// loadInPage runs theme.js the way a browser does, as a classic script in a global scope that
// holds the given globals, and returns that scope.
function loadInPage(globals) {
  const scope = { ...globals };
  vm.runInNewContext(fs.readFileSync(themePath, 'utf8'), scope);
  return scope;
}

test('normalize keeps the three modes', () => {
  for (const mode of ['auto', 'light', 'dark']) assert.equal(normalize(mode), mode);
});

test('normalize turns a missing or unknown value into auto', () => {
  for (const stored of [null, undefined, '', 'nope', 'Dark']) assert.equal(normalize(stored), 'auto');
});

test('next cycles auto, light, dark, auto', () => {
  assert.equal(next('auto'), 'light');
  assert.equal(next('light'), 'dark');
  assert.equal(next('dark'), 'auto');
});

test('apply sets data-theme for light and dark', () => {
  const root = { dataset: {} };
  apply('dark', root);
  assert.equal(root.dataset.theme, 'dark');
  apply('light', root);
  assert.equal(root.dataset.theme, 'light');
});

test('apply removes data-theme for auto and for an unknown mode', () => {
  const root = { dataset: { theme: 'dark' } };
  apply('auto', root);
  assert.equal('theme' in root.dataset, false);
  root.dataset.theme = 'light';
  apply('nope', root);
  assert.equal('theme' in root.dataset, false);
});

test('loading in a page applies the stored theme', () => {
  const root = { dataset: {} };
  const reads = [];
  loadInPage({
    document: { documentElement: root },
    localStorage: { getItem: (key) => { reads.push(key); return 'dark'; } },
  });
  assert.deepEqual(reads, ['chronicle-theme']);
  assert.equal(root.dataset.theme, 'dark');
});

test('loading in a page with no stored theme follows the system', () => {
  const root = { dataset: {} };
  loadInPage({ document: { documentElement: root }, localStorage: { getItem: () => null } });
  assert.equal('theme' in root.dataset, false);
});

test('loading in a page whose storage throws still defines the module', () => {
  const root = { dataset: {} };
  const scope = loadInPage({
    document: { documentElement: root },
    localStorage: { getItem: () => { throw new Error('storage is blocked'); } },
  });
  assert.equal('theme' in root.dataset, false);
  assert.equal(typeof scope.ChronicleTheme.next, 'function');
});
