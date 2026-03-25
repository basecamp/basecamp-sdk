/**
 * Tests for MyNotificationsService — verifies system actor normalization.
 */
import { describe, it, expect, beforeEach } from "vitest";
import { http, HttpResponse } from "msw";
import { server } from "../setup.js";
import { createBasecampClient } from "../../src/client.js";
import type { BasecampClient } from "../../src/client.js";

const BASE_URL = "https://3.basecampapi.com/12345";

describe("MyNotificationsService", () => {
  let client: BasecampClient;

  beforeEach(() => {
    client = createBasecampClient({
      accountId: "12345",
      accessToken: "test-token",
      enableRetry: false,
    });
  });

  describe("system actor normalization", () => {
    it("should normalize LocalPerson creator.id to number and preserve system_label", async () => {
      server.use(
        http.get(`${BASE_URL}/my/readings.json`, () => {
          return HttpResponse.json({
            unreads: [
              {
                id: 42,
                title: "System notification",
                created_at: "2024-01-01T00:00:00Z",
                updated_at: "2024-01-01T00:00:00Z",
                creator: {
                  id: "basecamp",
                  name: "Basecamp",
                  personable_type: "LocalPerson",
                },
              },
            ],
            reads: [],
            memories: [],
          });
        })
      );

      const result = await client.myNotifications.myNotifications();
      const creator = (result as Record<string, unknown[]>).unreads[0] as Record<string, unknown>;
      const creatorObj = creator.creator as Record<string, unknown>;

      expect(creatorObj.id).toBe(0);
      expect(typeof creatorObj.id).toBe("number");
      expect(creatorObj.system_label).toBe("basecamp");
      expect(creatorObj.personable_type).toBe("LocalPerson");
    });

    it("should leave numeric string creator.id as number", async () => {
      server.use(
        http.get(`${BASE_URL}/my/readings.json`, () => {
          return HttpResponse.json({
            unreads: [
              {
                id: 42,
                title: "Normal notification",
                created_at: "2024-01-01T00:00:00Z",
                updated_at: "2024-01-01T00:00:00Z",
                creator: {
                  id: "99999",
                  name: "Real Person",
                  personable_type: "User",
                },
              },
            ],
            reads: [],
            memories: [],
          });
        })
      );

      const result = await client.myNotifications.myNotifications();
      const creator = (result as Record<string, unknown[]>).unreads[0] as Record<string, unknown>;
      const creatorObj = creator.creator as Record<string, unknown>;

      expect(creatorObj.id).toBe(99999);
      expect(typeof creatorObj.id).toBe("number");
      expect(creatorObj.system_label).toBeUndefined();
    });

    it("should treat junk string as sentinel", async () => {
      server.use(
        http.get(`${BASE_URL}/my/readings.json`, () => {
          return HttpResponse.json({
            unreads: [
              {
                id: 42,
                title: "Junk notification",
                created_at: "2024-01-01T00:00:00Z",
                updated_at: "2024-01-01T00:00:00Z",
                creator: {
                  id: "123abc",
                  name: "Unknown",
                  personable_type: "LocalPerson",
                },
              },
            ],
            reads: [],
            memories: [],
          });
        })
      );

      const result = await client.myNotifications.myNotifications();
      const creator = (result as Record<string, unknown[]>).unreads[0] as Record<string, unknown>;
      const creatorObj = creator.creator as Record<string, unknown>;

      // "123abc" is not a valid ID — treated as sentinel
      expect(creatorObj.id).toBe(0);
      expect(creatorObj.system_label).toBe("123abc");
    });

    it("should treat overflow numeric string as sentinel (JS cannot represent losslessly)", async () => {
      server.use(
        http.get(`${BASE_URL}/my/readings.json`, () => {
          return HttpResponse.json({
            unreads: [
              {
                id: 42,
                title: "Overflow notification",
                created_at: "2024-01-01T00:00:00Z",
                updated_at: "2024-01-01T00:00:00Z",
                creator: {
                  id: "9223372036854775808",
                  name: "Overflow",
                  personable_type: "LocalPerson",
                },
              },
            ],
            reads: [],
            memories: [],
          });
        })
      );

      const result = await client.myNotifications.myNotifications();
      const creator = (result as Record<string, unknown[]>).unreads[0] as Record<string, unknown>;
      const creatorObj = creator.creator as Record<string, unknown>;

      // Overflow can't be represented as a safe integer — preserved as label
      expect(creatorObj.id).toBe(0);
      expect(creatorObj.system_label).toBe("9223372036854775808");
    });
  });
});
