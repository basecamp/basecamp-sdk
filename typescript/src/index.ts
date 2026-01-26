/**
 * Basecamp TypeScript SDK
 *
 * Type-safe client for the Basecamp 3 API.
 *
 * @example
 * ```ts
 * import { createBasecampClient } from "@basecamp/sdk";
 *
 * const client = createBasecampClient({
 *   accountId: "12345",
 *   accessToken: process.env.BASECAMP_TOKEN!,
 * });
 *
 * // Type-safe API calls
 * const { data, error } = await client.GET("/projects.json");
 *
 * if (data) {
 *   console.log(data.map(p => p.name));
 * }
 * ```
 *
 * @packageDocumentation
 */

// Main client factory
export {
  createBasecampClient,
  type BasecampClient,
  type BasecampClientOptions,
  type TokenProvider,
} from "./client.js";

// Pagination helpers
export { fetchAllPages, paginateAll } from "./client.js";

// Re-export generated types
export type { paths } from "./generated/schema.js";
