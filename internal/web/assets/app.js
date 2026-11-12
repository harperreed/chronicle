// ABOUTME: Browser client for Chronicle's web interface: it fills the page from the API, keeps it current from the live stream, and sends changes back.
// ABOUTME: Its theme button cycles auto, light and dark through the ChronicleTheme module that theme.js defines.
(() => {
  'use strict';
  const $ = (selector) => document.querySelector(selector);
  // The server marks the html tag with the mode. A read-only page can't change entries, so its script leaves those handlers off.
  const readOnly = document.documentElement.dataset.mode === 'read-only';
  const state = {
    entries: [], tags: [], limit: 50, selectedTags: new Set(),
    more: false, // The last list was full, so the server may hold more entries.
    total: null, // The entry count from /api/stats, or null while it is unknown.
    loading: false, // A list request is in flight.
    waiting: [], // Entries that the stream delivered during that request.
    marks: ChronicleTimeline.newMarks.create(document.visibilityState === 'visible'), // The entries that carry a NEW mark.
    listedFilters: null, // The query that the list on the page was loaded with, or null when the last load failed.
    afterGap: false // The stream came back after a gap, so the next list for the same filters marks the entries it adds.
  };
  const els = {
    form: $('#entry-form'), message: $('#message'), tags: $('#tags'), entries: $('#entries'), search: $('#search'),
    since: $('#since'), until: $('#until'), count: $('#result-count'), total: $('#total-count'), tagFilter: $('#tag-filter'),
    clear: $('#clear-filters'), more: $('#load-more'), dialog: $('#edit-dialog'), editForm: $('#edit-form'), liveStatus: $('#live-status')
  };

  async function api(path, options = {}) {
    const response = await fetch(path, { headers: { 'Content-Type': 'application/json', ...(options.headers || {}) }, ...options });
    if (response.status === 204) return null;
    const body = await response.json().catch(() => ({ error: 'Unexpected server response' }));
    if (!response.ok) throw new Error(body.error || `Request failed (${response.status})`);
    return body;
  }

  function parseTags(value) { return [...new Set(value.split(',').map(t => t.trim().replace(/^#/, '')).filter(Boolean))]; }
  function escapeText(value) { const node = document.createElement('span'); node.textContent = value; return node.innerHTML; }
  function debounce(fn, wait) { let timer; return (...args) => { clearTimeout(timer); timer = setTimeout(() => fn(...args), wait); }; }
  function toast(message, error = false) {
    const node = document.createElement('div'); node.className = `toast${error ? ' error' : ''}`; node.textContent = message;
    $('#toast-region').append(node); setTimeout(() => node.remove(), 3500);
  }

  // filterParams builds the query that /api/entries and /api/stream both take: the search text, the dates and the tags.
  function filterParams() {
    const params = new URLSearchParams();
    if (els.search.value.trim()) params.set('q', els.search.value.trim());
    if (els.since.value) params.set('since', els.since.value);
    if (els.until.value) params.set('until', els.until.value);
    state.selectedTags.forEach(tag => params.append('tag', tag));
    return params;
  }

  // filtersActive says whether a filter narrows the list. The stream then carries only the entries that match.
  function filtersActive() { return filterParams().toString() !== ''; }

  // loadEntries shows the first page of entries that match the filters. The latest call wins: when a
  // call overtakes one that still waits, the older response is dropped, so a stale list never
  // replaces a newer one, and a call made after the stream's ready always gets its own request.
  let latestLoad = 0;
  async function loadEntries() {
    const load = ++latestLoad;
    state.loading = true; els.entries.setAttribute('aria-busy', 'true');
    if (!state.entries.length) els.entries.innerHTML = '<div class="skeleton"></div><div class="skeleton"></div><div class="skeleton"></div>';
    const params = filterParams(); const query = params.toString(); params.set('limit', String(state.limit));
    try {
      const data = await api(`/api/entries?${params}`);
      if (load !== latestLoad) return;
      // After a gap, the entries that this list adds are the ones the page missed. Only a list for the
      // same filters can say so: a list for other filters differs from the old one in every entry.
      if (state.afterGap && state.listedFilters === query) markNew(ChronicleTimeline.addedIds(state.entries, data.entries));
      state.afterGap = false; state.listedFilters = query;
      state.entries = data.entries; state.more = data.count >= state.limit; showEntries(); showResultCount();
      els.more.hidden = !state.more || state.limit >= 500;
    } catch (error) {
      if (load !== latestLoad) return;
      state.listedFilters = null; state.entries = [];
      els.entries.innerHTML = `<div class="empty"><span>!</span><h3>Couldn't load entries</h3><p>${escapeText(error.message)}</p></div>`;
      toast(error.message, true);
    } finally {
      if (load === latestLoad) {
        state.loading = false; els.entries.setAttribute('aria-busy', 'false');
        state.waiting.splice(0).forEach(showStreamedEntry);
      }
    }
  }

  function showResultCount() {
    const count = state.entries.length;
    els.count.textContent = `${count}${state.more ? '+' : ''} result${count === 1 ? '' : 's'}`;
  }

  function renderEntries() {
    els.entries.replaceChildren();
    if (!state.entries.length) {
      const filtered = filtersActive();
      // A read-only page has no composer to point to.
      const hint = filtered ? 'Try broadening your filters.' : readOnly ? 'New entries appear here as they are written.' : 'Record your first entry above.';
      els.entries.innerHTML = `<div class="empty"><span>✦</span><h3>${filtered ? 'No matching moments' : 'Your chronicle starts here'}</h3><p>${hint}</p></div>`;
      return;
    }
    state.entries.forEach(entry => els.entries.append(buildCard(entry)));
  }

  // showEntries draws the list. When the reader has scrolled into it, the entry at the top of the
  // view stays where it was, so entries that a reload put above it don't move the page.
  function showEntries() {
    const anchor = [...els.entries.children].find(card => card.dataset.id && card.getBoundingClientRect().bottom > 0);
    const before = anchor && anchor.getBoundingClientRect().top;
    renderEntries();
    if (!anchor || before >= 0) return;
    const same = [...els.entries.children].find(card => card.dataset.id === anchor.dataset.id);
    if (same) window.scrollBy(0, same.getBoundingClientRect().top - before);
  }

  // buildCard makes an entry's card from the page's template. Entry text goes in through textContent
  // only, so a message that holds markup shows as plain text.
  function buildCard(entry) {
    const card = $('#entry-template').content.cloneNode(true).querySelector('.entry-card'); card.dataset.id = entry.id;
    if (ChronicleTimeline.newMarks.has(state.marks, entry.id)) card.dataset.new = ''; // The card shows the mark. state.marks owns it.
    const time = card.querySelector('time');
    time.dateTime = entry.timestamp; time.textContent = ChronicleTimeline.formatTime(entry.timestamp);
    card.querySelector('.entry-message').textContent = entry.message;
    const tags = card.querySelector('.entry-tags'); (entry.tags || []).forEach(tag => { const chip = document.createElement('span'); chip.textContent = `#${tag}`; tags.append(chip); });
    const context = [entry.username && entry.hostname ? `${entry.username}@${entry.hostname}` : entry.username || entry.hostname, entry.working_directory].filter(Boolean);
    card.querySelector('.entry-context').textContent = context.join(' · ');
    return card;
  }

  async function loadStats() {
    try {
      const data = await api('/api/stats'); state.total = data.total; showTotal(); state.tags = data.tags; renderTagFilters();
    } catch { state.total = null; showTotal(); }
  }
  function showTotal() { els.total.textContent = state.total === null ? '—' : state.total.toLocaleString(); }
  function renderTagFilters() {
    els.tagFilter.replaceChildren();
    if (!state.tags.length) { els.tagFilter.innerHTML = '<span class="muted">No tags yet</span>'; return; }
    state.tags.slice(0, 18).forEach(tag => {
      const button = document.createElement('button'); button.type = 'button'; button.className = `tag-filter${state.selectedTags.has(tag.name) ? ' active' : ''}`;
      button.textContent = `#${tag.name} · ${tag.count}`; button.dataset.tag = tag.name; button.setAttribute('aria-pressed', state.selectedTags.has(tag.name)); els.tagFilter.append(button);
    });
  }

  // The NEW marks are state (state.marks, with the rules in timeline.js), and a card only shows its
  // mark, so drawing the list again keeps every mark. What is left here is the wiring: one timer
  // for the next mark to run out, and the page's visibility, which holds the marks while it is hidden.
  let marksTimer = 0;
  function markNew(ids) {
    ids.forEach(id => ChronicleTimeline.newMarks.arrive(state.marks, id, performance.now()));
    scheduleSweep();
  }
  function scheduleSweep() {
    clearTimeout(marksTimer);
    const next = ChronicleTimeline.newMarks.nextExpiry(state.marks);
    if (next !== null) marksTimer = setTimeout(sweepMarks, Math.max(0, next - performance.now()));
  }
  function sweepMarks() {
    const expired = new Set(ChronicleTimeline.newMarks.expire(state.marks, performance.now()));
    for (const card of els.entries.children) if (expired.has(card.dataset.id)) delete card.dataset.new;
    scheduleSweep();
  }
  document.addEventListener('visibilitychange', () => {
    if (document.visibilityState === 'visible') ChronicleTimeline.newMarks.show(state.marks, performance.now());
    else ChronicleTimeline.newMarks.hide(state.marks);
    scheduleSweep();
  });

  // showStreamedEntry puts an entry from the stream where its timestamp belongs, marks it NEW and
  // counts it. The list can hold the entry already: a list request that the server answered after
  // the entry was written has it too.
  function showStreamedEntry(entry) {
    if (ChronicleTimeline.contains(state.entries, entry.id)) return;
    const wasEmpty = !state.entries.length;
    const index = ChronicleTimeline.insertionIndex(state.entries, entry);
    state.entries.splice(index, 0, entry);
    markNew([entry.id]); // Before the card is built, which shows the mark.
    let card;
    if (wasEmpty) {
      renderEntries(); card = els.entries.firstElementChild; // The list showed a message, not cards.
    } else {
      card = buildCard(entry); els.entries.insertBefore(card, els.entries.children[index] || null);
    }
    // The total counts every entry, but the stream carries only the entries that match the filters. With
    // a filter on, the others would never reach the page, so only an unfiltered page grows the total
    // from the stream. applyFilters reloads the total whenever the filters change.
    if (state.total !== null && !filtersActive()) { state.total += 1; showTotal(); }
    showResultCount();
    // A card above the viewport pushes the page down, so scroll by its height to keep the reader's place.
    const { top, height } = card.getBoundingClientRect();
    if (top < 0) window.scrollBy(0, height);
  }

  // The stream delivers the entries written after it opened, filtered like the list. At most one
  // EventSource is open: connectStream closes the old one before it opens another.
  const STREAM_RETRY_MS = 3000;
  const LIVE_LABELS = { connecting: 'Connecting', live: 'Live', reconnecting: 'Reconnecting' };
  let stream = null;
  let streamRetry = 0;
  let interrupted = false;
  function showLiveStatus(name) {
    if (els.liveStatus.dataset.state === name) return; // A screen reader would read the same label again.
    els.liveStatus.dataset.state = name; els.liveStatus.textContent = LIVE_LABELS[name];
  }
  function connectStream() {
    clearTimeout(streamRetry);
    if (stream) stream.close();
    state.waiting = []; // They matched the old stream's filters, and the reload after ready covers them.
    const source = new EventSource(`/api/stream?${filterParams()}`);
    stream = source;
    source.addEventListener('ready', () => {
      showLiveStatus('live');
      // After a gap the page missed what was written meanwhile: the list marks it as NEW, and the counts reload.
      if (interrupted) { state.afterGap = true; loadStats(); }
      interrupted = false;
      // The server opens its feed before it sends ready, so this list plus the stream covers every entry.
      loadEntries();
    });
    source.addEventListener('entry', event => {
      const entry = JSON.parse(event.data);
      if (state.loading) state.waiting.push(entry); else showStreamedEntry(entry);
    });
    source.addEventListener('error', () => {
      interrupted = true; showLiveStatus('reconnecting');
      // The browser reconnects after a clean end or a lost connection. After any answer but 200 it gives up.
      if (source.readyState === EventSource.CLOSED) streamRetry = setTimeout(connectStream, STREAM_RETRY_MS);
    });
  }

  // applyFilters shows the first page for the filters as they are now and points the stream at them.
  // It reloads the stats too: the total follows the stream only while no filter is on, so after a
  // filtered stretch it is stale, and the tag counts have never followed the stream.
  function applyFilters() { state.limit = 50; connectStream(); loadEntries(); loadStats(); }
  els.search.addEventListener('input', debounce(applyFilters, 300));
  [els.since, els.until].forEach(input => input.addEventListener('change', applyFilters));
  els.clear.addEventListener('click', () => { els.search.value = ''; els.since.value = ''; els.until.value = ''; state.selectedTags.clear(); renderTagFilters(); applyFilters(); });
  els.tagFilter.addEventListener('click', event => {
    const button = event.target.closest('[data-tag]'); if (!button) return; const tag = button.dataset.tag;
    state.selectedTags.has(tag) ? state.selectedTags.delete(tag) : state.selectedTags.add(tag); renderTagFilters(); applyFilters();
  });
  els.more.addEventListener('click', () => { state.limit = Math.min(500, state.limit + 50); loadEntries(); });

  if (!readOnly) {
    els.form.addEventListener('submit', async event => {
      event.preventDefault(); const submit = els.form.querySelector('[type=submit]'); submit.disabled = true;
      try {
        await api('/api/entries', { method: 'POST', body: JSON.stringify({ message: els.message.value, tags: parseTags(els.tags.value) }) });
        els.form.reset(); toast('Entry recorded'); state.limit = 50; await Promise.all([loadEntries(), loadStats()]); els.message.focus();
      } catch (error) { toast(error.message, true); } finally { submit.disabled = false; }
    });
    els.message.addEventListener('keydown', event => { if ((event.metaKey || event.ctrlKey) && event.key === 'Enter') els.form.requestSubmit(); });

    els.entries.addEventListener('click', async event => {
      const button = event.target.closest('[data-action]'); if (!button) return;
      const entry = state.entries.find(item => item.id === button.closest('.entry-card').dataset.id); if (!entry) return;
      if (button.dataset.action === 'edit') {
        $('#edit-id').value = entry.id; $('#edit-message').value = entry.message; $('#edit-tags').value = (entry.tags || []).join(', '); els.dialog.showModal(); $('#edit-message').focus();
      } else if (button.dataset.action === 'delete' && confirm('Delete this entry? This cannot be undone.')) {
        button.disabled = true;
        try { await api(`/api/entries/${encodeURIComponent(entry.id)}`, { method: 'DELETE' }); toast('Entry deleted'); await Promise.all([loadEntries(), loadStats()]); }
        catch (error) { toast(error.message, true); button.disabled = false; }
      }
    });
    els.editForm.addEventListener('submit', async event => {
      event.preventDefault(); const id = $('#edit-id').value; const submit = els.editForm.querySelector('[type=submit]'); submit.disabled = true;
      try {
        await api(`/api/entries/${encodeURIComponent(id)}`, { method: 'PUT', body: JSON.stringify({ message: $('#edit-message').value, tags: parseTags($('#edit-tags').value) }) });
        els.dialog.close(); toast('Entry updated'); await Promise.all([loadEntries(), loadStats()]);
      } catch (error) { toast(error.message, true); } finally { submit.disabled = false; }
    });
    els.dialog.addEventListener('click', event => { if (event.target === els.dialog) els.dialog.close(); });
  }

  // theme.js applied the stored mode before the first paint, so data-theme on the html element says which mode is on.
  const root = document.documentElement;
  const themeButton = $('#theme-toggle');
  const themeIcons = { auto: '◐', light: '☼', dark: '☾' };
  function showTheme(mode) {
    const name = mode[0].toUpperCase() + mode.slice(1);
    themeButton.setAttribute('aria-label', `Theme: ${name}`); themeButton.title = `Theme: ${name}`;
    themeButton.querySelector('.theme-icon').textContent = themeIcons[mode]; themeButton.querySelector('.theme-label').textContent = name;
  }
  showTheme(ChronicleTheme.normalize(root.dataset.theme));
  themeButton.addEventListener('click', () => {
    const mode = ChronicleTheme.next(ChronicleTheme.normalize(root.dataset.theme));
    ChronicleTheme.apply(mode, root); showTheme(mode);
    try { localStorage.setItem('chronicle-theme', mode); } catch { /* Without storage, the choice lasts until the page closes. */ }
  });

  Promise.all([loadEntries(), loadStats()]);
  connectStream();
})();
