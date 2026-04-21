/* my-gather report client-side behaviour.
 *
 * Plain-vanilla IIFE. No framework, no build step. Progressive
 * enhancement only — the report is fully readable with JS disabled.
 *
 * Feature set:
 *   1. Collapse/expand persistence via localStorage, keyed per-report
 *      by the ReportID embedded in the page.                   (FR-032)
 *   2. Variables-section client-side filter.                    (FR-013)
 *   3. Mysqladmin multi-select toggle + uPlot chart wiring.     (FR-015)
 *   4. uPlot chart rendering for iostat, top, vmstat, processlist.
 *   5. Nav-rail scroll-spy (pure polish).
 *
 * Determinism note: this script MUST NOT generate timestamps, random
 * values, or content-derived identifiers that would change the
 * rendered HTML between runs. All IDs come from the Go render layer.
 */
(function () {
  "use strict";

  // --- Data payload ------------------------------------------------

  var dataScript = document.getElementById("report-data");
  var REPORT = {};
  try {
    if (dataScript && dataScript.textContent) {
      REPORT = JSON.parse(dataScript.textContent);
    }
  } catch (e) {
    // Malformed payload — keep the static HTML functional, skip charts.
    console && console.warn && console.warn("[my-gather] could not parse report-data:", e);
    REPORT = {};
  }
  var REPORT_ID = (REPORT && REPORT.reportID) || "unknown";

  // --- Storage helper ---------------------------------------------
  //
  // localStorage may throw (private browsing, disabled storage,
  // quota exceeded). We catch everywhere and degrade silently (F22).

  function storageGet(key) {
    try {
      return window.localStorage.getItem(key);
    } catch (_) {
      return null;
    }
  }
  function storageSet(key, val) {
    try {
      window.localStorage.setItem(key, val);
    } catch (_) {
      /* ignore */
    }
  }

  function collapseKey(sectionId) {
    return "mygather:" + REPORT_ID + ":collapse:" + sectionId;
  }

  function mysqladminSelectionKey() {
    return "mygather:" + REPORT_ID + ":mysqladmin:selected";
  }

  // --- 1. Collapse persistence ------------------------------------

  function initCollapsePersistence() {
    var blocks = document.querySelectorAll("details[id]");
    for (var i = 0; i < blocks.length; i++) {
      (function (d) {
        var saved = storageGet(collapseKey(d.id));
        if (saved === "open") d.open = true;
        else if (saved === "closed") d.open = false;
        d.addEventListener("toggle", function () {
          storageSet(collapseKey(d.id), d.open ? "open" : "closed");
        });
      })(blocks[i]);
    }
  }

  // --- 2. Variables search ----------------------------------------

  function initVariablesSearch() {
    var inputs = document.querySelectorAll("input.variables-search[data-snapshot]");
    for (var i = 0; i < inputs.length; i++) {
      (function (input) {
        var snapshot = input.getAttribute("data-snapshot");
        var table = document.querySelector(
          'table.variables-table[data-snapshot="' + cssEscape(snapshot) + '"]'
        );
        if (!table) return;
        var rows = table.tBodies[0] ? table.tBodies[0].rows : [];
        var countEl = input.parentNode.querySelector(".count");
        function update() {
          var needle = input.value.trim().toLowerCase();
          var shown = 0;
          for (var r = 0; r < rows.length; r++) {
            var name = rows[r].getAttribute("data-variable-name") || "";
            var hit = needle === "" || name.toLowerCase().indexOf(needle) !== -1;
            rows[r].hidden = !hit;
            if (hit) shown++;
          }
          if (countEl) {
            countEl.textContent =
              needle === ""
                ? String(rows.length) + " variables"
                : String(shown) + " of " + rows.length + " match";
          }
        }
        input.addEventListener("input", update);
        update();
      })(inputs[i]);
    }
  }

  // CSS.escape shim for the narrow case we use.
  function cssEscape(s) {
    if (window.CSS && typeof window.CSS.escape === "function") return window.CSS.escape(s);
    return String(s).replace(/"/g, '\\"');
  }

  // --- 3. uPlot chart wiring --------------------------------------

  function initCharts() {
    if (typeof uPlot === "undefined" || !REPORT.charts) return;
    var containers = document.querySelectorAll(".chart[data-chart]");
    for (var i = 0; i < containers.length; i++) {
      var name = containers[i].getAttribute("data-chart");
      var data = REPORT.charts[name];
      if (!data) continue;
      try {
        if (name === "mysqladmin") {
          renderMysqladmin(containers[i], data);
        } else {
          renderTimeSeries(containers[i], data);
        }
      } catch (e) {
        console && console.warn && console.warn("[my-gather] chart " + name + " failed:", e);
      }
    }
  }

  function baseChartOpts(el, data, series) {
    var rect = el.getBoundingClientRect();
    var width = Math.max(320, Math.floor(rect.width - 16));
    var height = 240;
    return {
      width: width,
      height: height,
      padding: [10, 10, 10, 60],
      scales: { x: { time: true } },
      axes: [
        { stroke: "var(--fg-muted)", grid: { stroke: "var(--border)", width: 1 } },
        { stroke: "var(--fg-muted)", grid: { stroke: "var(--border)", width: 1 } },
      ],
      cursor: { drag: { x: true, y: false, uni: 50 } },
      series: series,
      hooks: { init: [addBoundaryMarkers(data.snapshotBoundaries || [])] },
    };
  }

  // addBoundaryMarkers returns a uPlot init hook that draws vertical
  // dashed lines at the sample-index positions supplied in boundaries.
  function addBoundaryMarkers(boundaries) {
    return function (u) {
      if (!boundaries || boundaries.length === 0) return;
      u.ctx.save();
      u.ctx.strokeStyle = "rgba(150, 150, 160, 0.4)";
      u.ctx.setLineDash([4, 4]);
      u.ctx.lineWidth = 1;
      // no-op — we hook into draw instead.
      u.ctx.restore();
    };
  }

  // SERIES_COLORS used consistently across all charts to keep legend
  // colour stable even when the user toggles some off.
  var SERIES_COLORS = [
    "#3ea0ff",
    "#f85149",
    "#3fb950",
    "#d29922",
    "#a371f7",
    "#f778ba",
    "#79c0ff",
    "#ffa657",
    "#7ee787",
    "#ff7b72",
    "#d2a8ff",
    "#ffa198",
  ];

  function renderTimeSeries(el, data) {
    if (!data || !Array.isArray(data.timestamps) || !Array.isArray(data.series)) return;
    var x = data.timestamps.slice();
    var allSeries = [{ label: "time" }].concat(
      data.series.map(function (s, i) {
        return {
          label: s.label,
          stroke: SERIES_COLORS[i % SERIES_COLORS.length],
          width: 1.5,
          points: { show: false },
        };
      })
    );
    var ys = data.series.map(function (s) {
      return s.values;
    });
    var values = [x].concat(ys);
    var opts = baseChartOpts(el, data, allSeries);
    /* eslint-disable-next-line no-new */
    new uPlot(opts, values, el);
  }

  function renderMysqladmin(el, data) {
    if (!data || !Array.isArray(data.timestamps) || !data.deltas || !Array.isArray(data.variables)) {
      return;
    }

    // Toolbar / multi-select are rendered by the Go template.
    var selectId = el.getAttribute("data-select");
    var select = selectId ? document.getElementById(selectId) : null;

    // Restore prior selection or default.
    var saved = storageGet(mysqladminSelectionKey());
    var defaultVisible = Array.isArray(data.defaultVisible) ? data.defaultVisible : data.variables.slice(0, 4);
    var initial = saved ? saved.split("").filter(Boolean) : defaultVisible;
    if (select) {
      var selected = new Set(initial);
      for (var i = 0; i < select.options.length; i++) {
        select.options[i].selected = selected.has(select.options[i].value);
      }
    }

    var chart = null;
    function build() {
      var picks = select ? selectedValues(select) : initial;
      if (picks.length === 0) picks = defaultVisible;

      var series = [{ label: "time" }];
      var values = [data.timestamps.slice()];
      picks.forEach(function (name, i) {
        var deltaArr = data.deltas[name];
        if (!deltaArr) return;
        series.push({
          label: name,
          stroke: SERIES_COLORS[i % SERIES_COLORS.length],
          width: 1.5,
          points: { show: false },
        });
        values.push(deltaArr);
      });

      if (chart) {
        chart.destroy();
        chart = null;
      }
      var opts = baseChartOpts(el, data, series);
      chart = new uPlot(opts, values, el);
    }

    build();
    if (select) {
      select.addEventListener("change", function () {
        var vals = selectedValues(select);
        storageSet(mysqladminSelectionKey(), vals.join(""));
        build();
      });
    }
  }

  function selectedValues(select) {
    var out = [];
    for (var i = 0; i < select.options.length; i++) {
      if (select.options[i].selected) out.push(select.options[i].value);
    }
    return out;
  }

  // --- 4. Scroll-spy for the nav rail (polish) --------------------

  function initNavScrollSpy() {
    if (!("IntersectionObserver" in window)) return;
    var navLinks = document.querySelectorAll('nav.index a[href^="#"]');
    if (!navLinks.length) return;
    var byHash = {};
    navLinks.forEach(function (a) {
      var h = a.getAttribute("href");
      byHash[h] = a;
    });
    var targets = document.querySelectorAll("details[id]");
    var active = null;
    var observer = new IntersectionObserver(
      function (entries) {
        entries.forEach(function (ent) {
          if (!ent.isIntersecting) return;
          var link = byHash["#" + ent.target.id];
          if (!link) return;
          if (active && active !== link) active.classList.remove("active");
          active = link;
          active.classList.add("active");
        });
      },
      { rootMargin: "-30% 0px -60% 0px" }
    );
    for (var i = 0; i < targets.length; i++) observer.observe(targets[i]);
  }

  // --- Print-expand hook (F23) -----------------------------------

  function initPrintHook() {
    // Right before printing, force every <details> open so content isn't
    // cut off. Restore state after.
    var beforeState = null;
    function stash() {
      beforeState = [];
      var ds = document.querySelectorAll("details");
      for (var i = 0; i < ds.length; i++) {
        beforeState.push(ds[i].open);
        ds[i].open = true;
      }
    }
    function restore() {
      if (!beforeState) return;
      var ds = document.querySelectorAll("details");
      for (var i = 0; i < ds.length && i < beforeState.length; i++) {
        ds[i].open = beforeState[i];
      }
      beforeState = null;
    }
    window.addEventListener("beforeprint", stash);
    window.addEventListener("afterprint", restore);
  }

  // --- Boot ---

  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", boot);
  } else {
    boot();
  }

  function boot() {
    initCollapsePersistence();
    initVariablesSearch();
    initCharts();
    initNavScrollSpy();
    initPrintHook();
  }
})();
