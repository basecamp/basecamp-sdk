/**
 * Tests for the PeopleService (generated from OpenAPI spec)
 */
import { describe, it, expect, beforeEach } from "vitest";
import { http, HttpResponse } from "msw";
import { server } from "../setup.js";
import { createBasecampClient } from "../../src/client.js";
import { BasecampError } from "../../src/errors.js";
import type { BasecampClient } from "../../src/client.js";

const BASE_URL = "https://3.basecampapi.com/12345";

const samplePerson = (id = 1) => ({
  id,
  name: "Jane Doe",
  email_address: "jane@example.com",
  admin: false,
  created_at: "2024-01-15T10:00:00Z",
  updated_at: "2024-01-15T10:00:00Z",
});

describe("PeopleService", () => {
  let client: BasecampClient;

  beforeEach(() => {
    client = createBasecampClient({
      accountId: "12345",
      accessToken: "test-token",
      enableRetry: false,
    });
  });

  describe("list", () => {
    it("should list all people", async () => {
      server.use(
        http.get(`${BASE_URL}/people.json`, () => {
          return HttpResponse.json([samplePerson(1), samplePerson(2)]);
        })
      );

      const people = await client.people.list();
      expect(people).toHaveLength(2);
      expect(people[0]!.id).toBe(1);
      expect(people[1]!.id).toBe(2);
    });

    it("should return empty array when no people exist", async () => {
      server.use(
        http.get(`${BASE_URL}/people.json`, () => {
          return HttpResponse.json([]);
        })
      );

      const people = await client.people.list();
      expect(people).toHaveLength(0);
    });
  });

  describe("get", () => {
    it("should return a single person", async () => {
      const personId = 42;

      server.use(
        http.get(`${BASE_URL}/people/${personId}`, () => {
          return HttpResponse.json(samplePerson(personId));
        })
      );

      const person = await client.people.get(personId);
      expect(person.id).toBe(personId);
      expect(person.name).toBe("Jane Doe");
    });

    it("should throw not_found for missing person", async () => {
      server.use(
        http.get(`${BASE_URL}/people/999`, () => {
          return HttpResponse.json({ error: "Not found" }, { status: 404 });
        })
      );

      await expect(client.people.get(999)).rejects.toThrow(BasecampError);
    });
  });

  describe("me", () => {
    it("should return the current user profile", async () => {
      server.use(
        http.get(`${BASE_URL}/my/profile.json`, () => {
          return HttpResponse.json(samplePerson(100));
        })
      );

      const me = await client.people.me();
      expect(me.id).toBe(100);
      expect(me.name).toBe("Jane Doe");
    });
  });

  describe("listPingable", () => {
    it("should list pingable people", async () => {
      server.use(
        http.get(`${BASE_URL}/circles/people.json`, () => {
          return HttpResponse.json([samplePerson(1), samplePerson(2)]);
        })
      );

      const people = await client.people.listPingable();
      expect(people).toHaveLength(2);
      expect(people[0]!.id).toBe(1);
    });
  });

  describe("listForProject", () => {
    it("should list people for a project", async () => {
      const projectId = 100;

      server.use(
        http.get(`${BASE_URL}/projects/${projectId}/people.json`, () => {
          return HttpResponse.json([samplePerson(1), samplePerson(2)]);
        })
      );

      const people = await client.people.listForProject(projectId);
      expect(people).toHaveLength(2);
      expect(people[0]!.id).toBe(1);
    });
  });

  describe("listAssignable", () => {
    it("should list assignable people", async () => {
      server.use(
        http.get(`${BASE_URL}/reports/todos/assigned.json`, () => {
          return HttpResponse.json([samplePerson(1), samplePerson(2)]);
        })
      );

      const people = await client.people.listAssignable();
      expect(people).toHaveLength(2);
      expect(people[0]!.id).toBe(1);
    });
  });
});
