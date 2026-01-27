/**
 * Vitest setup file for MSW (Mock Service Worker)
 */
import { afterAll, afterEach, beforeAll } from "vitest";
import { setupServer } from "msw/node";

// Create server instance outside of lifecycle hooks for reuse
export const server = setupServer();

// Start server before all tests
beforeAll(() => server.listen({ onUnhandledRequest: "error" }));

// Reset handlers after each test (important for test isolation)
afterEach(() => server.resetHandlers());

// Clean up after all tests
afterAll(() => server.close());
