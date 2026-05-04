
    // Scrollable list.
    var list = document.createElement("div");
    list.className = "ma-list";
    panel.appendChild(list);

    // Insert panel BEFORE the chart host container, wrapped inside a
    // .ma-panel-host that also carries the always-visible sticky
    // strip. The strip is the "quick access" summary; the full panel
    // floats as an overlay anchored to the host when expanded.
    var panelHost = document.createElement("div");
    panelHost.className = "ma-panel-host";

    var strip = document.createElement("button");
    strip.type = "button";
    strip.className = "ma-strip";
    strip.setAttribute("aria-expanded", "false");
    strip.setAttribute("aria-controls", "ma-panel-" + REPORT_ID);
    panel.id = strip.getAttribute("aria-controls");
    strip.innerHTML =
      '<span class="ma-strip-pencil" aria-hidden="true">✎</span>' +
      '<span class="ma-strip-editing"><span class="k">editing</span><span class="v">—</span></span>' +
      '<span class="ma-strip-sep" aria-hidden="true">·</span>' +
      '<span class="ma-strip-count">0 selected</span>' +
      '<span class="ma-strip-hint" aria-hidden="true">press <kbd>⌘</kbd><kbd>⇧</kbd><kbd>E</kbd> to toggle</span>';
    var stripValueEl = strip.querySelector(".ma-strip-editing .v");
    var stripCountEl = strip.querySelector(".ma-strip-count");

    // Pin + close controls injected into the floating panel header.
    // The pin lives flush-right inside the editing badge row so it's
    // easy to find without cluttering the strip.
    var panelControls = document.createElement("div");
    panelControls.className = "ma-panel-controls";
    var pinBtn = document.createElement("button");
    pinBtn.type = "button";
    pinBtn.className = "ma-panel-pin";
    pinBtn.setAttribute("aria-label", "Pin panel open");
    pinBtn.setAttribute("title", "Keep the panel open (ignore outside-clicks and Esc)");
    pinBtn.innerHTML = "📌";
    var closeBtn = document.createElement("button");
    closeBtn.type = "button";
    closeBtn.className = "ma-panel-close";
    closeBtn.setAttribute("aria-label", "Close panel");
    closeBtn.setAttribute("title", "Close (Esc)");
    closeBtn.innerHTML = "×";
    panelControls.appendChild(pinBtn);
    panelControls.appendChild(closeBtn);
    // Insert controls as the very first child of the panel so they
    // overlay the top-right corner without reshuffling existing rows.
    panel.insertBefore(panelControls, panel.firstChild);

    panelHost.appendChild(strip);
    // With Option A, the panel is a viewport-fixed popover. We lift it
    // out of the panelHost and append it directly to <body> so its
    // stacking context is independent of any ancestor overflow/transform
    // (which would otherwise confine a position:fixed element).
    panelHost.appendChild(panel);
    hostEl.parentNode.insertBefore(panelHost, hostEl);
    // Move panel to body now that host is in the DOM.
    document.body.appendChild(panel);

    // Dim-blur backdrop element (one per panel). Inserted on open,
    // removed on close. Clicking it closes the panel unless pinned.
    var backdrop = document.createElement("div");
    backdrop.className = "ma-backdrop";
    backdrop.setAttribute("aria-hidden", "true");
    document.body.appendChild(backdrop);

    // Floating pencil FAB — a second, always-visible affordance that
    // mirrors the strip's toggle behaviour. Useful when the user has
    // scrolled far below the strip and wants to open the editor
    // without hunting for the hotkey or scrolling back up.
    // Visibility is tied to subviewVisible (computed below) so the
    // FAB only appears while the mysqladmin section is in view; it
    // doesn't intrude on OS or processlist sections.
    // FAB stack: "Add chart" (+) sits above the "Edit counters" pencil.
    // Both live in a shared column-flex container at the bottom-right.
    var fabStack = document.createElement("div");
    fabStack.className = "ma-fab-stack";
    document.body.appendChild(fabStack);

    var fabAdd = document.createElement("button");
    fabAdd.type = "button";
    fabAdd.className = "ma-fab ma-fab-add";
    fabAdd.setAttribute("aria-label", "Add new chart");
    fabAdd.setAttribute("data-tooltip", "Add new chart");
    fabAdd.innerHTML = '<span class="ma-fab-pencil" aria-hidden="true">+</span>';
    fabStack.appendChild(fabAdd);
    // Click handler attached later, after createChart / setActive /
    // persistLayout are all defined — see end of renderMysqladmin.

    // Per-chart pencils (in each card header + empty-state big button)
    // have replaced the floating edit-pencil FAB. The add-chart FAB
    // stays because 'new chart' is a section-level action.

    // Sync the fixed-position panel's rect to the strip's bounding box
    // so it visually drops from the strip. Called on open, on scroll,
    // on window resize, and (via ResizeObserver) whenever the strip
    // width changes (layout/column-width shifts).
    function positionMaPanel() {
      if (!isOpen) return;
      var r = strip.getBoundingClientRect();
      var margin = 16;
      // Panel hangs off the strip's bottom-left, same width as the
      // strip — BUT always clamped inside the viewport. When the
      // user opened the panel from a chart far below the strip, the
      // strip may have scrolled above the viewport top (its
      // bounding rect.bottom is negative), in which case we pin the
      // popover near the viewport top so it's visible instead of
      // landing off-screen.
      var width = r.width > 0 ? r.width : (window.innerWidth - 2 * margin);
      if (width > window.innerWidth - 2 * margin) width = window.innerWidth - 2 * margin;
      var left = r.width > 0 ? r.left : margin;
      if (left < margin) left = margin;
      if (left + width > window.innerWidth - margin) {
        left = Math.max(margin, window.innerWidth - margin - width);
      }
      var top = r.bottom + 6;
      if (top < margin)                    top = margin;
      var maxTop = window.innerHeight - 120; // leave room for at least part of the panel
      if (top > maxTop)                    top = maxTop;
      panel.style.top   = top + "px";
      panel.style.left  = left + "px";
      panel.style.width = width + "px";
    }
    window.addEventListener("scroll",  positionMaPanel, { passive: true });
    window.addEventListener("resize",  positionMaPanel);
    if (typeof ResizeObserver === "function") {
      new ResizeObserver(positionMaPanel).observe(strip);
    }

    // Open/close + pin state, persisted under the v2 namespace so
    // reloading the same report remembers the reader's last choice.
    var OPEN_KEY = "mygather:v2:" + REPORT_ID + ":ma:panel-open";
    var PIN_KEY = "mygather:v2:" + REPORT_ID + ":ma:panel-pinned";
    var isOpen = storageGet(OPEN_KEY) === "true";
    var isPinned = storageGet(PIN_KEY) === "true";

    function applyPanelState() {
      panelHost.classList.toggle("is-open", isOpen);
      panelHost.classList.toggle("is-pinned", isPinned);
      // The panel lives on <body>, so mirror the open flag onto it
      // directly — CSS targets .ma-panel-host.is-open .ma-panel for the
      // visual state, but since the panel is no longer a descendant of
      // the host we add the matching class on the panel itself too.
      panel.classList.toggle("ma-panel-open", isOpen);
      backdrop.classList.toggle("is-visible", isOpen && !isPinned);
      strip.setAttribute("aria-expanded", isOpen ? "true" : "false");
      pinBtn.classList.toggle("active", isPinned);
      pinBtn.setAttribute("aria-pressed", isPinned ? "true" : "false");
      if (isOpen) positionMaPanel();
      if (typeof syncFabVisibility === "function") syncFabVisibility();
    }

    function setOpen(open) {
      var wasOpen = isOpen;
      isOpen = !!open;
      storageSet(OPEN_KEY, isOpen ? "true" : "false");
      // No scroll-into-view here: positionMaPanel() clamps the popover
      // into the viewport regardless of where the strip has scrolled
      // to, so the panel is always visible and the page doesn't need
      // to move. Keeping the user's scroll position stable is the
      // expected behaviour for a command-palette-style popover.
      applyPanelState();
      if (isOpen) {
        // Autofocus the search input so keystrokes go straight into
        // filtering without an extra click. Deferred to the next
        // tick so the CSS transition starts first.
        setTimeout(function () {
          var s = panel.querySelector(".ma-search input[type='search']");
          if (s) { try { s.focus({ preventScroll: true }); } catch (_) { s.focus(); } }
        }, 60);
      } else if (wasOpen) {
        // Restore focus to the strip so keyboard users aren't stranded.
        try { strip.focus({ preventScroll: true }); } catch (_) {
          if (typeof strip.focus === "function") strip.focus();
        }
      }
    }
    function setPinned(pinned) {
      isPinned = !!pinned;
      storageSet(PIN_KEY, isPinned ? "true" : "false");
      applyPanelState();
    }

    strip.addEventListener("click", function () { setOpen(!isOpen); });
    closeBtn.addEventListener("click", function () { setOpen(false); });

    // Tab focus trap inside the floating panel while open and not
    // pinned. When pinned the user explicitly opted into parallel
    // navigation, so we leave Tab alone.
    panel.addEventListener("keydown", function (ev) {
      if (!isOpen || isPinned || ev.key !== "Tab") return;
      var items = panel.querySelectorAll(
        'a[href], button:not([disabled]), input:not([disabled]), [tabindex]:not([tabindex="-1"])'
      );
      if (!items.length) return;
      var first = items[0];
      var last  = items[items.length - 1];
      if (ev.shiftKey && document.activeElement === first) {
        ev.preventDefault();
        last.focus();
      } else if (!ev.shiftKey && document.activeElement === last) {
        ev.preventDefault();
        first.focus();
      }
    });
    pinBtn.addEventListener("click", function (ev) {
      ev.stopPropagation();
      setPinned(!isPinned);
    });

    // Click-outside-to-close: the panel is now detached from panelHost
    // (it lives on <body>), so the check also has to exclude the panel
    // itself. Backdrop is treated as "outside" — clicking it closes.
    // Skipped when pinned.
    if (!panelHost._clickOutsideInit) {
      panelHost._clickOutsideInit = true;
      document.addEventListener("click", function (ev) {
        if (!isOpen || isPinned) return;
        if (panelHost.contains(ev.target)) return;
        if (panel.contains(ev.target)) return;
        setOpen(false);
      });
    }
    backdrop.addEventListener("click", function () {
      if (isOpen && !isPinned) setOpen(false);
    });

    // Hotkey `E` toggles the panel, but only when the mysqladmin
    // subview is actually in the viewport — so pressing `e` while
    // reading the OS section doesn't silently pop open a panel off
    // screen. IntersectionObserver tracks visibility; if no subview
    // ancestor can be found we fall back to always-active.
    var subviewEl = hostEl.closest("details.subview") || hostEl.closest("section") || hostEl;
    // Default to visible so the FAB appears on first paint even if
    // IntersectionObserver hasn't fired its initial callback yet. IO
    // then narrows visibility when the user scrolls past the section.
    var subviewVisible = true;
    function syncFabVisibility() {
      // Only the add-chart FAB remains — it shows while the counter-
      // deltas section is in view, regardless of popover state.
      fabAdd.classList.toggle("is-visible", subviewVisible);
    }
    if (window.IntersectionObserver && subviewEl) {
      var io = new IntersectionObserver(function (entries) {
        for (var i = 0; i < entries.length; i++) {
          subviewVisible = entries[i].isIntersecting;
        }
        syncFabVisibility();
      }, { rootMargin: "0px 0px -20% 0px", threshold: 0.01 });
      io.observe(subviewEl);
    }
    syncFabVisibility();
    document.addEventListener("keydown", function (ev) {
      if (ev.defaultPrevented) return;
      var t = ev.target;
      var typingInField = t && (t.tagName === "INPUT" || t.tagName === "TEXTAREA" || t.isContentEditable);
      // Cmd/Ctrl+Shift+E is a modifier combo — never collides with
      // typing, so we always let it toggle regardless of focus.
      // Check this BEFORE the typing-in-field early-return so closing
      // via the same hotkey works even when the panel's search input
      // has focus.
      if (
        (ev.key === "e" || ev.key === "E") &&
        ev.shiftKey &&
        (ev.metaKey || ev.ctrlKey) &&
        !ev.altKey &&
        subviewVisible
      ) {
        setOpen(!isOpen);
        ev.preventDefault();
        return;
      }
      if (typingInField) {
        // Exception: Esc inside the panel's search still closes.
        if (ev.key === "Escape" && isOpen && !isPinned && panel.contains(t)) {
          setOpen(false);
          ev.preventDefault();
        }
        return;
      }
      if (ev.key === "Escape" && isOpen && !isPinned) {
        setOpen(false);
        ev.preventDefault();
        return;
      }
    });

    applyPanelState();

    // List-filter state (shared across charts; the list grid itself
    // only reflects the *active* chart's selection state).
    var state = { category: "all", needle: "" };

    function categoryFilter(name) {
      if (!state.category || state.category === "all") return true;
      if (state.category === "selected") {
        var a = getActive();
        return a ? a.selected.has(name) : false;
      }
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
      var active = getActive();
      var sel = active ? active.selected : new Set();
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
        var checked = sel.has(name) ? ' checked' : '';
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
      selectedCount.textContent = sel.size + ' selected · ' + rows.length + ' shown / ' + data.variables.length;
    }

    list.addEventListener("change", function (e) {
      if (!e.target || e.target.tagName !== "INPUT") return;
      var active = getActive();
      if (!active) return;
      var name = e.target.value;
      if (e.target.checked) active.selected.add(name);
      else active.selected.delete(name);
      persistLayout();
      selectedCount.textContent = active.selected.size + ' selected · ' + list.children.length + ' shown / ' + data.variables.length;
      updateActiveCount();
      scheduleActiveRedraw();
    });
    btnSelectVisible.addEventListener("click", function () {
      var active = getActive();
      if (!active) return;
      var boxes = list.querySelectorAll("input[type=checkbox]");
      for (var i = 0; i < boxes.length; i++) { active.selected.add(boxes[i].value); boxes[i].checked = true; }
      persistLayout();
      selectedCount.textContent = active.selected.size + ' selected · ' + boxes.length + ' shown / ' + data.variables.length;
      updateActiveCount();
      scheduleActiveRedraw();
    });
    btnClear.addEventListener("click", function () {
      var active = getActive();
      if (!active) return;
      active.selected.clear();
      persistLayout();
      redrawList();
      updateActiveCount();
      scheduleActiveRedraw();
    });
    search.addEventListener("input", function () {
      state.needle = search.value.trim().toLowerCase();
      redrawList();
    });

    // ---- Turn hostEl into a toolbar + grid of chart cards ----
    hostEl.innerHTML = "";

    var toolbar = document.createElement("div");
    toolbar.className = "ma-charts-toolbar";
    var toolbarHint = document.createElement("span");
    toolbarHint.className = "ma-charts-hint";
    toolbarHint.textContent = "Stack charts to compare counters";
    var btnNew = document.createElement("button");
    btnNew.type = "button";
    btnNew.className = "ma-action ma-action-primary";
    btnNew.textContent = "+ New chart";
    btnNew.title = "Add another chart below — pick counters or a category for it, while other charts stay untouched";
    function addNewChart() {
      var cs = createChart({ selection: [] });
      setActive(cs.id);
      // Persist immediately so a reload right after "+ New chart"
      // preserves the empty chart — matches duplicate/remove.
      persistLayout();
      // Bring the freshly-added chart into view so the user sees
      // what they just created (matters most when they clicked
      // the floating-FAB equivalent from the bottom of the page).
      if (cs.cardEl && typeof cs.cardEl.scrollIntoView === "function") {
        try { cs.cardEl.scrollIntoView({ block: "center", behavior: "smooth" }); }
        catch (_) { cs.cardEl.scrollIntoView(true); }
      }
    }
    btnNew.addEventListener("click", addNewChart);
    fabAdd.addEventListener("click", function (ev) {
      ev.stopPropagation();
      addNewChart();
    });
    toolbar.appendChild(toolbarHint);
    toolbar.appendChild(btnNew);
    hostEl.appendChild(toolbar);

    var gridEl = document.createElement("div");
    gridEl.className = "ma-charts";
    hostEl.appendChild(gridEl);

    // ---- Chart lifecycle ----

    function createChart(opts) {
      opts = opts || {};
      var id = "c" + (nextChartNum++);
      var title = opts.title || ("Chart " + nextChartNum);

      var card = document.createElement("div");
      card.className = "ma-card";
      card.setAttribute("data-chart-id", id);

      var head = document.createElement("div");
      head.className = "ma-card-head";

      var titleEl = document.createElement("span");
      titleEl.className = "ma-card-title";
      titleEl.textContent = title;
      titleEl.title = title;

      var countEl = document.createElement("span");
      countEl.className = "ma-card-count";

      // Per-chart pencil: activates this chart and opens the editor so
      // the popover targets exactly this chart.
      var btnEdit = document.createElement("button");
      btnEdit.type = "button";
      btnEdit.className = "ma-card-btn ma-card-edit";
      btnEdit.innerHTML = "&#x270E;"; // pencil
      btnEdit.title = "Edit this chart's counters";
      btnEdit.setAttribute("aria-label", "Edit counters");
      btnEdit.addEventListener("click", function (ev) {
        ev.stopPropagation();
        if (activeChartId !== id) setActive(id);
        setOpen(true);
      });

      var btnZoom = document.createElement("button");
      btnZoom.type = "button";
      btnZoom.className = "ma-card-btn ma-card-zoom";
      btnZoom.textContent = "⛶";
      btnZoom.title = "Reset this chart's zoom to the full capture window (you can also double-click the chart)";
      btnZoom.setAttribute("aria-label", "Reset zoom");
      btnZoom.addEventListener("click", function (ev) {
        ev.stopPropagation();
        var cs = charts.get(id);
        if (cs && cs.plot) {
          cs.plot.setScale("x", { min: null, max: null });
        }
      });

      var btnDup = document.createElement("button");
      btnDup.type = "button";
      btnDup.className = "ma-card-btn ma-card-dup";
      btnDup.textContent = "⎘";
      btnDup.title = "Duplicate this chart — opens a copy with the same counter selection so you can diverge from it";
      btnDup.setAttribute("aria-label", "Duplicate chart");
      btnDup.addEventListener("click", function (ev) {
        ev.stopPropagation();
        duplicateChart(id);
      });

      var btnClose = document.createElement("button");
      btnClose.type = "button";
      btnClose.className = "ma-card-btn ma-card-close";
      btnClose.innerHTML = "&times;";
      btnClose.title = "Remove this chart (at least one chart is always kept)";
      btnClose.setAttribute("aria-label", "Close chart");
      btnClose.addEventListener("click", function (ev) {
        ev.stopPropagation();
        removeChart(id);
      });

      head.appendChild(titleEl);
      head.appendChild(countEl);
      head.appendChild(btnEdit);
      head.appendChild(btnZoom);
      head.appendChild(btnDup);
      head.appendChild(btnClose);
      card.appendChild(head);

      var plotEl = document.createElement("div");
      plotEl.className = "ma-card-plot";
      card.appendChild(plotEl);

      // Clicking anywhere in the card (except header buttons or a
      // legend pill) activates the chart. Use mousedown so it beats
      // focus shifts from other controls.
      card.addEventListener("mousedown", function (ev) {
        if (ev.target.closest && ev.target.closest(".ma-card-btn")) return;
        if (ev.target.closest && ev.target.closest(".series-pill")) return;
        if (activeChartId !== id) setActive(id);
      });

      gridEl.appendChild(card);

      var cs = {
        id: id,
        title: title,
        selected: new Set((opts.selection || []).filter(function (n) { return data.variables.indexOf(n) >= 0; })),
        plot: null,
        cardEl: card,
        titleEl: titleEl,
        countEl: countEl,
        plotEl: plotEl,
        throttle: null,
      };
      charts.set(id, cs);
      updateCloseButtons();
      updateCardCount(cs);
      // Defer the first draw until after the browser has laid the
      // card out, otherwise measureChartWidth reads 0.
      requestAnimationFrame(function () { rebuildChart(cs); });
      return cs;
    }

    function setActive(id) {
      if (!charts.has(id)) return;
      activeChartId = id;
      charts.forEach(function (c) {
        c.cardEl.classList.toggle("active", c.id === id);
      });
      var active = getActive();
      editingValueEl.textContent = active ? active.title : "—";
      stripValueEl.textContent = active ? active.title : "—";
      if (active) refreshStripCount(active);
      redrawList();
    }

    function updateActiveTitle(newTitle) {
      var active = getActive();
      if (!active) return;
      active.title = newTitle;
      active.titleEl.textContent = newTitle;
      active.titleEl.title = newTitle;
      editingValueEl.textContent = newTitle;
      stripValueEl.textContent = newTitle;
    }

    function updateCardCount(cs) {
      var n = cs.selected.size;
      cs.countEl.textContent = n === 0 ? "empty" : (n + " selected");
      if (cs.id === activeChartId) refreshStripCount(cs);
    }

    function refreshStripCount(cs) {
      var n = cs.selected.size;
      stripCountEl.textContent = n === 0 ? "empty" : (n + " selected");
    }

    function updateActiveCount() {
      var active = getActive();
      if (active) updateCardCount(active);
    }

    function updateCloseButtons() {
      var only = charts.size <= 1;
      charts.forEach(function (c) {
        var btn = c.cardEl.querySelector(".ma-card-close");
        if (!btn) return;
        btn.disabled = only;
        btn.title = only ? "At least one chart must remain" : "Remove this chart";
      });
    }

    function removeChart(id) {
      if (charts.size <= 1) return;
      var cs = charts.get(id);
      if (!cs) return;
      if (cs.throttle) { clearTimeout(cs.throttle); cs.throttle = null; }
      if (cs.plot) { unregisterChart(cs.plot); cs.plot.destroy(); cs.plot = null; }
      cs.cardEl.remove();
      charts.delete(id);
      if (activeChartId === id) {
        var firstId = charts.keys().next().value;
        setActive(firstId);
      }
      updateCloseButtons();
      persistLayout();
    }

    function duplicateChart(id) {
      var src = charts.get(id);
      if (!src) return;
      var copyTitle = src.title + " (copy)";
      var cs = createChart({ title: copyTitle, selection: Array.from(src.selected) });
      setActive(cs.id);
      persistLayout();
    }

    // ---- Chart rendering (per ChartState) ----

    function rebuildChart(cs) {
      if (cs.plot) { unregisterChart(cs.plot); cs.plot.destroy(); cs.plot = null; }
      // Strip any prior legend.
      var next = cs.plotEl.nextSibling;
      while (next && next.classList && next.classList.contains("series-legend")) {
        var doomed = next;
        next = next.nextSibling;
        doomed.remove();
      }
      // Remove any previous empty-state placeholder.
      var placeholder = cs.cardEl.querySelector(".ma-empty-chart");
      if (placeholder) placeholder.remove();

      var picks = Array.from(cs.selected).sort();

      if (picks.length === 0) {
        var msg = document.createElement("div");
        msg.className = "ma-empty-chart";
        var bigBtn = document.createElement("button");
        bigBtn.type = "button";
        bigBtn.className = "ma-empty-edit";
        bigBtn.innerHTML = '<span class="ma-empty-pencil" aria-hidden="true">&#x270E;</span>' +
                           '<span class="ma-empty-label">Pick counters</span>';
        bigBtn.setAttribute("aria-label", "Pick counters for this chart");
        bigBtn.addEventListener("click", function (ev) {
          ev.stopPropagation();
          if (activeChartId !== cs.id) setActive(cs.id);
          setOpen(true);
        });
        msg.appendChild(bigBtn);
        var hint = document.createElement("div");
        hint.className = "ma-empty-hint";
        hint.textContent = "or pick counters from the list above";
        msg.appendChild(hint);
        cs.plotEl.innerHTML = "";
        cs.cardEl.appendChild(msg);
        updateCardCount(cs);
        return;
      }

      // Drop the first sample (column 0): pt-mext stores the raw
      // tally there, which creates an artificial spike that crushes
      // the y-axis. Real per-sample deltas start at index 1.
      var tStart = data.timestamps.length > 1 ? 1 : 0;
      var truncatedTimestamps = data.timestamps.slice(tStart);

      var series = [{ label: "time" }];
      var values = [truncatedTimestamps];
      picks.forEach(function (name, i) {
        var deltaArr = data.deltas[name];
        if (!deltaArr) return;
        series.push(decorateSeries(name, i));
        values.push(deltaArr.slice(tStart));
      });
      var width = measureChartWidth(cs.plotEl);
      var opts = basePlotOpts(width, 340, series, "Δ / sample", data.snapshotBoundaries, data.timestamps);
      cs.plotEl.innerHTML = "";
      cs.plot = new uPlot(opts, values, cs.plotEl);
      registerChart(cs.plot, cs.plotEl, opts);
      mountLegend(cs.plotEl, series, cs.plot);
      mountResetZoomButton(cs.plotEl, function () { return cs.plot; });
      updateCardCount(cs);
    }

    function scheduleActiveRedraw() {
      var cs = getActive();
      if (!cs || cs.throttle) return;
      cs.throttle = setTimeout(function () {
        cs.throttle = null;
        rebuildChart(cs);
      }, 120);
    }

    // ---- Persistence ----

    function persistLayout() {
      var arr = [];
      charts.forEach(function (c) {
        arr.push({
          id: c.id,
          title: c.title,
          selected: Array.from(c.selected),
        });
      });
      try {
        storageSet(LAYOUT_KEY, JSON.stringify(arr));
      } catch (_) {}
      // Keep the legacy single-selection key in sync with chart #0
      // so anything still reading the old key (or migration paths)
      // sees a sensible value.
      if (arr.length > 0) {
        storageSet(LEGACY_KEY, arr[0].selected.join("\n"));
      }
    }

    function loadInitialCharts() {
      var raw = storageGet(LAYOUT_KEY);
      if (raw) {
        try {
          var arr = JSON.parse(raw);
          if (Array.isArray(arr) && arr.length > 0) {
            arr.forEach(function (s) {
              createChart({
                title: (s && typeof s.title === "string" && s.title) ? s.title : null,
                selection: Array.isArray(s && s.selected) ? s.selected : [],
              });
            });
            var firstId = charts.keys().next().value;
            setActive(firstId);
            return;
          }
        } catch (_) {}
      }
      // Legacy fallback: single selection in the old key.
      var legacy = storageGet(LEGACY_KEY);
      var defaults = Array.isArray(data.defaultVisible) ? data.defaultVisible : data.variables.slice(0, 5);
      var initial = legacy ? legacy.split("\n").filter(Boolean) : defaults;
      initial = initial.filter(function (n) { return data.variables.indexOf(n) >= 0; });
      if (initial.length === 0) initial = defaults;
      var cs = createChart({ title: "Chart 1", selection: initial });
      setActive(cs.id);
    }

    loadInitialCharts();
  }

  // --- 5. Nav-rail collapse + scroll-spy --------------------------

  // Floating nav-drawer controller.
  //
  // The left navigation is an overlay drawer triggered by the logo
  // button pinned to the top-left of the viewport. It always starts
  // CLOSED on page load (no persistence — per product decision) and is
  // opened by clicking the logo, pressing Cmd/Ctrl + `.`, or by
  // following an in-page anchor. It closes when the user clicks the
  // logo again, clicks the backdrop, presses Escape, or activates a
  // link inside the drawer.
  function initNavCollapse() {
    var toggle   = document.getElementById("nav-toggle");
    var drawer   = document.getElementById("nav-drawer");
    var backdrop = document.getElementById("nav-backdrop");
    if (!toggle || !drawer || !backdrop) return;
    var mainEl = document.querySelector("main.content");

    function isOpen() { return document.body.classList.contains("nav-open"); }

    function focusables() {
      return drawer.querySelectorAll(
        'a[href], button:not([disabled]), [tabindex]:not([tabindex="-1"])'
      );
    }

    function setOpen(open, opts) {
      var wasOpen = isOpen();
      // One-modal-at-a-time: opening the drawer unpins and closes any
      // floating mysqladmin panel. Otherwise `mainEl.inert = true` below
      // would strand a user-pinned panel inside an inert subtree (all
      // controls disabled) with no way to reach it.
      if (open) {
        // Unpin first (active-state on pin lives on .ma-panel-host,
        // but since Option A detached .ma-panel to <body> we target
        // the pin button inside any open popover directly).
        var pinnedPanels = document.querySelectorAll(".ma-panel-host.is-pinned .ma-panel-pin");
        for (var pi = 0; pi < pinnedPanels.length; pi++) {
          pinnedPanels[pi].click();
        }
        // Close any open popovers. After Option A the panel lives on
        // <body> (no longer inside .ma-panel-host), so match on the
        // .ma-panel.ma-panel-open selector we now apply on open.
        var openPopovers = document.querySelectorAll(".ma-panel.ma-panel-open .ma-panel-close");
        for (var ci = 0; ci < openPopovers.length; ci++) {
          openPopovers[ci].click();
        }
      }
      document.body.classList.toggle("nav-open", !!open);
      toggle.setAttribute("aria-expanded", open ? "true" : "false");
      drawer.setAttribute("aria-hidden", open ? "false" : "true");
      backdrop.setAttribute("aria-hidden", open ? "false" : "true");
      // a11y: make closed drawer non-interactive; when open, make main
      // content inert so Tab stays trapped in the drawer.
      drawer.inert = !open;
      if (mainEl) mainEl.inert = !!open;
      // Prevent background scroll while the drawer is open so the
      // overlay reads as modal.
      document.documentElement.style.overflow = open ? "hidden" : "";
      var manageFocus = !(opts && opts.silent);
      if (manageFocus) {
        if (open) {
          // Move focus into the drawer so keyboard users don't stay
          // stranded on the toggle.
          var first = drawer.querySelector("a, button") || drawer;
          if (first && typeof first.focus === "function") first.focus();
        } else if (wasOpen) {
          // Restore focus to the toggle so the user's keyboard context
          // is preserved.
          if (typeof toggle.focus === "function") toggle.focus();
        }
      }
      // Re-fit charts once the transition settles (content layout
      // doesn't reflow, but browser zoom + window size may have changed
      // while the drawer was open). Under prefers-reduced-motion the
      // transition is near-instant (0.01ms), so don't wait 360ms.
      if (!open) {
        var rmReduce = window.matchMedia && window.matchMedia("(prefers-reduced-motion: reduce)").matches;
        setTimeout(resizeAllCharts, rmReduce ? 0 : 360);
      }
    }

    toggle.addEventListener("click", function () { setOpen(!isOpen()); });
    backdrop.addEventListener("click", function () { setOpen(false); });

    // Close the drawer when any in-drawer link is clicked — the user
    // has navigated, so the drawer has served its purpose.
    drawer.addEventListener("click", function (e) {
      var t = e.target;
      while (t && t !== drawer) {
        if (t.tagName === "A") { setOpen(false); return; }
        t = t.parentNode;
      }
    });

    // Tab focus trap while the drawer is open. Wraps from last to
    // first (Tab) and first to last (Shift+Tab).
    drawer.addEventListener("keydown", function (e) {
      if (!isOpen() || e.key !== "Tab") return;
      var items = focusables();
      if (items.length === 0) return;
      var first = items[0];
      var last  = items[items.length - 1];
      if (e.shiftKey && document.activeElement === first) {
        e.preventDefault();
        last.focus();
      } else if (!e.shiftKey && document.activeElement === last) {
        e.preventDefault();
        first.focus();
      }
    });

    document.addEventListener("keydown", function (e) {
      if (e.defaultPrevented) return;
      // Escape closes.
      if (e.key === "Escape" && isOpen()) {
        // If the floating mysqladmin panel is open, let its own Esc handler
        // take this keypress — pressing Esc should close the topmost
        // overlay, not a background drawer.
        if (document.querySelector(".ma-panel-host.is-open") && !document.querySelector(".ma-panel-host.is-pinned")) {
          return;
        }
        e.preventDefault();
        setOpen(false);
        return;
      }
      // Cmd/Ctrl + . toggles. (Chose '.' over '\' per product request —
      // easier to reach on most keyboard layouts.)
      if ((e.metaKey || e.ctrlKey) && e.key === ".") {
        e.preventDefault();
        setOpen(!isOpen());
      }
    });

    // Start closed every page-load. Pass silent so we don't steal focus
    // on the initial render.
    setOpen(false, { silent: true });
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

    // The summary-title anchor (<a class="nav-title">) is what makes
    // Level-1 groups with no Level-2 children navigable. Clicking it
    // would otherwise also toggle the parent <details>; stopPropagation
    // keeps the semantics clean: anchor = navigate, rest of summary =
    // expand/collapse.
    var titleLinks = document.querySelectorAll("nav.index details.nav-group > summary > a.nav-title");
    for (var t = 0; t < titleLinks.length; t++) {
      titleLinks[t].addEventListener("click", function (ev) {
        ev.stopPropagation();
      });
