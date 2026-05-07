
  // buildLineChart: construct a multi-line uPlot on `el` and attach the
  // pill legend. Returns the uPlot instance (so callers that compose
  // this into a toolbar wrapper can re-mount a single reset-zoom
  // button over the top of sequential rebuilds). Does NOT mount a
  // reset button itself — that's the caller's responsibility.
  function buildLineChart(el, data, unit) {
    var width = measureChartWidth(el);
    var series = [{ label: "time" }].concat(
      data.series.map(function (s, i) { return decorateSeries(s.label, i); })
    );
    var values = [data.timestamps.slice()].concat(data.series.map(function (s) { return s.values; }));
    var opts = basePlotOpts(width, 320, series, unit, data.snapshotBoundaries, data.timestamps);
    var plot = new uPlot(opts, values, el);
    // Register with the data payload so the chart-sync store can
    // recompute windowed stats and apply the current shared zoom
    // window if one is active when this chart mounts.
    registerChart(plot, el, opts, {
      timestamps: data.timestamps,
      series: data.series,
    });
    var legendHandle = mountLegend(el, series, plot, {
      statsSource: { timestamps: data.timestamps, series: data.series },
    });
    plot.__legendHandle = legendHandle;
    return plot;
  }

  // buildStackedChart: construct a smooth stacked bars uPlot with
  // rounded top corners from a flat {timestamps, series[,
  // snapshotBoundaries]} payload (the same shape renderTimeSeries
  // consumes). Each series' cumulative value is drawn as a bar from
  // 0 up to that cumulative height; uPlot draws series[1] first
  // (the grand total, tallest) and each subsequent series paints on
  // top — so each band's visible region is the strip between its
  // stack-top and the next shorter series' stack-top. The rounded
  // top-corner radius softens the sparse-column look that plain
  // rectangular bars produce when samples are seconds apart.
  //
  // Hidden labels drive a zero-value substitution that keeps legend
  // indexes stable while removing the bucket's contribution from the
  // cumulative heights.
  //
  // hiddenLabels: mutable Set owned by the caller (we only read).
  // onRebuild: callback fired when the user toggles legend visibility
  //            so the caller can destroy + reconstruct with the new
  //            hidden set.
  // bucketizeStackSeries collapses dense samples into uniform wall-clock
  // buckets so each time slot contributes a single stacked bar rather
  // than several thin ones competing for pixels. The bucket size scales
  // with the capture span — short windows keep 30s granularity, long
  // windows coarsen up to ~5 minutes — and we aim to end with around
  // 20–60 visible bars across any capture length.
  //
  // Returns { timestamps, series[] } with the same shape as `data` but
  // re-indexed on bucket edges. Within a bucket each series' value is
  // the MEAN of its non-NaN samples (%CPU averages naturally). Empty
  // buckets are dropped so the x-axis has no gaps.
  function bucketizeStackSeries(data) {
    var ts  = data.timestamps || [];
    var ser = data.series || [];
    if (ts.length === 0) return { timestamps: [], series: ser.map(function (s) { return { label: s.label, values: [] }; }) };

    var span = ts[ts.length - 1] - ts[0];
    // Choose a bucket size (seconds) targeting ~30 bars. Snap to a
    // human-friendly step so ticks land on round wall-clock values.
    var STEPS = [30, 60, 120, 300, 600, 1800, 3600];
    var target = Math.max(1, Math.floor(span / 30));
    var step = STEPS[STEPS.length - 1];
    for (var i = 0; i < STEPS.length; i++) { if (STEPS[i] >= target) { step = STEPS[i]; break; } }

    // If samples are already sparser than the step, bucketing would be
    // a no-op that only introduces averaging error — keep the raw data.
    if (ts.length >= 2) {
      var avgGap = span / (ts.length - 1);
      if (avgGap >= step * 0.9) return data;
    }

    var buckets = {}; // key = floor(ts/step)*step → { sums[], counts[] }
    var keys = [];
    for (var j = 0; j < ts.length; j++) {
      var key = Math.floor(ts[j] / step) * step;
      var b = buckets[key];
      if (!b) {
        b = { sums: new Array(ser.length), counts: new Array(ser.length) };
        for (var k = 0; k < ser.length; k++) { b.sums[k] = 0; b.counts[k] = 0; }
        buckets[key] = b;
        keys.push(key);
      }
      for (var k2 = 0; k2 < ser.length; k2++) {
        var v = +ser[k2].values[j];
        if (!isNaN(v)) { b.sums[k2] += v; b.counts[k2] += 1; }
      }
    }
    keys.sort(function (a, b) { return a - b; });

    var outTs = keys;
    var outSeries = ser.map(function (s, k) {
      var vals = new Array(keys.length);
      for (var r = 0; r < keys.length; r++) {
        var bk = buckets[keys[r]];
        vals[r] = bk.counts[k] > 0 ? bk.sums[k] / bk.counts[k] : NaN;
      }
      return { label: s.label, values: vals };
    });
    return { timestamps: outTs, series: outSeries };
  }

  function buildStackedChart(el, data, unit, hiddenLabels, onRebuild) {
    // Coalesce dense samples into uniform wall-clock buckets so we get
    // ONE stacked bar per time slot instead of several thin ones fighting
    // for space. Bucket size is chosen from the capture span so short
    // captures stay granular and long ones don't drown in ticks.
    var bucketed = bucketizeStackSeries(data);
    var series   = bucketed.series;
    var n        = bucketed.timestamps.length;

    // Bar sizing: size[0] = 0.85 of the gap (small breathing room between
    // bars so neighbours don't kiss); size[1] caps the bar at 160px so
    // long captures with few buckets still look proportional.
    var stackedPath = uPlot.paths.bars({ size: [0.85, 160], align: 0, radius: 0.3 });

    // Raw per-series values, zero'd when hidden.
    var rawValues = series.map(function (s) {
      var out = new Array(n);
      var hidden = hiddenLabels.has(s.label);
      for (var j = 0; j < n; j++) {
        if (hidden) { out[j] = 0; continue; }
        var v = +s.values[j];
        out[j] = isNaN(v) ? 0 : v; // NaN→0 so the cumulative stays
                                   // continuous across snapshot
                                   // boundaries.
      }
      return out;
    });

    // Cumulative stacks — each index's value is the sum up to and
    // including that index.
    var stacked = rawValues.map(function () { return new Array(n); });
    for (var j = 0; j < n; j++) {
      var cum = 0;
      for (var i = 0; i < rawValues.length; i++) {
        cum += rawValues[i][j] || 0;
        stacked[i][j] = cum;
      }
    }

    // Raw (un-zeroed) values retained so the tooltip can show what a
    // hidden bucket's value WOULD be.
    var tooltipRaw = series.map(function (s) {
      var out = new Array(n);
      for (var j = 0; j < n; j++) {
        var v = +s.values[j];
        out[j] = isNaN(v) ? null : v;
      }
      return out;
    });

    // Push in REVERSE so uPlot paints the grand total first, then
    // each smaller stack covers it — yielding the visual layering a
    // reader expects and putting the legend in top→bottom stack order.
    var plotSeries = [{ label: "time" }];
    var plotData = [bucketed.timestamps.slice()];
    var plotRawByIdx = [null];
    for (var k = series.length - 1; k >= 0; k--) {
      var stroke = SERIES_COLORS[k % SERIES_COLORS.length];
      plotSeries.push({
        label: series[k].label,
        stroke: stroke,
        width: 0,
        fill: hexToRgba(stroke, 0.85),
        paths: stackedPath,
        points: { show: false },
        value: function (u, v) { return v == null ? "–" : v.toLocaleString(); },
      });
      plotData.push(stacked[k]);
      plotRawByIdx.push(tooltipRaw[k]);
    }

    var w = measureChartWidth(el);
    // Boundary lookup (drawSnapshotBoundariesWith) indexes into this
    // timestamps array with `data.snapshotBoundaries[i]` — which are
    // positions in the ORIGINAL unbucketed sample stream. The bucketed
    // x-axis has different indices and a shorter length, so pass the
    // original data.timestamps: the boundary index still resolves to
    // the correct wall-clock time, and u.valToPos maps that time to a
    // pixel on the bucketed plot independently of the data arrays.
    var opts = basePlotOpts(w, 320, plotSeries, unit, data.snapshotBoundaries, data.timestamps);
    var plot = new uPlot(opts, plotData, el);
    plot.__rawData = plotRawByIdx; // consumed by updateTooltipOnCursor
    // Stats are computed against the un-stacked, un-bucketed RAW values
    // (data.series), not the cumulative `stacked[]` arrays uPlot
    // draws. The pill order matches the REVERSE of `series`, so build
    // an aligned source array in the same reverse order.
    var statsSeries = [];
    for (var sk = series.length - 1; sk >= 0; sk--) {
      statsSeries.push({ label: series[sk].label, values: series[sk].values });
    }
    registerChart(plot, el, opts, {
      timestamps: data.timestamps,
      series: statsSeries,
    });
    var legendHandle = mountLegend(el, plotSeries, plot, {
      initialVisible: function (idx) {
        return !hiddenLabels.has(plotSeries[idx].label);
      },
      onVisibilityChange: function (visibleIdxs) {
        var active = new Set(visibleIdxs);
        hiddenLabels.clear();
        for (var idx = 1; idx < plotSeries.length; idx++) {
          if (!active.has(idx)) hiddenLabels.add(plotSeries[idx].label);
        }
        if (typeof onRebuild === "function") onRebuild();
      },
      statsSource: { timestamps: data.timestamps, series: statsSeries },
    });
    plot.__legendHandle = legendHandle;
    return plot;
  }

  // renderTimeSeries: generic multi-line chart with the pill legend
  // and an adjacent "reset zoom" button. Thin wrapper over
  // buildLineChart for charts that don't need a style toggle.
  function renderTimeSeries(el, data, unit) {
    if (!data || !Array.isArray(data.timestamps) || !Array.isArray(data.series)) return;
    var plot = buildLineChart(el, data, unit);
    mountResetZoomButton(el, function () { return plot; });
  }

  // renderTopChart: Top CPU processes chart with a segmented toolbar
  // letting the reader toggle between line + stacked-bar views.
  // Default is the line view; the choice persists per-report under
  // the v2 localStorage namespace. Each mode fully rebuilds the uPlot
  // instance (destroys + recreates); zoom state is intentionally not
  // preserved across switches — the toggle is meant for "show me a
  // different angle", not "keep my zoom".
  function renderTopChart(el, data) {
    if (!data || !Array.isArray(data.timestamps) || !Array.isArray(data.series)) return;

    var STORAGE_KEY = "mygather:v2:" + REPORT_ID + ":chart-type:top";
    var current = storageGet(STORAGE_KEY);
    if (current !== "line" && current !== "stacked") current = "line";

    var toolbar = document.createElement("div");
    toolbar.className = "chart-view-toolbar";
    toolbar.setAttribute("role", "tablist");
    toolbar.setAttribute("aria-label", "Top CPU chart style");

    function makeBtn(key, label, tooltip) {
      var b = document.createElement("button");
      b.type = "button";
      b.className = "view-btn";
      b.setAttribute("role", "tab");
      b.setAttribute("data-chart-type", key);
      b.textContent = label;
      b.title = tooltip;
      return b;
    }
    var btnLine  = makeBtn("line",    "Lines",        "Plot each process as a separate smooth line — shows each one's curve over time");
    var btnStack = makeBtn("stacked", "Stacked bars", "Stack per-process %CPU into smooth bars with rounded tops — shows composition and total load over time at a glance");
    toolbar.appendChild(btnLine);
    toolbar.appendChild(btnStack);
    el.parentNode.insertBefore(toolbar, el);

    // Stacked-mode hidden-label Set is preserved across rebuilds so
    // toggling visibility of a process in stacked mode, then switching
    // back to lines, then back to stacked, remembers the hidden set.
    // Clearing on mode switch would be annoying for a reader iterating
    // between views.
    var stackedHidden = new Set();
    var plot = null, legendEl = null;

    function setAria() {
      btnLine.classList.toggle("active",  current === "line");
      btnStack.classList.toggle("active", current === "stacked");
      btnLine.setAttribute("aria-selected",  current === "line"    ? "true" : "false");
      btnStack.setAttribute("aria-selected", current === "stacked" ? "true" : "false");
    }

    function cleanup() {
      if (plot) { unregisterChart(plot); plot.destroy(); plot = null; }
      if (legendEl && legendEl.parentNode) {
        legendEl.parentNode.removeChild(legendEl);
        legendEl = null;
      }
    }

    function draw() {
      cleanup();
      var topUnit = {
        label: "%CPU",
        title: "Per-process CPU utilisation — percent of one logical core",
      };
      if (current === "stacked") {
        plot = buildStackedChart(el, data, topUnit, stackedHidden, draw);
      } else {
        plot = buildLineChart(el, data, topUnit);
      }
      legendEl = el.nextSibling;
    }

    setAria();
    draw();
    mountResetZoomButton(el, function () { return plot; });

    function switchTo(next) {
      if (current === next) return;
      current = next;
      storageSet(STORAGE_KEY, current);
      setAria();
      draw();
    }
    btnLine.addEventListener("click",  function () { switchTo("line"); });
    btnStack.addEventListener("click", function () { switchTo("stacked"); });
  }

  // renderNetworkSockets: stacked-bar chart of socket-state counts
  // over time. Each sample becomes one bar; stack segments are the
  // per-state counts (ESTABLISHED / TIME_WAIT / CLOSE_WAIT / LISTEN /
  // UDP / …). Reuses buildStackedChart so the bar sizing, bucketing,
  // and legend behaviour match the other stacked views (mysqladmin,
  // processlist, top).
  function renderNetworkSockets(el, data) {
    if (!data || !Array.isArray(data.timestamps) || !Array.isArray(data.series)) return;
    var hidden = new Set();
    var plot = null;
    var legendEl = null;
    function draw() {
      if (plot) { unregisterChart(plot); plot.destroy(); plot = null; }
      // buildStackedChart appends a fresh .series-legend sibling after
      // el on every call. Without removing the previous one, toggling
      // a legend chip (which re-enters draw()) stacks duplicate legend
      // rows under the chart — see the bug report with 5 rows.
      if (legendEl && legendEl.parentNode) {
        legendEl.parentNode.removeChild(legendEl);
        legendEl = null;
      }
      plot = buildStackedChart(el, data, {
        label: "sockets",
        title: "Socket count — number of sockets in each TCP state at each snapshot (absolute, not a rate)",
      }, hidden, draw);
      var next = el.nextSibling;
      if (next && next.classList && next.classList.contains("series-legend")) {
        legendEl = next;
      }
    }
    draw();
    mountResetZoomButton(el, function () { return plot; });
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
    btnUtil.title = "Show per-device %util — fraction of wall time the device was busy";
    var btnAqu  = document.createElement("button");
    btnAqu.type = "button";
    btnAqu.className = "view-btn";
    btnAqu.textContent = "Avg queue size";
    btnAqu.title = "Show per-device aqu-sz — average queue depth of requests issued to the device";
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
      var unit   = currentView === "util"
        ? { label: "%util",  title: "Percent of wall time the block device was busy servicing any I/O (iostat -x `%util`)" }
        : { label: "aqu-sz", title: "Average queue depth — mean number of requests in the device queue over the sample interval (iostat -x `aqu-sz`)" };
      var series = [{ label: "time" }].concat(labels.map(function (lbl, i) { return decorateSeries(lbl, i); }));
      var values = [data.timestamps.slice()].concat(rows);
      var w = measureChartWidth(el);
      var opts = basePlotOpts(w, 320, series, unit, data.snapshotBoundaries, data.timestamps);
      plot = new uPlot(opts, values, el);
      // statsSource carries one entry per visible series in the same
      // order as the legend pills (1..N), excluding the time axis.
      var iostatStatsSeries = labels.map(function (lbl, i) {
        return { label: lbl, values: rows[i] };
      });
      registerChart(plot, el, opts, {
        timestamps: data.timestamps,
        series: iostatStatsSeries,
      });
      var iostatLegendHandle = mountLegend(el, series, plot, {
        statsSource: { timestamps: data.timestamps, series: iostatStatsSeries },
      });
      plot.__legendHandle = iostatLegendHandle;
      legendEl = el.nextSibling;
    }
    draw();
    mountResetZoomButton(el, function () { return plot; });
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

  // renderProcesslist: stacked-bar thread-count chart with a dimension
  // toggle (State / User / Host / Command / db). Each time sample renders
  // as a single vertical bar; each bucket (state/user/…) contributes a
  // coloured segment, so the total bar height equals the total thread
  // count at that instant. A lightweight top-N cap keeps the legend
  // readable when a high-cardinality dimension (e.g. Host) has many
  // buckets.
  function renderProcesslist(el, data) {
    if (!data || !Array.isArray(data.timestamps) || !Array.isArray(data.dimensions)) return;

    var dims = data.dimensions.filter(function (d) {
      return d && Array.isArray(d.series) && d.series.length > 0;
    });
    if (dims.length === 0) return;

    var TOP_N = 16; // cap series per dimension to keep the legend sane.
    function rankSeries(series) {
      // Sort by peak value descending so the most prominent buckets
      // stay near the bottom of the stack (drawn first, visually
      // foundational); everything past TOP_N folds into "Other" sum.
      var ranked = series.map(function (s, i) {
        var peak = 0;
        for (var j = 0; j < s.values.length; j++) {
          var v = +s.values[j];
          if (v > peak) peak = v;
        }
        return { s: s, peak: peak, i: i };
      });
      ranked.sort(function (a, b) {
        if (b.peak !== a.peak) return b.peak - a.peak;
        return a.i - b.i;
      });
      if (ranked.length <= TOP_N) return ranked.map(function (r) { return r.s; });
      var kept = ranked.slice(0, TOP_N).map(function (r) { return r.s; });
      var n = data.timestamps.length;
      var otherVals = new Array(n);
      for (var k = 0; k < n; k++) otherVals[k] = 0;
      ranked.slice(TOP_N).forEach(function (r) {
        for (var j = 0; j < n; j++) otherVals[j] += +r.s.values[j] || 0;
      });
      kept.push({ label: "Other (" + (ranked.length - TOP_N) + ")", values: otherVals });
      return kept;
    }

    var toolbar = document.createElement("div");
    toolbar.className = "chart-view-toolbar";
    el.parentNode.insertBefore(toolbar, el);

    var buttons = [];
    var currentIdx = 0;
    var plot = null, legendEl = null;
    // Hidden-label state is scoped per dimension. Each grouping
    // (State / User / Host / Command / db) aggregates threads
    // differently, and a label like "Other" in one dimension means
    // something completely different in another — so hiding "Other"
    // in State must NOT also hide "Other" in Host. Map<dimKey, Set>
    // gives each dimension its own independent hidden-set that
    // persists across dimension switches and draw() rebuilds.
    var hiddenByDim = new Map();
    function currentHidden() {
      var key = dims[currentIdx].label;
      var s = hiddenByDim.get(key);
      if (!s) { s = new Set(); hiddenByDim.set(key, s); }
      return s;
    }

    var barsPath = uPlot.paths.bars({ size: [1.0, Infinity], align: 0 });

    function draw() {
      if (plot) { unregisterChart(plot); plot.destroy(); plot = null; }
      if (legendEl && legendEl.parentNode) { legendEl.parentNode.removeChild(legendEl); legendEl = null; }
      var dim = dims[currentIdx];
      var seriesData = rankSeries(dim.series);
      var n = data.timestamps.length;
      var hiddenLabels = currentHidden();

      if (dim.key === "activity") {
        plot = buildLineChart(el, {
          timestamps: data.timestamps,
          series: seriesData,
          snapshotBoundaries: data.snapshotBoundaries,
        }, dim.unit || "threads");
        legendEl = el.nextSibling;
        return;
      }

      // Raw per-segment values per bucket. When a bucket is hidden via
      // the legend, its row is replaced with zeros so it contributes
      // nothing to the cumulative stack — the visible buckets then
      // stack together as if the hidden ones weren't there. Keeping
      // hidden buckets in the plotSeries array (just at zero height)
      // keeps legend indexes stable across toggles and lets the user
      // un-hide them by clicking the now-inactive pill.
      var rawValues = seriesData.map(function (s) {
        var out = new Array(n);
        var hidden = hiddenLabels.has(s.label);
        for (var j = 0; j < n; j++) {
          if (hidden) { out[j] = 0; continue; }
          var v = +s.values[j];
          out[j] = isNaN(v) ? null : v;
        }
        return out;
      });

      // Cumulative stacked values. stacked[i][j] = sum of rawValues[0..i][j].
      // These are what uPlot plots as bar heights.
      var stacked = rawValues.map(function () { return new Array(n); });
      for (var j = 0; j < n; j++) {
        var cum = 0;
        for (var i = 0; i < rawValues.length; i++) {
          cum += rawValues[i][j] || 0;
          stacked[i][j] = cum;
        }
      }

      // Render buckets in REVERSE order: the tallest stack (grand
      // total) draws first and each smaller stack paints on top of
      // it, producing the visual layering of a stacked column chart.
      // The legend then naturally reads top→bottom, matching the
      // visual stack order.
      var plotSeries = [{ label: "time" }];
      var plotData = [data.timestamps.slice()];
      var plotRawByIdx = [null];
      // Raw (un-zeroed) values for the tooltip, even for hidden
      // buckets, so hover can still tell you what a bucket's value
      // would be if it were visible.
      var tooltipRaw = seriesData.map(function (s) {
        var out = new Array(n);
        for (var j = 0; j < n; j++) {
          var v = +s.values[j];
          out[j] = isNaN(v) ? null : v;
        }
        return out;
      });
      for (var k = seriesData.length - 1; k >= 0; k--) {
        var stroke = SERIES_COLORS[k % SERIES_COLORS.length];
        plotSeries.push({
          label: seriesData[k].label,
          stroke: stroke,
          width: 0,
          fill: hexToRgba(stroke, 0.95),
          paths: barsPath,
          points: { show: false },
          value: function (u, v) { return v == null ? "–" : v.toLocaleString(); },
        });
        plotData.push(stacked[k]);
        plotRawByIdx.push(tooltipRaw[k]);
      }

      var w = measureChartWidth(el);
      var opts = basePlotOpts(w, 320, plotSeries, dim.unit || "threads", data.snapshotBoundaries, data.timestamps);
      plot = new uPlot(opts, plotData, el);
      plot.__rawData = plotRawByIdx; // consumed by updateTooltipOnCursor
      registerChart(plot, el, opts);
      mountLegend(el, plotSeries, plot, {
        // Pill is active ⇔ bucket is NOT hidden in the current dim.
        initialVisible: function (idx) {
          var label = plotSeries[idx].label;
          return !hiddenLabels.has(label);
        },
        // Rebuild the entire chart from the new visibility set so the
        // stack geometry (cumulative heights) matches the visible
        // subset. Write-through to the per-dimension set so switches
        // to another dimension start fresh with their own state.
        onVisibilityChange: function (visibleIdxs) {
          var active = new Set(visibleIdxs);
          var next = new Set();
          for (var idx = 1; idx < plotSeries.length; idx++) {
            if (!active.has(idx)) next.add(plotSeries[idx].label);
          }
          hiddenByDim.set(dims[currentIdx].label, next);
          draw();
        },
      });
      legendEl = el.nextSibling;
    }

    mountResetZoomButton(el, function () { return plot; });
    dims.forEach(function (dim, i) {
      var btn = document.createElement("button");
      btn.type = "button";
      btn.className = "view-btn" + (i === 0 ? " active" : "");
      btn.textContent = "By " + dim.label;
      btn.title = "Re-group the thread-state chart by " + dim.label.toLowerCase() + " (e.g. host, command). Switches the chart in place.";
      btn.addEventListener("click", function () {
        if (currentIdx === i) return;
        currentIdx = i;
        buttons.forEach(function (b, j) { b.classList.toggle("active", j === i); });
        draw();
      });
      toolbar.appendChild(btn);
      buttons.push(btn);
    });

    draw();
  }

  // --- Mysqladmin filter panel + multi-chart grid ---------------
  //
  // The mysqladmin section supports multiple side-by-side charts so
  // users can compare categories or counter groups visually. A single
  // control panel (view filter, category picker, search, list) drives
  // whichever chart is currently *active*. Charts are laid out in a
  // responsive CSS grid.

  function renderMysqladmin(hostEl, data) {
    if (!data || !Array.isArray(data.timestamps) || !data.deltas || !Array.isArray(data.variables)) {
      return;
    }

    // ---- Multi-chart state ----
    var charts = new Map();   // id -> ChartState
    var activeChartId = null;
    var nextChartNum = 0;

    var LAYOUT_KEY = "mygather:" + REPORT_ID + ":mysqladmin:charts";

    function getActive() { return charts.get(activeChartId); }

    // Build the filter panel.
    var panel = document.createElement("div");
    panel.className = "ma-panel";

    // "Editing: <title>" badge makes the active target unambiguous.
    var editingBadge = document.createElement("div");
    editingBadge.className = "ma-editing";
    editingBadge.innerHTML = '<span class="k">EDITING</span><span class="v"></span>';
    var editingValueEl = editingBadge.querySelector(".v");
    panel.appendChild(editingBadge);

    // Built-in chips (filter the list only; they don't touch the chart).
    var builtinChips = [
      { k: "all",      label: "All",      filter: null },
      { k: "selected", label: "Selected", filter: function (name) { var a = getActive(); return a ? a.selected.has(name) : false; } },
      { k: "counters", label: "Counters", filter: function (name) { return data.isCounter[name]; } },
      { k: "gauges",   label: "Gauges",   filter: function (name) { return !data.isCounter[name]; } },
    ];
    // Category chips driven by the embedded metadata. Plain click
    // REPLACES the active chart's selection with the category's
    // variables; Shift/Cmd-click ADDS to the existing selection.
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
        ? "Click to load this group on the active chart · Shift/Cmd+Click to add to it"
        : "Click to filter the list");
      b.innerHTML = '<span class="lbl">' + escapeHTML(def.label) + '</span>' +
        (def.count != null ? ' <span class="ct">' + def.count + '</span>' : '');
      if (def.k === "all") b.classList.add("active");
      b.addEventListener("click", function (ev) {
        var additive = ev.shiftKey || ev.metaKey || ev.ctrlKey;
        if (kind === "category" && def.filter) {
          var active = getActive();
          if (!active) return;
          if (!additive) active.selected.clear();
          data.variables.forEach(function (n) {
            if (def.filter(n)) active.selected.add(n);
          });
          updateActiveTitle(def.label);
          persistLayout();
          // Filter list to this category as well, for context.
          Object.keys(chipButtons).forEach(function (k) { chipButtons[k].classList.remove("active"); });
          b.classList.add("active");
          state.category = def.k;
          redrawList();
          scheduleActiveRedraw();
          b.classList.add("flash");
          setTimeout(function () { b.classList.remove("flash"); }, 250);
          return;
        }
        // Built-in chips: filter-only (charts untouched).
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
    var catCurrentLabel = "";
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
      li.setAttribute("tabindex", "0");
      li.title = c.description || c.label;
      li.innerHTML =
        '<span class="opt-lbl">' + escapeHTML(c.label) + '</span>' +
        '<span class="opt-ct">' + c.count + '</span>';
      function activate() {
        catCurrentKey = c.key;
        catCurrentLabel = c.label;
        setCatLabel(c.key);
        closeCatPopup();
        // Picking a category now only filters the list below — it
        // does NOT push those counters onto the chart. The user
        // explicitly clicks "Load on chart" to apply the selection.
        filterListToCategory();
      }
      li.addEventListener("click", activate);
      li.addEventListener("keydown", function (ev) {
        if (ev.key === "Enter" || ev.key === " " || ev.key === "Spacebar") {
          ev.preventDefault();
          activate();
        }
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
      if (ev.key === "Escape" && !catPopup.hidden) {
        closeCatPopup();
        ev.preventDefault();
        ev.stopPropagation();
      }
    });

    catDD.appendChild(catToggle);
    catDD.appendChild(catPopup);

    var btnLoad = document.createElement("button");
    btnLoad.type = "button";
    btnLoad.className = "ma-action ma-cat-load";
    btnLoad.textContent = "Load on chart";
    btnLoad.title = "Replace chart selection with the picked category";

    catActions.appendChild(catDD);
    catActions.appendChild(btnLoad);
    catRow.appendChild(catLabel);
    catRow.appendChild(catActions);
    panel.appendChild(catRow);

    // Picking a category updates the list filter only — the chart
    // itself stays untouched until the user commits via "Load on
    // chart". Splitting these two signals lets a reader browse
    // what's in a category before deciding to blow away the current
    // selection.
    function filterListToCategory() {
      if (!catCurrentKey) return;
      Object.keys(chipButtons).forEach(function (k) { chipButtons[k].classList.remove("active"); });
      state.category = "cat:" + catCurrentKey;
      redrawList();
    }

    function loadCategoryOnChart() {
      if (!catCurrentKey) return;
      var fn = catFilterByKey[catCurrentKey];
      if (!fn) return;
      var active = getActive();
      if (!active) return;
      active.selected.clear();
      data.variables.forEach(function (n) {
        if (fn(n)) active.selected.add(n);
      });
      if (catCurrentLabel) updateActiveTitle(catCurrentLabel);
      persistLayout();
      // After loading, switch the list filter to 'selected' so the
      // reader immediately sees the counters that went onto the chart.
      Object.keys(chipButtons).forEach(function (k) { chipButtons[k].classList.remove("active"); });
      if (chipButtons["selected"]) chipButtons["selected"].classList.add("active");
      state.category = "selected";
      redrawList();
      scheduleActiveRedraw();
    }

    btnLoad.addEventListener("click", loadCategoryOnChart);

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
