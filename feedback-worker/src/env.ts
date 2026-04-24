// Shared Env binding shape. The fields here must match wrangler.toml bindings
// plus the secrets set via `wrangler secret put`.

export interface Env {
  // KV / R2 bindings (defined in wrangler.toml)
  FEEDBACK_RATELIMIT: KVNamespace;
  FEEDBACK_IDEMP: KVNamespace;
  FEEDBACK_ATTACHMENTS: R2Bucket;

  // Secrets (wrangler secret put ...)
  GITHUB_APP_ID: string;
  GITHUB_INSTALLATION_ID: string;
  GITHUB_REPO_ID: string;
  GITHUB_APP_PRIVATE_KEY: string;
  R2_PUBLIC_URL_PREFIX: string;
}
