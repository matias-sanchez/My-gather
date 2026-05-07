// Build the GitHub Issue markdown body from a validated payload.
// Shape matches specs/003-feedback-backend-worker/data-model.md
// "Issue body markdown composition".

import type { ValidatedPayload } from "./validate";

export interface BuildIssueBodyInput {
  payload: ValidatedPayload;
  imageUrl?: string;
  voiceUrl?: string;
}

export function buildIssueBody(input: BuildIssueBodyInput): string {
  const { payload, imageUrl, voiceUrl } = input;
  const parts: string[] = [];

  // Spec 021-feedback-author-field: triagers need to know who
  // reported the issue. Author is a required validated field, so
  // this line is always present and always first.
  parts.push(`Submitted by: ${payload.author}`);
  parts.push("");

  if (payload.category) {
    parts.push(`> Category: ${payload.category}`);
    parts.push("");
  }

  parts.push(payload.body);

  if (imageUrl) {
    parts.push("");
    parts.push("### Attached screenshot");
    parts.push("");
    parts.push(`![screenshot](${imageUrl})`);
  }

  if (voiceUrl) {
    parts.push("");
    parts.push("### Attached voice note");
    parts.push("");
    // GitHub auto-renders raw audio URLs as inline players.
    parts.push(voiceUrl);
  }

  parts.push("");
  parts.push("---");
  parts.push(`_Submitted via my-gather Report Feedback (v${payload.reportVersion})._`);

  return parts.join("\n");
}
