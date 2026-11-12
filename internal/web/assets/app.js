// ABOUTME: Browser client for Chronicle's web interface: it fills the page from the API and sends changes back.
// ABOUTME: Its theme button cycles auto, light and dark through the ChronicleTheme module that theme.js defines.
(() => {
  'use strict';
  const $ = (selector) => document.querySelector(selector);
  const state = { entries: [], tags: [], limit: 50, selectedTags: new Set(), loading: false };
  const els = {
    form: $('#entry-form'), message: $('#message'), tags: $('#tags'), entries: $('#entries'), search: $('#search'),
    since: $('#since'), until: $('#until'), count: $('#result-count'), total: $('#total-count'), tagFilter: $('#tag-filter'),
    clear: $('#clear-filters'), more: $('#load-more'), dialog: $('#edit-dialog'), editForm: $('#edit-form')
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

  async function loadEntries() {
    if (state.loading) return;
    state.loading = true; els.entries.setAttribute('aria-busy', 'true');
    if (!state.entries.length) els.entries.innerHTML = '<div class="skeleton"></div><div class="skeleton"></div><div class="skeleton"></div>';
    const params = new URLSearchParams({ limit: String(state.limit) });
    if (els.search.value.trim()) params.set('q', els.search.value.trim());
    if (els.since.value) params.set('since', els.since.value);
    if (els.until.value) params.set('until', els.until.value);
    state.selectedTags.forEach(tag => params.append('tag', tag));
    try {
      const data = await api(`/api/entries?${params}`); state.entries = data.entries; renderEntries();
      els.count.textContent = `${data.count}${data.count === state.limit ? '+' : ''} result${data.count === 1 ? '' : 's'}`;
      els.more.hidden = data.count < state.limit || state.limit >= 500;
    } catch (error) {
      els.entries.innerHTML = `<div class="empty"><span>!</span><h3>Couldn't load entries</h3><p>${escapeText(error.message)}</p></div>`;
      toast(error.message, true);
    } finally { state.loading = false; els.entries.setAttribute('aria-busy', 'false'); }
  }

  function renderEntries() {
    els.entries.replaceChildren();
    if (!state.entries.length) {
      const filtered = els.search.value || els.since.value || els.until.value || state.selectedTags.size;
      els.entries.innerHTML = `<div class="empty"><span>✦</span><h3>${filtered ? 'No matching moments' : 'Your chronicle starts here'}</h3><p>${filtered ? 'Try broadening your filters.' : 'Record your first entry above.'}</p></div>`;
      return;
    }
    const template = $('#entry-template');
    state.entries.forEach(entry => {
      const fragment = template.content.cloneNode(true); const card = fragment.querySelector('.entry-card'); card.dataset.id = entry.id;
      const date = new Date(entry.timestamp); const time = fragment.querySelector('time');
      time.dateTime = entry.timestamp; time.textContent = date.toLocaleString(undefined, { weekday: 'short', month: 'short', day: 'numeric', hour: 'numeric', minute: '2-digit' });
      fragment.querySelector('.entry-message').textContent = entry.message;
      const tags = fragment.querySelector('.entry-tags'); (entry.tags || []).forEach(tag => { const chip = document.createElement('span'); chip.textContent = `#${tag}`; tags.append(chip); });
      const context = [entry.username && entry.hostname ? `${entry.username}@${entry.hostname}` : entry.username || entry.hostname, entry.working_directory].filter(Boolean);
      fragment.querySelector('.entry-context').textContent = context.join(' · ');
      els.entries.append(fragment);
    });
  }

  async function loadStats() {
    try {
      const data = await api('/api/stats'); els.total.textContent = data.total.toLocaleString(); state.tags = data.tags; renderTagFilters();
    } catch { els.total.textContent = '—'; }
  }
  function renderTagFilters() {
    els.tagFilter.replaceChildren();
    if (!state.tags.length) { els.tagFilter.innerHTML = '<span class="muted">No tags yet</span>'; return; }
    state.tags.slice(0, 18).forEach(tag => {
      const button = document.createElement('button'); button.type = 'button'; button.className = `tag-filter${state.selectedTags.has(tag.name) ? ' active' : ''}`;
      button.textContent = `#${tag.name} · ${tag.count}`; button.dataset.tag = tag.name; button.setAttribute('aria-pressed', state.selectedTags.has(tag.name)); els.tagFilter.append(button);
    });
  }

  els.form.addEventListener('submit', async event => {
    event.preventDefault(); const submit = els.form.querySelector('[type=submit]'); submit.disabled = true;
    try {
      await api('/api/entries', { method: 'POST', body: JSON.stringify({ message: els.message.value, tags: parseTags(els.tags.value) }) });
      els.form.reset(); toast('Entry recorded'); state.limit = 50; await Promise.all([loadEntries(), loadStats()]); els.message.focus();
    } catch (error) { toast(error.message, true); } finally { submit.disabled = false; }
  });
  els.message.addEventListener('keydown', event => { if ((event.metaKey || event.ctrlKey) && event.key === 'Enter') els.form.requestSubmit(); });
  els.search.addEventListener('input', debounce(() => { state.limit = 50; loadEntries(); }, 300));
  [els.since, els.until].forEach(input => input.addEventListener('change', () => { state.limit = 50; loadEntries(); }));
  els.clear.addEventListener('click', () => { els.search.value = ''; els.since.value = ''; els.until.value = ''; state.selectedTags.clear(); state.limit = 50; renderTagFilters(); loadEntries(); });
  els.tagFilter.addEventListener('click', event => {
    const button = event.target.closest('[data-tag]'); if (!button) return; const tag = button.dataset.tag;
    state.selectedTags.has(tag) ? state.selectedTags.delete(tag) : state.selectedTags.add(tag); state.limit = 50; renderTagFilters(); loadEntries();
  });
  els.more.addEventListener('click', () => { state.limit = Math.min(500, state.limit + 50); loadEntries(); });

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
})();
