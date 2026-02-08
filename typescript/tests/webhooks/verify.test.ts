import { describe, it, expect } from "vitest";
import { verifyWebhookSignature, signWebhookPayload } from "../../src/webhooks/verify.js";

describe("signWebhookPayload", () => {
  it("produces deterministic signatures", () => {
    const sig1 = signWebhookPayload("test", "secret");
    const sig2 = signWebhookPayload("test", "secret");
    expect(sig1).toBe(sig2);
  });

  it("produces different signatures for different payloads", () => {
    const sig1 = signWebhookPayload("payload1", "secret");
    const sig2 = signWebhookPayload("payload2", "secret");
    expect(sig1).not.toBe(sig2);
  });

  it("produces different signatures for different secrets", () => {
    const sig1 = signWebhookPayload("test", "secret1");
    const sig2 = signWebhookPayload("test", "secret2");
    expect(sig1).not.toBe(sig2);
  });
});

describe("verifyWebhookSignature", () => {
  it("round-trips with signWebhookPayload", () => {
    const payload = '{"id":1,"kind":"todo_created"}';
    const secret = "test-secret-key";
    const signature = signWebhookPayload(payload, secret);

    expect(verifyWebhookSignature(payload, signature, secret)).toBe(true);
  });

  it("works with Buffer payloads", () => {
    const payload = Buffer.from('{"id":1}');
    const secret = "key";
    const signature = signWebhookPayload(payload, secret);

    expect(verifyWebhookSignature(payload, signature, secret)).toBe(true);
  });

  it("rejects invalid signatures", () => {
    expect(verifyWebhookSignature("data", "wrong", "secret")).toBe(false);
  });

  it("returns false for empty secret", () => {
    expect(verifyWebhookSignature("data", "sig", "")).toBe(false);
  });

  it("returns false for empty signature", () => {
    expect(verifyWebhookSignature("data", "", "secret")).toBe(false);
  });

  it("returns false when signature length differs", () => {
    expect(verifyWebhookSignature("data", "short", "secret")).toBe(false);
  });
});
