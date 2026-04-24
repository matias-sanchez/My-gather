# Phase 1 Quickstart — Report Feedback Button

A short, deterministic walkthrough for verifying the feature end-to-end once it's implemented. Maps to the acceptance scenarios in `spec.md`.

## Prerequisites

- macOS or Linux laptop with Chrome / Edge (Chromium family) **and** Firefox installed — both within their two most recent releases (per SC-006).
- A working pt-stalk capture on disk, e.g. `testdata/example2/` inside this repo.
- The ability to disable networking for one of the browsers (Airplane mode / System Network off) to test the offline path.

## Build

```bash
make build
# produces bin/my-gather
```

## Generate a sample report

```bash
bin/my-gather \
    -root testdata/example2 \
    -out /tmp/report-feedback.html
```

## Smoke-test scenarios

### Scenario A — US1: text-only feedback

1. Open `/tmp/report-feedback.html` in Chrome/Edge.
2. Expect a "Report feedback" button in the top-right of the report header.
3. Click it. A centred modal dialog opens with title + body + category selector + Submit + Cancel.
4. Type a title "Log-scale toggle for IOPS chart" and a body "Would help compare idle vs peak."
5. Click Submit.
6. A new tab opens to `https://github.com/matias-sanchez/My-gather/discussions/new?category=ideas&title=Log-scale%20toggle%20for%20IOPS%20chart&body=Would%20help%20compare%20idle%20vs%20peak.` — verify that GitHub renders the title and body pre-filled.
7. Do not actually submit on GitHub (this is a smoke test).

### Scenario B — US2: keyboard-only flow

1. Same starting conditions as A.
2. Press Tab repeatedly until the "Report feedback" button receives focus. Verify visible focus ring.
3. Press Enter. Dialog opens; focus is on the title input.
4. Type a title. Tab into the body. Type a body. Tab into category. Pick "UI".
5. Press Cmd+Enter (macOS) or Ctrl+Enter (Linux). New tab opens with the pre-filled URL including the category marker in the body.
6. Press Escape. Dialog closes. Focus returns to the "Report feedback" button.

### Scenario C — US3 + US4: category marker + image paste (Chromium)

1. Take a screenshot with the OS tool (macOS: `Cmd+Ctrl+Shift+4` region; Linux: any screenshot tool that lands on the clipboard).
2. Open `/tmp/report-feedback.html`, click "Report feedback".
3. Paste (Cmd/Ctrl+V) anywhere inside the dialog's body area. A thumbnail appears in the Attachments area.
4. Pick a category ("Parser"). Fill title + body.
5. Click Submit.
6. GitHub opens in a new tab with the pre-filled title and a body beginning with `> Category: Parser\n\n<body>`.
7. Switch to the new GitHub tab, focus the body textarea, press Cmd/Ctrl+V. The image uploads inline (GitHub auto-uploads pasted images).

### Scenario D — US5: voice recording (Firefox)

1. Open `/tmp/report-feedback.html` in Firefox (to verify the non-Chromium path).
2. Click "Report feedback". Fill title + body.
3. Click "Record voice note". Firefox prompts for microphone permission. Allow it.
4. Speak for ~5 seconds. Click Stop.
5. A playback control appears in Attachments.
6. Click Submit.
7. The browser downloads a file named `feedback-voice-<short>.webm` (Firefox) or `.mp4` (Safari, if tested).
8. In the new GitHub tab, drag-drop the downloaded file into the body textarea. GitHub auto-uploads and embeds.

### Scenario E — SC-002 / SC-007: offline drafting

1. Turn off WiFi (airplane mode on laptop).
2. Open `/tmp/report-feedback.html` (it was previously saved, so it loads from disk).
3. Click "Report feedback". Dialog opens — good.
4. Type title + body. Paste an image. Record a voice note. Remove the image. Paste another.
5. Verify every interaction works with no network.
6. Click Submit. The new GitHub tab fails to load (expected offline). The feedback dialog stays open with the draft intact. No data lost.
7. Turn WiFi back on and click the fallback inline link the dialog surfaced after the failed open. Verify GitHub now loads with the pre-filled URL.

### Scenario F — SC-003: determinism

```bash
bin/my-gather -root testdata/example2 -out /tmp/a.html
bin/my-gather -root testdata/example2 -out /tmp/b.html
diff /tmp/a.html /tmp/b.html
```

Expect zero-byte diff. Same on Linux and macOS.

### Scenario G — popup-blocker fallback (FR-015)

1. Enable popup blocking (Chrome: Settings → Privacy → Pop-ups → Don't allow sites to send).
2. Open report, click "Report feedback", fill form, Submit.
3. Since the click is user-initiated, the blocker should NOT engage. If the browser still blocks, verify an inline "Click here to open GitHub" link appears with the same pre-filled URL.
4. Click the link. A new tab opens with the pre-filled discussion.

### Scenario H — clipboard-write blocked (FR-021)

1. Use a browser where `navigator.clipboard.write` is unavailable or permission-denied (Firefox on HTTP sometimes does this).
2. Attach a pasted image, Submit.
3. Verify the dialog displays an inline "Your browser blocked the clipboard copy. Download the image and drag it into GitHub's body." message with a visible Download link.
4. Click Download. Image downloads.
5. Drag the downloaded file into the GitHub tab. GitHub uploads and embeds.

## Acceptance checklist

- [ ] Scenario A passes on Chromium-family browser.
- [ ] Scenario B passes (keyboard-only).
- [ ] Scenario C passes (image paste → clipboard → paste into GitHub).
- [ ] Scenario D passes (voice → download → drag into GitHub).
- [ ] Scenario E passes (offline drafting + recovery).
- [ ] Scenario F: determinism diff is empty.
- [ ] Scenario G: popup-blocker fallback surfaces inline link.
- [ ] Scenario H: clipboard-blocked fallback surfaces inline download.

Any failure blocks the feature from shipping.
