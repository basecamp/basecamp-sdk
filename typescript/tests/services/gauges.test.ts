/**
 * Tests for the GaugesService (generated from OpenAPI spec)
 *
 * All seven wire operations are covered with a happy path and an error case.
 * Two of them carry client-side guards that short-circuit before any HTTP call
 * (createGaugeNeedle's missing gauge_needle, toggleGauge's missing gauge); both
 * are asserted to make no request at all.
 *
 * Paths are pinned exactly as generated. GET/PUT/DELETE /gauge_needles/{id}
 * carry NO `.json` suffix — that is deliberate in the Smithy spec, so the stubs
 * below register the unsuffixed path and a suffixed request would fail via
 * MSW's onUnhandledRequest: "error".
 */
import { describe, it, expect, beforeEach } from "vitest";
import { http, HttpResponse } from "msw";
import { server } from "../setup.js";
import { createBasecampClient } from "../../src/client.js";
import { BasecampError } from "../../src/errors.js";
import { ListResult } from "../../src/pagination.js";
import type { ListMeta } from "../../src/pagination.js";
import type { BasecampClient } from "../../src/client.js";
import type {
  CreateGaugeNeedleGaugeRequest,
  ToggleGaugeGaugeRequest,
} from "../../src/generated/services/gauges.js";
import gaugeFixture from "../../../spec/fixtures/gauges/get.json";
import needleFixture from "../../../spec/fixtures/gauges/needle_get.json";

const BASE_URL = "https://3.basecampapi.com/12345";

// Sourced from the shared, coverage-guarded fixtures (spec/fixtures/manifest.yaml)
// so these stubs cannot drift from the validated Gauge / Gauge::Needle shapes;
// `id` is overridable per call.
const sampleGauge = (id = gaugeFixture.id) => ({ ...gaugeFixture, id });
const sampleNeedle = (id = needleFixture.id) => ({ ...needleFixture, id });

const rejection = async (promise: Promise<unknown>): Promise<unknown> =>
  promise.then(
    () => {
      throw new Error("expected the call to reject, but it resolved");
    },
    (error: unknown) => error
  );

const asBasecampError = (error: unknown): BasecampError => {
  expect(error).toBeInstanceOf(BasecampError);
  return error as BasecampError;
};

// Both gauge list methods return a ListResult at runtime — requestPaginated
// builds one — but their generated signatures declare the bare
// `List*ResponseContent` ARRAY, so `.meta` is invisible to the compiler. Nearly
// every other generated list method declares `Promise<ListResult<T>>`;
// ListGauges, ListGaugeNeedles, Search and Checkins#reminders are the four that
// don't. The repo's `tsc --noEmit` excludes tests, so this reads clean today
// either way; the helper asserts the runtime class first, so the pagination
// contract below is pinned against what the object IS, not what the (currently
// under-specified) return type claims. Reported, not fixed — the fix belongs in
// the service generator, not in a test.
const metaOf = (list: unknown): ListMeta => {
  expect(list).toBeInstanceOf(ListResult);
  return (list as ListResult<unknown>).meta;
};

