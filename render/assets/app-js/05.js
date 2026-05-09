  // ===============================================================
  // app-js/05.js — chart sync store + windowed legend stats
  // ===============================================================
  //
  // This part lives inside the same IIFE opened in app-js/00.js. The
  // closing `})();` for the IIFE is at the bottom of THIS file so any
  // future part appended after it would land OUTSIDE the IIFE; that
  // is intentional — the IIFE contract ends here.
  //
  // The sync store implements GitHub issue #52 (synchronized x-axis
  // zoom + pan across all registered charts) and feeds the windowed
  // legend stats from issue #51 (per-series Min · Avg · Max over the
  // visible window). Both are driven from the same canonical
  // chart-registry walk so the broadcaster fires once per
  // user-initiated zoom/pan/reset and every chart updates within one
  // animation frame.
  //
  // The store is attached to window.__chartSyncStore so the existing
  // basePlotOpts setScale hook (00.js → broadcastXScaleChange) can
  // reach it without a circular import dance — there is no module
  // system in this codebase. Tests grep the embedded JS for the
  // canonical store identifier and helpers.
  //
  // Re-entrancy: the broadcaster sets isBroadcasting = true around
  // every plot.setScale("x", …) call. The basePlotOpts setScale hook
  // short-circuits when isBroadcasting is true, so the cycle "user
  // zoom → store → setScale on chart B → chart B's setScale hook →
  // store → setScale on chart A" cannot recurse.
  //
  // The HLL sparkline (render/assets/app-js/03.js → renderHLLSparkline)
  // intentionally does NOT call registerChart. That is the canonical
  // way to opt a chart out of sync; no separate "synced:false" flag
  // is introduced.

  // computeWindowedStats returns one {min, max, avg, count} record per
  // input series, computed over samples whose timestamp falls inside
  // [win.min, win.max]. When win is {null, null} (no synced zoom) the
  // entire timestamps array is scanned. null / undefined / NaN sample
  // values are skipped. When the windowed slice has zero non-null
  // samples for a series, the record is {min:null, max:null, avg:null,
  // count:0} and the legend renders "–".
  //
  // Binary search finds the index range so the per-series scan is
  // O(N_window × S) rather than O(N_total × S) per chart per zoom
  // event — important when the capture is long but the visible
  // window is narrow.
  function computeWindowedStats(timestamps, series, win) {
    var ts = timestamps || [];
    var ser = series || [];
    var n = ts.length;
    if (n === 0 || ser.length === 0) {
      return ser.map(function () { return { min: null, max: null, avg: null, count: 0 }; });
    }
    var lo = (win && win.min != null) ? win.min : ts[0];
    var hi = (win && win.max != null) ? win.max : ts[n - 1];
    if (lo > hi) { var tmp = lo; lo = hi; hi = tmp; }

    var i0 = lowerBound(ts, lo);
    var i1 = upperBound(ts, hi);
    if (i1 <= i0) {
      return ser.map(function () { return { min: null, max: null, avg: null, count: 0 }; });
    }

    return ser.map(function (s) {
      var values = s.values || [];
      var min = Infinity, max = -Infinity, sum = 0, count = 0;
      for (var i = i0; i < i1; i++) {
        var v = values[i];
        if (v === null || v === undefined) continue;
        var n2 = +v;
        if (isNaN(n2) || !isFinite(n2)) continue;
        if (n2 < min) min = n2;
        if (n2 > max) max = n2;
        sum += n2;
        count++;
      }
      if (count === 0) return { min: null, max: null, avg: null, count: 0 };
      return { min: min, max: max, avg: sum / count, count: count };
    });
  }

  // lowerBound: smallest index i such that ts[i] >= v.
  function lowerBound(ts, v) {
    var lo = 0, hi = ts.length;
    while (lo < hi) {
      var mid = (lo + hi) >>> 1;
      if (ts[mid] < v) lo = mid + 1;
      else hi = mid;
    }
    return lo;
  }

  // upperBound: smallest index i such that ts[i] > v.
  function upperBound(ts, v) {
    var lo = 0, hi = ts.length;
    while (lo < hi) {
      var mid = (lo + hi) >>> 1;
      if (ts[mid] <= v) lo = mid + 1;
      else hi = mid;
    }
    return lo;
  }

  // The shared zoom store. Singleton attached to window so the
  // basePlotOpts setScale hook (which has no closure over this scope)
  // can reach it. State and methods documented in
  // specs/017-chart-zoom-sync-stats/contracts/chart-sync.md.
  var chartSyncStore = (function () {
    var win = { min: null, max: null };
    var generation = 0;
    var subscribers = [];
    var api;

    function broadcast(sourcePlot) {
      generation++;
      api.isBroadcasting = true;
      try {
        var entries = chartRegistry();
        for (var i = 0; i < entries.length; i++) {
          var entry = entries[i];
          if (!entry || !entry.plot) continue;
          // Honour the contract (specs/017-chart-zoom-sync-stats/contracts/chart-sync.md
          // §1 setWindow): skip the source plot — uPlot's own scale on
          // it already reflects the user's drag, and re-applying the
          // window via setScale on the source is redundant work.
          if (entry.plot === sourcePlot) continue;
          if (!entry.el || !entry.el.isConnected) continue;
          try {
            entry.plot.setScale("x", { min: win.min, max: win.max });
          } catch (_) { /* uPlot can throw on a destroyed plot — skip */ }
        }
      } finally {
        api.isBroadcasting = false;
      }
      // Fire subscribers AFTER the registry walk so a subscriber that
      // recomputes legend stats sees the just-applied scales on every
      // chart, not the half-applied state mid-broadcast.
      for (var s = 0; s < subscribers.length; s++) {
        try { subscribers[s](win, generation); } catch (_) { /* subscriber bug — keep going */ }
      }
    }

    function applyToChart(entry) {
      if (!entry || !entry.plot) return;
      // No synced zoom yet — nothing to apply. Legend stats default
      // to the full data extent at first paint.
      if (win.min == null && win.max == null) return;
      api.isBroadcasting = true;
      try {
        entry.plot.setScale("x", { min: win.min, max: win.max });
      } catch (_) { /* destroyed or not yet ready — skip */ }
      api.isBroadcasting = false;
      if (typeof entry.setLegendStats === "function") {
        try { entry.setLegendStats(win); } catch (_) { /* ignore */ }
        return;
      }
      // applyToChart is invoked synchronously from registerChart, but
      // each chart-builder calls registerChart BEFORE mountLegend, so
      // entry.setLegendStats and plot.__legendHandle are not yet
      // assigned at this point. Defer one microtask so the synchronous
      // mountLegend call has had a chance to attach the handle, then
      // adopt the resolver onto the entry (mirroring the cache pattern
      // in initChartSync's subscriber). Without this, a chart that
      // mounts post-zoom (e.g. user expands a collapsed <details> after
      // zooming a sibling) keeps its legend at full-extent stats until
      // the next user interaction.
      Promise.resolve().then(function () {
        if (win.min == null && win.max == null) return;
        var handle = entry.plot && entry.plot.__legendHandle;
        if (handle && typeof handle.setStats === "function") {
          entry.setLegendStats = handle.setStats;
          try { handle.setStats(win); } catch (_) { /* ignore */ }
        }
      });
    }

    api = {
      isBroadcasting: false,
      computeWindowedStats: computeWindowedStats,
      getWindow: function () { return { min: win.min, max: win.max }; },
      setWindow: function (next, sourcePlot) {
        var nMin = (next && next.min != null) ? next.min : null;
        var nMax = (next && next.max != null) ? next.max : null;
        // Coalesce identical windows so a chart whose own setScale
        // hook fires with the same numbers we just broadcast doesn't
        // trigger a redundant pass through every other chart.
        if (nMin === win.min && nMax === win.max) return;
        win = { min: nMin, max: nMax };
        // Per the contract, the registry walk skips the source plot —
        // uPlot's own scale on it already reflects the user's drag, so
        // re-applying the window to it would be redundant work and
        // makes the contract/impl drift the reviewer flagged.
        broadcast(sourcePlot);
      },
      reset: function () {
        // Restore each chart to its own data extent so charts with
        // narrower data than the previously-synced window come back
        // to their full range, not the broadcast window.
        api.isBroadcasting = true;
        try {
          var entries = chartRegistry();
          for (var i = 0; i < entries.length; i++) {
            var entry = entries[i];
            if (!entry || !entry.plot) continue;
            try {
              var xs = entry.plot.data && entry.plot.data[0];
              if (xs && xs.length) {
                entry.plot.setScale("x", { min: xs[0], max: xs[xs.length - 1] });
              } else {
                entry.plot.setScale("x", { min: null, max: null });
              }
            } catch (_) { /* skip destroyed plot */ }
          }
        } finally {
          api.isBroadcasting = false;
        }
        win = { min: null, max: null };
        generation++;
        for (var s = 0; s < subscribers.length; s++) {
          try { subscribers[s](win, generation); } catch (_) { /* ignore */ }
        }
      },
      subscribe: function (fn) {
        if (typeof fn !== "function") return function () {};
        subscribers.push(fn);
        return function unsubscribe() {
          for (var i = subscribers.length - 1; i >= 0; i--) {
            if (subscribers[i] === fn) subscribers.splice(i, 1);
          }
        };
      },
      applyToChart: applyToChart,
    };
    return api;
  })();

  // Publish the store before initCharts runs so the basePlotOpts
  // setScale hook (and registerChart's applyToChart probe) can find
  // it on first paint of the very first chart.
  window.__chartSyncStore = chartSyncStore;

  // initChartSync wires the windowed-stats subscriber once initCharts
  // has populated the registry. Called from boot() in app-js/04.js.
  function initChartSync() {
    // First pass: fire setStats({null, null}) on every chart so the
    // legend renders full-extent stats at first paint without waiting
    // for a user interaction.
    var entries = chartRegistry();
    for (var i = 0; i < entries.length; i++) {
      var entry = entries[i];
      if (!entry || !entry.plot) continue;
      var handle = entry.plot.__legendHandle;
      if (handle && typeof handle.setStats === "function") {
        entry.setLegendStats = handle.setStats;
        try { handle.setStats({ min: null, max: null }); } catch (_) { /* ignore */ }
      }
    }
    // Subscribe once: every store update walks the registry and pings
    // each chart's legend handle (re-resolved on each tick because
    // vmstat tab rebuilds and similar replace the plot underneath).
    chartSyncStore.subscribe(function (win) {
      var ents = chartRegistry();
      for (var i = 0; i < ents.length; i++) {
        var ent = ents[i];
        if (!ent || !ent.plot) continue;
        var h = ent.plot.__legendHandle;
        if (h && typeof h.setStats === "function") {
          // Cache the resolver on the entry so chart-mount timing
          // (applyToChart for a chart that mounted post-zoom) and
          // store updates use the same code path.
          ent.setLegendStats = h.setStats;
          try { h.setStats(win); } catch (_) { /* ignore */ }
        }
      }
    });
  }
})();
