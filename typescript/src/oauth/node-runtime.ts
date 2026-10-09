/**
 * Runtime guard for the OAuth helpers that depend on Node.js built-ins.
 *
 * `performInteractiveLogin` and `startCallbackServer` listen on a local HTTP
 * server (`node:http`), and `FileTokenStore` writes to disk (`node:fs`).
 * React Native and browsers have neither. Their bundlers either refuse the
 * import or substitute an empty module, and the helper then fails much later
 * with an error that names a missing function or a network fetch instead of
 * the cause. Checking up front turns that into one clear `usage` error.
 */

import { BasecampError } from "../errors.js";

const NON_NODE_HINT =
  "In React Native or a browser, open the authorization URL in the platform's auth session, " +
  "exchange the code on your server, and keep tokens in the platform's secure storage. " +
  'See "React Native and browsers" in the SDK README.';

/**
 * True when the current runtime provides Node.js built-ins: Node itself, or a
 * runtime that implements them and reports `process.versions.node` (Bun,
 * Deno's Node compatibility, Electron's main process). React Native and
 * browsers do not report it, including under the common `process` polyfills,
 * which leave `process.versions` empty.
 */
export function isNodeRuntime(): boolean {
  const proc = (globalThis as { process?: { versions?: { node?: unknown } } }).process;
  return typeof proc?.versions?.node === "string";
}

/**
 * Throws a `usage` BasecampError naming `feature` when the runtime is not Node.js.
 *
 * @param feature - What needs Node, e.g. "performInteractiveLogin (local HTTP callback server)"
 */
export function requireNodeRuntime(feature: string): void {
  if (!isNodeRuntime()) {
    throw new BasecampError("usage", `${feature} requires Node.js and cannot run in React Native or a browser`, {
      hint: NON_NODE_HINT,
    });
  }
}