describe("GaugesService", () => {
  let client: BasecampClient;

  beforeEach(() => {
    client = createBasecampClient({
      accountId: "12345",
      accessToken: "test-token",
      enableRetry: false,
    });
  });

  describe("listGauges", () => {
    it("should list gauges across projects with their needle readings", async () => {
      server.use(
        http.get(`${BASE_URL}/reports/gauges.json`, () =>
          HttpResponse.json([sampleGauge(), sampleGauge(2)], {
            headers: { "X-Total-Count": "2" },
          })
        )
      );

      const gauges = await client.gauges.listGauges();

      expect(gauges).toHaveLength(2);
      const gauge = gauges[0]!;
      expect(gauge.id).toBe(gaugeFixture.id);
      expect(gauge.type).toBe("Gauge");
      expect(gauge.enabled).toBe(true);
      // The reading fields are the whole point of the gauge report: the current
      // needle's color and position, plus the previous position for the trend.
      expect(gauge.last_needle_color).toBe("green");
      expect(gauge.last_needle_position).toBe(72);
      expect(gauge.previous_needle_position).toBe(45);
      // Cross-project report, so each row carries its own bucket.
      expect(gauge.bucket?.id).toBe(gaugeFixture.bucket.id);
      expect(gauge.bucket?.type).toBe("Project");
      expect(gauges[1]!.id).toBe(2);
      expect(metaOf(gauges).totalCount).toBe(2);
      expect(metaOf(gauges).truncated).toBe(false);
    });

    // The filter is spelled `bucket_ids` on the wire (snake_case), not
    // `bucketIds` — the camelCase name exists only on the TS options object.
    // Asserting the whole key set, not just the value, so a renamed key cannot
    // pass by leaving the expected one absent-and-unread.
    it("sends the project filter as the bucket_ids query parameter", async () => {
      let keys: string[] = [];
      let value: string | null = null;

      server.use(
        http.get(`${BASE_URL}/reports/gauges.json`, ({ request }) => {
          const url = new URL(request.url);
          keys = [...url.searchParams.keys()];
          value = url.searchParams.get("bucket_ids");
          return HttpResponse.json([sampleGauge()]);
        })
      );

      const gauges = await client.gauges.listGauges({ bucketIds: "2085958500,2085958501" });

      expect(keys).toEqual(["bucket_ids"]);
      expect(value).toBe("2085958500,2085958501");
      expect(gauges).toHaveLength(1);
    });

    // SPEC section 8: a positive `page` selects exactly that page in exactly one
    // request, and reports truncation from the rel="next" link it deliberately
    // did not follow. This mirrors go/pkg/basecamp/gauges_test.go.
    it("selects exactly one page and reports the unfollowed next link", async () => {
      const requested: string[] = [];

      server.use(
        http.get(`${BASE_URL}/reports/gauges.json`, ({ request }) => {
          const url = new URL(request.url);
          requested.push(url.searchParams.get("page") ?? "(none)");
          return HttpResponse.json([sampleGauge(30), sampleGauge(31)], {
            headers: {
              "X-Total-Count": "9",
              Link: `<${BASE_URL}/reports/gauges.json?page=4>; rel="next"`,
            },
          });
        })
      );

      const gauges = await client.gauges.listGauges({ page: 3 });

      expect(requested).toEqual(["3"]);
      expect(gauges).toHaveLength(2);
      expect(metaOf(gauges).totalCount).toBe(9);
      expect(metaOf(gauges).truncated).toBe(true);
    });

    it("follows Link headers across pages when no page is pinned", async () => {
      const requested: string[] = [];

      server.use(
        http.get(`${BASE_URL}/reports/gauges.json`, ({ request }) => {
          const url = new URL(request.url);
          const page = url.searchParams.get("page");
          requested.push(page ?? "(none)");
          if (page === "2") {
            return HttpResponse.json([sampleGauge(3)]);
          }
          return HttpResponse.json([sampleGauge(1), sampleGauge(2)], {
            headers: {
              "X-Total-Count": "3",
              Link: `<${BASE_URL}/reports/gauges.json?page=2>; rel="next"`,
            },
          });
        })
      );

      const gauges = await client.gauges.listGauges();

      expect(requested).toEqual(["(none)", "2"]);
      expect(gauges.map((g) => g.id)).toEqual([1, 2, 3]);
      expect(metaOf(gauges).totalCount).toBe(3);
      expect(metaOf(gauges).truncated).toBe(false);
    });

    // ListGauges lists ForbiddenError/UnauthorizedError/RateLimitError/
    // InternalServerError; 401 is the one every caller hits first, when the
    // token expires.
    it("maps a 401 to auth_required with the server's message", async () => {
      server.use(
        http.get(`${BASE_URL}/reports/gauges.json`, () =>
          HttpResponse.json({ error: "Your access token expired" }, { status: 401 })
        )
      );

      const error = asBasecampError(await rejection(client.gauges.listGauges()));
      expect(error.code).toBe("auth_required");
      expect(error.httpStatus).toBe(401);
      expect(error.message).toBe("Your access token expired");
      expect(error.retryable).toBe(false);
      expect(error.exitCode).toBe(3);
    });
  });

  describe("listGaugeNeedles", () => {
    it("should list a project's needles newest first", async () => {
      const projectId = gaugeFixture.bucket.id;

      server.use(
        http.get(`${BASE_URL}/projects/${projectId}/gauge/needles.json`, () =>
          HttpResponse.json([sampleNeedle(), sampleNeedle(2)], {
            headers: { "X-Total-Count": "2" },
          })
        )
      );

      const needles = await client.gauges.listGaugeNeedles(projectId);

      expect(needles).toHaveLength(2);
      const needle = needles[0]!;
      expect(needle.id).toBe(needleFixture.id);
      expect(needle.type).toBe("Gauge::Needle");
      expect(needle.color).toBe("green");
      expect(needle.position).toBe(72);
      // A needle hangs off its gauge, so the recording parent is the Gauge.
      expect(needle.parent?.id).toBe(needleFixture.parent.id);
      expect(metaOf(needles).totalCount).toBe(2);
      expect(metaOf(needles).truncated).toBe(false);
    });

    it("selects exactly one page and reports the unfollowed next link", async () => {
      const projectId = 7;
      const requested: string[] = [];

      server.use(
        http.get(`${BASE_URL}/projects/${projectId}/gauge/needles.json`, ({ request }) => {
          const url = new URL(request.url);
          requested.push(url.searchParams.get("page") ?? "(none)");
          return HttpResponse.json([sampleNeedle(20)], {
            headers: {
              "X-Total-Count": "5",
              Link: `<${BASE_URL}/projects/${projectId}/gauge/needles.json?page=3>; rel="next"`,
            },
          });
        })
      );

      const needles = await client.gauges.listGaugeNeedles(projectId, { page: 2 });

      expect(requested).toEqual(["2"]);
      expect(needles).toHaveLength(1);
      expect(metaOf(needles).totalCount).toBe(5);
      expect(metaOf(needles).truncated).toBe(true);
    });

    it("follows Link headers across pages when no page is pinned", async () => {
      const projectId = 7;
      const requested: string[] = [];

      server.use(
        http.get(`${BASE_URL}/projects/${projectId}/gauge/needles.json`, ({ request }) => {
          const url = new URL(request.url);
          const page = url.searchParams.get("page");
          requested.push(page ?? "(none)");
          if (page === "2") {
            return HttpResponse.json([sampleNeedle(3)]);
          }
          return HttpResponse.json([sampleNeedle(1), sampleNeedle(2)], {
            headers: {
              "X-Total-Count": "3",
              Link: `<${BASE_URL}/projects/${projectId}/gauge/needles.json?page=2>; rel="next"`,
            },
          });
        })
      );

      const needles = await client.gauges.listGaugeNeedles(projectId);

      expect(requested).toEqual(["(none)", "2"]);
      expect(needles.map((n) => n.id)).toEqual([1, 2, 3]);
      expect(metaOf(needles).truncated).toBe(false);
    });

    it("maps a 404 on an unknown project to not_found", async () => {
      server.use(
        http.get(`${BASE_URL}/projects/999/gauge/needles.json`, () =>
          HttpResponse.json({ error: "Project not found" }, { status: 404 })
        )
      );

      const error = asBasecampError(await rejection(client.gauges.listGaugeNeedles(999)));
      expect(error.code).toBe("not_found");
      expect(error.httpStatus).toBe(404);
      expect(error.message).toBe("Project not found");
      expect(error.retryable).toBe(false);
      expect(error.exitCode).toBe(2);
    });
  });

  describe("gaugeNeedle", () => {
    // The route is unsuffixed: GET /gauge_needles/{needleId}, no `.json`.
    it("should get a needle by id from the unsuffixed path", async () => {
      const needleId = needleFixture.id;
      let pathname = "";

      server.use(
        http.get(`${BASE_URL}/gauge_needles/${needleId}`, ({ request }) => {
          pathname = new URL(request.url).pathname;
          return HttpResponse.json(sampleNeedle(needleId));
        })
      );

      const needle = await client.gauges.gaugeNeedle(needleId);

      expect(pathname).toBe(`/12345/gauge_needles/${needleId}`);
      expect(pathname.endsWith(".json")).toBe(false);
      expect(needle.id).toBe(needleId);
      expect(needle.type).toBe("Gauge::Needle");
      expect(needle.color).toBe("green");
      expect(needle.position).toBe(72);
      expect(needle.parent?.id).toBe(needleFixture.parent.id);
    });

    // Rich-text attachments ride along on the needle's description. `width` is
    // float-spelled (`1024.0`) on the image and `null` on the PDF. JavaScript
    // has a single numeric type, so the float spelling is unobservable here —
    // `1024.0` and `1024` are the same value, and no assertion can separate
    // them. What this pins is the half that IS observable: the array survives,
    // and `null` stays null rather than collapsing to 0 or undefined (SPEC
    // section 10 Type Fidelity). Python and Ruby carry the type assertion.
    it("decodes float-spelled and null attachment dimensions", async () => {
      const needleId = needleFixture.id;

      server.use(
        http.get(`${BASE_URL}/gauge_needles/${needleId}`, () =>
          HttpResponse.json(sampleNeedle(needleId))
        )
      );

      const needle = await client.gauges.gaugeNeedle(needleId);

      expect(needle.description_attachments).toHaveLength(2);
      const [image, pdf] = needle.description_attachments;
      expect(image!.filename).toBe("burndown.png");
      expect(image!.width).toBe(1024);
      expect(image!.height).toBe(768);
      expect(pdf!.filename).toBe("retro-notes.pdf");
      expect(pdf!.width).toBeNull();
      expect(pdf!.height).toBeNull();
    });

    it("maps a 404 on an unknown needle to not_found", async () => {
      server.use(
        http.get(`${BASE_URL}/gauge_needles/999`, () =>
          HttpResponse.json({ error: "Gauge needle not found" }, { status: 404 })
        )
      );

      const error = asBasecampError(await rejection(client.gauges.gaugeNeedle(999)));
      expect(error.code).toBe("not_found");
      expect(error.httpStatus).toBe(404);
      expect(error.message).toBe("Gauge needle not found");
      expect(error.retryable).toBe(false);
    });
  });

  describe("createGaugeNeedle", () => {
    it("should post a needle under the gauge_needle key and return it", async () => {
      const projectId = 7;
      let body: Record<string, unknown> = {};

      server.use(
        http.post(`${BASE_URL}/projects/${projectId}/gauge/needles.json`, async ({ request }) => {
          body = (await request.json()) as Record<string, unknown>;
          return HttpResponse.json(sampleNeedle(), { status: 201 });
        })
      );

      const needle = await client.gauges.createGaugeNeedle(projectId, {
        gaugeNeedle: { position: 72, color: "green", description: "<div>Shipped it</div>" },
        notify: "custom",
        subscriptions: [1049715915],
      });

      expect(body).toEqual({
        gauge_needle: { position: 72, color: "green", description: "<div>Shipped it</div>" },
        notify: "custom",
        subscriptions: [1049715915],
      });
      expect(needle.id).toBe(needleFixture.id);
      expect(needle.type).toBe("Gauge::Needle");
      expect(needle.color).toBe("green");
      expect(needle.position).toBe(72);
      expect(needle.parent?.id).toBe(needleFixture.parent.id);
    });

    it("omits notify and subscriptions when not provided", async () => {
      const projectId = 7;
      let body: Record<string, unknown> = {};

      server.use(
        http.post(`${BASE_URL}/projects/${projectId}/gauge/needles.json`, async ({ request }) => {
          body = (await request.json()) as Record<string, unknown>;
          return HttpResponse.json(sampleNeedle(), { status: 201 });
        })
      );

      await client.gauges.createGaugeNeedle(projectId, { gaugeNeedle: { position: 10 } });

      expect(Object.keys(body)).toEqual(["gauge_needle"]);
      expect(body.gauge_needle).toEqual({ position: 10 });
    });

    // Client-side guard: no HTTP call at all. The counting handler is registered
    // so a leaked request would be SERVED (and the promise would resolve),
    // rather than merely failing on onUnhandledRequest — that keeps the
    // assertion about the guard, not about MSW.
    it("rejects a missing gauge needle without making a request", async () => {
      const projectId = 7;
      let requests = 0;

      server.use(
        http.post(`${BASE_URL}/projects/${projectId}/gauge/needles.json`, () => {
          requests++;
          return HttpResponse.json(sampleNeedle(), { status: 201 });
        })
      );

      const error = asBasecampError(
        await rejection(
          client.gauges.createGaugeNeedle(projectId, {} as CreateGaugeNeedleGaugeRequest)
        )
      );
      expect(error.code).toBe("validation");
      expect(error.message).toBe("Gauge needle is required");
      expect(error.httpStatus).toBe(400);
      expect(error.retryable).toBe(false);
      expect(requests).toBe(0);
    });

    // The server-side half: BC3 validates the 0-100 position and renders the
    // Rails RecordInvalid map, which the SDK folds into `fieldErrors` and
    // flattens into the message.
    it("maps a 422 to validation with the field errors intact", async () => {
      const projectId = 7;

      server.use(
        http.post(`${BASE_URL}/projects/${projectId}/gauge/needles.json`, () =>
          HttpResponse.json(
            { errors: { position: ["is not included in the list"] } },
            { status: 422 }
          )
        )
      );

      const error = asBasecampError(
        await rejection(
          client.gauges.createGaugeNeedle(projectId, { gaugeNeedle: { position: 500 } })
        )
      );
      expect(error.code).toBe("validation");
      expect(error.httpStatus).toBe(422);
      expect(error.message).toBe("position: is not included in the list");
      expect(error.fieldErrors).toEqual({ position: ["is not included in the list"] });
      expect(error.retryable).toBe(false);
      expect(error.exitCode).toBe(9);
    });
  });

  describe("updateGaugeNeedle", () => {
    // Only the description is writable — position and color are immutable once
    // the needle is recorded — and it goes under the same gauge_needle key.
    it("should put the description under the gauge_needle key", async () => {
      const needleId = needleFixture.id;
      let body: Record<string, unknown> = {};
      let pathname = "";

      server.use(
        http.put(`${BASE_URL}/gauge_needles/${needleId}`, async ({ request }) => {
          pathname = new URL(request.url).pathname;
          body = (await request.json()) as Record<string, unknown>;
          return HttpResponse.json({ ...sampleNeedle(needleId), description: "<div>Revised</div>" });
        })
      );

      const needle = await client.gauges.updateGaugeNeedle(needleId, {
        gaugeNeedle: { description: "<div>Revised</div>" },
      });

      expect(pathname).toBe(`/12345/gauge_needles/${needleId}`);
      expect(body).toEqual({ gauge_needle: { description: "<div>Revised</div>" } });
      expect(needle.id).toBe(needleId);
      expect(needle.description).toBe("<div>Revised</div>");
      // Immutable fields come back unchanged.
      expect(needle.color).toBe("green");
      expect(needle.position).toBe(72);
    });

    it("maps a 404 on an unknown needle to not_found", async () => {
      server.use(
        http.put(`${BASE_URL}/gauge_needles/999`, () =>
          HttpResponse.json({ error: "Gauge needle not found" }, { status: 404 })
        )
      );

      const error = asBasecampError(
        await rejection(
          client.gauges.updateGaugeNeedle(999, { gaugeNeedle: { description: "<div>x</div>" } })
        )
      );
      expect(error.code).toBe("not_found");
      expect(error.httpStatus).toBe(404);
      expect(error.message).toBe("Gauge needle not found");
      expect(error.retryable).toBe(false);
    });
  });

  describe("destroyGaugeNeedle", () => {
    it("should delete a needle and resolve void on 204", async () => {
      const needleId = needleFixture.id;
      let pathname = "";

      server.use(
        http.delete(`${BASE_URL}/gauge_needles/${needleId}`, ({ request }) => {
          pathname = new URL(request.url).pathname;
          return new HttpResponse(null, { status: 204 });
        })
      );

      await expect(client.gauges.destroyGaugeNeedle(needleId)).resolves.toBeUndefined();
      expect(pathname).toBe(`/12345/gauge_needles/${needleId}`);
    });

    it("maps a 404 on an unknown needle to not_found", async () => {
      server.use(
        http.delete(`${BASE_URL}/gauge_needles/999`, () =>
          HttpResponse.json({ error: "Gauge needle not found" }, { status: 404 })
        )
      );

      const error = asBasecampError(await rejection(client.gauges.destroyGaugeNeedle(999)));
      expect(error.code).toBe("not_found");
      expect(error.httpStatus).toBe(404);
      expect(error.message).toBe("Gauge needle not found");
      expect(error.retryable).toBe(false);
    });
  });

  // bc3's Projects::GaugesController#update answers `head :ok` — a 200 with an
  // EMPTY body, not a 204. These stubs say 200 deliberately: an empty 200 is
  // where a void decode can trip over zero-length input, and a 204 (defined to
  // carry no body) would never exercise that path. destroyGaugeNeedle above
  // really is a 204 — bc3 answers that one with `head :no_content`.
  describe("toggleGauge", () => {
    it("should enable the gauge for a project", async () => {
      const projectId = 7;
      let body: Record<string, unknown> = {};

      server.use(
        http.put(`${BASE_URL}/projects/${projectId}/gauge.json`, async ({ request }) => {
          body = (await request.json()) as Record<string, unknown>;
          return new HttpResponse(null, { status: 200 });
        })
      );

      await expect(
        client.gauges.toggleGauge(projectId, { gauge: { enabled: true } })
      ).resolves.toBeUndefined();
      expect(body).toEqual({ gauge: { enabled: true } });
    });

    // `enabled: false` is a real value, not an omission: it must reach the wire.
    it("should disable the gauge for a project", async () => {
      const projectId = 7;
      let body: Record<string, unknown> = {};

      server.use(
        http.put(`${BASE_URL}/projects/${projectId}/gauge.json`, async ({ request }) => {
          body = (await request.json()) as Record<string, unknown>;
          return new HttpResponse(null, { status: 200 });
        })
      );

      await client.gauges.toggleGauge(projectId, { gauge: { enabled: false } });

      expect(body).toEqual({ gauge: { enabled: false } });
      expect((body.gauge as Record<string, unknown>).enabled).toBe(false);
    });

    // Client-side guard, same shape as createGaugeNeedle's: a served handler is
    // registered so a leaked request would resolve rather than error out.
    it("rejects a missing gauge without making a request", async () => {
      const projectId = 7;
      let requests = 0;

      server.use(
        http.put(`${BASE_URL}/projects/${projectId}/gauge.json`, () => {
          requests++;
          return new HttpResponse(null, { status: 200 });
        })
      );

      const error = asBasecampError(
        await rejection(client.gauges.toggleGauge(projectId, {} as ToggleGaugeGaugeRequest))
      );
      expect(error.code).toBe("validation");
      expect(error.message).toBe("Gauge is required");
      expect(error.httpStatus).toBe(400);
      expect(error.retryable).toBe(false);
      expect(requests).toBe(0);
    });

    // Only project admins may toggle a gauge, so 403 is this operation's
    // characteristic failure.
    it("maps a 403 to forbidden for a non-admin", async () => {
      const projectId = 7;

      server.use(
        http.put(`${BASE_URL}/projects/${projectId}/gauge.json`, () =>
          HttpResponse.json({ error: "Only project admins can toggle gauges" }, { status: 403 })
        )
      );

      const error = asBasecampError(
        await rejection(client.gauges.toggleGauge(projectId, { gauge: { enabled: true } }))
      );
      expect(error.code).toBe("forbidden");
      expect(error.httpStatus).toBe(403);
      expect(error.message).toBe("Only project admins can toggle gauges");
      expect(error.retryable).toBe(false);
      expect(error.exitCode).toBe(4);
    });
  });
});
