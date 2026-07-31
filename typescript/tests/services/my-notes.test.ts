import { describe, it, expect, beforeEach } from "vitest";
import { http, HttpResponse } from "msw";
import { server } from "../setup.js";
import { createBasecampClient } from "../../src/client.js";
import { BasecampError } from "../../src/errors.js";
import type { BasecampClient } from "../../src/client.js";

const BASE_URL = "https://3.basecampapi.com/12345";

const writtenNote = {
  id: 5,
  type: "Notebook::Note",
  created_at: "2026-07-21T00:02:30.308Z",
  updated_at: "2026-07-21T00:02:30.308Z",
  content: '<div dir="auto">Things to remember…</div>',
  content_attachments: [],
  url: "https://3.basecampapi.com/12345/my/notes.json",
  app_url: "https://3.basecamp.com/12345/my/navigation/notes",
};

const unwrittenNote = {
  ...writtenNote,
  id: null,
  created_at: null,
  updated_at: null,
  content: "",
};

describe("MyNotesService", () => {
  let client: BasecampClient;

  beforeEach(() => {
    client = createBasecampClient({
      accountId: "12345",
      accessToken: "test-token",
      enableRetry: false,
    });
  });

  describe("getMyNote", () => {
    it("returns the written note", async () => {
      server.use(http.get(`${BASE_URL}/my/notes.json`, () => HttpResponse.json(writtenNote)));

      const note = await client.myNotes.getMyNote();
      expect(note.id).toBe(5);
      expect(note.type).toBe("Notebook::Note");
    });

    it("returns the pre-first-write shape with null id and timestamps", async () => {
      server.use(http.get(`${BASE_URL}/my/notes.json`, () => HttpResponse.json(unwrittenNote)));

      const note = await client.myNotes.getMyNote();
      expect(note.id).toBeNull();
      expect(note.created_at).toBeNull();
      expect(note.updated_at).toBeNull();
      expect(note.content).toBe("");
    });

    it("surfaces 401 as BasecampError", async () => {
      server.use(
        http.get(`${BASE_URL}/my/notes.json`, () =>
          HttpResponse.json({ error: "Unauthorized" }, { status: 401 })
        )
      );

      const error = await client.myNotes.getMyNote().catch((e: unknown) => e);
      expect(error).toBeInstanceOf(BasecampError);
      expect((error as BasecampError).httpStatus).toBe(401);
    });
  });

  describe("updateMyNote", () => {
    it("sends the nested {note: {content}} envelope and returns the note", async () => {
      server.use(
        http.put(`${BASE_URL}/my/notes.json`, async ({ request }) => {
          const body = (await request.json()) as Record<string, unknown>;
          expect(body).toEqual({ note: { content: "<div>Updated</div>" } });
          return HttpResponse.json({ ...writtenNote, content: "<div>Updated</div>" });
        })
      );

      const note = await client.myNotes.updateMyNote({ note: { content: "<div>Updated</div>" } });
      expect(note.content).toBe("<div>Updated</div>");
    });

    it("surfaces 422 as BasecampError", async () => {
      server.use(
        http.put(`${BASE_URL}/my/notes.json`, () =>
          HttpResponse.json({ error: "Unprocessable" }, { status: 422 })
        )
      );

      const error = await client.myNotes
        .updateMyNote({ note: { content: "x" } })
        .catch((e: unknown) => e);
      expect(error).toBeInstanceOf(BasecampError);
      expect((error as BasecampError).httpStatus).toBe(422);
    });
  });
});
