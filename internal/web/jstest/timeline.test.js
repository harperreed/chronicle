// ABOUTME: Tests for the page's timeline helpers: where a streamed entry goes, whether a list holds it, and how a card shows its time.
// ABOUTME: Runs under Node's test runner against plain objects, so it needs no browser.
'use strict';

// A card shows local time, so fix the zone before any date is built. Chicago is on standard time
// (UTC-6) in November, which makes every expected time below six hours behind UTC.
process.env.TZ = 'America/Chicago';

const test = require('node:test');
const assert = require('node:assert/strict');

require('../assets/timeline.js');
const { insertionIndex, contains, formatTime } = globalThis.ChronicleTimeline;

const entry = (id, timestamp) => ({ id, timestamp });

test('insertionIndex of an empty list is 0', () => {
  assert.equal(insertionIndex([], entry('new', '2026-11-11T16:00:00Z')), 0);
});

test('insertionIndex puts a newer entry first', () => {
  const entries = [entry('a', '2026-11-11T16:00:00Z'), entry('b', '2026-11-11T15:00:00Z')];
  assert.equal(insertionIndex(entries, entry('new', '2026-11-11T17:00:00Z')), 0);
});

test('insertionIndex puts an older entry last', () => {
  const entries = [entry('a', '2026-11-11T16:00:00Z'), entry('b', '2026-11-11T15:00:00Z')];
  assert.equal(insertionIndex(entries, entry('new', '2026-11-11T14:00:00Z')), entries.length);
});

test('insertionIndex puts an entry between its neighbors', () => {
  const entries = [entry('a', '2026-11-11T17:00:00Z'), entry('b', '2026-11-11T15:00:00Z')];
  assert.equal(insertionIndex(entries, entry('new', '2026-11-11T16:00:00Z')), 1);
});

test('insertionIndex puts an entry with an equal timestamp after the existing equal ones', () => {
  const entries = [
    entry('a', '2026-11-11T17:00:00Z'),
    entry('b', '2026-11-11T16:00:00Z'),
    entry('c', '2026-11-11T16:00:00Z'),
    entry('d', '2026-11-11T15:00:00Z'),
  ];
  assert.equal(insertionIndex(entries, entry('new', '2026-11-11T16:00:00Z')), 3);
});

test('insertionIndex compares instants across UTC offsets, not the timestamp strings', () => {
  // 17:00 at -06:00 is 23:00Z, so it is newer than 22:00Z although its string sorts lower.
  const utc = entry('utc', '2026-11-11T22:00:00Z');
  const local = entry('local', '2026-11-11T17:00:00-06:00');
  assert.equal(insertionIndex([utc], local), 0);
  assert.equal(insertionIndex([local], utc), 1);
});

test('insertionIndex compares fractions of a second', () => {
  const whole = entry('whole', '2026-11-11T16:04:43Z');
  const half = entry('half', '2026-11-11T16:04:43.5Z');
  assert.equal(insertionIndex([whole], half), 0);
  assert.equal(insertionIndex([half], whole), 1);
});

test('contains finds an entry by id', () => {
  const entries = [entry('a', '2026-11-11T16:00:00Z'), entry('b', '2026-11-11T15:00:00Z')];
  assert.equal(contains(entries, 'a'), true);
  assert.equal(contains(entries, 'b'), true);
});

test('contains is false for an unknown id and for an empty list', () => {
  const entries = [entry('a', '2026-11-11T16:00:00Z')];
  assert.equal(contains(entries, 'missing'), false);
  assert.equal(contains([], 'a'), false);
});

test('formatTime shows weekday, month, day, a middle dot and a 24-hour time in local time', () => {
  assert.equal(formatTime('2026-11-11T22:04:43Z'), 'Wed Nov 11 · 16:04:43');
});

test('formatTime converts any UTC offset to local time', () => {
  assert.equal(formatTime('2026-11-11T17:04:43-05:00'), 'Wed Nov 11 · 16:04:43');
  assert.equal(formatTime('2026-11-12T04:04:43+06:00'), 'Wed Nov 11 · 16:04:43');
});

test('formatTime pads the time and leaves the day plain', () => {
  assert.equal(formatTime('2026-11-05T09:03:07Z'), 'Thu Nov 5 · 03:03:07');
});

test('formatTime shows the local day, which can differ from the day in UTC', () => {
  assert.equal(formatTime('2026-11-12T02:30:00Z'), 'Wed Nov 11 · 20:30:00');
});

test('formatTime shows midnight as 00:00:00', () => {
  assert.equal(formatTime('2026-11-11T06:00:00Z'), 'Wed Nov 11 · 00:00:00');
});

test('formatTime drops fractions of a second', () => {
  assert.equal(formatTime('2026-11-11T22:04:43.987654321Z'), 'Wed Nov 11 · 16:04:43');
});
