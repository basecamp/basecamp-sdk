/**
 * Premise verification for the `page` option's semantics.
 *
 * These tests pin what the SDK does TODAY so the cross-SDK consistency
 * decision (see the follow-up issue referenced in SPEC §8) is made against
 * measured behavior rather than assumption.
 */
import { describe, it, expect } from "vitest";
import { http, HttpResponse } from "msw";
import { server } from "./setup.js";
import { createBasecampClient } from "../src/client.js";
import { ProjectsService } from "../src/generated/services/projects.js";

const BASE_URL = "https://3.basecampapi.com/12345";

describe("page option semantics", () => {
  it("keeps following Link rel=next after the pinned page", async () => {
    const requested: string[] = [];

    server.use(
      http.get(`${BASE_URL}/projects.json`, ({ request }) => {
        const url = new URL(request.url);
        requested.push(url.searchParams.get("page") ?? "(none)");
        const page = Number(url.searchParams.get("page") ?? "1");
        // Pages 3 and 4 exist; page 4 is the last.
        const headers: Record<string, string> = {};
        if (page < 4) {
          headers.Link = `<${BASE_URL}/projects.json?page=${page + 1}>; rel="next"`;
        }
        return HttpResponse.json([{ id: page * 10, name: `p${page}` }], { headers });
      })
    );

    const client = createBasecampClient({ accountId: "12345", accessToken: "t" });
    const projects = new ProjectsService(client);

    const result = await projects.list({ page: 3 });

    // The premise: asking for page 3 does NOT return just page 3.
    expect(requested).toEqual(["3", "4"]);
    expect(result.length).toBe(2);
  });

  it("stops at the pinned page when maxItems bounds the walk", async () => {
    const requested: string[] = [];

    server.use(
      http.get(`${BASE_URL}/projects.json`, ({ request }) => {
        const url = new URL(request.url);
        requested.push(url.searchParams.get("page") ?? "(none)");
        const page = Number(url.searchParams.get("page") ?? "1");
        return HttpResponse.json([{ id: page * 10 }], {
          headers: { Link: `<${BASE_URL}/projects.json?page=${page + 1}>; rel="next"` },
        });
      })
    );

    const client = createBasecampClient({ accountId: "12345", accessToken: "t" });
    const projects = new ProjectsService(client);

    const result = await projects.list({ page: 3, maxItems: 1 });

    // maxItems is the only brake available to a caller who wants one page.
    expect(requested).toEqual(["3"]);
    expect(result.length).toBe(1);
  });
});
