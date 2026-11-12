// ABOUTME: Theme modes for the web page: auto follows the system setting, light and dark override it.
// ABOUTME: Loads in the page head before the stylesheet, so a stored choice applies before the first paint.
(() => {
  'use strict';

  const STORAGE_KEY = 'chronicle-theme';
  const MODES = ['auto', 'light', 'dark'];

  // normalize returns the mode that a stored value names. A missing or unknown value means auto.
  function normalize(stored) {
    return MODES.includes(stored) ? stored : 'auto';
  }

  // next returns the mode after mode in the cycle the theme button follows.
  function next(mode) {
    return MODES[(MODES.indexOf(normalize(mode)) + 1) % MODES.length];
  }

  // apply marks root with mode as data-theme. Auto takes the mark off, so that the stylesheet
  // follows the system setting.
  function apply(mode, root) {
    const applied = normalize(mode);
    if (applied === 'auto') delete root.dataset.theme;
    else root.dataset.theme = applied;
  }

  globalThis.ChronicleTheme = { normalize, next, apply };

  if (typeof document !== 'undefined') {
    try {
      apply(normalize(localStorage.getItem(STORAGE_KEY)), document.documentElement);
    } catch {
      // Storage can be blocked. The page then follows the system setting.
    }
  }
})();
