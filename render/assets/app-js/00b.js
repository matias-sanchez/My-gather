/* my-gather report client-side behaviour — part 00b.
 *
 * Theme helpers extracted from 00.js to keep the file under the
 * 1000-line cap (Principle XV). All declarations here live inside
 * the same outer IIFE that 00.js opens (closed in 05.js): function
 * declarations are hoisted, so callers in 00.js (e.g. makeFillFn
 * → hexToRgba) and downstream chart-builders in 01.js / 02.js /
 * 03.js see them as if they were defined at the top of the IIFE.
 *
 * Series colors flow from the canonical CSS design-token block in
 * `render/assets/app-css/04.css`: --series-1 through --series-16,
 * one set per theme. The chart code resolves them via cssVar() at
 * series-decoration time and re-resolves them on the
 * `mygather:theme` event so live theme switches re-paint open
 * charts.
 */

  function cssVar(name, fallback) {
    try {
      var v = getComputedStyle(document.documentElement).getPropertyValue(name).trim();
      return v || fallback;
    } catch (_) { return fallback; }
  }

  function seriesStrokeFor(idx) {
    var slot = ((idx % SERIES_PALETTE_SIZE) + SERIES_PALETTE_SIZE) % SERIES_PALETTE_SIZE;
    // Fallback `#60a5fa` is the prior default series-1 color and
    // covers the brief window where the document is mid-rebuild and
    // the custom property has not resolved yet.
    return cssVar("--series-" + (slot + 1), "#60a5fa");
  }

  // Convert a #RRGGBB stroke into an rgba() with given alpha — used
  // for gradient fills under each line so busy charts still read.
  function hexToRgba(hex, alpha) {
    var s = String(hex || "").replace("#", "");
    if (s.length === 3) s = s[0]+s[0]+s[1]+s[1]+s[2]+s[2];
    if (s.length !== 6) return "rgba(96,165,250," + alpha + ")";
    var r = parseInt(s.substring(0,2), 16);
    var g = parseInt(s.substring(2,4), 16);
    var b = parseInt(s.substring(4,6), 16);
    return "rgba(" + r + "," + g + "," + b + "," + alpha + ")";
  }

  function repaintChartsForTheme() {
    for (var i = 0; i < CHARTS.length; i++) {
      var entry = CHARTS[i];
      var u = entry && entry.plot;
      if (!u || !u.series || !u.axes) continue;
      // Two fill flavors: line/area uses the gradient closure from
      // makeFillFn(); stacked-area / bar series in 01.js carry an
      // explicit __themeFillBuilder that returns an alpha-tint rgba
      // string. Honor whichever one the series declared.
      for (var s = 0; s < u.series.length; s++) {
        var ser = u.series[s];
        if (ser == null || ser.__themeIdx == null) continue;
        var newStroke = seriesStrokeFor(ser.__themeIdx);
        ser.stroke = newStroke;
        ser.fill = (typeof ser.__themeFillBuilder === "function")
          ? ser.__themeFillBuilder(newStroke)
          : makeFillFn(newStroke);
      }
      var axisStroke = cssVar("--axis-stroke", "#9aa5b4");
      var gridStroke = cssVar("--grid-stroke", "rgba(130, 150, 175, 0.09)");
      var tickStroke = cssVar("--tick-stroke", "rgba(130, 150, 175, 0.35)");
      for (var a = 0; a < u.axes.length; a++) {
        var ax = u.axes[a];
        if (!ax) continue;
        ax.stroke = axisStroke;
        if (ax.grid)  ax.grid.stroke  = gridStroke;
        if (ax.ticks) ax.ticks.stroke = tickStroke;
      }
      try { u.redraw(false); } catch (_) {}
    }
  }
  document.addEventListener("mygather:theme", repaintChartsForTheme);
