/* my-gather report theme module.
 *
 * Single canonical owner of report theming behaviour:
 *   - reads the user's theme choice from localStorage,
 *   - validates it against the closed set {dark, light, colorblind},
 *   - applies it by setting `data-theme` on <html>,
 *   - persists user changes back to localStorage,
 *   - dispatches a `mygather:theme` CustomEvent on `document` so chart
 *     code in app-js/00.js can re-resolve series / axis / grid colors
 *     for currently-rendered charts (uPlot caches stroke values on
 *     each series at construction; a bare resize would re-paint with
 *     stale colors).
 *
 * The pre-paint <head> script in render/templates/report.html.tmpl
 * applies the saved theme before CSS parses, eliminating the
 * dark-flash-then-saved-theme race. This module then handles every
 * runtime change (the <select> in header.app-header).
 *
 * No fallback paths; no parallel theme source. The
 * @media (prefers-color-scheme: light) block in app-css/00.css and
 * the SERIES_COLORS array in app-js/00.js were deleted in the same
 * change that introduced this module (Principle XIII).
 */
(function () {
  "use strict";

  var KNOWN = ["dark", "light", "colorblind"];
  var STORAGE_KEY = "my-gather:theme";
  var DEFAULT_THEME = "dark";

  function isKnown(name) {
    for (var i = 0; i < KNOWN.length; i++) {
      if (KNOWN[i] === name) return true;
    }
    return false;
  }

  function readStored() {
    try {
      var v = window.localStorage.getItem(STORAGE_KEY);
      if (isKnown(v)) return v;
    } catch (_) {
      // localStorage may throw in private browsing or when disabled.
      // Silent fallback — the visible degradation is "your choice
      // does not persist", which the user can see by reloading.
    }
    return DEFAULT_THEME;
  }

  function writeStored(name) {
    try {
      window.localStorage.setItem(STORAGE_KEY, name);
    } catch (_) {
      // Same swallow as readStored; persistence is best-effort.
    }
  }

  // applyTheme is the only function that mutates the active theme.
  // It validates, sets the document attribute, persists, and notifies
  // listeners (chart code) so they can re-pick token-driven colors.
  function applyTheme(name) {
    var applied = isKnown(name) ? name : DEFAULT_THEME;
    document.documentElement.dataset.theme = applied;
    writeStored(applied);
    try {
      document.dispatchEvent(new CustomEvent("mygather:theme", {
        detail: { theme: applied },
      }));
    } catch (_) {
      // Old browsers without CustomEvent constructor: chart re-pick
      // does not fire, but the new CSS variables still drive any
      // future render. Acceptable degradation.
    }
    return applied;
  }

  function syncSelectToActive() {
    var sel = document.getElementById("theme-picker");
    if (!sel) return;
    var active = document.documentElement.dataset.theme;
    if (!isKnown(active)) active = DEFAULT_THEME;
    sel.value = active;
  }

  function wirePicker() {
    var sel = document.getElementById("theme-picker");
    if (!sel) return;
    sel.addEventListener("change", function () {
      applyTheme(sel.value);
    });
  }

  function init() {
    // The pre-paint <head> script already applied data-theme before
    // CSS parsing. Make sure the <select> reflects that, then wire
    // the change handler.
    if (!isKnown(document.documentElement.dataset.theme)) {
      document.documentElement.dataset.theme = DEFAULT_THEME;
    }
    syncSelectToActive();
    wirePicker();
  }

  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", init);
  } else {
    init();
  }
})();
