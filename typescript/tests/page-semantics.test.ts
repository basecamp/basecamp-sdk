/**
 * The `page` option's contract (SPEC §8): a positive `page` selects exactly
 * that page — one request, that page's items, no `Link: rel="next"` follow.
 *
 * This file used to pin the opposite as the measured premise for #566: `page`
 * as a starting offset that link-following continued from. #566 converged all
 * six SDKs on Go's selector semantics, so the assertions flipped with it.
 */
import { describe, it, expect } from "vitest";
import { http, HttpResponse } from "msw";
import { server } from "./setup.js";
import { createBasecampClient } from "../src/client.js";
import { ProjectsService } from "../src/generated/services/projects.js";

const BASE_URL = "https://3.basecampapi.com/12345";

describe("page option semantics", () => {
  it("returns only the pinned page and does not follow Link rel=next", async () => {
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

    expect(requested).toEqual(["3"]);
    expect(result.length).toBe(1);
    // Page 4 existed and was deliberately not fetched, so what came back is a
    // partial view of the collection.
    expect(result.meta.truncated).toBe(true);
  });

  it("reports a pinned final page as complete", async () => {
    const requested: string[] = [];

    server.use(
      http.get(`${BASE_URL}/projects.json`, ({ request }) => {
        const url = new URL(request.url);
        requested.push(url.searchParams.get("page") ?? "(none)");
        // No Link header: this is the last page.
        return HttpResponse.json([{ id: 40, name: "p4" }]);
      })
    );

    const client = createBasecampClient({ accountId: "12345", accessToken: "t" });
    const projects = new ProjectsService(client);

    const result = await projects.list({ page: 4 });

    expect(requested).toEqual(["4"]);
    expect(result.meta.truncated).toBe(false);
  });

  it("still applies maxItems to the pinned page", async () => {
    const requested: string[] = [];

    server.use(
      http.get(`${BASE_URL}/projects.json`, ({ request }) => {
        const url = new URL(request.url);
        requested.push(url.searchParams.get("page") ?? "(none)");
        const page = Number(url.searchParams.get("page") ?? "1");
        return HttpResponse.json([{ id: page * 10 }, { id: page * 10 + 1 }], {
          headers: { Link: `<${BASE_URL}/projects.json?page=${page + 1}>; rel="next"` },
        });
      })
    );

    const client = createBasecampClient({ accountId: "12345", accessToken: "t" });
    const projects = new ProjectsService(client);

    const result = await projects.list({ page: 3, maxItems: 1 });

    expect(requested).toEqual(["3"]);
    expect(result.length).toBe(1);
    expect(result.meta.truncated).toBe(true);
  });

  it("treats a non-positive page as absent and auto-paginates", async () => {
    const requested: string[] = [];

    server.use(
      http.get(`${BASE_URL}/projects.json`, ({ request }) => {
        const url = new URL(request.url);
        requested.push(url.searchParams.get("page") ?? "(none)");
        const page = Number(url.searchParams.get("page") ?? "1");
        const headers: Record<string, string> = {};
        if (page < 2) {
          headers.Link = `<${BASE_URL}/projects.json?page=2>; rel="next"`;
        }
        return HttpResponse.json([{ id: page * 10 }], { headers });
      })
    );

    const client = createBasecampClient({ accountId: "12345", accessToken: "t" });
    const projects = new ProjectsService(client);

    const result = await projects.list({ page: 0 });

    expect(requested).toEqual(["0", "2"]);
    expect(result.length).toBe(2);
    expect(result.meta.truncated).toBe(false);
  });
});
