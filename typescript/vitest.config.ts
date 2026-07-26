import { defineConfig } from "vitest/config";
import { fileURLToPath } from "node:url";

// Repo root (parent of typescript/). Service tests source their stub bodies from
// the shared, coverage-guarded fixtures under <repoRoot>/spec/fixtures via JSON
// imports; pin the fs allow-list to the repo root so those out-of-package
// imports resolve regardless of Vite's workspace-root auto-detection (which a
// nested package.json or a strict downstream environment could otherwise
// narrow).
const repoRoot = fileURLToPath(new URL("..", import.meta.url));

export default defineConfig({
  server: {
    fs: {
      allow: [repoRoot],
    },
  },
  test: {
    globals: true,
    environment: "node",
    include: ["tests/**/*.test.ts"],
    coverage: {
      provider: "v8",
      reporter: ["text", "json", "html"],
      include: ["src/**/*.ts"],
      exclude: ["src/generated/**"],
      thresholds: {
        branches: 80,
        functions: 85,
        lines: 85,
        statements: 85,
      },
    },
    setupFiles: ["./tests/setup.ts"],
  },
});
