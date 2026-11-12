// ABOUTME: Tests for the page's timeline helpers: where a streamed entry goes, how a card shows its time, and how long a NEW mark lasts.
// ABOUTME: Runs under Node's test runner against plain objects and a clock that each test passes in, so it needs no browser.
'use strict';

// A card shows local time, so fix the zone before any date is built. Chicago is on standard time
// (UTC-6) in November, which makes every expected time below six hours behind UTC.
process.env.TZ = 'America/Chicago';

const test = require('node:test');
const assert = require('node:assert/strict');

require('../assets/timeline.js');
const { insertionIndex, contains, formatTime, addedIds, newMarks } = globalThis.ChronicleTimeline;

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

// The NEW marks. A mark lasts until the page has been visible for 10 seconds. The tests pass the
// time in as milliseconds, so they run on a clock that only moves when they say so.
const aSecond = 1000;
const anHour = 3600 * aSecond;

test('a mark on a visible page lasts 10 seconds from the arrival', () => {
  const marks = newMarks.create(true);
  newMarks.arrive(marks, 'a', 1 * aSecond);
  assert.equal(newMarks.has(marks, 'a'), true);
  assert.equal(newMarks.nextExpiry(marks), 11 * aSecond);
  assert.deepEqual(newMarks.expire(marks, 11 * aSecond - 1), []);
  assert.equal(newMarks.has(marks, 'a'), true);
  assert.deepEqual(newMarks.expire(marks, 11 * aSecond), ['a']);
  assert.equal(newMarks.has(marks, 'a'), false);
  assert.equal(newMarks.nextExpiry(marks), null);
});

test('marks run out each on its own deadline', () => {
  const marks = newMarks.create(true);
  newMarks.arrive(marks, 'a', 0);
  newMarks.arrive(marks, 'b', 4 * aSecond);
  assert.equal(newMarks.nextExpiry(marks), 10 * aSecond);
  assert.deepEqual(newMarks.expire(marks, 10 * aSecond), ['a']);
  assert.equal(newMarks.nextExpiry(marks), 14 * aSecond);
  assert.deepEqual(newMarks.expire(marks, 14 * aSecond), ['b']);
});

test('a mark that arrives on a hidden page waits, however long the page stays hidden', () => {
  const marks = newMarks.create(false);
  newMarks.arrive(marks, 'a', 0);
  newMarks.arrive(marks, 'b', 5 * aSecond);
  assert.equal(newMarks.nextExpiry(marks), null);
  assert.deepEqual(newMarks.expire(marks, anHour), []);
  assert.equal(newMarks.has(marks, 'a'), true);
  assert.equal(newMarks.has(marks, 'b'), true);

  newMarks.show(marks, anHour);
  assert.equal(newMarks.nextExpiry(marks), anHour + 10 * aSecond);
  assert.deepEqual(newMarks.expire(marks, anHour + 10 * aSecond - 1), []);
  assert.deepEqual(newMarks.expire(marks, anHour + 10 * aSecond), ['a', 'b']);
});

test('hiding at 2 seconds and showing at 9 clears the mark at 19 seconds, not at 10', () => {
  const marks = newMarks.create(true);
  newMarks.arrive(marks, 'a', 0);
  newMarks.hide(marks); // The page goes to the background at 2 seconds.
  assert.equal(newMarks.nextExpiry(marks), null);
  newMarks.show(marks, 9 * aSecond);

  // At 10 seconds the mark has been seen for 1 second since the page came back, so it stays.
  assert.deepEqual(newMarks.expire(marks, 10 * aSecond), []);
  assert.equal(newMarks.has(marks, 'a'), true);
  assert.equal(newMarks.nextExpiry(marks), 19 * aSecond);
  assert.deepEqual(newMarks.expire(marks, 19 * aSecond - 1), []);
  assert.deepEqual(newMarks.expire(marks, 19 * aSecond), ['a']);
});

