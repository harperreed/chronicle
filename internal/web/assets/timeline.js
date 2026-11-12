// ABOUTME: Timeline helpers for the web page: where a streamed entry goes in the list, how a card shows its time, and how long a NEW mark lasts.
// ABOUTME: Plain functions that never touch the page or read a clock, so Node's test runner can load them without a browser.
(() => {
  'use strict';

  const WEEKDAYS = ['Sun', 'Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat'];
  const MONTHS = ['Jan', 'Feb', 'Mar', 'Apr', 'May', 'Jun', 'Jul', 'Aug', 'Sep', 'Oct', 'Nov', 'Dec'];

  // How long a NEW mark lasts, counted while the page is visible.
  const NEW_MARK_MS = 10000;

  // parseTime returns the instant of an RFC 3339 timestamp, as milliseconds since the epoch. Go
  // writes up to nine fractional digits and none when the fraction is zero, but the ECMAScript date
  // format guarantees only three, so another engine may return NaN for any other count. The fraction
  // is cut or padded to three digits first. A cut never rounds, so a time can't move into the next second.
  function parseTime(timestamp) {
    return Date.parse(timestamp.replace(/\.(\d+)/, (_, digits) => `.${digits.slice(0, 3).padEnd(3, '0')}`));
  }

  // insertionIndex returns where entry goes in entries, which are sorted newest first: the index of
  // the first entry that is older than it, or entries.length if none is. An entry with the same
  // timestamp as others goes after them, so equal entries keep the order they arrived in.
  //
  // Timestamps are compared as instants. They carry different UTC offsets, so comparing the
  // strings would put 17:00-06:00 before 22:00Z, although it is an hour later.
  function insertionIndex(entries, entry) {
    const at = parseTime(entry.timestamp);
    const index = entries.findIndex((existing) => parseTime(existing.timestamp) < at);
    return index === -1 ? entries.length : index;
  }

  // contains reports whether entries holds the entry with this id.
  function contains(entries, id) {
    return entries.some((entry) => entry.id === id);
  }

  // addedIds returns the ids of the entries in next that previous lacks, in the order of next. When
  // the stream comes back after a gap, these are the entries that were written while the page was away.
  function addedIds(previous, next) {
    const known = new Set(previous.map((entry) => entry.id));
    return next.filter((entry) => !known.has(entry.id)).map((entry) => entry.id);
  }

  function twoDigits(number) {
    return String(number).padStart(2, '0');
  }

  // formatTime returns a card's time in the viewer's local time zone, as in "Wed Nov 11 · 16:04:43".
  // Names are English and fixed, as the rest of the page is, and the clock has 24 hours.
  function formatTime(timestamp) {
    const date = new Date(parseTime(timestamp));
    const day =`${WEEKDAYS[date.getDay()]} ${MONTHS[date.getMonth()]} ${date.getDate()}`;
    const clock = [date.getHours(), date.getMinutes(), date.getSeconds()].map(twoDigits).join(':');
    return `${day} · ${clock}`;
  }

  // ---- NEW marks ----
  // An entry that arrives while the page is open gets a NEW mark, which lasts until the page has
  // been visible for 10 seconds. A reader who comes back to a hidden page still sees what landed
  // while it was hidden.
  //
  // The marks are state, kept by entry id, and a card only shows them, so drawing the list again
  // can't lose one. Each id holds the time its mark runs out, or null while the page is hidden. These
  // functions read no clock: the caller passes the time in, as milliseconds on a clock that doesn't
  // run backwards. They change the state they are given.

  // createMarks returns the state of a page with no marks, visible or hidden.
  function createMarks(visible) {
    return { visible, deadlines: new Map() };
  }

  // hasMark reports whether the entry with this id carries a mark.
  function hasMark(marks, id) {
    return marks.deadlines.has(id);
  }

  // arrive marks an entry that has just arrived. On a hidden page the mark waits for the page to
  // show. An entry that is marked already keeps its deadline.
  function arrive(marks, id, now) {
    if (marks.deadlines.has(id)) return;
    marks.deadlines.set(id, marks.visible ? now + NEW_MARK_MS : null);
  }

  // hide takes every running mark back to waiting, so that none runs out while the page is hidden.
  function hide(marks) {
    marks.visible = false;
    for (const id of marks.deadlines.keys()) marks.deadlines.set(id, null);
  }

  // show starts the full 10 seconds for every mark that waited. A mark that is running keeps its deadline.
  function show(marks, now) {
    marks.visible = true;
    for (const [id, deadline] of marks.deadlines) {
      if (deadline === null) marks.deadlines.set(id, now + NEW_MARK_MS);
    }
  }

  // expire removes the marks that have run out by now, and returns their ids in the order they were marked.
  function expire(marks, now) {
    const expired = [];
    for (const [id, deadline] of marks.deadlines) {
      if (deadline !== null && deadline <= now) expired.push(id);
    }
    expired.forEach((id) => marks.deadlines.delete(id));
    return expired;
  }

  // nextExpiry returns the earliest time a mark runs out, or null while no mark is running.
  function nextExpiry(marks) {
    let next = null;
    for (const deadline of marks.deadlines.values()) {
      if (deadline !== null && (next === null || deadline < next)) next = deadline;
    }
    return next;
  }

  const newMarks = { create: createMarks, has: hasMark, arrive, hide, show, expire, nextExpiry };

  globalThis.ChronicleTimeline = { parseTime, insertionIndex, contains, formatTime, addedIds, newMarks };
})();
