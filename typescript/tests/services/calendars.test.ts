import { describe, it, expect, beforeEach } from "vitest";
import { http, HttpResponse } from "msw";
import { server } from "../setup.js";
import { createBasecampClient } from "../../src/client.js";
import { BasecampError } from "../../src/errors.js";
import type { BasecampClient } from "../../src/client.js";

const BASE_URL = "https://3.basecampapi.com/12345";

const sampleCalendar = {
  id: 2085958497,
  type: "Calendar",
  name: "Honcho Design Calendar",
  color: "blue",
  created_at: "2026-05-28T17:22:22.133Z",
  updated_at: "2026-07-20T04:05:52.374Z",
  url: "https://3.basecampapi.com/12345/calendars/2085958497.json",
  app_url: "https://3.basecamp.com/12345/calendars/2085958497",
  schedule_url: "https://3.basecampapi.com/12345/schedules/1069478892.json",
};

describe("CalendarsService", () => {
  let client: BasecampClient;

  beforeEach(() => {
    client = createBasecampClient({
      accountId: "12345",
      accessToken: "test-token",
      enableRetry: false,
    });
  });

  describe("getCalendar", () => {
    it("returns the calendar with its schedule link", async () => {
      server.use(http.get(`${BASE_URL}/calendars/2085958497`, () => HttpResponse.json(sampleCalendar)));

      const calendar = await client.calendars.getCalendar(2085958497);
      expect(calendar.id).toBe(2085958497);
      expect(calendar.color).toBe("blue");
      expect(calendar.schedule_url).toContain("/schedules/");
    });

    it("surfaces 404 as BasecampError", async () => {
      server.use(
        http.get(`${BASE_URL}/calendars/999`, () =>
          HttpResponse.json({ error: "Not found" }, { status: 404 })
        )
      );

      const error = await client.calendars.getCalendar(999).catch((e: unknown) => e);
      expect(error).toBeInstanceOf(BasecampError);
      expect((error as BasecampError).httpStatus).toBe(404);
    });
  });

  describe("updateCalendar", () => {
    it("sends the nested {calendar: {color}} envelope", async () => {
      server.use(
        http.put(`${BASE_URL}/calendars/2085958497`, async ({ request }) => {
          const body = (await request.json()) as Record<string, unknown>;
          expect(body).toEqual({ calendar: { color: "green" } });
          return HttpResponse.json({ ...sampleCalendar, color: "green" });
        })
      );

      const calendar = await client.calendars.updateCalendar(2085958497, {
        calendar: { color: "green" },
      });
      expect(calendar.color).toBe("green");
    });

    it("surfaces the 422 field-keyed errors payload as BasecampError", async () => {
      server.use(
        http.put(`${BASE_URL}/calendars/2085958497`, () =>
          HttpResponse.json({ errors: { color: ["is not a valid color"] } }, { status: 422 })
        )
      );

      const error = await client.calendars
        .updateCalendar(2085958497, { calendar: { color: "chartreuse" } })
        .catch((e: unknown) => e);
      expect(error).toBeInstanceOf(BasecampError);
      expect((error as BasecampError).httpStatus).toBe(422);
    });
  });
});
