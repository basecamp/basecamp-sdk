/**
 * Tests for updateAccountLogo (hand-written multipart upload)
 */
import { describe, it, expect, vi, beforeEach } from "vitest";
import { http, HttpResponse } from "msw";
import { server } from "./setup.js";
import { createBasecampClient } from "../src/client.js";

const BASE_URL = "https://3.basecampapi.com/12345";

describe("updateAccountLogo", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("should send a multipart PUT and succeed on 204", async () => {
    let capturedRequest: Request | null = null;

    server.use(
      http.put(`${BASE_URL}/account/logo.json`, async ({ request }) => {
        capturedRequest = request;
        return new HttpResponse(null, { status: 204 });
      }),
    );

    const client = createBasecampClient({
      accountId: "12345",
      accessToken: "test-token",
    });

    const blob = new Blob(["fake-png-data"], { type: "image/png" });
    await client.account.updateAccountLogo(blob, "logo.png");

    expect(capturedRequest).not.toBeNull();
    expect(capturedRequest!.method).toBe("PUT");
    expect(capturedRequest!.headers.get("Authorization")).toBe("Bearer test-token");

    // Verify multipart body contains the file
    const formData = await capturedRequest!.formData();
    const file = formData.get("logo");
    expect(file).toBeInstanceOf(File);
    expect((file as File).name).toBe("logo.png");
  });

  it("should throw on non-204 response", async () => {
    server.use(
      http.put(`${BASE_URL}/account/logo.json`, () => {
        return HttpResponse.json(
          { error: "File too large" },
          { status: 422 },
        );
      }),
    );

    const client = createBasecampClient({
      accountId: "12345",
      accessToken: "test-token",
    });

    const blob = new Blob(["data"], { type: "image/png" });
    await expect(client.account.updateAccountLogo(blob)).rejects.toThrow();
  });

  it("should fire operation hooks", async () => {
    server.use(
      http.put(`${BASE_URL}/account/logo.json`, () => {
        return new HttpResponse(null, { status: 204 });
      }),
    );

    const onOperationStart = vi.fn();
    const onOperationEnd = vi.fn();

    const client = createBasecampClient({
      accountId: "12345",
      accessToken: "test-token",
      hooks: { onOperationStart, onOperationEnd },
    });

    const blob = new Blob(["data"], { type: "image/png" });
    await client.account.updateAccountLogo(blob, "logo.png");

    expect(onOperationStart).toHaveBeenCalledWith(
      expect.objectContaining({
        service: "Account",
        operation: "UpdateAccountLogo",
        isMutation: true,
      }),
    );
    expect(onOperationEnd).toHaveBeenCalledWith(
      expect.objectContaining({ operation: "UpdateAccountLogo" }),
      expect.objectContaining({ durationMs: expect.any(Number) }),
    );
  });
});
