import { describe, it, expect } from "vitest";
import { EventEmitter } from "node:events";
import { readFileSync } from "node:fs";
import { join } from "node:path";
import { WebhookReceiver } from "../../../src/webhooks/handler.js";
import { signWebhookPayload } from "../../../src/webhooks/verify.js";
import { createNodeHandler } from "../../../src/webhooks/adapters/node-http.js";
import type { IncomingMessage, ServerResponse } from "node:http";

const fixturesDir = join(__dirname, "..", "..", "..", "..", "spec", "fixtures", "webhooks");
const todoCreatedBody = readFileSync(join(fixturesDir, "event-todo-created.json"), "utf8");

/** Create a minimal mock IncomingMessage */
function mockRequest(options: {
  method?: string;
  url?: string;
  headers?: Record<string, string>;
  body?: string;
}): IncomingMessage {
  const emitter = new EventEmitter() as IncomingMessage;
  emitter.method = options.method ?? "POST";
  emitter.url = options.url ?? "/webhooks/basecamp";
  emitter.headers = {};
  if (options.headers) {
    for (const [k, v] of Object.entries(options.headers)) {
      emitter.headers[k.toLowerCase()] = v;
    }
  }
  // Emit data and end on next tick
  process.nextTick(() => {
    if (options.body) {
      emitter.emit("data", Buffer.from(options.body));
    }
    emitter.emit("end");
  });
  return emitter;
}

/** Create a minimal mock ServerResponse that captures status and body */
function mockResponse(): ServerResponse & { _status: number; _body: string } {
  const res = {
    _status: 0,
    _body: "",
    writeHead(status: number) {
      res._status = status;
      return res;
    },
    end(body?: string) {
      if (body) res._body = body;
    },
  } as unknown as ServerResponse & { _status: number; _body: string };
  return res;
}

/** Wait for the handler to complete (next tick for body collection + async handler) */
function waitForResponse(res: ReturnType<typeof mockResponse>): Promise<void> {
  return new Promise((resolve) => {
    const check = () => {
      if (res._status !== 0) {
        resolve();
      } else {
        setTimeout(check, 5);
      }
    };
    setTimeout(check, 10);
  });
}

describe("createNodeHandler", () => {
  it("returns 200 on valid POST", async () => {
    const receiver = new WebhookReceiver();
    receiver.onAny(() => {});
    const handler = createNodeHandler(receiver);

    const req = mockRequest({ body: todoCreatedBody });
    const res = mockResponse();
    handler(req, res);
    await waitForResponse(res);

    expect(res._status).toBe(200);
  });

  it("returns 405 on GET", async () => {
    const receiver = new WebhookReceiver();
    const handler = createNodeHandler(receiver);

    const req = mockRequest({ method: "GET" });
    const res = mockResponse();
    handler(req, res);
    await waitForResponse(res);

    expect(res._status).toBe(405);
  });

  it("returns 404 on wrong path", async () => {
    const receiver = new WebhookReceiver();
    const handler = createNodeHandler(receiver);

    const req = mockRequest({ url: "/other", body: todoCreatedBody });
    const res = mockResponse();
    handler(req, res);
    await waitForResponse(res);

    expect(res._status).toBe(404);
  });

  it("returns 401 on bad signature when secret is set", async () => {
    const receiver = new WebhookReceiver({ secret: "test-secret" });
    const handler = createNodeHandler(receiver);

    const req = mockRequest({
      body: todoCreatedBody,
      headers: { "x-basecamp-signature": "bad" },
    });
    const res = mockResponse();
    handler(req, res);
    await waitForResponse(res);

    expect(res._status).toBe(401);
  });

  it("returns 200 with valid signature", async () => {
    const secret = "test-secret";
    const sig = signWebhookPayload(todoCreatedBody, secret);
    const receiver = new WebhookReceiver({ secret });
    receiver.onAny(() => {});
    const handler = createNodeHandler(receiver);

    const req = mockRequest({
      body: todoCreatedBody,
      headers: { "x-basecamp-signature": sig },
    });
    const res = mockResponse();
    handler(req, res);
    await waitForResponse(res);

    expect(res._status).toBe(200);
  });

  it("returns 413 on oversized body", async () => {
    const receiver = new WebhookReceiver();
    receiver.onAny(() => {});
    const handler = createNodeHandler(receiver, { maxBodyBytes: 50 });

    const req = mockRequest({ body: todoCreatedBody }); // well over 50 bytes
    const res = mockResponse();
    handler(req, res);
    await waitForResponse(res);

    expect(res._status).toBe(413);
  });

  it("uses custom path", async () => {
    const receiver = new WebhookReceiver();
    receiver.onAny(() => {});
    const handler = createNodeHandler(receiver, { path: "/custom" });

    // Default path should 404
    const req1 = mockRequest({ body: todoCreatedBody });
    const res1 = mockResponse();
    handler(req1, res1);
    await waitForResponse(res1);
    expect(res1._status).toBe(404);

    // Custom path should 200
    const req2 = mockRequest({ url: "/custom", body: todoCreatedBody });
    const res2 = mockResponse();
    handler(req2, res2);
    await waitForResponse(res2);
    expect(res2._status).toBe(200);
  });
});
