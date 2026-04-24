// Build the GitHub Issue markdown body from a validated payload.
// Shape matches the "Issue body markdown composition" block in the Agent W
// task prompt (which supersedes data-model.md's older Discussion shape).

import type { ValidatedPayload } from "./validate";

export interface BuildIssueBodyInput {
  payload: ValidatedPayload;
  imageUrl?: string;
  voiceUrl?: string;
}

export function buildIssueBody(input: BuildIssueBodyInput): string {
  const { payload, imageUrl, voiceUrl } = input;
  const parts: string[] = [];

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
