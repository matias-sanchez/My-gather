          // recordBtn so the next dialog open can start a fresh
          // recording (recordBtn is persistent DOM across open/close
          // — closeDialog doesn't reset its disabled state, so a
          // stale-session exit that skips this line strands the
          // button disabled for the rest of the page session), and
          // return without touching module recorder state.
          try { stream.getTracks().forEach(function (t) { t.stop(); }); } catch (_) {}
          recordBtn.disabled = false;
          return;
        }
        recStream = stream;
        try { recorder = new MediaRecorder(stream); }
        catch (e) {
          setErr("Could not start recorder: " + (e && e.message ? e.message : e));
          try { stream.getTracks().forEach(function (t) { t.stop(); }); } catch (_) {}
          recStream = null; recordBtn.disabled = false; return;
        }
        // Bind the async recorder listeners to THIS session's locals
        // so a delayed event from a previous recording can't
        // clobber a new session's state. If the user cancels
        // mid-recording, quickly reopens the dialog, and starts a
        // second recording before the first MediaRecorder dispatches
        // its `stop` event, the old callback would otherwise reach
        // through to the module-level `recorder` / `recChunks`
        // variables (which now hold the NEW session's state) and
        // null them out — the new recording would lose its chunks
        // mid-stream.
        var sessionRecorder = recorder;
        var sessionChunks = [];
        recChunks = sessionChunks; // closeDialog nulls this to silence cancel
        sessionRecorder.addEventListener("dataavailable", function (ev) {
          // A cancel sets recChunks = null on the module-level. Our
          // own sessionChunks is still allocated — check the module
          // reference to decide whether to collect or drop chunks.
          if (recChunks === sessionChunks && ev.data && ev.data.size) {
            sessionChunks.push(ev.data);
          }
        });
        sessionRecorder.addEventListener("stop", function () {
          var mime = sessionRecorder.mimeType || "audio/webm";
          // Only materialise an attachment when THIS session is
          // still the active one AND wasn't silenced by closeDialog.
          // sessionChunks.length === 0 covers a clean cancel (data
          // path silenced) and the "mic never produced any data"
          // case alike.
          if (recChunks === sessionChunks && sessionChunks.length > 0) {
            addAttachment("voice", new Blob(sessionChunks, { type: mime }));
          }
          if (recorder === sessionRecorder) {
            recorder = null;
            recChunks = null;
          }
        });
        sessionRecorder.start();
        recStart = performance.now();
        // Contract markup: wrapper + pulsing dot + timer + Stop button
        // (see specs/002-report-feedback-button/contracts/ui.md).
        var wrap = document.createElement("div");
        wrap.className = "feedback-attachment feedback-attachment--recording";
        wrap.setAttribute("data-kind", "recording");
        var dot = document.createElement("span");
        dot.className = "feedback-recording-dot";
        dot.setAttribute("aria-hidden", "true");
        recLabel = document.createElement("span");
        recLabel.className = "feedback-recording-time";
        recLabel.textContent = "0:00 / 10:00";
        var stopBtn = document.createElement("button");
        stopBtn.type = "button";
        stopBtn.className = "feedback-record-stop";
        stopBtn.textContent = "Stop";
        stopBtn.addEventListener("click", stopRecording);
        wrap.appendChild(dot); wrap.appendChild(recLabel); wrap.appendChild(stopBtn);
        attachments.appendChild(wrap);
        recordBtn.disabled = true;
        recRAF = requestAnimationFrame(tickTimer);
      }).catch(function (err) {
        setErr("Microphone access denied: " + (err && err.message ? err.message : err));
        recordBtn.disabled = false;
      });
    }

    // --- Open / close / submit ------------------------------------

    function openDialog() {
      setErr(""); hide(fallback); hide(hint);
      if (successEl) hide(successEl);
      form.hidden = false;
      // Spec 021-feedback-author-field US2: pre-fill the Author
      // field from the last successful submission. Read at open
      // time (not boot time) because closeDialog resets the field
      // between cycles. R6: re-evaluate the cap on each load so a
      // stale value from a release with a higher cap is silently
      // truncated rather than blocking the Submit gate.
      if (authorInput.value === "") {
        authorInput.value = loadPersistedAuthor();
      }
      dialog.showModal();
      try { titleInput.focus(); } catch (_) {}
      updateSubmitEnabled();
    }
    function closeDialog() {
      // Invalidate any in-flight getUserMedia permission prompt so its
      // delayed .then callback won't attach a recorder to the
      // now-dismissed dialog. The bumped recSessionId is what the
      // callback compares against.
      recSessionId++;
      if (recorder || recStream || recRAF) {
        // MediaRecorder.stop is async: the "stop" listener below runs
        // on a later tick and — if it finds a non-empty recChunks —
        // calls addAttachment("voice", …). When the user cancels
        // mid-recording we've already cleared attachments by the time
        // that callback fires, so a canceled recording would
        // silently repopulate the voice attachment on the NEXT
        // dialog open. Null recChunks first so the stop guard
        // (`if (recChunks && recChunks.length)`) is false and the
        // late callback is a no-op.
        recChunks = null;
        stopRecording();
      }
      clearAttachment("image"); clearAttachment("voice");
      authorInput.value = "";
      titleInput.value = ""; bodyInput.value = ""; catSelect.value = "";
      setErr(""); hide(fallback); hide(hint);
      // Dialog dismissal ends the current logical submission. The
      // next time the user opens the dialog and clicks Submit, they
      // are composing a fresh message and deserve a fresh
      // idempotencyKey — not a replay of whatever the backend
      // cached against the old key.
      idempotencyKey = null;
      // Reset the success panel so reopening the dialog after a
      // successful submit starts back at the form, not the stale
      // "Feedback posted" state.
      if (successEl) hide(successEl);
      if (successLink) successLink.href = "#";
      form.hidden = false;
      updateSubmitEnabled();
      if (dialog.open) { try { dialog.close(); } catch (_) {} }
    }
    function renderImageDownload() {
      if (!imgBlob || !imgURL) return;
      if (attachments.querySelector('[data-kind="image-download"]')) return;
      // Reuse the existing .feedback-attachment surface so the download
      // fallback inherits its padding/border. No separate CSS class.
      var wrap = document.createElement("div");
      wrap.className = "feedback-attachment";
      wrap.setAttribute("data-kind", "image-download");
      var a = document.createElement("a");
      a.href = imgURL;
      a.download = "feedback-image-" + shortID() + ".png";
      a.textContent = "Download image";
      wrap.appendChild(a);
      attachments.appendChild(wrap);
      hint.textContent = "Clipboard blocked — download the image and drop it into GitHub's body.";
      show(hint);
    }

    function finishActiveRecordingForSubmit() {
      if (!recorder || recorder.state !== "recording") return Promise.resolve();
      var activeRecorder = recorder;
      return new Promise(function (resolve) {
        var done = false;
        function finish() {
          if (done) return;
          done = true;
          resolve();
        }
        activeRecorder.addEventListener("stop", function () {
          // The recorder's own stop listener, registered in
          // startRecording(), materialises voiceBlob via addAttachment().
          // Resolve on the next turn so that listener has completed
          // before doSubmit snapshots voiceBlob for the Worker payload.
          setTimeout(finish, 0);
        }, { once: true });
        stopRecording();
        // Defensive escape hatch: some browser implementations can
        // fail to dispatch stop after a permission/runtime edge. Do
        // not strand Submit forever; proceed without a voice blob in
        // that abnormal case.
        setTimeout(finish, 1500);
      });
    }

    // blobToBase64 reads the given Blob via FileReader.readAsDataURL
    // and returns the raw base64 payload (the "data:<mime>;base64,"
    // prefix stripped). Rejects if the reader errors. Used to pack
    // image + voice attachments into the Worker JSON payload (spec
    // 003, research R6).
    function blobToBase64(blob) {
      return new Promise(function (resolve, reject) {
        var reader = new FileReader();
        reader.onerror = function () { reject(reader.error || new Error("FileReader error")); };
        reader.onload = function () {
          var s = String(reader.result || "");
          var comma = s.indexOf(",");
          resolve(comma === -1 ? s : s.slice(comma + 1));
        };
        try { reader.readAsDataURL(blob); } catch (e) { reject(e); }
      });
    }

    // renderSuccess hides the form and reveals the post-submit panel
    // with a link to the freshly-created GitHub issue.
    function renderSuccess(issueUrl, issueNumber) {
      if (!successEl || !successLink) return;
      // Definitive success — the next Submit (after a form reset or
      // dialog close) is a new logical attempt and deserves a fresh
      // idempotencyKey. Clear the cached key so lazy-init in
      // doSubmit mints a new one.
      idempotencyKey = null;
      successLink.href = issueUrl;
      if (typeof issueNumber === "number" && isFinite(issueNumber) && issueNumber > 0) {
        successLink.textContent = "View issue #" + issueNumber + " on GitHub";
      } else {
        successLink.textContent = "View issue on GitHub";
      }
      form.hidden = true;
      show(successEl);
      try { successLink.focus(); } catch (_) {}
    }

    // doLegacyFallback is the feature-002 submit path: open GitHub's
    // new-issue URL in a new tab and hand media off via clipboard
    // (images) or programmatic download (voice). Invoked only on the
    // Worker-path failure arm of doSubmit() (Principle III graceful
    // degradation; Principle XIII single code path — this is the
    // failure branch of doSubmit, not a parallel handler).
    function doLegacyFallback() {
      var url = buildURL();

      // 1. Try to open GitHub. Attempt this FIRST so the user-gesture
      //    is consumed on a real navigation, not on a download that
      //    might satisfy the browser's popup heuristic instead.
      var win;
      try { win = window.open(url, "_blank", "noopener,noreferrer"); } catch (_) { win = null; }
      if (!win) {
        fallbackLink.href = url; show(fallback);
        setErr("Popup blocked. Use the link below to open GitHub — your attachments are ready to paste/drop.");
      }

      // 2. Hand media off regardless of the popup outcome. The same
      //    user-click is still an active gesture, so clipboard-write
      //    and programmatic <a download> both work. When the popup is
      //    blocked, the user still gets the image on the clipboard
      //    and the voice file on disk, so by the time they click the
      //    fallback link everything is ready to paste/drop on GitHub.
      if (imgBlob) {
        var wrote = false;
        try {
          if (navigator.clipboard && typeof navigator.clipboard.write === "function" &&
              typeof window.ClipboardItem !== "undefined") {
            navigator.clipboard.write([new window.ClipboardItem({ "image/png": imgBlob })])
              .catch(function () { renderImageDownload(); });
            wrote = true;
          }
        } catch (_) { /* fall through */ }
        if (!wrote) renderImageDownload();
      }
      if (voiceBlob && voiceURL) {
        var a = document.createElement("a");
        a.href = voiceURL;
        a.download = "feedback-voice-" + shortID() + "." + extFromMime(voiceBlob.type);
        document.body.appendChild(a); a.click(); document.body.removeChild(a);
      }
    }

    // doSubmit — single code path for the Submit gesture (Principle
    // XIII). Validates (via updateSubmitEnabled gating the button),
    // POSTs a JSON payload to the Worker, and routes by response:
    //   200 → inline success panel, no auto-close
    //   400 → inline validation error, no fallback (R8)
    //   429 → throttle message with retryAfterSeconds, no fallback
    //   everything else (5xx, network error, timeout, abort) →
    //     doLegacyFallback() with a note in #feedback-hint.
    function doSubmit() {
      if (submitBtn.disabled) return;
      setErr(""); hide(fallback); hide(hint);
      if (successEl) hide(successEl);

      // Missing fetch is an external browser capability boundary, so
      // route to the observable GitHub handoff path.
      if (typeof window.fetch !== "function") {
        doLegacyFallback();
        return;
      }

      submitBtn.disabled = true;
      finishActiveRecordingForSubmit().then(function () {
        var reportVersion = "unknown";
        var genMeta = document.querySelector('meta[name="generator"]');
        if (genMeta && genMeta.content) reportVersion = genMeta.content.slice(0, LIMITS.reportVersionMaxChars);

        // Mint the idempotency key lazily, and only if we don't
        // already have one. A persisted key carried across retries is
        // the entire point of the Worker's 5-minute replay cache:
        // when a first POST actually reaches the backend but the
        // client times out waiting for the response, the retry with
        // the SAME key replays the original success instead of
        // creating a duplicate issue. The enclosing scope resets
        // idempotencyKey to null on definitive success and on
        // closeDialog — those are the only two events that mean
        // "the next Submit is a new logical attempt".
        if (!idempotencyKey) {
          idempotencyKey = generateIdempotencyKey();
        }

        // Snapshot the attachment blob references up front so the
        // async base64 encoding reads a frozen view even if the user
        // clears or replaces the attachment mid-submit. Without
        // these locals, imgBlob/voiceBlob could go null (throwing
        // on `.type`) or swap to a different blob between the
        // `blobToBase64(imgBlob)` kick-off and the `imgBlob.type`
        // read inside the .then callback — so the POST would carry
        // base64 bytes from one blob but a MIME type from another.
        var imgBlobAtSubmit = imgBlob;
        var voiceBlobAtSubmit = voiceBlob;

        // Build attachment promises first; any base64 encoding error
        // routes through the same fallback arm as a Worker failure.
        var imgP = imgBlobAtSubmit ? blobToBase64(imgBlobAtSubmit).then(function (b64) {
          return { mime: imgBlobAtSubmit.type || "image/png", base64: b64 };
        }) : Promise.resolve(null);
        var voiceP = voiceBlobAtSubmit ? blobToBase64(voiceBlobAtSubmit).then(function (b64) {
          return { mime: voiceBlobAtSubmit.type || "audio/webm", base64: b64 };
        }) : Promise.resolve(null);

        return Promise.all([imgP, voiceP]).then(function (parts) {
          // Send the RAW textarea body here — the worker adds the
          // "> Category: …" prefix itself when payload.category is
          // present (see feedback-worker/src/body.ts). maybePrefixBody
          // is only for the fallback GitHub URL, where the worker
          // isn't in the loop. Sending the prefixed body here would
          // double-prepend the category block in worker submissions.
          // Snapshot the trimmed author at payload-construction time
          // so the value persisted on success matches the value sent
          // in the POST. Reading authorInput.value on the success arm
          // would race against user edits made while the request is
          // in flight, leaving localStorage out of sync with the
          // "Submitted by:" line on the actual GitHub issue.
          var authorAtSubmit = authorInput.value.trim();
          var payload = {
            title: titleInput.value,
            // Spec 021-feedback-author-field FR-006: Author is a
            // dedicated required field on the worker payload, not
            // folded into the body. The worker composes the
            // "Submitted by:" line in feedback-worker/src/body.ts.
            author: authorAtSubmit,
            body: bodyInput.value,
            idempotencyKey: idempotencyKey,
            reportVersion: reportVersion
          };
          if (catSelect.value) payload.category = catSelect.value;
          if (parts[0]) payload.image = parts[0];
          if (parts[1]) payload.voice = parts[1];

          var controller = new AbortController();
          var timer = setTimeout(function () { try { controller.abort(); } catch (_) {} }, WORKER_TIMEOUT_MS);

          return fetch(WORKER_URL, {
            method: "POST",
            headers: { "Content-Type": "application/json" },
            body: JSON.stringify(payload),
            signal: controller.signal,
            // `no-store` mirrors the Worker's cache contract.
            cache: "no-store",
            mode: "cors"
          }).then(function (res) {
            clearTimeout(timer);
            return res.json().then(function (data) {
              return { res: res, data: data, retryAfter: res.headers.get("Retry-After"), authorAtSubmit: authorAtSubmit };
            }, function () { return { res: res, data: null, retryAfter: res.headers.get("Retry-After"), authorAtSubmit: authorAtSubmit }; });
          }, function (err) {
            clearTimeout(timer);
            throw err;
          });
        });
      }).then(function (r) {
        var res = r.res, data = r.data || {};
        if (res.status === 200 && data && data.ok && data.issueUrl) {
          // Spec 021-feedback-author-field US2 / R5: persist on
          // definitive worker success only, so a half-typed
          // abandoned name does not pollute the next session.
          // Persist the snapshot captured at submit time (sent in
          // the payload), not the live input — the user may have
          // edited the field while the request was in flight, and
          // the persisted value must match what the issue body says.
          savePersistedAuthor(r.authorAtSubmit);
          renderSuccess(data.issueUrl, data.issueNumber);
          return;
        }
        if (res.status === 400) {
          var msg = (data && data.message) || "Your submission was rejected.";
          setErr(msg);
          submitBtn.disabled = false;
          return;
        }
        if (res.status === 429) {
          var headerRetry = r.retryAfter ? parseInt(r.retryAfter, 10) : 0;
          var retry = headerRetry > 0 ? headerRetry : ((data && typeof data.retryAfterSeconds === "number") ? data.retryAfterSeconds : 0);
          var base = (data && data.message) || "Rate limit reached.";
          var suffix = retry > 0 ? " Try again in " + Math.ceil(retry / 60) + " minute(s)." : "";
          setErr(base + suffix);
          submitBtn.disabled = false;
          return;
        }
        if (res.status === 409) {
          // duplicate_inflight: the Worker is still processing a
          // prior request with the SAME idempotencyKey and hasn't
          // decided yet whether it succeeded. Opening the legacy
          // GitHub URL now would let the user post a second manual
          // issue even if the first request lands successfully on
          // the Worker side — the very duplicate the key is meant
          // to prevent. Show a "still processing" message, keep
          // submitBtn enabled, and keep the sticky idempotencyKey
          // so a retry a few seconds later either replays the
          // cached 200 (first request succeeded) or creates a
          // single issue (inflight marker expired).
          var msg409 = (data && data.message) ||
            "A previous submission with the same key is still being processed. Please wait a few seconds and try again.";
          setErr(msg409);
          submitBtn.disabled = false;
          return;
        }
        // 5xx, 413, unexpected status → fall back.
        doLegacyFallback();
        hint.textContent = "Backend unavailable — opened GitHub with pre-filled form.";
        show(hint);
        submitBtn.disabled = false;
      }).catch(function () {
        // Network error, timeout, abort, or JSON parse/encoding failure.
        doLegacyFallback();
        hint.textContent = "Backend unavailable — opened GitHub with pre-filled form.";
        show(hint);
        submitBtn.disabled = false;
      });
      // Do not auto-close; the user decides when to dismiss the dialog.
    }

    // --- Wiring ---------------------------------------------------

    openBtn.addEventListener("click", openDialog);
    cancelBtn.addEventListener("click", function () { closeDialog(); });
    if (successCloseBtn) {
      successCloseBtn.addEventListener("click", function () { closeDialog(); });
    }
    // Escape fires "cancel"; funnel it through our cleanup path.
    dialog.addEventListener("cancel", function (ev) { ev.preventDefault(); closeDialog(); });
    dialog.addEventListener("close", function () { closeDialog(); });
    // Backdrop click: target is the dialog element itself.
    dialog.addEventListener("click", function (ev) { if (ev.target === dialog) closeDialog(); });
    // Single submit path — catches button click, Enter-in-input, and Cmd/Ctrl+Enter.
    form.addEventListener("submit", function (ev) { ev.preventDefault(); doSubmit(); });
    dialog.addEventListener("keydown", function (ev) {
      if ((ev.metaKey || ev.ctrlKey) && ev.key === "Enter") {
        ev.preventDefault();
        if (!submitBtn.disabled) { form.requestSubmit ? form.requestSubmit() : doSubmit(); }
      }
    });
    // Any content mutation invalidates the current idempotencyKey —
    // the Worker deduplicates by key alone (reserveResponse /
    // cacheResponse replay don't compare payload content). Without
    // this, a retry after an in-flight timeout where the first POST
    // actually succeeded would replay the ORIGINAL cached issueUrl
    // even though the user corrected the title / body / category /
    // attachments before the retry, silently dropping the edits.
    // Calling this on every input/change event is cheap (a single
    // assignment) and the key is re-minted lazily on the next
    // doSubmit.
    function onFormContentChange() {
      idempotencyKey = null;
      updateSubmitEnabled();
    }
    authorInput.addEventListener("input", onFormContentChange);
    titleInput.addEventListener("input", onFormContentChange);
    bodyInput.addEventListener("input", onFormContentChange);
    catSelect.addEventListener("change", onFormContentChange);
    dialog.addEventListener("paste", function (ev) {
      var items = ev.clipboardData && ev.clipboardData.items;
      if (!items) return;
      for (var i = 0; i < items.length; i++) {
        var it = items[i];
        if (it.kind === "file" && it.type && it.type.indexOf("image/") === 0) {
          var blob = it.getAsFile();
          if (blob) { ev.preventDefault(); addAttachment("image", blob); return; }
        }
      }
    });
    recordBtn.addEventListener("click", function () {
      if (recorder && recorder.state === "recording") stopRecording();
      else startRecording();
    });
    updateSubmitEnabled();
  }

  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", boot);
  } else {
    boot();
  }
  function boot() {
    initCollapsePersistence();
    initVariablesSearch();
    initSlowQueryFilters();
    initCharts();
    initNavGroups();
    initNavCollapse();
    initNavScrollSpy();
    initAdvisorFilter();
    initSemaphoreBreakdown();
    initEnvTabs();
    initPrintHook();
    initFeedbackDialog();
    observeContentColumn();
    // Also re-fit on any <details> toggle (open/close affects
    // content-column scrollbar which affects chart width).
    document.addEventListener("toggle", function (ev) {
      // Only react to content-area <details> toggles; nav-group
      // open/close must not trigger a chart-wide resize pass.
      if (!ev.target || !ev.target.closest || !ev.target.closest("main.content")) return;
      window.requestAnimationFrame(resizeAllCharts);
    }, true);
    // The chart sync store + windowed legend stats subscriber wire up
    // their own DOM-time hooks once initCharts has populated the
    // CHARTS registry. The boot path calls initChartSync() (defined in
    // app-js/05.js, last in lexical order) here so the order is
    // explicit rather than implicit-by-load-order.
    initChartSync();
  }
