// Upload validated attachments to R2 under a content-hashed key.
// Same content → same key, so the put is skipped when the object is already there.

import type { Env } from "./env";

export interface AttachmentInput {
  mime: string;
  bytes: Uint8Array;
}

export interface AttachmentUpload {
  publicUrl: string;
  sha256: string;
  key: string;
  created: boolean;
}

const MIME_TO_EXT: Record<string, string> = {
  "image/png": "png",
  "image/jpeg": "jpg",
  "image/gif": "gif",
  "image/webp": "webp",
  "audio/webm": "webm",
  "audio/mp4": "m4a",
  "audio/ogg": "ogg",
  "audio/mpeg": "mp3",
};

export function extForMime(mime: string): string {
  // Strip any ;parameters from audio mimes (e.g., "audio/webm;codecs=opus").
  const base = mime.split(";")[0]!.trim().toLowerCase();
  const ext = MIME_TO_EXT[base];
  return ext ?? "bin";
}

async function sha256Hex(bytes: Uint8Array): Promise<string> {
  // Copy into a fresh ArrayBuffer so Web Crypto never sees a SharedArrayBuffer view.
  const view = new Uint8Array(bytes.byteLength);
  view.set(bytes);
  const digest = await crypto.subtle.digest("SHA-256", view.buffer);
  const out = new Uint8Array(digest);
  let hex = "";
  for (let i = 0; i < out.byteLength; i++) {
    hex += out[i]!.toString(16).padStart(2, "0");
  }
  return hex;
}

function trimSlash(s: string): string {
  return s.endsWith("/") ? s.slice(0, -1) : s;
}

export async function uploadAttachment(
  env: Env,
  attachment: AttachmentInput,
): Promise<AttachmentUpload> {
  const sha256 = await sha256Hex(attachment.bytes);
  const ext = extForMime(attachment.mime);
  const key = `attachments/${sha256}.${ext}`;

  const existing = await env.FEEDBACK_ATTACHMENTS.head(key);
  let created = false;
  if (!existing) {
    await env.FEEDBACK_ATTACHMENTS.put(key, attachment.bytes, {
      httpMetadata: { contentType: attachment.mime },
    });
    created = true;
  }

  const publicUrl = `${trimSlash(env.R2_PUBLIC_URL_PREFIX)}/${key}`;
  return { publicUrl, sha256, key, created };
}

export async function deleteAttachment(env: Env, upload: AttachmentUpload): Promise<void> {
  if (!upload.created) return;
  await env.FEEDBACK_ATTACHMENTS.delete(upload.key);
}
