import { describe, expect, it } from "vitest";

import { buildIssueBody } from "../src/body";
import type { ValidatedPayload } from "../src/validate";

function mkPayload(overrides: Partial<ValidatedPayload> = {}): ValidatedPayload {
  return {
    title: "Anything",
    author: "Jane Doe",
    body: "A short description of the problem.",
    idempotencyKey: "550e8400-e29b-41d4-a716-446655440000",
    reportVersion: "0.3.1-54-g29734aa",
    ...overrides,
  };
}

describe("buildIssueBody", () => {
  it("renders attribution + body + footer when no attachments and no category", () => {
    const out = buildIssueBody({ payload: mkPayload() });
    expect(out).toMatchInlineSnapshot(`
      "Submitted by: Jane Doe

      A short description of the problem.

      ---
      _Submitted via my-gather Report Feedback (v0.3.1-54-g29734aa)._"
    `);
  });

  it("places attribution before the category blockquote when category is set", () => {
    const out = buildIssueBody({ payload: mkPayload({ category: "UI" }) });
    expect(out).toMatchInlineSnapshot(`
      "Submitted by: Jane Doe

      > Category: UI

      A short description of the problem.

      ---
      _Submitted via my-gather Report Feedback (v0.3.1-54-g29734aa)._"
    `);
  });

  it("includes the screenshot section when an imageUrl is passed", () => {
    const out = buildIssueBody({
      payload: mkPayload(),
      imageUrl: "https://assets.test/attachments/abc.png",
    });
    expect(out).toMatchInlineSnapshot(`
      "Submitted by: Jane Doe

      A short description of the problem.

      ### Attached screenshot

      ![screenshot](https://assets.test/attachments/abc.png)

      ---
      _Submitted via my-gather Report Feedback (v0.3.1-54-g29734aa)._"
    `);
  });

  it("includes the voice note section when a voiceUrl is passed", () => {
    const out = buildIssueBody({
      payload: mkPayload(),
      voiceUrl: "https://assets.test/attachments/def.webm",
    });
    expect(out).toMatchInlineSnapshot(`
      "Submitted by: Jane Doe

      A short description of the problem.

      ### Attached voice note

      https://assets.test/attachments/def.webm

      ---
      _Submitted via my-gather Report Feedback (v0.3.1-54-g29734aa)._"
    `);
  });

  it("orders attribution, category, body, screenshot, voice, footer", () => {
    const out = buildIssueBody({
      payload: mkPayload({ category: "Parser" }),
      imageUrl: "https://assets.test/attachments/abc.png",
      voiceUrl: "https://assets.test/attachments/def.webm",
    });
    expect(out).toMatchInlineSnapshot(`
      "Submitted by: Jane Doe

      > Category: Parser

      A short description of the problem.

      ### Attached screenshot

      ![screenshot](https://assets.test/attachments/abc.png)

      ### Attached voice note

      https://assets.test/attachments/def.webm

      ---
      _Submitted via my-gather Report Feedback (v0.3.1-54-g29734aa)._"
    `);
  });

  it("uses the validated author verbatim in the attribution line", () => {
    const out = buildIssueBody({ payload: mkPayload({ author: "Alex (Triage)" }) });
    expect(out.split("\n")[0]).toBe("Submitted by: Alex (Triage)");
    expect(out.split("\n")[1]).toBe("");
  });
});
