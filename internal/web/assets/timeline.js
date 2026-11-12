// ABOUTME: Timeline helpers for the web page: where a streamed entry goes in the list, and how a card shows its time.
// ABOUTME: Plain functions that never touch the page, so Node's test runner can load them without a browser.
(() => {
  'use strict';

  const WEEKDAYS = ['Sun', 'Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat'];
  const MONTHS = ['Jan', 'Feb', 'Mar', 'Apr', 'May', 'Jun', 'Jul', 'Aug', 'Sep', 'Oct', 'Nov', 'Dec'];

  // insertionIndex returns where entry goes in entries, which are sorted newest first: the index of
  // the first entry that is older than it, or entries.length if none is. An entry with the same
  // timestamp as others goes after them, so equal entries keep the order they arrived in.
  //
  // Timestamps are compared as instants. They carry different UTC offsets, so comparing the
  // strings would put 17:00-06:00 before 22:00Z, although it is an hour later.
  function insertionIndex(entries, entry) {
    const at = Date.parse(entry.timestamp);
    const index = entries.findIndex((existing) => Date.parse(existing.timestamp) < at);
    return index === -1 ? entries.length : index;
  }

  // contains reports whether entries holds the entry with this id.
  function contains(entries, id) {
    return entries.some((entry) => entry.id === id);
  }

  function twoDigits(number) {
    return String(number).padStart(2, '0');
  }

  // formatTime returns a card's time in the viewer's local time zone, as in "Wed Nov 11 · 16:04:43".
  // Names are English and fixed, as the rest of the page is, and the clock has 24 hours.
  function formatTime(timestamp) {
    const date = new Date(timestamp);
    const day = `${WEEKDAYS[date.getDay()]} ${MONTHS[date.getMonth()]} ${date.getDate()}`;
    const clock = [date.getHours(), date.getMinutes(), date.getSeconds()].map(twoDigits).join(':');
    return `${day} · ${clock}`;
  }

  globalThis.ChronicleTimeline = { insertionIndex, contains, formatTime };
})();
