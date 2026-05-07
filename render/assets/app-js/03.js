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
    var targets = document.querySelectorAll(MAIN_SECTIONS_SELECTOR);
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

  // --- Advisor severity filter ----------------------------------
  //
  // The counts strip at the top of the Advisor section doubles as a
  // multi-select severity filter. Each chip is an independent toggle:
  // when a severity is off, findings of that class are hidden, and any
  // subsystem group whose children are *all* hidden disappears too.
  // Filter state persists under a ReportID-scoped localStorage key so
  // repeated visits to the same report remember what the reader last
  // focused on.
  function initAdvisorFilter() {
    var summary = document.querySelector(".advisor-summary");
    if (!summary) return;
    var chips = summary.querySelectorAll(".advisor-count");
    if (!chips.length) return;

    // v2 key: new default (Crit + Warn only). The older v1 key used
    // all-severities-on as default, so we bump the namespace rather
    // than migrate silently — otherwise returning readers would see
    // the legacy "everything on" state and never notice the new
    // default-to-signal behaviour.
    var STORAGE_KEY = "mygather:v2:" + REPORT_ID + ":advisor-filter";
    // Default on first load: focus the reader on actionable severities.
    var active = { crit: true, warn: true, info: false, ok: false };
    try {
      var raw = localStorage.getItem(STORAGE_KEY);
      if (raw) {
        var parsed = JSON.parse(raw);
        if (parsed && typeof parsed === "object") {
          ["crit", "warn", "info", "ok"].forEach(function (k) {
            if (typeof parsed[k] === "boolean") active[k] = parsed[k];
          });
        }
      }
    } catch (_) { /* ignore corrupt state */ }

    // Empty-state banner shown when the active filter matches no
    // findings — typically when the default (Crit+Warn) is applied
    // to a capture where the only findings are Info/OK.
    var groupsContainer = document.querySelector(".advisor-groups");
    var emptyBanner = null;
    if (groupsContainer) {
      emptyBanner = document.createElement("p");
      emptyBanner.className = "banner advisor-empty-filter";
      emptyBanner.hidden = true;
      emptyBanner.textContent =
        "No Critical or Warning findings match the current filter — " +
        "toggle Info or OK above to see other findings.";
      groupsContainer.parentNode.insertBefore(emptyBanner, groupsContainer);
    }

    function persist() {
      try { localStorage.setItem(STORAGE_KEY, JSON.stringify(active)); } catch (_) {}
    }

    function apply() {
      // Toggle each chip's active class so the visual state matches.
      chips.forEach(function (chip) {
        var sev = chip.getAttribute("data-sev");
        var on = !!active[sev];
        chip.classList.toggle("is-active", on);
        chip.setAttribute("aria-pressed", on ? "true" : "false");
      });
      // Show/hide findings by severity.
      var findings = document.querySelectorAll(".finding[data-sev]");
      var visibleFindings = 0;
      findings.forEach(function (f) {
        var sev = f.getAttribute("data-sev");
        var hidden = !active[sev];
        f.classList.toggle("is-hidden", hidden);
        if (!hidden) visibleFindings++;
      });
      // Hide subsystem groups with no visible children.
      var groups = document.querySelectorAll(".advisor-group");
      groups.forEach(function (g) {
        var visible = g.querySelectorAll(".finding:not(.is-hidden)").length;
        g.classList.toggle("is-hidden", visible === 0);
      });
      if (emptyBanner) {
        emptyBanner.hidden = visibleFindings !== 0 || findings.length === 0;
      }
    }

    chips.forEach(function (chip) {
      chip.addEventListener("click", function () {
        var sev = chip.getAttribute("data-sev");
        active[sev] = !active[sev];
        apply();
        persist();
      });
    });

    apply();
  }

  // --- Print-expand hook ----------------------------------------

  // Semaphore contention-breakdown panel wiring: tab switcher
  // between "At peak" and "Over window" views, plus the
  // "Show all (N sites)" button that reveals rows past the top-10.
  // Both interactions are strictly local to the <details> block
  // rendered by db.html.tmpl; no cross-card state.
  function initSemaphoreBreakdown() {
    var blocks = document.querySelectorAll("details.semaphore-breakdown");
    for (var i = 0; i < blocks.length; i++) {
      (function (block) {
        var tabs = block.querySelectorAll(".cb-tab");
        var views = block.querySelectorAll(".cb-tabview");
        tabs.forEach(function (tab) {
          tab.addEventListener("click", function () {
            var key = tab.getAttribute("data-view");
            tabs.forEach(function (t) {
              var on = t === tab;
              t.classList.toggle("active", on);
              t.setAttribute("aria-selected", on ? "true" : "false");
            });
            views.forEach(function (v) {
              v.hidden = v.getAttribute("data-view") !== key;
            });
          });
        });
        block.querySelectorAll(".cb-more").forEach(function (btn) {
          btn.addEventListener("click", function () {
            var scope = btn.getAttribute("data-scope");
            var view = block.querySelector('.cb-tabview[data-view="' + scope + '"]');
            if (!view) return;
            view.querySelectorAll("tr.cb-tail").forEach(function (row) { row.hidden = false; });
            btn.hidden = true;
          });
        });
      })(blocks[i]);
    }
  }

  // initEnvTabs wires the Host / MySQL tab switcher in the Environment
  // section. Mirrors the ARIA tablist pattern used by vmstat. Selection
  // persists in localStorage under the v2 namespace so reopening the
  // report remembers which panel was on screen.
  function initEnvTabs() {
    var roots = document.querySelectorAll("[data-env-root]");
    if (!roots.length) return;
    var KEY = "mygather:v2:" + REPORT_ID + ":env-tab";
    roots.forEach(function (root) {
      var tabs   = root.querySelectorAll("[role='tab'][data-env-tab]");
      var panels = root.querySelectorAll("[role='tabpanel'][data-env-panel]");
      function select(which, opts) {
        opts = opts || {};
        var ok = false;
        tabs.forEach(function (t) {
          var on = t.getAttribute("data-env-tab") === which;
          if (on) ok = true;
          t.classList.toggle("active", on);
          t.setAttribute("aria-selected", on ? "true" : "false");
          t.setAttribute("tabindex", on ? "0" : "-1");
          if (on && opts.focus) { try { t.focus({ preventScroll: true }); } catch (_) { t.focus(); } }
        });
        if (!ok) return;
        panels.forEach(function (p) {
          p.hidden = p.getAttribute("data-env-panel") !== which;
        });
        try { storageSet(KEY, which); } catch (_) {}
      }
      tabs.forEach(function (t) {
        t.addEventListener("click", function () { select(t.getAttribute("data-env-tab")); });
      });
      root.addEventListener("keydown", function (ev) {
        if (ev.target.getAttribute("role") !== "tab") return;
        var order = Array.from(tabs);
        var idx = order.indexOf(ev.target);
        if (idx < 0) return;
        var next = null;
        if      (ev.key === "ArrowRight") next = order[(idx + 1) % order.length];
        else if (ev.key === "ArrowLeft")  next = order[(idx - 1 + order.length) % order.length];
        else if (ev.key === "Home")       next = order[0];
        else if (ev.key === "End")        next = order[order.length - 1];
        if (next) { ev.preventDefault(); select(next.getAttribute("data-env-tab"), { focus: true }); }
      });
      var saved = null;
      try { saved = storageGet(KEY); } catch (_) {}
      // "mysql only" captures (host sidecars absent/unreadable but
      // -variables parsed) leave the Host panel as an all-"—" block
      // and the MySQL panel populated. If we blindly default to host
      // the reader lands on an empty view and can misread the whole
      // Environment section as unavailable. Detect panel emptiness
      // (no <dd> with content other than the missing marker) and
      // prefer the populated side when no persisted choice exists.
      function panelHasData(which) {
        var panel = root.querySelector("[data-env-panel='" + which + "']");
        if (!panel) return false;
        var dds = panel.querySelectorAll("dd");
        for (var i = 0; i < dds.length; i++) {
          var txt = (dds[i].textContent || "").trim();
          if (txt && txt !== "—") return true;
        }
        return false;
      }
      var initial = "host";
      if (saved === "host" || saved === "mysql") {
        initial = saved;
      } else if (!panelHasData("host") && panelHasData("mysql")) {
        initial = "mysql";
      }
      // Template renders both panels visible so no-JS readers can reach
      // MySQL content. Always call select() once JS is running so the
      // inactive panel is hidden and the tab UX matches other tablists.
      select(initial);
    });
  }

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

  // --- vmstat category tabs ---------------------------------------
  //
  // The OS section renders a single uPlot for vmstat. Rather than
  // cramming every column into one legend, the template ships a
  // tablist above the chart container; clicking a tab tears down the
  // existing plot and rebuilds it with only that category's series.
  // Tab selection persists per-report under a stable v2 localStorage
  // key so a reader's choice survives reloads. Category → series
  // mapping is done client-side against the labels the Go payload
  // already emits (e.g. "cpu_user", "free_kb"); unknown labels are
  // ignored so the chart degrades cleanly if a future vmstat variant
  // adds columns we don't classify.
  var VMSTAT_CATEGORIES = {
    // Series names here must match what vmstatChartPayload emits (see
    // parse/vmstat.go's vmstatCols table). Any label listed that the
    // parser doesn't emit is ignored at render time, so future columns
    // just need to be added here AND to vmstatCols.
    cpu:    ["cpu_user", "cpu_sys", "cpu_idle", "cpu_iowait"],
    memory: ["free_kb", "buff_kb", "cache_kb"],
    io:     ["io_in", "io_out", "swap_in", "swap_out"],
    procs:  ["runqueue", "blocked"],
  };
  function vmstatCategoryKey() {
    return "mygather:v2:" + REPORT_ID + ":vmstat-tab";
  }
  function filterVmstatByCategory(data, cat) {
    var allow = VMSTAT_CATEGORIES[cat] || [];
    var filtered = [];
    for (var i = 0; i < data.series.length; i++) {
      if (allow.indexOf(data.series[i].label) !== -1) {
        filtered.push(data.series[i]);
      }
    }
    return {
      timestamps:         data.timestamps,
      series:             filtered,
      snapshotBoundaries: data.snapshotBoundaries,
    };
  }
  function renderVmstatTabs(el, data) {
    if (!data || !Array.isArray(data.timestamps) || !Array.isArray(data.series)) return;
    // Locate the sibling tablist emitted deterministically by os.html.tmpl.
    var tablist = el.parentNode && el.parentNode.querySelector
      ? el.parentNode.querySelector(".vmstat-tablist[role=\"tablist\"]")
      : null;
    if (!tablist) {
      // Fallback: no tablist markup present — behave like the old flat chart.
      renderTimeSeries(el, data, unitForChart("vmstat"));
      return;
    }
    var tabs = tablist.querySelectorAll("[role=\"tab\"][data-vmstat-category]");
    var DEFAULT_CAT = "cpu";
    var stored = storageGet(vmstatCategoryKey());
    var initial = DEFAULT_CAT;
    if (stored && VMSTAT_CATEGORIES[stored]) initial = stored;

    var plot = null;
    function cleanup() {
      if (plot) { unregisterChart(plot); plot.destroy(); plot = null; }
      // Legend is mounted as el's immediate next sibling by mountLegend.
      var legendEl = el.nextSibling;
      if (legendEl && legendEl.classList && legendEl.classList.contains("series-legend")) {
        legendEl.parentNode.removeChild(legendEl);
      }
      // Also strip any prior reset-zoom button mount — mountResetZoomButton
      // wraps the chart; if the wrapping moved, skip silently.
    }
    function draw(cat) {
      cleanup();
      var filtered = filterVmstatByCategory(data, cat);
      if (filtered.series.length === 0) {
        // Nothing to plot for this category (e.g. pt-stalk vmstat variant
        // without `in`/`cs`). Leave the container empty with a caption.
        var msg = document.createElement("p");
        msg.className = "banner missing vmstat-empty";
        msg.textContent = "No series available for this category in the captured vmstat.";
        el.appendChild(msg);
        return;
      }
      plot = buildLineChart(el, filtered, unitForChart("vmstat"));
    }
    function selectTab(cat, opts) {
      if (!VMSTAT_CATEGORIES[cat]) cat = DEFAULT_CAT;
      for (var i = 0; i < tabs.length; i++) {
        var isActive = tabs[i].getAttribute("data-vmstat-category") === cat;
        tabs[i].classList.toggle("active", isActive);
        tabs[i].setAttribute("aria-selected", isActive ? "true" : "false");
        tabs[i].setAttribute("tabindex", isActive ? "0" : "-1");
        if (isActive && opts && opts.focus) tabs[i].focus();
      }
      storageSet(vmstatCategoryKey(), cat);
      // Wipe any lingering empty-state banner before draw.
      var empties = el.querySelectorAll(".vmstat-empty");
      for (var k = 0; k < empties.length; k++) empties[k].parentNode.removeChild(empties[k]);
      draw(cat);
    }
    for (var i = 0; i < tabs.length; i++) {
      (function (btn) {
        btn.addEventListener("click", function () {
          selectTab(btn.getAttribute("data-vmstat-category"));
        });
      })(tabs[i]);
    }
    tablist.addEventListener("keydown", function (ev) {
      var order = [];
      for (var j = 0; j < tabs.length; j++) order.push(tabs[j]);
      var idx = order.indexOf(document.activeElement);
      if (idx < 0) idx = 0;
      if (ev.key === "ArrowRight") {
        ev.preventDefault();
        selectTab(order[(idx + 1) % order.length].getAttribute("data-vmstat-category"), { focus: true });
      } else if (ev.key === "ArrowLeft") {
        ev.preventDefault();
        selectTab(order[(idx - 1 + order.length) % order.length].getAttribute("data-vmstat-category"), { focus: true });
      } else if (ev.key === "Home") {
        ev.preventDefault();
        selectTab(order[0].getAttribute("data-vmstat-category"), { focus: true });
      } else if (ev.key === "End") {
        ev.preventDefault();
        selectTab(order[order.length - 1].getAttribute("data-vmstat-category"), { focus: true });
      }
    });
    selectTab(initial);
    mountResetZoomButton(el, function () { return plot; });
  }

  // --- History list length sparkline ------------------------------
  //
  // Small uPlot rendered directly under the "History list" InnoDB
  // callout. 1-sample fallback shows a single centred dot with its
  // formatted value; 2 samples draw a thin segment with start/end
  // value labels; 3+ samples render a filled smooth spline. Reuses
  // the bundled uPlot instance — no extra chart library.
  function renderHLLSparkline(el, data) {
    if (!data || !Array.isArray(data.values) || data.values.length === 0) return;
    var vals = data.values;
    var ts   = Array.isArray(data.timestamps) ? data.timestamps : null;
    // Fit the sparkline to its container (the surrounding .callout card).
    // Both dimensions are dynamic so the chart fills whatever box the
    // flex column grants it — avoids the narrow-strip look when the
    // sibling callouts (Semaphores / Pending I/O) stretch the grid row.
    function measureW() {
      var w = el.clientWidth || (el.parentNode && el.parentNode.clientWidth) || 0;
      if (!w || w < 180) w = 240;
      if (w > 720) w = 720;
      return w;
    }
    function measureH() {
      var h = el.clientHeight || 0;
      if (!h || h < 80) h = 80;
      if (h > 260) h = 260;
      return h;
    }
    var W = measureW(), H = measureH();
    // Clear any prior render (idempotent if initCharts re-runs).
    while (el.firstChild) el.removeChild(el.firstChild);
    el.classList.add("hll-sparkline-ready");

    function fmt(v) {
      if (v == null || isNaN(v)) return "–";
      v = +v;
      if (v >= 1e6) return (v / 1e6).toFixed(1) + "M";
      if (v >= 1e3) return (v / 1e3).toFixed(1) + "k";
      return String(Math.round(v));
    }

    if (vals.length === 1) {
      // Single sample: render a centred dot + label via plain DOM.
      // Avoids inline SVG namespaces (render tests forbid any plain
      // http-scheme substring in the output to guard against accidental
      // network fetches).
      var single = document.createElement("div");
      single.className = "hll-spark-single";
      single.style.width  = "100%";
      single.style.height = H + "px";
      var dot = document.createElement("span");
      dot.className = "hll-spark-dot";
      single.appendChild(dot);
      var lbl = document.createElement("span");
      lbl.className = "hll-spark-label";
      lbl.textContent = fmt(vals[0]);
      single.appendChild(lbl);
      el.appendChild(single);
      return;
    }

    // Compute y range with a small pad so the line isn't flush with edges.
    var mn = Infinity, mx = -Infinity;
    for (var i = 0; i < vals.length; i++) {
      var v = +vals[i];
      if (!isNaN(v)) { if (v < mn) mn = v; if (v > mx) mx = v; }
    }
    if (mn === Infinity) { mn = 0; mx = 1; }
    if (mn === mx) { mn -= 1; mx += 1; }

    // Synthesize timestamps if missing (shouldn't happen, but safe).
    var xs = ts && ts.length === vals.length
      ? ts.slice()
      : vals.map(function (_, idx) { return idx; });

    var splinePath = (typeof uPlot !== "undefined" && uPlot.paths && uPlot.paths.spline)
      ? uPlot.paths.spline()
      : null;

    // Tooltip element (shared for 2+ samples). Positioned absolutely
    // inside the sparkline container and updated via cursor hook.
    el.style.position = el.style.position || "relative";
    var tip = document.createElement("div");
    tip.className = "hll-spark-tip";
    tip.style.display = "none";
    el.appendChild(tip);

    // Min / max reference labels (always visible, corner-anchored).
    var refs = document.createElement("div");
    refs.className = "hll-spark-refs";
    refs.innerHTML =
      '<span class="hll-spark-max">max ' + escapeHTML(fmt(mx)) + '</span>' +
      '<span class="hll-spark-min">min ' + escapeHTML(fmt(mn)) + '</span>';
    el.appendChild(refs);

    function fmtTs(sec) {
      if (!ts || sec == null) return "";
      var d = new Date(sec * 1000);
      var hh = String(d.getHours()).padStart(2, "0");
      var mm = String(d.getMinutes()).padStart(2, "0");
      var ss = String(d.getSeconds()).padStart(2, "0");
      return hh + ":" + mm + ":" + ss;
    }

    // Single-series sparkline. Stroke flows from --series-1 (slot 0
     // via seriesStrokeFor) so the sparkline tracks the active theme;
     // __themeIdx + __themeFillBuilder let the runtime theme-change
     // handler in app-js/00.js re-paint this chart on theme switch.
    var sparkStroke = seriesStrokeFor(0);
    var opts = {
      width: W,
      height: H,
      padding: [6, 6, 6, 6],
      cursor: {
        show: true,
        drag: { x: false, y: false },
        points: { show: true, size: 7, stroke: sparkStroke, fill: cssVar("--bg", "#0b1220") },
        y: false,
      },
      legend: { show: false },
      scales: {
        x: { time: ts != null },
        y: { auto: false, range: [mn - (mx - mn) * 0.15, mx + (mx - mn) * 0.15] },
      },
      axes: [{ show: false }, { show: false }],
      series: [
        { label: "t" },
        {
          label: "HLL",
          stroke: sparkStroke,
          width: 1.25,
          fill:  hexToRgba(sparkStroke, 0.18),
          paths: splinePath || undefined,
          points: vals.length === 2 ? { show: true, size: 4 } : { show: false },
          __themeIdx: 0,
          __themeFillBuilder: function (s) { return hexToRgba(s, 0.18); },
        },
      ],
      hooks: {
        setCursor: [
          function (u) {
            var idx = u.cursor.idx;
            if (idx == null || idx < 0 || idx >= vals.length) {
              tip.style.display = "none";
              return;
            }
            var v = vals[idx];
            var t = ts ? ts[idx] : null;
            tip.innerHTML =
              '<strong>' + escapeHTML(fmt(v)) + '</strong>' +
              (t != null
                ? '<span class="hll-spark-tip-sep">·</span><span>' + escapeHTML(fmtTs(t)) + '</span>'
                : '');
            tip.style.display = "block";
            // Position tip near cursor, clamped inside the sparkline rect.
            var left = u.cursor.left;
            var tipW = tip.offsetWidth || 80;
            var maxLeft = W - tipW - 4;
            if (left > maxLeft) left = maxLeft;
            if (left < 4) left = 4;
            tip.style.left = left + "px";
          },
        ],
      },
    };
    var plot = new uPlot(opts, [xs, vals.map(function (v) { return +v; })], el);
    // Hide tooltip when leaving the sparkline entirely.
    el.addEventListener("mouseleave", function () { tip.style.display = "none"; });
    // Resize to follow its .callout container (page zoom, grid reflow).
    if (typeof ResizeObserver !== "undefined") {
      var ro = new ResizeObserver(function () {
        var nw = measureW();
        var nh = measureH();
        if (Math.abs(nw - W) > 2 || Math.abs(nh - H) > 2) {
          W = nw;
          H = nh;
          plot.setSize({ width: W, height: H });
        }
      });
      ro.observe(el);
    }
    // Do NOT registerChart — sparkline follows its own container and
    // should not participate in global reset-zoom flows.
    if (vals.length === 2) {
      // Label the two endpoints with their values so a two-snapshot
      // capture still reads as a trend rather than a bare stub.
      var overlay = document.createElement("div");
      overlay.className = "hll-spark-endpoints";
      overlay.innerHTML =
        '<span class="hll-spark-start">' + escapeHTML(fmt(vals[0])) + '</span>' +
        '<span class="hll-spark-end">'   + escapeHTML(fmt(vals[vals.length - 1])) + '</span>';
      el.appendChild(overlay);
    }
    // Prevent "unused plot" linter concerns — handle kept for future reuse.
    void plot;
  }

  // --- Feedback dialog ---------------------------------------------
  // Wires the static <dialog id="feedback-dialog"> emitted by the
  // template. See specs/002-report-feedback-button and
  // specs/003-feedback-backend-worker. One submit code path
  // (Principle XIII): doSubmit() POSTs JSON to the Worker; on any
  // failure it calls doLegacyFallback() (feature-002 window.open +
  // clipboard/download handoff) so the feature degrades gracefully
  // (Principle III). Static-node mutations are limited to form
  // values, `disabled`, `hidden`, `href`, `textContent`.
  function initFeedbackDialog() {
    var dialog = document.getElementById("feedback-dialog");
    var openBtn = document.getElementById("feedback-open");
    if (!dialog || !openBtn || typeof dialog.showModal !== "function") return;
    var form = document.getElementById("feedback-form");
    var titleInput = document.getElementById("feedback-field-title");
    var bodyInput = document.getElementById("feedback-field-body");
    var catSelect = document.getElementById("feedback-field-category");
    var submitBtn = document.getElementById("feedback-submit");
    var cancelBtn = document.getElementById("feedback-cancel");
    var recordBtn = document.getElementById("feedback-record");
    var attachments = document.getElementById("feedback-attachments");
    var hint = document.getElementById("feedback-hint");
    var errEl = document.getElementById("feedback-error");
    var fallback = document.getElementById("feedback-fallback");
    var fallbackLink = document.getElementById("feedback-fallback-link");
    var successEl = document.getElementById("feedback-success");
    var successLink = document.getElementById("feedback-success-link");
    var successCloseBtn = document.getElementById("feedback-success-close");

    function parseFeedbackContract(raw) {
      if (!raw) return null;
      try {
        var contract = JSON.parse(raw);
        var limits = contract && contract.limits;
        if (!contract.githubUrl || !contract.workerUrl || !limits) return null;
        if (!Array.isArray(contract.categories) || !contract.categories.length) return null;
        if (typeof limits.titleMaxChars !== "number" ||
            typeof limits.bodyMaxBytes !== "number" ||
            typeof limits.reportVersionMaxChars !== "number" ||
            typeof limits.legacyUrlMaxChars !== "number" ||
            typeof limits.workerTimeoutMs !== "number") return null;
        return contract;
      } catch (_) {
        return null;
      }
    }

    // Single source of truth for URLs and validation limits: rendered
    // onto the <dialog> by the Go template from the canonical feedback
    // contract JSON (Principle XIII).
    var CONTRACT = parseFeedbackContract(dialog.dataset.feedbackContract);
    if (!CONTRACT) return;
    var LIMITS = CONTRACT.limits;
    var BASE_URL = CONTRACT.githubUrl;
    var WORKER_URL = CONTRACT.workerUrl;
    var URL_MAX = LIMITS.legacyUrlMaxChars;
    var WORKER_TIMEOUT_MS = LIMITS.workerTimeoutMs;

    var imgBlob = null, imgURL = null;
    var voiceBlob = null, voiceURL = null;
    var recorder = null, recStream = null, recChunks = null;
    // idempotencyKey persists across doSubmit retries of the SAME
    // logical submission attempt. Without this, a timeout/network
    // error on the first POST would let the user re-click Submit
    // and the Worker — whose 5-minute replay cache is keyed on the
    // idempotencyKey — would see a new key and create a duplicate
    // issue in the common case where the first request actually
    // reached the backend but the client gave up waiting. The key
    // is minted lazily inside doSubmit the first time it's needed,
    // cleared when the submission lands a definitive success
    // (renderSuccess) and when the dialog is dismissed (closeDialog)
    // — both signals mean the next submit is a different logical
    // attempt that deserves a fresh key.
    var idempotencyKey = null;
    // Monotonic session counter: bumped by startRecording (claims a
    // fresh session) and closeDialog (invalidates any in-flight
    // permission grant). stopRecording does NOT bump because it runs
    // after the stream is already attached (past the getUserMedia
    // session guard), so there's no stale-promise to invalidate at
    // that point. Captured in the getUserMedia promise's `.then` so a
    // delayed permission grant that lands on a dismissed dialog can
    // detect it and tear down the stream instead of attaching a
    // recorder to a closed session.
    var recSessionId = 0;
    var recStart = 0, recRAF = 0, recLabel = null;

    function show(el) { if (el) el.hidden = false; }
    function hide(el) { if (el) el.hidden = true; }
    function setErr(msg) {
      if (!msg) { errEl.textContent = ""; hide(errEl); return; }
      errEl.textContent = msg; show(errEl);
    }
    function shortID() {
      try {
        if (window.crypto && typeof window.crypto.randomUUID === "function") {
          return window.crypto.randomUUID().slice(0, 8);
        }
      } catch (_) { /* fall through */ }
      return Math.random().toString(36).slice(2, 10);
    }
    function extFromMime(m) { return (m && m.indexOf("mp4") !== -1) ? "mp4" : "webm"; }
    // generateIdempotencyKey emits an RFC4122 v4 UUID per call. The
    // native crypto.randomUUID path handles modern browsers; the
    // fallback fills 16 bytes from crypto.getRandomValues and
    // formats them with the v4 variant bits so the key is still
    // unique per submission. A constant fallback (previous code)
    // collided across every submission in browsers that ship fetch
    // but lack randomUUID (older WebViews), letting the worker's
    // 5-minute idempotency cache return another user's cached
    // success or a stale duplicate_inflight — i.e., lost
    // submissions and cross-user response mixups.
    function generateIdempotencyKey() {
      try {
        if (window.crypto && typeof window.crypto.randomUUID === "function") {
          return window.crypto.randomUUID();
        }
        if (window.crypto && typeof window.crypto.getRandomValues === "function") {
          var b = new Uint8Array(16);
          window.crypto.getRandomValues(b);
          b[6] = (b[6] & 0x0f) | 0x40; // version 4
          b[8] = (b[8] & 0x3f) | 0x80; // variant 1 (RFC 4122)
          var h = "";
          for (var i = 0; i < 16; i++) h += (b[i] + 0x100).toString(16).slice(1);
          return h.slice(0, 8) + "-" + h.slice(8, 12) + "-" + h.slice(12, 16) + "-" +
                 h.slice(16, 20) + "-" + h.slice(20, 32);
        }
      } catch (_) { /* fall through */ }
      // Math.random is not cryptographically strong but still emits
      // a unique-per-call value — strictly better than a constant
      // fallback for idempotency-key purposes. Reachable only on
      // environments where crypto.getRandomValues is also absent.
      function rhex(n) {
        var s = "";
        for (var i = 0; i < n; i++) s += Math.floor(Math.random() * 16).toString(16);
        return s;
      }
      return rhex(8) + "-" + rhex(4) + "-4" + rhex(3) + "-" +
             ((Math.floor(Math.random() * 4) + 8).toString(16)) + rhex(3) + "-" + rhex(12);
    }
    function maybePrefixBody() {
      var cat = catSelect.value;
      return cat ? "> Category: " + cat + "\n\n" + bodyInput.value : bodyInput.value;
    }
    function buildURL() {
      var sep = BASE_URL.indexOf("?") === -1 ? "?" : "&";
      return BASE_URL + sep +
        "title=" + encodeURIComponent(titleInput.value) +
        "&body=" + encodeURIComponent(maybePrefixBody());
    }
    function refreshHint() {
      // Worker path: the hint doubles as a small "attachments will be
      // uploaded" reassurance so the user knows the media is part of
      // the submission and not lost in clipboard limbo.
      var items = [];
      if (imgBlob) items.push("image");
      if (voiceBlob) items.push("voice note");
      if (!items.length) { hint.textContent = ""; hide(hint); return; }
      hint.textContent = "Your " + items.join(" and ") +
        (items.length === 1 ? " will be attached to the issue." : " will be attached to the issue.");
      show(hint);
    }
    function updateSubmitEnabled() {
      // URL_MAX guards the LEGACY window.open pre-fill path where the
      // title+body ride as query parameters under GitHub's ~8 KB URL
      // cap. The Worker path accepts a ~10 KB body as plain JSON —
      // much more headroom — so gating Submit on the encoded URL
      // length there would block genuinely-valid submissions whose
      // bodies fit the Worker but bloat past the URL budget (long
      // paragraphs, code blocks, etc). Gate by the runtime path.
      var titleLen = titleInput.value.trim().length;
      if (titleLen === 0) { submitBtn.disabled = true; return; }
      if (typeof window.fetch === "function") {
        // Worker path: cap by the backend's real limits from the
        // canonical feedback contract. Leaving the button enabled
        // past the shared caps would let the user submit a payload
        // the Worker immediately rejects with 400 validation errors.
        // Use TextEncoder for exact UTF-8 size (multi-byte glyphs
        // don't collapse to .length).
        var titleOK = titleInput.value.length <= LIMITS.titleMaxChars;
        var bodyBytes = new TextEncoder().encode(bodyInput.value).byteLength;
        submitBtn.disabled = !(titleOK && bodyBytes <= LIMITS.bodyMaxBytes);
      } else {
        // Legacy fallback path — gate by GitHub URL length.
        submitBtn.disabled = buildURL().length > URL_MAX;
      }
    }

    // --- Attachments (single helper, kind-parameterised) ----------

    function clearAttachment(kind) {
      // Attachment mutation changes the payload the Worker would
      // cache, so invalidate the sticky idempotencyKey so the next
      // Submit mints a fresh key instead of replaying the old
      // issueUrl against edited content. Covers addAttachment too,
      // which starts by calling clearAttachment(kind).
      idempotencyKey = null;
      if (kind === "image") {
        if (imgURL) { try { URL.revokeObjectURL(imgURL); } catch (_) {} }
        imgBlob = null; imgURL = null;
      } else {
        if (voiceURL) { try { URL.revokeObjectURL(voiceURL); } catch (_) {} }
        voiceBlob = null; voiceURL = null;
      }
      var n = attachments.querySelector('[data-kind="' + kind + '"]');
      if (n) n.parentNode.removeChild(n);
      var dl = attachments.querySelector('[data-kind="image-download"]');
      if (kind === "image" && dl) dl.parentNode.removeChild(dl);
      refreshHint(); updateSubmitEnabled();
    }
    function addAttachment(kind, blob) {
      clearAttachment(kind);
      var url = URL.createObjectURL(blob);
      var media;
      if (kind === "image") {
        imgBlob = blob; imgURL = url;
        media = document.createElement("img");
        media.alt = "Pasted screenshot preview";
      } else {
        voiceBlob = blob; voiceURL = url;
        media = document.createElement("audio");
        media.controls = true;
      }
      media.src = url;
      var wrap = document.createElement("div");
      // Base class + per-kind modifier (see contracts/ui.md).
      wrap.className = "feedback-attachment feedback-attachment--" + kind;
      wrap.setAttribute("data-kind", kind);
      var rm = document.createElement("button");
      rm.type = "button";
      rm.className = "feedback-attachment-remove";
      rm.setAttribute("aria-label", kind === "image" ? "Remove image" : "Remove voice note");
      rm.textContent = "×";
      rm.addEventListener("click", function () { clearAttachment(kind); });
      if (kind === "image") {
        // Wrap the <img> in a thumb-wrap so CSS can contain it inside
        // a bordered card without ripping the aspect ratio. The remove
        // button is positioned absolutely over the top-right corner.
        var thumbWrap = document.createElement("div");
        thumbWrap.className = "feedback-thumb-wrap";
        media.className = "feedback-thumb";
        thumbWrap.appendChild(media);
        wrap.appendChild(thumbWrap);
        wrap.appendChild(rm);
      } else {
        media.className = "feedback-audio";
        wrap.appendChild(media);
        wrap.appendChild(rm);
      }
      attachments.appendChild(wrap);
      refreshHint(); updateSubmitEnabled();
    }

    // --- Recording ------------------------------------------------

    function stopRecording() {
      if (recRAF) { cancelAnimationFrame(recRAF); recRAF = 0; }
      if (recorder && recorder.state === "recording") {
        try { recorder.stop(); } catch (_) {}
      }
      if (recStream) {
        try { recStream.getTracks().forEach(function (t) { t.stop(); }); } catch (_) {}
        recStream = null;
      }
      // Remove the whole .feedback-attachment--recording wrapper, not
      // just the timer span, so the pulsing dot and Stop button come
      // off together.
      if (recLabel && recLabel.parentNode) {
        var w = recLabel.parentNode;
        if (w.parentNode) w.parentNode.removeChild(w);
      }
      recLabel = null;
      recordBtn.disabled = false;
    }
    function tickTimer() {
      if (!recLabel) return;
      var elapsed = (performance.now() - recStart) / 1000;
      if (elapsed >= 600) { stopRecording(); return; }
      var m = Math.floor(elapsed / 60);
      var s = Math.floor(elapsed % 60);
      recLabel.textContent = m + ":" + (s < 10 ? "0" : "") + s + " / 10:00";
      recRAF = requestAnimationFrame(tickTimer);
    }
    function startRecording() {
      setErr("");
      if (!navigator.mediaDevices || !navigator.mediaDevices.getUserMedia ||
          typeof window.MediaRecorder === "undefined") {
        setErr("Voice recording is not supported by this browser."); return;
      }
      recordBtn.disabled = true;
      // Claim a new session id so the getUserMedia .then below can
      // detect "dialog closed while we were awaiting mic permission"
      // by comparing against the module-level recSessionId. If the
      // user dismisses the dialog before permission resolves,
      // closeDialog bumps recSessionId and our captured mySession
      // won't match → we tear down the stream and exit cleanly.
      // (The cloud-Claude variant used just `!dialog.open` to detect
      // the dismissed-dialog race; the session counter also catches a
      // second recording started before the first permission resolves,
      // which dialog.open alone would miss.)
      var mySession = ++recSessionId;
      navigator.mediaDevices.getUserMedia({ audio: true }).then(function (stream) {
        if (mySession !== recSessionId || !dialog.open) {
          // Dialog was closed (or another startRecording superseded
          // us) before permission resolved. Drop the stream so the
          // browser's recording indicator goes away, re-enable
