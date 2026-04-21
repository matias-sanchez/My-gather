/* my-gather report client-side behaviour.
 *
 * Plain-vanilla IIFE. No framework, no build step. Progressive
 * enhancement only — the report is fully readable with JS disabled.
 *
 * Feature set:
 *   1. Collapse/expand persistence via localStorage, keyed per-report
 *      by the ReportID embedded in the page.                   (FR-032)
 *   2. Variables-section client-side filter.                    (FR-013)
 *   3. Mysqladmin filter panel: search + category chips + scrollable
 *      checkbox list + chart.                                   (FR-015)
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
    console && console.warn && console.warn("[my-gather] could not parse report-data:", e);
    REPORT = {};
  }
  var REPORT_ID = (REPORT && REPORT.reportID) || "unknown";

  // --- Storage helper ---------------------------------------------

  function storageGet(key) {
    try { return window.localStorage.getItem(key); } catch (_) { return null; }
  }
  function storageSet(key, val) {
    try { window.localStorage.setItem(key, val); } catch (_) { /* ignore */ }
  }
  function collapseKey(sectionId) {
    return "mygather:" + REPORT_ID + ":collapse:" + sectionId;
  }
  function mysqladminSelectionKey() {
    return "mygather:" + REPORT_ID + ":mysqladmin:selected";
  }

  // --- 1. Collapse persistence ------------------------------------

  function initCollapsePersistence() {
    // Scope strictly to main-content details; nav groups have their
    // own namespace under initNavGroups().
    var blocks = document.querySelectorAll("main.content details[id]");
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
                ? rows.length + " variables"
                : shown + " of " + rows.length + " match";
          }
        }
        input.addEventListener("input", update);
        update();
      })(inputs[i]);
    }
  }

  function cssEscape(s) {
    if (window.CSS && typeof window.CSS.escape === "function") return window.CSS.escape(s);
    return String(s).replace(/"/g, '\\"');
  }

  // --- 3. Chart palette + helpers --------------------------------

  var SERIES_COLORS = [
    "#3ea0ff", "#f85149", "#3fb950", "#d29922", "#a371f7",
    "#f778ba", "#79c0ff", "#ffa657", "#7ee787", "#ff7b72",
    "#d2a8ff", "#ffa198", "#56d4dd", "#ff9e64", "#b392f0",
    "#7dcfff",
  ];

  function cssVar(name, fallback) {
    try {
      var v = getComputedStyle(document.documentElement).getPropertyValue(name).trim();
      return v || fallback;
    } catch (_) { return fallback; }
  }

  function basePlotOpts(width, height, labelSeries, unit) {
    return {
      width: width,
      height: height,
      padding: [12, 12, 12, 56],
      scales: { x: { time: true }, y: { auto: true } },
      axes: [
        {
          stroke: cssVar("--fg-muted", "#9aa5b4"),
          grid:  { stroke: cssVar("--border", "#262d38"), width: 1 },
          ticks: { stroke: cssVar("--fg-dim", "#6e7a8a"), width: 1, size: 6 },
          font:  '11px ui-monospace, Menlo, monospace',
          space: 80,
        },
        {
          stroke: cssVar("--fg-muted", "#9aa5b4"),
          grid:  { stroke: cssVar("--border", "#262d38"), width: 1 },
          ticks: { stroke: cssVar("--fg-dim", "#6e7a8a"), width: 1, size: 6 },
          font:  '11px ui-monospace, Menlo, monospace',
          size:  56,
          label: unit ? unit : undefined,
          labelFont: '11px ui-monospace, Menlo, monospace',
        },
      ],
      cursor: { drag: { x: true, y: false, uni: 50 } },
      legend: { show: false },
      series: labelSeries,
    };
  }

  function measureChartWidth(el) {
    var rect = el.getBoundingClientRect();
    return Math.max(320, Math.floor(rect.width - 2));
  }

  // Build a clickable legend rendered OUTSIDE uPlot (uPlot's default
  // legend is too verbose for 16-series charts).
  function mountLegend(containerEl, series, plot) {
    var legend = document.createElement("div");
    legend.className = "series-legend";
    series.forEach(function (s, i) {
      if (i === 0) return; // time
      var btn = document.createElement("button");
      btn.type = "button";
      btn.className = "series-pill active";
      btn.setAttribute("data-idx", String(i));
      btn.innerHTML =
        '<span class="swatch" style="background:' + s.stroke + '"></span>' +
        '<span class="lbl">' + escapeHTML(s.label) + '</span>';
      btn.addEventListener("click", function () {
        var idx = Number(btn.getAttribute("data-idx"));
        var showing = plot.series[idx].show;
        plot.setSeries(idx, { show: !showing });
        btn.classList.toggle("active", !showing);
      });
      legend.appendChild(btn);
    });
    containerEl.parentNode.insertBefore(legend, containerEl.nextSibling);
  }

  function escapeHTML(s) {
    return String(s)
      .replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;")
      .replace(/"/g, "&quot;").replace(/'/g, "&#39;");
  }

  // --- 4. Chart rendering ----------------------------------------

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
        } else if (name === "iostat") {
          renderIostat(containers[i], data);
        } else {
          renderTimeSeries(containers[i], data, unitForChart(name));
        }
      } catch (e) {
        console && console.warn && console.warn("[my-gather] chart " + name + " failed:", e);
      }
    }
  }

  function unitForChart(name) {
    if (name === "top")         return "%CPU";
    if (name === "processlist") return "threads";
    if (name === "vmstat")      return "mixed";
    return "";
  }

  // renderTimeSeries: generic multi-line chart with the pill legend.
  function renderTimeSeries(el, data, unit) {
    if (!data || !Array.isArray(data.timestamps) || !Array.isArray(data.series)) return;
    var width = measureChartWidth(el);
    var series = [{ label: "time" }].concat(
      data.series.map(function (s, i) {
        return {
          label: s.label,
          stroke: SERIES_COLORS[i % SERIES_COLORS.length],
          width: 1.5,
          points: { show: false },
          value: (u, v) => v == null ? "–" : v.toLocaleString(),
        };
      })
    );
    var values = [data.timestamps.slice()].concat(data.series.map(function (s) { return s.values; }));
    var opts = basePlotOpts(width, 260, series, unit);
    var plot = new uPlot(opts, values, el);
    mountLegend(el, series, plot);
  }

  // renderIostat: split into two charts sharing the same pill legend —
  // top chart is %util per device, bottom is aqu-sz per device. Only
  // the %util chart is visible by default; an adjacent control toggles
  // between the two views.
  function renderIostat(el, data) {
    if (!data || !Array.isArray(data.timestamps) || !Array.isArray(data.series)) return;

    // Partition series by suffix (" %util" vs " aqu-sz").
    var utilSeries = [], utilLabels = [];
    var aquSeries  = [], aquLabels  = [];
    data.series.forEach(function (s) {
      if (/ %util$/.test(s.label)) {
        utilSeries.push(s.values);
        utilLabels.push(s.label.replace(/ %util$/, ""));
      } else if (/ aqu-sz$/.test(s.label)) {
        aquSeries.push(s.values);
        aquLabels.push(s.label.replace(/ aqu-sz$/, ""));
      }
    });

    // Wrap el with a toolbar + chart body.
    var wrap = document.createElement("div");
    wrap.className = "iostat-wrap";
    var toolbar = document.createElement("div");
    toolbar.className = "chart-view-toolbar";
    var btnUtil = document.createElement("button");
    btnUtil.type = "button";
    btnUtil.className = "view-btn active";
    btnUtil.textContent = "Utilization (%)";
    var btnAqu  = document.createElement("button");
    btnAqu.type = "button";
    btnAqu.className = "view-btn";
    btnAqu.textContent = "Avg queue size";
    toolbar.appendChild(btnUtil);
    toolbar.appendChild(btnAqu);
    el.parentNode.insertBefore(toolbar, el);

    var width = measureChartWidth(el);
    var currentView = "util";
    var plot = null, legendEl = null;

    function draw() {
      if (plot) { plot.destroy(); plot = null; }
      if (legendEl && legendEl.parentNode) { legendEl.parentNode.removeChild(legendEl); legendEl = null; }
      var labels = currentView === "util" ? utilLabels : aquLabels;
      var rows   = currentView === "util" ? utilSeries : aquSeries;
      var unit   = currentView === "util" ? "% util"   : "aqu-sz";
      var series = [{ label: "time" }].concat(labels.map(function (lbl, i) {
        return {
          label: lbl,
          stroke: SERIES_COLORS[i % SERIES_COLORS.length],
          width: 1.5,
          points: { show: false },
          value: (u, v) => v == null ? "–" : v.toLocaleString(),
        };
      }));
      var values = [data.timestamps.slice()].concat(rows);
      var opts = basePlotOpts(width, 260, series, unit);
      plot = new uPlot(opts, values, el);
      mountLegend(el, series, plot);
      legendEl = el.nextSibling;
    }
    draw();
    btnUtil.addEventListener("click", function () {
      if (currentView === "util") return;
      currentView = "util";
      btnUtil.classList.add("active"); btnAqu.classList.remove("active");
      draw();
    });
    btnAqu.addEventListener("click", function () {
      if (currentView === "aqu") return;
      currentView = "aqu";
      btnAqu.classList.add("active"); btnUtil.classList.remove("active");
      draw();
    });
  }

  // --- Mysqladmin filter panel ----------------------------------

  function renderMysqladmin(el, data) {
    if (!data || !Array.isArray(data.timestamps) || !data.deltas || !Array.isArray(data.variables)) {
      return;
    }

    var host = el.closest("[data-mysqladmin-host]") || el.parentNode;

    // Build the filter panel.
    var panel = document.createElement("div");
    panel.className = "ma-panel";

    var chipsRow = document.createElement("div");
    chipsRow.className = "ma-chips";
    var chipDefs = [
      { k: "all",      label: "All" },
      { k: "counters", label: "Counters", filter: function (name) { return data.isCounter[name]; } },
      { k: "gauges",   label: "Gauges",   filter: function (name) { return !data.isCounter[name]; } },
      { k: "com",      label: "Com_*",    filter: function (name) { return /^Com_/.test(name); } },
      { k: "innodb",   label: "Innodb_*", filter: function (name) { return /^Innodb_/.test(name); } },
      { k: "handler",  label: "Handler_*",filter: function (name) { return /^Handler_/.test(name); } },
      { k: "bytes",    label: "Bytes_*",  filter: function (name) { return /^Bytes_/.test(name); } },
      { k: "selected", label: "Selected", filter: null /* computed at runtime */ },
    ];
    var chipButtons = {};
    chipDefs.forEach(function (def) {
      var b = document.createElement("button");
      b.type = "button";
      b.className = "ma-chip";
      b.setAttribute("data-k", def.k);
      b.textContent = def.label;
      if (def.k === "all") b.classList.add("active");
      b.addEventListener("click", function () {
        Object.keys(chipButtons).forEach(function (k) { chipButtons[k].classList.remove("active"); });
        b.classList.add("active");
        state.category = def.k;
        redrawList();
      });
      chipsRow.appendChild(b);
      chipButtons[def.k] = b;
    });
    panel.appendChild(chipsRow);

    // Search row.
    var searchRow = document.createElement("div");
    searchRow.className = "ma-search";
    var search = document.createElement("input");
    search.type = "search";
    search.placeholder = "Filter counters by name…";
    search.autocomplete = "off";
    search.spellcheck = false;
    var selectedCount = document.createElement("span");
    selectedCount.className = "ma-count";
    var actions = document.createElement("div");
    actions.className = "ma-actions";
    var btnSelectVisible = document.createElement("button");
    btnSelectVisible.type = "button";
    btnSelectVisible.className = "ma-action";
    btnSelectVisible.textContent = "Select visible";
    var btnClear = document.createElement("button");
    btnClear.type = "button";
    btnClear.className = "ma-action";
    btnClear.textContent = "Clear";
    actions.appendChild(btnSelectVisible);
    actions.appendChild(btnClear);
    searchRow.appendChild(search);
    searchRow.appendChild(selectedCount);
    searchRow.appendChild(actions);
    panel.appendChild(searchRow);

    // Scrollable list.
    var list = document.createElement("div");
    list.className = "ma-list";
    panel.appendChild(list);

    // Insert panel BEFORE the chart container.
    el.parentNode.insertBefore(panel, el);

    // State.
    var saved = storageGet(mysqladminSelectionKey());
    var defaults = Array.isArray(data.defaultVisible) ? data.defaultVisible : data.variables.slice(0, 5);
    var initial = saved ? saved.split("\n").filter(Boolean) : defaults;
    var selected = new Set(initial.filter(function (n) { return data.variables.indexOf(n) >= 0; }));
    if (selected.size === 0) defaults.forEach(function (n) { selected.add(n); });

    var state = { category: "all", needle: "" };
    var _throttle = null;

    function categoryFilter(name) {
      var def = null;
      for (var i = 0; i < chipDefs.length; i++) if (chipDefs[i].k === state.category) { def = chipDefs[i]; break; }
      if (!def) return true;
      if (def.k === "selected") return selected.has(name);
      if (!def.filter) return true;
      return def.filter(name);
    }

    function redrawList() {
      var needle = state.needle;
      var rows = [];
      data.variables.forEach(function (name) {
        if (!categoryFilter(name)) return;
        if (needle && name.toLowerCase().indexOf(needle) === -1) return;
        rows.push(name);
      });
      // Always alphabetical so the grid column layout looks tidy.
      rows.sort(function (a, b) { return a.toLowerCase() < b.toLowerCase() ? -1 : (a.toLowerCase() > b.toLowerCase() ? 1 : 0); });

      // Render as plain innerHTML for speed; 541 items is fine.
      var html = rows.map(function (name) {
        var checked = selected.has(name) ? ' checked' : '';
        var cls = data.isCounter[name] ? 'counter' : 'gauge';
        return '<label class="ma-row ' + cls + '">' +
               '<input type="checkbox" value="' + escapeHTML(name) + '"' + checked + '>' +
               '<span class="tag">' + (data.isCounter[name] ? 'C' : 'G') + '</span>' +
               '<span class="name">' + escapeHTML(name) + '</span>' +
               '</label>';
      }).join("");
      if (rows.length === 0) {
        html = '<p class="ma-empty">No variables match the current filter.</p>';
      }
      list.innerHTML = html;
      selectedCount.textContent = selected.size + ' selected · ' + rows.length + ' shown / ' + data.variables.length;
    }

    list.addEventListener("change", function (e) {
      if (!e.target || e.target.tagName !== "INPUT") return;
      var name = e.target.value;
      if (e.target.checked) selected.add(name);
      else selected.delete(name);
      persistSelection();
      selectedCount.textContent = selected.size + ' selected · ' + list.children.length + ' shown / ' + data.variables.length;
      scheduleRedraw();
    });
    btnSelectVisible.addEventListener("click", function () {
      var boxes = list.querySelectorAll("input[type=checkbox]");
      for (var i = 0; i < boxes.length; i++) { selected.add(boxes[i].value); boxes[i].checked = true; }
      persistSelection();
      selectedCount.textContent = selected.size + ' selected · ' + boxes.length + ' shown / ' + data.variables.length;
      scheduleRedraw();
    });
    btnClear.addEventListener("click", function () {
      selected.clear();
      persistSelection();
      redrawList();
      scheduleRedraw();
    });
    search.addEventListener("input", function () {
      state.needle = search.value.trim().toLowerCase();
      redrawList();
    });

    function persistSelection() {
      storageSet(mysqladminSelectionKey(), Array.from(selected).join("\n"));
    }

    // Chart.
    var chart = null;
    function drawChart() {
      if (chart) { chart.destroy(); chart = null; }
      var picks = Array.from(selected);
      if (picks.length === 0) picks = defaults.slice();
      picks.sort();
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
          value: (u, v) => v == null ? "–" : v.toLocaleString(),
        });
        values.push(deltaArr);
      });
      var width = measureChartWidth(el);
      var opts = basePlotOpts(width, 300, series, "counter delta / sample");
      chart = new uPlot(opts, values, el);
      // legend pills under the chart (replacing prior ones).
      var prev = el.nextSibling;
      if (prev && prev.classList && prev.classList.contains("series-legend")) prev.remove();
      mountLegend(el, series, chart);
    }
    function scheduleRedraw() {
      if (_throttle) return;
      _throttle = setTimeout(function () { _throttle = null; drawChart(); }, 120);
    }
    redrawList();
    drawChart();
  }

  // --- 5. Nav-rail collapse + scroll-spy --------------------------

  function initNavGroups() {
    // The nav groups use a separate localStorage namespace so their
    // collapse state doesn't collide with the main-content section
    // collapse keys (which use ID "sec-os", "sec-db", ...).
    var groups = document.querySelectorAll("nav.index details.nav-group");
    for (var i = 0; i < groups.length; i++) {
      (function (g) {
        var key = "mygather:" + REPORT_ID + ":nav:" + g.id;
        var saved = storageGet(key);
        if (saved === "closed") g.open = false;
        else if (saved === "open") g.open = true;
        g.addEventListener("toggle", function () {
          storageSet(key, g.open ? "open" : "closed");
        });
      })(groups[i]);
    }

    // Clicking a child link auto-opens its parent nav-group (in case
    // the user had it collapsed) and, for robustness, also opens the
    // target main-content <details> so jump-to-anchor scrolls land on
    // a visible element.
    var navLinks = document.querySelectorAll("nav.index a[href^='#']");
    for (var j = 0; j < navLinks.length; j++) {
      (function (a) {
        a.addEventListener("click", function () {
          var href = a.getAttribute("href") || "";
          if (!href || href.length < 2) return;
          var id = href.slice(1);
          // Open the parent nav-group in the nav rail.
          var parentNavGroup = a.closest("details.nav-group");
          if (parentNavGroup) parentNavGroup.open = true;
          // Open the main-content target and any ancestor details.
          var target = document.getElementById(id);
          while (target) {
            if (target.tagName === "DETAILS") target.open = true;
            target = target.parentElement && target.parentElement.closest("details");
          }
        });
      })(navLinks[j]);
    }
  }

  function initNavScrollSpy() {
    if (!("IntersectionObserver" in window)) return;
    var navLinks = document.querySelectorAll('nav.index a[href^="#"]');
    if (!navLinks.length) return;
    var byHash = {};
    navLinks.forEach(function (a) { byHash[a.getAttribute("href")] = a; });
    var targets = document.querySelectorAll("main.content details[id]");
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

  // --- Print-expand hook ----------------------------------------

  function initPrintHook() {
    var beforeState = null;
    function stash() {
      beforeState = [];
      var ds = document.querySelectorAll("details");
      for (var i = 0; i < ds.length; i++) { beforeState.push(ds[i].open); ds[i].open = true; }
    }
    function restore() {
      if (!beforeState) return;
      var ds = document.querySelectorAll("details");
      for (var i = 0; i < ds.length && i < beforeState.length; i++) ds[i].open = beforeState[i];
      beforeState = null;
    }
    window.addEventListener("beforeprint", stash);
    window.addEventListener("afterprint", restore);
  }

  // --- Boot -------------------------------------------------------

  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", boot);
  } else {
    boot();
  }
  function boot() {
    initCollapsePersistence();
    initVariablesSearch();
    initCharts();
    initNavGroups();
    initNavScrollSpy();
    initPrintHook();
  }
})();
