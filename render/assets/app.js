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
        var chips = input.parentNode.querySelectorAll(
          '.var-chip[data-snapshot="' + cssEscape(snapshot) + '"]'
        );
        var state = { needle: "", statusFilter: "all" };

        function update() {
          var needle = state.needle;
          var sf = state.statusFilter;
          var shown = 0, modified = 0;
          for (var r = 0; r < rows.length; r++) {
            var row = rows[r];
            var name = row.getAttribute("data-variable-name") || "";
            var status = row.getAttribute("data-status") || "unknown";
            if (status === "modified") modified++;
            var nameHit = needle === "" || name.toLowerCase().indexOf(needle) !== -1;
            var statusHit = sf === "all" || sf === status;
            var hit = nameHit && statusHit;
            row.hidden = !hit;
            if (hit) shown++;
          }
          if (countEl) {
            countEl.textContent = shown + " of " + rows.length + " shown · " + modified + " modified";
          }
        }

        input.addEventListener("input", function () {
          state.needle = input.value.trim().toLowerCase();
          update();
        });
        chips.forEach(function (c) {
          c.addEventListener("click", function () {
            state.statusFilter = c.getAttribute("data-filter") || "all";
            chips.forEach(function (x) { x.classList.toggle("active", x === c); });
            update();
          });
        });
        update();
      })(inputs[i]);
    }
  }

  function cssEscape(s) {
    if (window.CSS && typeof window.CSS.escape === "function") return window.CSS.escape(s);
    return String(s).replace(/"/g, '\\"');
  }

  // --- Chart registry + resize handling ---------------------------

  // Every uPlot instance registers itself here so layout changes
  // (nav collapse/expand, window resize) can recompute width and call
  // chart.setSize.
  var CHARTS = [];
  function registerChart(plot, containerEl, options) {
    CHARTS.push({ plot: plot, el: containerEl, opts: options });
  }
  function unregisterChart(plot) {
    for (var i = CHARTS.length - 1; i >= 0; i--) {
      if (CHARTS[i].plot === plot) CHARTS.splice(i, 1);
    }
  }
  function resizeAllCharts() {
    for (var i = 0; i < CHARTS.length; i++) {
      var entry = CHARTS[i];
      if (!entry.plot || !entry.el || !entry.el.isConnected) continue;
      // Skip if the container is inside a collapsed <details>, has
      // display:none, or has been momentarily laid out at 0 width —
      // setSize with a nonsense width would blank the chart.
      var w = measureChartWidth(entry.el);
      if (w < 200) continue;
      var h = entry.opts && entry.opts.height ? entry.opts.height : 240;
      try { entry.plot.setSize({ width: w, height: h }); } catch (_) {}
    }
  }
  // Debounced window resize listener.
  var _resizeTimer = null;
  window.addEventListener("resize", function () {
    if (_resizeTimer) clearTimeout(_resizeTimer);
    _resizeTimer = setTimeout(resizeAllCharts, 80);
  });

  // Observe the main content column; when its width changes for any
  // reason (nav toggle, details open/close, external CSS), re-fit.
  function observeContentColumn() {
    if (typeof ResizeObserver !== "function") return;
    var main = document.querySelector("main.content");
    if (!main) return;
    var obs = new ResizeObserver(function () {
      if (_resizeTimer) clearTimeout(_resizeTimer);
      _resizeTimer = setTimeout(resizeAllCharts, 60);
    });
    obs.observe(main);
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
      // Larger left padding so wide y-axis tick labels (e.g.
      // "2,000,000,000") fit without overlapping the rotated label.
      padding: [12, 12, 12, 16],
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
          size:  88,                  // wider gutter for big tick labels
          values: (u, splits) => splits.map(formatYTick),
          // No rotated `label` — it overlapped tick labels and was
          // illegible. Per-chart unit is shown via the chart-summary
          // strip and the toolbar label instead.
        },
      ],
      cursor: {
        drag:  { x: true, y: false, uni: 50 },
        focus: { prox: 30 },
        points: { size: 6, width: 2 },
      },
      legend: { show: false },
      series: labelSeries,
      hooks: {
        init:  [attachTooltip],
        setCursor: [updateTooltipOnCursor],
      },
    };
  }

  // --- Hover tooltip -----------------------------------------------

  // attachTooltip builds one tooltip node per chart and hangs it on
  // the chart's `over` layer. It persists for the lifetime of the
  // uPlot instance.
  function attachTooltip(u) {
    var tt = document.createElement("div");
    tt.className = "chart-tooltip";
    tt.style.display = "none";
    u.over.appendChild(tt);
    u.__tooltip = tt;

    u.over.addEventListener("mouseleave", function () {
      tt.style.display = "none";
    });
    u.over.addEventListener("mouseenter", function () {
      tt.style.display = "block";
    });
  }

  // updateTooltipOnCursor runs on every uPlot setCursor, which fires
  // on mouse-move inside the chart's plotting area.
  function updateTooltipOnCursor(u) {
    var tt = u.__tooltip;
    if (!tt) return;
    var idx = u.cursor.idx;
    if (idx == null || u.cursor.left < 0) {
      tt.style.display = "none";
      return;
    }
    tt.style.display = "block";

    // Build rows: timestamp header + one row per visible series.
    var x = u.data[0][idx];
    var tsLabel = formatTooltipTime(x);
    var rows = ['<div class="tt-ts">' + escapeHTML(tsLabel) + '</div>'];
    // Sort entries by value descending (most-prominent first) so the
    // tooltip reads at a glance.
    var entries = [];
    for (var i = 1; i < u.series.length; i++) {
      var s = u.series[i];
      if (s.show === false) continue;
      var v = u.data[i][idx];
      if (v == null || (typeof v === "number" && isNaN(v))) continue;
      entries.push({ label: s.label, color: s.stroke, value: v });
    }
    entries.sort(function (a, b) { return Math.abs(b.value) - Math.abs(a.value); });
    if (entries.length === 0) {
      rows.push('<div class="tt-empty">no data at this point</div>');
    } else {
      entries.forEach(function (e) {
        rows.push(
          '<div class="tt-row">' +
            '<span class="tt-sw" style="background:' + e.color + '"></span>' +
            '<span class="tt-label">' + escapeHTML(String(e.label)) + '</span>' +
            '<span class="tt-value">' + formatTooltipValue(e.value) + '</span>' +
          '</div>'
        );
      });
    }
    tt.innerHTML = rows.join("");

    // Position: offset +12 px right/below the cursor; flip to the
    // left if near the right edge.
    var left = u.cursor.left + 14;
    var top  = u.cursor.top  + 14;
    var rect = tt.getBoundingClientRect();
    var plotW = u.bbox.width / devicePixelRatio;
    var plotH = u.bbox.height / devicePixelRatio;
    if (left + rect.width > plotW) left = u.cursor.left - rect.width - 14;
    if (top + rect.height > plotH) top = Math.max(0, u.cursor.top - rect.height - 14);
    if (left < 0) left = 0;
    if (top < 0) top = 0;
    tt.style.left = left + "px";
    tt.style.top = top + "px";
  }

  function formatTooltipTime(unixSec) {
    if (unixSec == null) return "";
    var d = new Date(unixSec * 1000);
    function pad(n) { return n < 10 ? "0" + n : "" + n; }
    return d.toLocaleDateString() + " " +
           pad(d.getHours()) + ":" + pad(d.getMinutes()) + ":" + pad(d.getSeconds());
  }

  // Compact y-axis tick formatter: 1.2K / 3.4M / 1.2B / 4.5T.
  function formatYTick(v) {
    if (v == null) return "";
    var n = Number(v);
    if (!isFinite(n)) return "";
    var abs = Math.abs(n);
    if (abs >= 1e12) return (n / 1e12).toFixed(abs >= 10e12 ? 0 : 1).replace(/\.0$/, "") + "T";
    if (abs >= 1e9)  return (n / 1e9 ).toFixed(abs >= 10e9  ? 0 : 1).replace(/\.0$/, "") + "B";
    if (abs >= 1e6)  return (n / 1e6 ).toFixed(abs >= 10e6  ? 0 : 1).replace(/\.0$/, "") + "M";
    if (abs >= 1e3)  return (n / 1e3 ).toFixed(abs >= 10e3  ? 0 : 1).replace(/\.0$/, "") + "K";
    if (Math.abs(n - Math.round(n)) < 1e-9) return String(Math.round(n));
    return n.toFixed(2);
  }

  function formatTooltipValue(v) {
    if (typeof v !== "number") return escapeHTML(String(v));
    if (!isFinite(v))           return "∞";
    // Trim to 4 decimals when fractional; otherwise thousands-separated integer.
    if (Math.abs(v - Math.round(v)) < 1e-9) return Math.round(v).toLocaleString();
    return v.toLocaleString(undefined, { maximumFractionDigits: 4 });
  }

  function measureChartWidth(el) {
    // Use the container's width minus a small padding; fall back to
    // parent width if the container itself hasn't been laid out yet
    // (e.g., because the parent <details> is still animating open).
    var w = el.clientWidth || (el.parentNode && el.parentNode.clientWidth) || 0;
    if (!w) {
      var rect = el.getBoundingClientRect();
      w = Math.floor(rect.width);
    }
    return Math.max(320, Math.floor(w - 2));
  }

  // Build a clickable legend rendered OUTSIDE uPlot (uPlot's default
  // legend is too verbose for 16-series charts).
  //
  // Click behaviour:
  //   - Plain click on a pill  -> SOLO that series (hide everything
  //     else). Clicking the already-soloed pill while it's the only
  //     visible one RESTORES all series.
  //   - Shift / Cmd / Ctrl click -> additive toggle (the old
  //     behaviour): flip just this pill, leave the others alone.
  function mountLegend(containerEl, series, plot) {
    var legend = document.createElement("div");
    legend.className = "series-legend";
    var pills = [];
    series.forEach(function (s, i) {
      if (i === 0) return; // time
      var btn = document.createElement("button");
      btn.type = "button";
      btn.className = "series-pill active";
      btn.setAttribute("data-idx", String(i));
      btn.title = "Click to solo · Shift/Cmd+Click to toggle";
      btn.innerHTML =
        '<span class="swatch" style="background:' + s.stroke + '"></span>' +
        '<span class="lbl">' + escapeHTML(s.label) + '</span>';
      btn.addEventListener("click", function (ev) {
        var idx = Number(btn.getAttribute("data-idx"));
        var additive = ev.shiftKey || ev.metaKey || ev.ctrlKey;
        if (additive) {
          var showing = plot.series[idx].show;
          plot.setSeries(idx, { show: !showing });
          btn.classList.toggle("active", !showing);
          return;
        }
        // Solo path: count currently-visible series.
        var visibleCount = 0, thisIsVisible = false;
        for (var k = 1; k < plot.series.length; k++) {
          if (plot.series[k].show !== false) {
            visibleCount++;
            if (k === idx) thisIsVisible = true;
          }
        }
        var isSoloed = visibleCount === 1 && thisIsVisible;
        if (isSoloed) {
          // Already soloed — restore all.
          for (var j = 1; j < plot.series.length; j++) {
            plot.setSeries(j, { show: true });
          }
          pills.forEach(function (p) { p.classList.add("active"); });
        } else {
          // Hide all others; show this one.
          for (var m = 1; m < plot.series.length; m++) {
            plot.setSeries(m, { show: m === idx });
          }
          pills.forEach(function (p) {
            p.classList.toggle("active", Number(p.getAttribute("data-idx")) === idx);
          });
        }
      });
      legend.appendChild(btn);
      pills.push(btn);
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
    registerChart(plot, el, opts);
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
      if (plot) { unregisterChart(plot); plot.destroy(); plot = null; }
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
      var w = measureChartWidth(el);
      var opts = basePlotOpts(w, 260, series, unit);
      plot = new uPlot(opts, values, el);
      registerChart(plot, el, opts);
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

    // Built-in chips (filter the list only; they don't touch the chart).
    var builtinChips = [
      { k: "all",      label: "All",      filter: null },
      { k: "selected", label: "Selected", filter: function (name) { return selected.has(name); } },
      { k: "counters", label: "Counters", filter: function (name) { return data.isCounter[name]; } },
      { k: "gauges",   label: "Gauges",   filter: function (name) { return !data.isCounter[name]; } },
    ];
    // Category chips driven by the embedded metadata. Plain click
    // REPLACES the chart selection with the category's variables;
    // Shift/Cmd-click ADDS to the existing chart selection.
    var categoryDefs = Array.isArray(data.categories) ? data.categories : [];
    var catFilterByKey = {};
    function buildCategoryFilter(key) {
      return function (name) {
        var hits = (data.categoryMap && data.categoryMap[name]) || [];
        for (var i = 0; i < hits.length; i++) if (hits[i] === key) return true;
        return false;
      };
    }
    categoryDefs.forEach(function (c) { catFilterByKey[c.key] = buildCategoryFilter(c.key); });

    // Two-row layout. Each row has a small label and a wrapping chip group.
    var chipButtons = {};

    function makeChip(parent, def, kind) {
      var b = document.createElement("button");
      b.type = "button";
      b.className = "ma-chip ma-chip-" + kind;
      b.setAttribute("data-k", def.k);
      b.title = def.title || (kind === "category"
        ? "Click to load this group on the chart · Shift/Cmd+Click to add to current selection"
        : "Click to filter the list");
      b.innerHTML = '<span class="lbl">' + escapeHTML(def.label) + '</span>' +
        (def.count != null ? ' <span class="ct">' + def.count + '</span>' : '');
      if (def.k === "all") b.classList.add("active");
      b.addEventListener("click", function (ev) {
        var additive = ev.shiftKey || ev.metaKey || ev.ctrlKey;
        if (kind === "category" && def.filter) {
          if (additive) {
            // Add-to-selection (preserve existing).
            data.variables.forEach(function (n) {
              if (def.filter(n)) selected.add(n);
            });
          } else {
            // Replace selection with this category's members so the
            // chart immediately shows the picked group.
            selected.clear();
            data.variables.forEach(function (n) {
              if (def.filter(n)) selected.add(n);
            });
          }
          persistSelection();
          // Filter list to this category as well, for context.
          Object.keys(chipButtons).forEach(function (k) { chipButtons[k].classList.remove("active"); });
          b.classList.add("active");
          state.category = def.k;
          redrawList();
          scheduleRedraw();
          b.classList.add("flash");
          setTimeout(function () { b.classList.remove("flash"); }, 250);
          return;
        }
        // Built-in chips: filter-only (chart untouched).
        Object.keys(chipButtons).forEach(function (k) { chipButtons[k].classList.remove("active"); });
        b.classList.add("active");
        state.category = def.k;
        redrawList();
      });
      parent.appendChild(b);
      chipButtons[def.k] = b;
    }

    function makeChipRow(labelText) {
      var row = document.createElement("div");
      row.className = "ma-chip-row";
      var lbl = document.createElement("span");
      lbl.className = "ma-chip-row-label";
      lbl.textContent = labelText;
      var bag = document.createElement("div");
      bag.className = "ma-chip-bag";
      row.appendChild(lbl);
      row.appendChild(bag);
      panel.appendChild(row);
      return bag;
    }

    var viewBag = makeChipRow("View");
    builtinChips.forEach(function (def) { makeChip(viewBag, def, "builtin"); });

    // CATEGORIES row → custom button-dropdown + two action buttons.
    // Native <select> with appearance:none collapsed in some browsers,
    // hiding the label text. Custom popup gives us full control.
    var catRow = document.createElement("div");
    catRow.className = "ma-chip-row";
    var catLabel = document.createElement("span");
    catLabel.className = "ma-chip-row-label";
    catLabel.textContent = "Categories";
    var catActions = document.createElement("div");
    catActions.className = "ma-cat-actions";

    // Wrapper around the toggle button + popup so the popup positions
    // relative to the button.
    var catDD = document.createElement("div");
    catDD.className = "ma-cat-dd";

    var catToggle = document.createElement("button");
    catToggle.type = "button";
    catToggle.className = "ma-cat-dd-toggle";
    catToggle.setAttribute("aria-haspopup", "listbox");
    catToggle.setAttribute("aria-expanded", "false");
    catToggle.title = "Pick a category to load its variables on the chart";

    var catCurrentKey = "";
    function setCatLabel(key) {
      var lbl = "— pick a category —";
      if (key) {
        for (var i = 0; i < categoryDefs.length; i++) {
          if (categoryDefs[i].key === key) {
            lbl = categoryDefs[i].label + " (" + categoryDefs[i].count + ")";
            break;
          }
        }
      }
      catToggle.innerHTML =
        '<span class="ma-cat-dd-text">' + escapeHTML(lbl) + '</span>' +
        '<span class="ma-cat-dd-chev" aria-hidden="true">▾</span>';
    }
    setCatLabel("");

    var catPopup = document.createElement("ul");
    catPopup.className = "ma-cat-dd-popup";
    catPopup.setAttribute("role", "listbox");
    catPopup.hidden = true;
    categoryDefs.forEach(function (c) {
      if (!c || c.count === 0) return;
      var li = document.createElement("li");
      li.className = "ma-cat-dd-opt";
      li.setAttribute("role", "option");
      li.setAttribute("data-key", c.key);
      li.title = c.description || c.label;
      li.innerHTML =
        '<span class="opt-lbl">' + escapeHTML(c.label) + '</span>' +
        '<span class="opt-ct">' + c.count + '</span>';
      li.addEventListener("click", function () {
        catCurrentKey = c.key;
        setCatLabel(c.key);
        closeCatPopup();
        applyCategory(true);
      });
      catPopup.appendChild(li);
    });

    function openCatPopup() {
      catPopup.hidden = false;
      catToggle.setAttribute("aria-expanded", "true");
      catToggle.classList.add("open");
      // Close on outside click.
      setTimeout(function () {
        document.addEventListener("click", outsideClickClose, true);
      }, 0);
    }
    function closeCatPopup() {
      catPopup.hidden = true;
      catToggle.setAttribute("aria-expanded", "false");
      catToggle.classList.remove("open");
      document.removeEventListener("click", outsideClickClose, true);
    }
    function outsideClickClose(ev) {
      if (catDD.contains(ev.target)) return;
      closeCatPopup();
    }
    catToggle.addEventListener("click", function () {
      if (catPopup.hidden) openCatPopup(); else closeCatPopup();
    });
    document.addEventListener("keydown", function (ev) {
      if (ev.key === "Escape" && !catPopup.hidden) closeCatPopup();
    });

    catDD.appendChild(catToggle);
    catDD.appendChild(catPopup);

    var btnLoad = document.createElement("button");
    btnLoad.type = "button";
    btnLoad.className = "ma-action ma-cat-load";
    btnLoad.textContent = "Load on chart";
    btnLoad.title = "Replace chart selection with the picked category";
    var btnAdd = document.createElement("button");
    btnAdd.type = "button";
    btnAdd.className = "ma-action ma-cat-add";
    btnAdd.textContent = "+ Add to chart";
    btnAdd.title = "Add the picked category to the existing chart selection";

    catActions.appendChild(catDD);
    catActions.appendChild(btnLoad);
    catActions.appendChild(btnAdd);
    catRow.appendChild(catLabel);
    catRow.appendChild(catActions);
    panel.appendChild(catRow);

    function applyCategory(replace) {
      if (!catCurrentKey) return;
      var fn = catFilterByKey[catCurrentKey];
      if (!fn) return;
      if (replace) selected.clear();
      data.variables.forEach(function (n) {
        if (fn(n)) selected.add(n);
      });
      persistSelection();
      // Filter the list to show the chosen category for context.
      Object.keys(chipButtons).forEach(function (k) { chipButtons[k].classList.remove("active"); });
      state.category = "cat:" + catCurrentKey;
      if (chipButtons["selected"]) chipButtons["selected"].classList.add("active");
      redrawList();
      scheduleRedraw();
    }
    btnLoad.addEventListener("click", function () { applyCategory(true); });
    btnAdd.addEventListener("click", function () { applyCategory(false); });

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
      if (!state.category || state.category === "all") return true;
      if (state.category === "selected") return selected.has(name);
      if (state.category === "counters") return !!data.isCounter[name];
      if (state.category === "gauges")   return !data.isCounter[name];
      if (state.category.indexOf("cat:") === 0) {
        var key = state.category.slice(4);
        var fn = catFilterByKey[key];
        return fn ? fn(name) : false;
      }
      return true;
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
      if (chart) { unregisterChart(chart); chart.destroy(); chart = null; }
      // Strip prior legend regardless of selection state.
      var prev = el.nextSibling;
      if (prev && prev.classList && prev.classList.contains("series-legend")) prev.remove();

      var picks = Array.from(selected);
      // Empty selection ⇒ empty chart (FIX: previously fell back to
      // defaults which made "Clear" feel broken).
      picks.sort();

      // Drop the first sample (column 0). pt-mext stores the initial
      // raw tally there for counters; that single huge spike crushed
      // the y-axis. Real per-sample deltas start at index 1.
      var tStart = data.timestamps.length > 1 ? 1 : 0;
      var truncatedTimestamps = data.timestamps.slice(tStart);

      var series = [{ label: "time" }];
      var values = [truncatedTimestamps];
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
        values.push(deltaArr.slice(tStart));
      });
      var width = measureChartWidth(el);
      // No rotated y-axis label — the chart's heading carries the unit.
      var opts = basePlotOpts(width, 300, series, "");
      chart = new uPlot(opts, values, el);
      registerChart(chart, el, opts);
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

  function initNavCollapse() {
    var layout = document.getElementById("app-layout");
    var collapseBtn = document.querySelector("nav.index .nav-collapse-btn");
    var expandBtn = document.querySelector(".nav-expand-btn");
    if (!layout || !expandBtn) return;
    var key = "mygather:" + REPORT_ID + ":nav:collapsed";

    function apply(collapsed) {
      if (collapsed) {
        layout.classList.add("nav-hidden");
        expandBtn.hidden = false;
      } else {
        layout.classList.remove("nav-hidden");
        expandBtn.hidden = true;
      }
      // Re-fit charts after the grid transition settles (180ms ease +
      // one extra frame for safety).
      setTimeout(resizeAllCharts, 220);
    }

    var saved = storageGet(key);
    if (saved === "true") apply(true);

    if (collapseBtn) {
      collapseBtn.addEventListener("click", function () {
        storageSet(key, "true");
        apply(true);
      });
    }
    expandBtn.addEventListener("click", function () {
      storageSet(key, "false");
      apply(false);
    });

    // Keyboard shortcut: Cmd/Ctrl + \
    document.addEventListener("keydown", function (e) {
      if ((e.metaKey || e.ctrlKey) && e.key === "\\") {
        e.preventDefault();
        var isCollapsed = layout.classList.contains("nav-hidden");
        storageSet(key, isCollapsed ? "false" : "true");
        apply(!isCollapsed);
      }
    });
  }

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
    initNavCollapse();
    initNavScrollSpy();
    initPrintHook();
    observeContentColumn();
    // Also re-fit on any <details> toggle (open/close affects
    // content-column scrollbar which affects chart width).
    document.addEventListener("toggle", function () {
      window.requestAnimationFrame(resizeAllCharts);
    }, true);
  }
})();
