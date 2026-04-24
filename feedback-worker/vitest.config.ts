import { defineConfig } from "vitest/config";

// Unit tests run in the default node pool — pure/ish validators,
// hand-mocked KV / fetch. No Workers runtime needed.
export default defineConfig({
  test: {
    include: ["test/**/*.test.ts"],
    environment: "node",
  },
});