test('a mark does not run out while the page is hidden', () => {
  const marks = newMarks.create(true);
  newMarks.arrive(marks, 'a', 0);
  newMarks.hide(marks);
  assert.deepEqual(newMarks.expire(marks, anHour), []);
  assert.equal(newMarks.has(marks, 'a'), true);
});

test('showing a page that is already visible does not extend a running mark', () => {
  const marks = newMarks.create(true);
  newMarks.arrive(marks, 'a', 0);
  newMarks.show(marks, 5 * aSecond);
  assert.equal(newMarks.nextExpiry(marks), 10 * aSecond);
});

test('hiding twice and showing twice give a waiting mark one 10-second run', () => {
  const marks = newMarks.create(true);
  newMarks.arrive(marks, 'a', 0);
  newMarks.hide(marks);
  newMarks.hide(marks);
  newMarks.show(marks, 20 * aSecond);
  newMarks.show(marks, 25 * aSecond);
  assert.equal(newMarks.nextExpiry(marks), 30 * aSecond);
});

test('a mark that arrives again keeps its first deadline', () => {
  const marks = newMarks.create(true);
  newMarks.arrive(marks, 'a', 0);
  newMarks.arrive(marks, 'a', 6 * aSecond);
  assert.equal(newMarks.nextExpiry(marks), 10 * aSecond);
});

test('nextExpiry of a page with no marks is null', () => {
  assert.equal(newMarks.nextExpiry(newMarks.create(true)), null);
  assert.equal(newMarks.nextExpiry(newMarks.create(false)), null);
});

test('addedIds returns the ids that the new list has and the old one lacks, in the new list order', () => {
  const previous = [entry('b', '2026-11-11T16:00:00Z'), entry('a', '2026-11-11T15:00:00Z')];
  const next = [
    entry('d', '2026-11-11T18:00:00Z'),
    entry('c', '2026-11-11T17:00:00Z'),
    entry('b', '2026-11-11T16:00:00Z'),
    entry('a', '2026-11-11T15:00:00Z'),
  ];
  assert.deepEqual(addedIds(previous, next), ['d', 'c']);
});

test('addedIds returns every id of the new list when the old list is empty, and none when the new one is', () => {
  const entries = [entry('b', '2026-11-11T16:00:00Z'), entry('a', '2026-11-11T15:00:00Z')];
  assert.deepEqual(addedIds([], entries), ['b', 'a']);
  assert.deepEqual(addedIds(entries, []), []);
});

test('addedIds ignores an entry that only changed place', () => {
  const previous = [entry('a', '2026-11-11T16:00:00Z'), entry('b', '2026-11-11T15:00:00Z')];
  const next = [entry('b', '2026-11-11T15:00:00Z'), entry('a', '2026-11-11T16:00:00Z')];
  assert.deepEqual(addedIds(previous, next), []);
});

test('a reload keeps the marks nobody has seen and adds marks for the entries it brings in', () => {
  // The page is in a background tab, and two entries stream in.
  const marks = newMarks.create(false);
  newMarks.arrive(marks, 'a', 0);
  newMarks.arrive(marks, 'b', 1 * aSecond);

  // The connection drops and comes back. The reload holds one more entry, written during the outage.
  const before = [entry('b', '2026-11-11T16:01:00Z'), entry('a', '2026-11-11T16:00:00Z'), entry('old', '2026-11-11T15:00:00Z')];
  const after = [entry('c', '2026-11-11T16:02:00Z'), ...before];
  addedIds(before, after).forEach((id) => newMarks.arrive(marks, id, 60 * aSecond));
  for (const id of ['a', 'b', 'c']) assert.equal(newMarks.has(marks, id), true, `${id} is marked`);
  assert.equal(newMarks.has(marks, 'old'), false);

  // The reader comes back, and all three run for 10 seconds from then.
  newMarks.show(marks, 90 * aSecond);
  assert.deepEqual(newMarks.expire(marks, 100 * aSecond - 1), []);
  assert.deepEqual(newMarks.expire(marks, 100 * aSecond), ['a', 'b', 'c']);
});
