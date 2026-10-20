import { describe, it, expect, beforeEach } from "vitest";
import { http, HttpResponse } from "msw";
import { server } from "../setup.js";
import { createBasecampClient } from "../../src/client.js";
import { BasecampError } from "../../src/errors.js";
import type { BasecampClient } from "../../src/client.js";
import pendingFixture from "../../../spec/fixtures/embeds/pending.json" with { type: "json" };
import activeFixture from "../../../spec/fixtures/embeds/active.json" with { type: "json" };

const BASE_URL = "https://3.basecampapi.com/12345";
const YOUTUBE_URL = "https://www.youtube.com/watch?v=dQw4w9WgXcQ";

describe("EmbedsService", () => {
  let client: BasecampClient;

  beforeEach(() => {
    client = createBasecampClient({
      accountId: "12345",
      accessToken: "test-token",
      enableRetry: false,
    });
  });

  describe("create", () => {
    it("should send the url and return the pending embed", async () => {
      let body: unknown;
      server.use(
        http.post(`${BASE_URL}/embeds.json`, async ({ request }) => {
          body = await request.json();
          return HttpResponse.json(pendingFixture);
        })
      );

      const embed = await client.embeds.create({ url: YOUTUBE_URL });
      expect(body).toEqual({ url: YOUTUBE_URL });
      expect(embed.status).toBe("pending");
      expect(embed.sgid).toBe(pendingFixture.sgid);
      expect(embed.content_type).toBeUndefined();
    });

    it("should return an existing active embed", async () => {
      server.use(
        http.post(`${BASE_URL}/embeds.json`, () => HttpResponse.json(activeFixture))
      );

      const embed = await client.embeds.create({ url: YOUTUBE_URL });
      expect(embed.status).toBe("active");
      expect(embed.content_type).toBe("embed/youtube-video");
      expect(embed.image_url).toBeNull();
    });

    it("should surface a 422 for a URL no provider embeds", async () => {
      server.use(
        http.post(`${BASE_URL}/embeds.json`, () =>
          HttpResponse.json({ error: "Basecamp can't embed this URL. Link to it instead." }, { status: 422 })
        )
      );

      const err = await client.embeds.create({ url: "https://example.com/page" }).catch((e: unknown) => e);
      expect(err).toBeInstanceOf(BasecampError);
      expect((err as BasecampError).code).toBe("validation");
      expect((err as BasecampError).httpStatus).toBe(422);
      expect((err as BasecampError).message).toBe("Basecamp can't embed this URL. Link to it instead.");
    });

    it("should surface a 429 as rate_limit on the first attempt, even with retry enabled", async () => {
      let attempts = 0;
      server.use(
        http.post(`${BASE_URL}/embeds.json`, () => {
          attempts++;
          return HttpResponse.json({ error: "Too many new embeds. Try again later." }, { status: 429 });
        })
      );
      const retrying = createBasecampClient({ accountId: "12345", accessToken: "test-token" });

      const err = await retrying.embeds.create({ url: YOUTUBE_URL }).catch((e: unknown) => e);
      expect(attempts).toBe(1);
      expect(err).toBeInstanceOf(BasecampError);
      expect((err as BasecampError).code).toBe("rate_limit");
      expect((err as BasecampError).httpStatus).toBe(429);
    });
  });

  describe("get", () => {
    it("should escape the sgid as one path segment", async () => {
      let requested = "";
      server.use(
        http.get(`${BASE_URL}/embeds/*`, ({ request }) => {
          requested = new URL(request.url).pathname;
          return HttpResponse.json(activeFixture);
        })
      );

      const embed = await client.embeds.get(pendingFixture.sgid);
      expect(embed.status).toBe("active");
      expect(requested).toBe(`/12345/embeds/${encodeURIComponent(pendingFixture.sgid)}`);
      expect(requested).toContain("%2F");
    });

    it("should throw not_found for an unknown sgid", async () => {
      server.use(
        http.get(`${BASE_URL}/embeds/missing`, () =>
          HttpResponse.json({ error: "Not found" }, { status: 404 })
        )
      );

      const err = await client.embeds.get("missing").catch((e: unknown) => e);
      expect(err).toBeInstanceOf(BasecampError);
      expect((err as BasecampError).code).toBe("not_found");
    });
  });
});
