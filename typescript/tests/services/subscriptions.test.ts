/**
 * Tests for the SubscriptionsService class
 */
import { describe, it, expect, beforeEach } from "vitest";
import { http, HttpResponse } from "msw";
import { server } from "../setup.js";
import { createBasecampClient, type BasecampClient } from "../../src/client.js";
import { BasecampError } from "../../src/errors.js";

const BASE_URL = "https://3.basecampapi.com/12345";

describe("SubscriptionsService", () => {
  let client: BasecampClient;

  beforeEach(() => {
    client = createBasecampClient({
      accountId: "12345",
      accessToken: "test-token",
    });
  });

  describe("get", () => {
    it("should get subscription information for a recording", async () => {
      const mockSubscription = {
        subscribed: true,
        count: 3,
        url: "https://3.basecampapi.com/12345/buckets/1/recordings/100/subscription.json",
        subscribers: [
          { id: 1, name: "User One", email_address: "one@example.com" },
          { id: 2, name: "User Two", email_address: "two@example.com" },
          { id: 3, name: "User Three", email_address: "three@example.com" },
        ],
      };

      server.use(
        http.get(`${BASE_URL}/buckets/1/recordings/100/subscription.json`, () => {
          return HttpResponse.json(mockSubscription);
        })
      );

      const subscription = await client.subscriptions.get(1, 100);

      expect(subscription.subscribed).toBe(true);
      expect(subscription.count).toBe(3);
      expect(subscription.subscribers).toHaveLength(3);
      expect(subscription.subscribers[0].name).toBe("User One");
    });
  });

  describe("subscribe", () => {
    it("should subscribe the current user to a recording", async () => {
      const mockSubscription = {
        subscribed: true,
        count: 4,
        url: "https://3.basecampapi.com/12345/buckets/1/recordings/100/subscription.json",
        subscribers: [
          { id: 1, name: "User One" },
          { id: 2, name: "User Two" },
          { id: 3, name: "User Three" },
          { id: 999, name: "Current User" },
        ],
      };

      server.use(
        http.post(`${BASE_URL}/buckets/1/recordings/100/subscription.json`, () => {
          return HttpResponse.json(mockSubscription);
        })
      );

      const subscription = await client.subscriptions.subscribe(1, 100);

      expect(subscription.subscribed).toBe(true);
      expect(subscription.count).toBe(4);
    });
  });

  describe("unsubscribe", () => {
    it("should unsubscribe the current user from a recording", async () => {
      server.use(
        http.delete(`${BASE_URL}/buckets/1/recordings/100/subscription.json`, () => {
          return new HttpResponse(null, { status: 204 });
        })
      );

      // Should not throw
      await client.subscriptions.unsubscribe(1, 100);
    });
  });

  describe("update", () => {
    it("should batch update subscriptions", async () => {
      const mockSubscription = {
        subscribed: true,
        count: 4,
        url: "https://3.basecampapi.com/12345/buckets/1/recordings/100/subscription.json",
        subscribers: [
          { id: 1, name: "User One" },
          { id: 2, name: "User Two" },
          { id: 4, name: "User Four" },
          { id: 5, name: "User Five" },
        ],
      };

      server.use(
        http.put(`${BASE_URL}/buckets/1/recordings/100/subscription.json`, async ({ request }) => {
          const body = await request.json() as { subscriptions: number[]; unsubscriptions: number[] };
          expect(body.subscriptions).toEqual([4, 5]);
          expect(body.unsubscriptions).toEqual([3]);
          return HttpResponse.json(mockSubscription);
        })
      );

      const subscription = await client.subscriptions.update(1, 100, {
        subscriptions: [4, 5],
        unsubscriptions: [3],
      });

      expect(subscription.count).toBe(4);
      expect(subscription.subscribers.map(s => s.id)).toContain(4);
      expect(subscription.subscribers.map(s => s.id)).toContain(5);
      expect(subscription.subscribers.map(s => s.id)).not.toContain(3);
    });

    it("should work with only subscriptions", async () => {
      const mockSubscription = {
        subscribed: true,
        count: 5,
        url: "https://3.basecampapi.com/12345/buckets/1/recordings/100/subscription.json",
        subscribers: [],
      };

      server.use(
        http.put(`${BASE_URL}/buckets/1/recordings/100/subscription.json`, async ({ request }) => {
          const body = await request.json() as { subscriptions: number[] };
          expect(body.subscriptions).toEqual([6, 7]);
          return HttpResponse.json(mockSubscription);
        })
      );

      const subscription = await client.subscriptions.update(1, 100, {
        subscriptions: [6, 7],
      });

      expect(subscription.count).toBe(5);
    });

    it("should work with only unsubscriptions", async () => {
      const mockSubscription = {
        subscribed: true,
        count: 2,
        url: "https://3.basecampapi.com/12345/buckets/1/recordings/100/subscription.json",
        subscribers: [],
      };

      server.use(
        http.put(`${BASE_URL}/buckets/1/recordings/100/subscription.json`, async ({ request }) => {
          const body = await request.json() as { unsubscriptions: number[] };
          expect(body.unsubscriptions).toEqual([1, 2]);
          return HttpResponse.json(mockSubscription);
        })
      );

      const subscription = await client.subscriptions.update(1, 100, {
        unsubscriptions: [1, 2],
      });

      expect(subscription.count).toBe(2);
    });

    it("should throw validation error when neither subscriptions nor unsubscriptions provided", async () => {
      try {
        await client.subscriptions.update(1, 100, {});
        expect.fail("Should have thrown");
      } catch (err) {
        expect(err).toBeInstanceOf(BasecampError);
        expect((err as BasecampError).code).toBe("validation");
      }
    });

    it("should throw validation error when both are empty arrays", async () => {
      try {
        await client.subscriptions.update(1, 100, {
          subscriptions: [],
          unsubscriptions: [],
        });
        expect.fail("Should have thrown");
      } catch (err) {
        expect(err).toBeInstanceOf(BasecampError);
        expect((err as BasecampError).code).toBe("validation");
      }
    });
  });
});
